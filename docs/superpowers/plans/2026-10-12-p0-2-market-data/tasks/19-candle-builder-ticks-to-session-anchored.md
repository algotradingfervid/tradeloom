> **P0-2 Time & Market Data Implementation Plan — Task 19.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 19: Candle builder: ticks to session-anchored candles, partial vs closed

**Files:**
- Create: `internal/candles/builder/builder.go`
- Test: `internal/candles/builder/builder_test.go`

**Interfaces:**
- Consumes: Task 1: `clock.Clock`, `clock.Fake`, `clock.BucketStart`, `clock.MidnightIST`. Task 4: `(*calendar.Service).PhaseAt`, `calendar.Phase`, `PhaseContinuous`. Task 10/11: `candles.Store.Upsert`, `candles.Candle`, `sqlite.Open` (tests). Task 17: `ticker.Tick`, `(*ticker.Manager).Raw`. P0-1: `bus.*`.
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
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
type Options struct {
	Clock      clock.Clock
	Calendar   *calendar.Service
	Store      candles.Store // nil: closed candles are only published
	Bus        *bus.Bus
	ExchangeOf func(token uint32) calendar.Exchange // instruments.Master.ExchangeOf
	Grace      time.Duration                        // default 1 s (FR-DEP-006)
	Log        *slog.Logger
}
type Builder struct{ /* unexported fields */ }
func New(o Options) *Builder
func (b *Builder) Track(token uint32, ivs ...candles.Interval)
func (b *Builder) Untrack(token uint32, ivs ...candles.Interval)
func (b *Builder) LateTicks() uint64
func (b *Builder) OnTick(t ticker.Tick)
func (b *Builder) CloseDue(now time.Time) int
func (b *Builder) Run(ctx context.Context, ticks <-chan ticker.Tick)
```

FR-CHART-003 / FR-DEP-006 / FR-DATA-002: buckets anchored at the calendar's continuous-phase start and cut at its end, exchange timestamp when present else app-clock arrival, close at boundary + 1 s grace on the app clock, late ticks after close dropped and counted, volume as the difference of cumulative day volume, ticks outside the continuous phase ignored, a candle that tracking joined mid-way published as `Partial` and never stored, closed candles stored tick-built and incomplete until the backfill replaces them.

- [ ] **Step 1: Write the failing tests**

`internal/candles/builder/builder_test.go`:

```go
package builder

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/candles/sqlite"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite/ticker"
)

func ist(y int, m time.Month, d, h, min, s int) time.Time {
	return time.Date(y, m, d, h, min, s, 0, clock.IST)
}

type rig struct {
	b       *Builder
	clk     *clock.Fake
	cal     *calendar.Service
	st      *sqlite.Store
	closed  *bus.Sub[Event]
	forming *bus.Sub[Event]
}

func newRig(t *testing.T, now time.Time) *rig {
	t.Helper()
	cal, _ := calendar.NewDefault()
	fc := clock.NewFake(now)
	st, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "c.db"), sqlite.Options{Clock: fc, Calendar: cal})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	b := bus.New(nil)
	r := &rig{clk: fc, cal: cal, st: st,
		closed: bus.Subscribe(b, TopicClosed, bus.SubOpts[Event]{Buffer: 64}), forming: bus.Subscribe(b, TopicForming, bus.SubOpts[Event]{Buffer: 1024})}
	r.b = New(Options{Clock: fc, Calendar: cal, Store: st, Bus: b})
	return r
}

func tick(tok uint32, at time.Time, ltp float64, vol uint32) ticker.Tick {
	return ticker.Tick{Token: tok, LTP: ltp, Volume: vol, ExchangeTS: at, Received: at}
}

func next(t *testing.T, s *bus.Sub[Event]) Event {
	t.Helper()
	select {
	case e := <-s.C():
		return e
	case <-time.After(2 * time.Second):
		t.Fatal("no event")
	}
	return Event{}
}

