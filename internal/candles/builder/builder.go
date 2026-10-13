// Package builder turns ticks into candles aligned to the calendar's session anchor (FR-DATA-002,
// FR-CHART-003, FR-DEP-006): forming candles are published as they change; a candle closes on
// the app clock at its boundary + grace, is published as closed and then stored, by one writer
// goroutine, as tick-built (complete = false) until a historical fetch replaces it.
package builder

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/logging"
)

// Event is the payload of candle.closed and candle.forming (§3.4).
type Event struct {
	Token    uint32           `json:"token"`
	Interval candles.Interval `json:"interval"`
	Candle   candles.Candle   `json:"candle"`
	// Partial: tracking started after this candle began, so its open/volume are unknown; it is
	// shown but never stored (the backfill supplies the real candle).
	Partial bool `json:"partial"`
}

var (
	TopicClosed  = bus.NewTopic[Event]("candle.closed")
	TopicForming = bus.NewTopic[Event]("candle.forming")
)

// Options configures a Builder.
type Options struct {
	Clock      clock.Clock
	Calendar   *calendar.Service
	Store      candles.Store // nil: closed candles are only published
	Bus        *bus.Bus
	ExchangeOf func(token uint32) calendar.Exchange // instruments.Master.ExchangeOf
	Grace      time.Duration                        // default 1 s (FR-DEP-006)
	Log        *slog.Logger
}

// writeQueue bounds the closed-candle batches waiting for the store writer; beyond it a batch
// is dropped, logged and counted (the backfill supplies those candles later).
const writeQueue = 64

type forming struct {
	have    bool
	c       candles.Candle
	end     time.Time // exclusive bucket end (cut at the session end)
	partial bool
	base    uint32    // cumulative day volume before this candle
	last    time.Time // timestamp of the newest tick applied (out-of-order ticks keep Close)
}

type series struct {
	trackedAt time.Time
	cur       forming
	closedEnd time.Time // end of the last closed candle (late-tick guard)
}

// tokenState holds a token's series and its cumulative-volume baseline, shared by every
// interval so that an interval tracked later on a streaming token starts exact (Ruling 44).
type tokenState struct {
	trackedAt time.Time     // when the token was first tracked (earliest live series)
	lastVol   uint32        // highest cumulative day volume seen on volDay
	volKnown  bool          // lastVol is a baseline for volDay
	volDay    calendar.Date // IST day lastVol belongs to (Kite's cumulative volume restarts daily)
	series    map[candles.Interval]*series
}

// Builder is safe for concurrent use; Run drives it from the ticker's raw stream.
type Builder struct {
	o       Options
	mu      sync.Mutex
	tokens  map[uint32]*tokenState
	late    uint64
	dropped uint64
	// next close (end + grace of the earliest forming candle), recomputed only after a
	// candle opened or closed.
	next      time.Time
	nextDirty bool
	wake      chan struct{}
	writes    chan []Event
	written   atomic.Uint64 // batches the writer finished (tests)
}

