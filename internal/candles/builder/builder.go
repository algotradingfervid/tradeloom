// Package builder turns ticks into candles aligned to the calendar's session anchor (FR-DATA-002,
// FR-CHART-003, FR-DEP-006): forming candles are published as they change; a candle closes on
// the app clock at its boundary + grace, is published as closed and stored as tick-built
// (complete = false) until a historical fetch replaces it.
package builder

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite/ticker"
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

type forming struct {
	have    bool
	c       candles.Candle
	end     time.Time // exclusive bucket end (cut at session end)
	partial bool
	base    uint32    // cumulative day volume before this candle
	last    time.Time // timestamp of the newest tick applied (out-of-order ticks keep Close)
}

type series struct {
	iv        candles.Interval
	trackedAt time.Time
	lastVol   uint32        // highest cumulative day volume seen on volDay
	volKnown  bool          // lastVol is a baseline for volDay
	volDay    calendar.Date // IST day lastVol belongs to (Kite's cumulative volume restarts daily)
	cur       forming
	closedEnd time.Time // end of the last closed candle (late-tick guard)
}

type key struct {
	token uint32
	iv    candles.Interval
}

// Builder is safe for concurrent use; Run drives it from the ticker's raw stream.
type Builder struct {
	o      Options
	mu     sync.Mutex
	series map[key]*series
	late   uint64
	wake   chan struct{}
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
	return &Builder{o: o, series: map[key]*series{}, wake: make(chan struct{}, 1)}
}

// Track starts building ivs (intraday minute intervals or Day) for token.
func (b *Builder) Track(token uint32, ivs ...candles.Interval) {
	now := b.o.Clock.Now()
	b.mu.Lock()
	for _, iv := range ivs {
		k := key{token, iv}
		if _, ok := b.series[k]; !ok && (iv.IsIntraday() || iv == candles.Day) {
			b.series[k] = &series{iv: iv, trackedAt: now}
		}
	}
	b.mu.Unlock()
}

// Untrack stops building ivs for token (the forming candle is discarded).
func (b *Builder) Untrack(token uint32, ivs ...candles.Interval) {
	b.mu.Lock()
	for _, iv := range ivs {
		delete(b.series, key{token, iv})
	}
	b.mu.Unlock()
}

// LateTicks counts ticks dropped because their candle had already closed.
func (b *Builder) LateTicks() uint64 { b.mu.Lock(); defer b.mu.Unlock(); return b.late }

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
	day := calendar.DateOf(ts)
	phase, _ := b.o.Calendar.PhaseAt(b.o.ExchangeOf(t.Token), ts)
	if phase.Kind != calendar.PhaseContinuous {
		return
	}
	var out []Event
	var closed []Event
	b.mu.Lock()
	for k, s := range b.series {
		if k.token != t.Token {
			continue
		}
		start, end := bucket(s.iv, ts, phase)
		if (!s.closedEnd.IsZero() && ts.Before(s.closedEnd)) || (s.cur.have && start.Before(s.cur.c.Start)) {
			b.late++ // its candle already closed, or is older than the forming one (out of order)
			continue
		}
		if s.volKnown && s.volDay != day {
			s.volKnown = false
		}
		if s.cur.have && start.After(s.cur.c.Start) {
			closed = append(closed, b.closeLocked(k, s))
		}
		if !s.cur.have {
			s.cur = forming{have: true, end: end, partial: s.trackedAt.After(start),
				last: ts, c: candles.Candle{Start: start, Open: t.LTP, High: t.LTP, Low: t.LTP, Source: candles.SourceTick}}
			s.cur.base = s.lastVol
			if !s.volKnown {
				s.cur.base = 0
			}
		}
		c := &s.cur.c
		if s.iv == candles.Day && t.Open > 0 { // quote/full ticks carry the day's OHLC
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
		// Cumulative day volume only grows; an older (lower) value never shrinks the candle.
		if !s.volKnown || t.Volume > s.lastVol {
			s.lastVol = t.Volume
		}
		s.volKnown, s.volDay = true, day
		if s.iv == candles.Day {
			c.Volume = int64(s.lastVol) // the day candle's volume is the cumulative day volume
		} else if s.lastVol >= s.cur.base {
			c.Volume = int64(s.lastVol - s.cur.base)
		}
		out = append(out, Event{Token: k.token, Interval: k.iv, Candle: *c, Partial: s.cur.partial})
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

// bucket returns the candle containing ts: anchored at the session start, cut at its end.
func bucket(iv candles.Interval, ts time.Time, phase calendar.Phase) (time.Time, time.Time) {
	if iv == candles.Day {
		return clock.MidnightIST(ts), phase.End
	}
	start := clock.BucketStart(ts, phase.Start, iv.Duration())
	end := start.Add(iv.Duration())
	if end.After(phase.End) {
		end = phase.End
	}
	return start, end
}

func (b *Builder) closeLocked(k key, s *series) Event {
	e := Event{Token: k.token, Interval: k.iv, Candle: s.cur.c, Partial: s.cur.partial}
	s.closedEnd = s.cur.end
	s.cur = forming{}
	return e
}

func (b *Builder) emitClosed(es []Event) {
	sort.Slice(es, func(i, j int) bool {
		if es[i].Token != es[j].Token {
			return es[i].Token < es[j].Token
		}
		return es[i].Interval.Duration() < es[j].Interval.Duration()
	})
	for _, e := range es {
		if b.o.Store != nil && !e.Partial && e.Interval.IsNative() {
			if err := b.o.Store.Upsert(context.Background(), e.Token, e.Interval, []candles.Candle{e.Candle}, candles.SourceTick); err != nil {
				b.o.Log.Error("builder: store candle", "token", e.Token, "interval", string(e.Interval), "err", err)
			}
		}
		publish(b.o.Bus, TopicClosed, e)
	}
}

// CloseDue closes every forming candle whose end + grace is not after now; returns how many.
func (b *Builder) CloseDue(now time.Time) int {
	var closed []Event
	b.mu.Lock()
	for k, s := range b.series {
		if s.cur.have && !s.cur.end.Add(b.o.Grace).After(now) {
			closed = append(closed, b.closeLocked(k, s))
		}
	}
	b.mu.Unlock()
	b.emitClosed(closed)
	return len(closed)
}

// nextClose is the earliest end + grace among forming candles.
func (b *Builder) nextClose() (time.Time, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var next time.Time
	for _, s := range b.series {
		if s.cur.have {
			if t := s.cur.end.Add(b.o.Grace); next.IsZero() || t.Before(next) {
				next = t
			}
		}
	}
	return next, !next.IsZero()
}

// Run applies ticks from the ticker's raw stream and closes candles on app-clock timers.
func (b *Builder) Run(ctx context.Context, ticks <-chan ticker.Tick) {
	var timer clock.Timer
	var armed time.Time
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
			if timer != nil {
				timer.Stop()
			}
			return
		case t, ok := <-ticks:
			if !ok { // the ticker's Raw stream is closed when its subscription is cancelled
				if timer != nil {
					timer.Stop()
				}
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