func TestFiveMinuteCandleFromTicksClosesAtBoundaryPlusGrace(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 9, 50, 0))
	r.b.Track(1, candles.Minute5)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticks := make(chan ticker.Tick, 16)
	go r.b.Run(ctx, ticks)

	r.clk.Set(ist(2026, 10, 12, 10, 0, 0))
	for _, tk := range []ticker.Tick{
		tick(1, ist(2026, 10, 12, 10, 0, 0), 100, 1000), tick(1, ist(2026, 10, 12, 10, 1, 0), 103, 1100),
		tick(1, ist(2026, 10, 12, 10, 2, 0), 98, 1250), tick(1, ist(2026, 10, 12, 10, 4, 59), 101, 1300),
	} {
		ticks <- tk
	}
	if !r.clk.WaitTimers(1, 2*time.Second) {
		t.Fatal("no close timer")
	}
	r.clk.Set(ist(2026, 10, 12, 10, 5, 0)) // boundary: still inside the 1 s grace
	ticks <- ticker.Tick{Token: 1, LTP: 102, Volume: 1320, ExchangeTS: ist(2026, 10, 12, 10, 4, 59), Received: r.clk.Now()}
	time.Sleep(20 * time.Millisecond)
	select {
	case e := <-r.closed.C():
		t.Fatalf("closed before grace: %+v", e)
	default:
	}
	r.clk.Set(ist(2026, 10, 12, 10, 5, 1))
	e := next(t, r.closed)
	want := candles.Candle{Start: ist(2026, 10, 12, 10, 0, 0), Open: 100, High: 103, Low: 98, Close: 102, Volume: 1320, Source: candles.SourceTick}
	if e.Candle != want || e.Partial || e.Interval != candles.Minute5 {
		t.Fatalf("closed %+v\nwant %+v", e.Candle, want)
	}
	got, _ := r.st.Read(ctx, 1, candles.Minute5, want.Start, want.Start.Add(5*time.Minute))
	if len(got) != 1 || got[0].Complete || got[0].Source != candles.SourceTick || got[0].Close != 102 {
		t.Fatalf("stored %+v, want tick-built incomplete", got)
	}
	ticks <- tick(1, ist(2026, 10, 12, 10, 4, 59), 99, 1330) // after close: dropped
	ticks <- tick(1, ist(2026, 10, 12, 10, 5, 2), 104, 1400)
	time.Sleep(20 * time.Millisecond)
	if r.b.LateTicks() != 1 {
		t.Fatalf("late ticks = %d", r.b.LateTicks())
	}
	var last Event
	for len(r.forming.C()) > 0 {
		last = <-r.forming.C()
	}
	if last.Candle.Start != ist(2026, 10, 12, 10, 5, 0) || last.Candle.Volume != 80 || last.Candle.Open != 104 {
		t.Fatalf("next forming %+v (volume is the cumulative difference)", last.Candle)
	}
}

func TestPartialFirstCandleIsPublishedButNotStored(t *testing.T) { // Review Focus 4
	r := newRig(t, ist(2026, 10, 12, 10, 2, 30)) // chart opened mid-candle
	r.b.Track(2, candles.Minute5)
	r.b.OnTick(tick(2, ist(2026, 10, 12, 10, 2, 31), 50, 9000))
	if e := next(t, r.forming); !e.Partial {
		t.Fatal("first candle after tracking mid-bucket must be partial")
	}
	r.clk.Set(ist(2026, 10, 12, 10, 5, 1))
	r.b.CloseDue(r.clk.Now())
	if e := next(t, r.closed); !e.Partial {
		t.Fatal("closed event must keep Partial")
	}
	if got, _ := r.st.Read(context.Background(), 2, candles.Minute5, ist(2026, 10, 12, 10, 0, 0), ist(2026, 10, 12, 10, 5, 0)); len(got) != 0 {
		t.Fatalf("partial candle stored: %+v", got)
	}
	r.b.OnTick(tick(2, ist(2026, 10, 12, 10, 5, 3), 51, 9100))
	if e := next(t, r.forming); e.Partial || e.Candle.Volume != 100 {
		t.Fatalf("second candle %+v", e)
	}
}