// New builds a Builder.
func New(o Options) *Builder {
	if o.Grace == 0 {
		o.Grace = time.Second
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.ExchangeOf == nil {
		o.ExchangeOf = func(uint32) calendar.Exchange { return calendar.NSE }
	}
	return &Builder{o: o, tokens: map[uint32]*tokenState{}, wake: make(chan struct{}, 1),
		writes: make(chan []Event, writeQueue)}
}

// Track starts building ivs (intraday minute intervals or Day) for token.
func (b *Builder) Track(token uint32, ivs ...candles.Interval) {
	now := b.o.Clock.Now()
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, iv := range ivs {
		if !iv.IsIntraday() && iv != candles.Day {
			continue
		}
		st := b.tokens[token]
		if st == nil {
			st = &tokenState{trackedAt: now, series: map[candles.Interval]*series{}}
			b.tokens[token] = st
		}
		if _, ok := st.series[iv]; !ok {
			st.series[iv] = &series{trackedAt: now}
		}
	}
}

// Untrack stops building ivs for token (the forming candle is discarded). A token with no
// series left forgets its volume baseline.
func (b *Builder) Untrack(token uint32, ivs ...candles.Interval) {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := b.tokens[token]
	if st == nil {
		return
	}
	for _, iv := range ivs {
		if s, ok := st.series[iv]; ok {
			if s.cur.have {
				b.nextDirty = true
			}
			delete(st.series, iv)
		}
	}
	if len(st.series) == 0 {
		delete(b.tokens, token)
	}
}

// LateTicks counts ticks dropped because their candle had already closed.
func (b *Builder) LateTicks() uint64 { b.mu.Lock(); defer b.mu.Unlock(); return b.late }

// DroppedWrites counts closed candles not stored because the writer queue was full.
func (b *Builder) DroppedWrites() uint64 { b.mu.Lock(); defer b.mu.Unlock(); return b.dropped }

// continuous returns the first continuous-phase start and the last continuous-phase end of day.
func continuous(day calendar.Day) (start, end time.Time) {
	for _, p := range day.Phases {
		if p.Kind != calendar.PhaseContinuous {
			continue
		}
		if start.IsZero() || p.Start.Before(start) {
			start = p.Start
		}
		if p.End.After(end) {
			end = p.End
		}
	}
	return start, end
}

// OnTick applies one tick. Its time is the exchange timestamp when present (full mode), else
// the app-clock arrival time. Ticks outside the continuous phase are ignored.
func (b *Builder) OnTick(t ticker.Tick) {
	if t.LTP <= 0 { // no trade yet (illiquid snapshot): carries no price
		return
	}
	ts := t.ExchangeTS
	if ts.IsZero() {
		ts = t.Received
	}
	phase, day := b.o.Calendar.PhaseAt(b.o.ExchangeOf(t.Token), ts)
	if phase.Kind != calendar.PhaseContinuous {
		return
	}
	sessStart, sessEnd := continuous(day)
	var out, closed []Event
	b.mu.Lock()
	st := b.tokens[t.Token]
	if st == nil {
		b.mu.Unlock()
		return
	}
	if st.volKnown && st.volDay != day.Date {
		st.volKnown = false
	}
	// Cumulative day volume only grows; an older (lower) value never shrinks a candle.
	vol := st.lastVol
	if !st.volKnown || t.Volume > vol {
		vol = t.Volume
	}
	applied := false
	for iv, s := range st.series {
		start, end := bucket(iv, ts, phase, sessEnd)
		if (!s.closedEnd.IsZero() && ts.Before(s.closedEnd)) || (s.cur.have && start.Before(s.cur.c.Start)) {
			b.late++ // its candle already closed, or is older than the forming one (out of order)
			continue
		}
		if s.cur.have && start.After(s.cur.c.Start) {
			closed = append(closed, b.closeLocked(t.Token, iv, s))
		}
		if !s.cur.have {
			s.cur = forming{have: true, end: end, last: ts,
				c: candles.Candle{Start: start, Open: t.LTP, High: t.LTP, Low: t.LTP, Source: candles.SourceTick}}
			if iv == candles.Day {
				// The day candle's volume is the cumulative day volume itself; it is partial
				// only when tracking began after the session opened.
				s.cur.partial = s.trackedAt.After(sessStart)
			} else {
				// Without a baseline the candle's volume is unknown unless the token was
				// tracked before the session opened (cumulative volume then starts at 0).
				baseKnown := st.volKnown || !st.trackedAt.After(sessStart)
				if st.volKnown {
					s.cur.base = st.lastVol
				}
				s.cur.partial = s.trackedAt.After(start) || !baseKnown
			}
			b.nextDirty = true
		}
		applied = true
		c := &s.cur.c
		if iv == candles.Day && t.Open > 0 { // quote/full ticks carry the day's OHLC
			c.Open, c.High, c.Low = t.Open, t.High, t.Low
		}
		if t.LTP > c.High {
			c.High = t.LTP
		}
		if t.LTP < c.Low {
			c.Low = t.LTP
		}
		if !ts.Before(s.cur.last) {
			c.Close, s.cur.last = t.LTP, ts
			if t.OI > 0 {
				c.OI, c.HasOI = int64(t.OI), true
			}
		}
		if iv == candles.Day {
			c.Volume = int64(vol)
		} else if vol >= s.cur.base {
			c.Volume = int64(vol - s.cur.base)
		}
		out = append(out, Event{Token: t.Token, Interval: iv, Candle: *c, Partial: s.cur.partial})
	}
	if applied {
		st.lastVol, st.volKnown, st.volDay = vol, true, day.Date
	}
	b.mu.Unlock()
	b.emitClosed(closed)
	for _, e := range out {
		publish(b.o.Bus, TopicForming, e)
	}
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// bucket returns the candle containing ts: anchored at the phase start, cut at its end; the
// day candle runs to the end of the day's last continuous phase.
func bucket(iv candles.Interval, ts time.Time, phase calendar.Phase, sessEnd time.Time) (time.Time, time.Time) {
	if iv == candles.Day {
		return clock.MidnightIST(ts), sessEnd
	}
	start := clock.BucketStart(ts, phase.Start, iv.Duration())
	end := start.Add(iv.Duration())
	if end.After(phase.End) {
		end = phase.End
	}
	return start, end
}

func (b *Builder) closeLocked(token uint32, iv candles.Interval, s *series) Event {
	e := Event{Token: token, Interval: iv, Candle: s.cur.c, Partial: s.cur.partial}
	s.closedEnd = s.cur.end
	s.cur = forming{}
	b.nextDirty = true
	return e
}

// emitClosed publishes closed candles first, then hands the storable ones to the writer as one
// batch (Ruling 44): the tick path never waits on the database.
func (b *Builder) emitClosed(es []Event) {
	if len(es) == 0 {
		return
	}
	sort.Slice(es, func(i, j int) bool {
		if es[i].Token != es[j].Token {
			return es[i].Token < es[j].Token
		}
		return es[i].Interval.Duration() < es[j].Interval.Duration()
	})
	var batch []Event
	for _, e := range es {
		publish(b.o.Bus, TopicClosed, e)
		if b.o.Store != nil && !e.Partial && e.Interval.IsNative() {
			batch = append(batch, e)
		}
	}
	if len(batch) == 0 {
		return
	}
	select {
	case b.writes <- batch:
	default:
		b.mu.Lock()
		b.dropped += uint64(len(batch))
		n := b.dropped
		b.mu.Unlock()
		b.o.Log.Warn("builder: store queue full, closed candles not stored", "candles", len(batch), "dropped", n)
	}
}

// writer stores queued batches until ctx ends, or until stop closes (then it drains the queue).
func (b *Builder) writer(ctx context.Context, stop <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-b.writes:
			b.store(ctx, batch)
		case <-stop:
			for {
				select {
				case batch := <-b.writes:
					b.store(ctx, batch)
				default:
					return
				}
			}
		}
	}
}

// store writes one boundary's batch, one Upsert per (token, interval): candles.Store has no
// multi-series write, so the batch is not a single transaction.
func (b *Builder) store(ctx context.Context, batch []Event) {
	defer b.written.Add(1)
	for i := 0; i < len(batch); {
		j := i + 1
		for j < len(batch) && batch[j].Token == batch[i].Token && batch[j].Interval == batch[i].Interval {
			j++
		}
		cs := make([]candles.Candle, 0, j-i)
		for _, e := range batch[i:j] {
			cs = append(cs, e.Candle)
		}
		if err := b.o.Store.Upsert(ctx, batch[i].Token, batch[i].Interval, cs, candles.SourceTick); err != nil && ctx.Err() == nil {
			b.o.Log.Error("builder: store candle", "token", batch[i].Token, "interval", string(batch[i].Interval), "err", err)
		}
		i = j
	}
}

// CloseDue closes every forming candle whose end + grace is not after now; returns how many.
func (b *Builder) CloseDue(now time.Time) int {
	var closed []Event
	b.mu.Lock()
	for tok, st := range b.tokens {
		for iv, s := range st.series {
			if s.cur.have && !s.cur.end.Add(b.o.Grace).After(now) {
				closed = append(closed, b.closeLocked(tok, iv, s))
			}
		}
	}
	b.mu.Unlock()
	b.emitClosed(closed)
	return len(closed)
}

// nextClose is the earliest end + grace among forming candles, recomputed only when a candle
// opened or closed since the last call.
func (b *Builder) nextClose() (time.Time, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.nextDirty {
		b.next, b.nextDirty = time.Time{}, false
		for _, st := range b.tokens {
			for _, s := range st.series {
				if s.cur.have {
					if t := s.cur.end.Add(b.o.Grace); b.next.IsZero() || t.Before(b.next) {
						b.next = t
					}
				}
			}
		}
	}
	return b.next, !b.next.IsZero()
}

// Run applies ticks from the ticker's raw stream, closes candles on app-clock timers and runs
// the store writer. It returns when ctx ends or ticks is closed; call it once.
func (b *Builder) Run(ctx context.Context, ticks <-chan ticker.Tick) {
	stop, done := make(chan struct{}), make(chan struct{})
	logging.Go("candles.builder.writer", func() {
		defer close(done)
		b.writer(ctx, stop)
	})
	defer func() {
		close(stop)
		<-done
	}()
	var timer clock.Timer
	var armed time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		var tc <-chan time.Time
		if next, ok := b.nextClose(); ok {
			if !next.Equal(armed) {
				if timer != nil {
					timer.Stop()
				}
				timer, armed = b.o.Clock.At(next), next
			}
			tc = timer.C()
		}
		select {
		case <-ctx.Done():
			return
		case t, ok := <-ticks:
			if !ok { // the ticker's Raw stream is closed when its subscription is cancelled
				return
			}
			b.OnTick(t)
		case now := <-tc:
			b.CloseDue(now)
			timer, armed = nil, time.Time{}
		case <-b.wake:
		}
	}
}

// publish sends v on t when a bus is configured (tests and tools may run without one).
func publish[T any](b *bus.Bus, t bus.Topic[T], v T) {
	if b != nil {
		bus.Publish(b, t, v)
	}
}