func TestSessionAnchorsHourlyAndSpecialSession(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 9, 0, 0))
	r.b.Track(3, candles.Minute60)
	r.b.OnTick(tick(3, ist(2026, 10, 12, 9, 10, 0), 10, 1)) // pre-open: ignored
	r.b.OnTick(tick(3, ist(2026, 10, 12, 15, 20, 0), 11, 5))
	if e := next(t, r.forming); e.Candle.Start != ist(2026, 10, 12, 15, 15, 0) {
		t.Fatalf("60m bucket start %v, want 15:15 (anchored at 09:15)", e.Candle.Start)
	}
	r.clk.Set(ist(2026, 10, 12, 15, 30, 1))
	if r.b.CloseDue(r.clk.Now()) != 1 {
		t.Fatal("last hourly candle must close at the 15:30 session end")
	}

	f, _ := calendar.ParseFile(strings.NewReader(calendar.Header+"\n2026-11-08,NSE,special,continuous,18:15,19:15\n"), "test")
	r.cal.MergeOverrides(f.Entries)
	r.b.Track(4, candles.Minute5)
	r.clk.Set(ist(2026, 11, 8, 18, 0, 0))
	r.b.OnTick(tick(4, ist(2026, 11, 8, 18, 16, 10), 20, 1))
	if e := next(t, r.forming); e.Candle.Start != ist(2026, 11, 8, 18, 15, 0) {
		t.Fatalf("special-session 5m start %v, want 18:15", e.Candle.Start)
	}
}

func TestTicksWithoutExchangeTimestampUseAppClock(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 11, 0, 0))
	r.b.Track(5, candles.Minute)
	r.b.OnTick(ticker.Tick{Token: 5, LTP: 7, Received: ist(2026, 10, 12, 11, 0, 30)})
	if e := next(t, r.forming); e.Candle.Start != ist(2026, 10, 12, 11, 0, 0) {
		t.Fatalf("start %v", e.Candle.Start)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/candles/builder/`
Expected: the build fails — output contains `undefined: New` and `[build failed]`.

- [ ] **Step 3: Write the implementation**

`internal/candles/builder/builder.go`:

```go
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
	base    uint32 // cumulative day volume before this candle
}

type series struct {
	iv        candles.Interval
	trackedAt time.Time
	lastVol   uint32
	volKnown  bool
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
	ts := t.ExchangeTS
	if ts.IsZero() {
		ts = t.Received
	}
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
		if !s.closedEnd.IsZero() && ts.Before(s.closedEnd) {
			b.late++
			continue
		}
		if s.cur.have && start.After(s.cur.c.Start) {
			closed = append(closed, b.closeLocked(k, s))
		}
		if !s.cur.have {
			s.cur = forming{have: true, end: end, partial: s.trackedAt.After(start),
				c: candles.Candle{Start: start, Open: t.LTP, High: t.LTP, Low: t.LTP, Source: candles.SourceTick}}
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
		c.Close = t.LTP
		if t.Volume >= s.cur.base {
			c.Volume = int64(t.Volume - s.cur.base)
		}
		if t.OI > 0 {
			c.OI, c.HasOI = int64(t.OI), true
		}
		s.lastVol, s.volKnown = t.Volume, true
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
		case t := <-ticks:
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
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/candles/builder/`
Expected: `ok  	tradeloom/internal/candles/builder`; no `DATA RACE`.

- [ ] **Step 5: Commit**

```bash
git add internal/candles/builder/builder.go \
  internal/candles/builder/builder_test.go
git commit -m "$(cat <<'EOF'
feat(candles/builder): session-anchored tick-to-candle builder with grace and partial handling

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
