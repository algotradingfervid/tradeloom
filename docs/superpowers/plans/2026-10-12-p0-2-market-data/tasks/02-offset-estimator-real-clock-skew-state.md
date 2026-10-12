> **P0-2 Time & Market Data Implementation Plan — Task 2.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 2: Offset estimator, Real clock, skew state, re-arming timers

**Files:**
- Create: `internal/clock/estimator.go`
- Create: `internal/clock/real.go`
- Test: `internal/clock/real_test.go`

**Interfaces:**
- Consumes: Task 1: `Clock`, `Timer`, `Offset`, `Source*`, `IST`. P0-1: `clock.SystemNow` (default `Options.System`), `bus.Bus`, `bus.NewTopic`, `bus.Publish`, `bus.Subscribe`, `bus.SubOpts[T]`, `(*bus.Sub[T]).C`, `bus.New(nil)` (test).
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
var TopicOffset = bus.NewTopic[Offset]("clock.offset")
type Options struct {
	Bus         *bus.Bus     // nil: no events
	Log         *slog.Logger // nil: slog.Default()
	System      func() time.Time
	TickWindow  time.Duration // default 60 s
	HTTPSamples int           // default 5
	WarnAbove   time.Duration // default 1 s
	SkewAbove   time.Duration // default 5 s
	ClearBelow  time.Duration // default 2 s
	Rearm       time.Duration // default 100 ms: offset moves larger than this re-arm timers
}
type Real struct{ /* unexported fields */ }
func New(o Options) *Real
func (r *Real) Now() time.Time
func (r *Real) System() time.Time
func (r *Real) Offset() Offset
func (r *Real) ObserveTick(exchangeTS, arrival time.Time)
func (r *Real) ObserveHTTPDate(date, sent, received time.Time)
func (r *Real) Acknowledge()
func (r *Real) At(target time.Time) Timer
func (r *Real) After(d time.Duration) Timer
```

FR-DATA-004: offset from tick exchange timestamps (median of 60 s) or HTTP `Date` (median of 5), warn > 1 s, skew > 5 s latched until < 2 s or acknowledged, timers re-armed when the offset moves. `Real` uses `time.AfterFunc` (no goroutines of its own), so it needs no `logging.Go`.

- [ ] **Step 1: Write the failing tests**

`internal/clock/real_test.go`:

```go
package clock

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/bus"
)

// manualSystem is a settable system clock for Real.
type manualSystem struct {
	mu sync.Mutex
	t  time.Time
}

func (m *manualSystem) now() time.Time  { m.mu.Lock(); defer m.mu.Unlock(); return m.t }
func (m *manualSystem) set(t time.Time) { m.mu.Lock(); m.t = t; m.mu.Unlock() }

// feedTicks simulates 60 s of ~2 ticks/s from an exchange whose true time is exch, received
// by a system clock that runs `skew` ahead, with 20 ms network latency and random sub-second
// phase. It returns the true exchange time at the end.
func feedTicks(r *Real, ms *manualSystem, exch time.Time, skew time.Duration, seed int64) time.Time {
	rng := rand.New(rand.NewSource(seed))
	for i := 0; i < 120; i++ {
		exch = exch.Add(time.Duration(400+rng.Intn(200)) * time.Millisecond)
		arrival := exch.Add(skew).Add(20 * time.Millisecond)
		ms.set(arrival)
		r.ObserveTick(exch.Truncate(time.Second), arrival)
	}
	return exch
}

func TestOffsetFromTicksWithSystemClock3sFast(t *testing.T) { // FR-DATA-004 AC1
	ms := &manualSystem{}
	r := New(Options{System: ms.now})
	exch := time.Date(2026, 10, 12, 10, 0, 0, 0, IST)
	exch = feedTicks(r, ms, exch, 3*time.Second, 1)

	off := r.Offset()
	if off.Source != SourceTicks {
		t.Fatalf("source = %s, want ticks", off.Source)
	}
	if d := off.Value + 3*time.Second; d > 500*time.Millisecond || d < -500*time.Millisecond {
		t.Fatalf("offset = %v, want -3s ± 0.5s", off.Value)
	}
	// App clock equals exchange time ± 0.5 s, so a 1-minute candle boundary scheduled with
	// At() fires within ± 0.5 s of the exchange boundary.
	if d := r.Now().Sub(exch.Add(20 * time.Millisecond)); d > 500*time.Millisecond || d < -500*time.Millisecond {
		t.Fatalf("app clock off exchange by %v", d)
	}
	if !off.Warn || off.Skewed {
		t.Fatalf("3 s offset: warn=%v skewed=%v, want warn only", off.Warn, off.Skewed)
	}
}

func TestOffsetFromHTTPDateWhenNoTicks(t *testing.T) { // FR-DATA-004 AC4
	ms := &manualSystem{t: time.Date(2026, 10, 12, 18, 0, 0, 0, IST)}
	r := New(Options{System: ms.now})
	for i := 0; i < 5; i++ {
		sent := ms.now()
		received := sent.Add(80 * time.Millisecond)
		server := sent.Add(40 * time.Millisecond).Add(-2 * time.Second) // system 2 s fast
		r.ObserveHTTPDate(server.Truncate(time.Second), sent, received)
		ms.set(received.Add(time.Duration(300+100*i) * time.Millisecond))
	}
	off := r.Offset()
	if off.Source != SourceHTTPDate || off.Samples != 5 {
		t.Fatalf("source=%s samples=%d, want http-date/5", off.Source, off.Samples)
	}
	if d := off.Value + 2*time.Second; d > 500*time.Millisecond || d < -500*time.Millisecond {
		t.Fatalf("offset = %v, want -2s ± 0.5s", off.Value)
	}
}

func TestTicksWinOverHTTPAndExpireAfterWindow(t *testing.T) {
	ms := &manualSystem{t: time.Date(2026, 10, 12, 10, 0, 0, 0, IST)}
	r := New(Options{System: ms.now})
	r.ObserveHTTPDate(ms.now().Add(-4*time.Second), ms.now(), ms.now())
	feedTicks(r, ms, ms.now(), 0, 2)
	if r.Offset().Source != SourceTicks {
		t.Fatalf("ticks should win, got %s", r.Offset().Source)
	}
	ms.set(ms.now().Add(61 * time.Second)) // market closes; tick samples age out
	r.ObserveHTTPDate(ms.now().Truncate(time.Second), ms.now(), ms.now())
	if r.Offset().Source != SourceHTTPDate {
		t.Fatalf("after 61 s without ticks source = %s, want http-date", r.Offset().Source)
	}
}

func TestSkewHysteresisAndAcknowledge(t *testing.T) { // FR-DATA-004 AC3 (clock side)
	b := bus.New(nil)
	sub := bus.Subscribe(b, TopicOffset, bus.SubOpts[Offset]{Buffer: 64})
	ms := &manualSystem{t: time.Date(2026, 10, 12, 10, 0, 0, 0, IST)}
	r := New(Options{System: ms.now, Bus: b})
	observe := func(off time.Duration) {
		for i := 0; i < 130; i++ { // replace the whole 60 s window
			ms.set(ms.now().Add(500 * time.Millisecond))
			r.ObserveTick(ms.now().Add(off).Add(-halfSecond), ms.now())
		}
	}
	observe(7 * time.Second)
	if !r.Offset().Skewed {
		t.Fatal("7 s offset not skewed")
	}
	select {
	case o := <-sub.C():
		_ = o
	default:
		t.Fatal("no clock.offset event published")
	}
	observe(3 * time.Second)
	if !r.Offset().Skewed {
		t.Fatal("3 s is above the 2 s clear threshold; must stay skewed")
	}
	observe(1500 * time.Millisecond)
	if r.Offset().Skewed {
		t.Fatal("1.5 s must clear skew")
	}
	observe(7 * time.Second)
	r.Acknowledge()
	if r.Offset().Skewed {
		t.Fatal("acknowledged skew still reported")
	}
}

func TestRealTimerFiresAndRearmsOnOffsetChange(t *testing.T) {
	r := New(Options{}) // real system clock
	start := time.Now()
	tm := r.At(r.Now().Add(300 * time.Millisecond))
	// The exchange turns out to be 200 ms ahead: the timer must fire ~200 ms earlier.
	for i := 0; i < 5; i++ {
		r.ObserveHTTPDate(time.Now().Add(200*time.Millisecond-halfSecond), time.Now(), time.Now())
	}
	select {
	case <-tm.C():
		if el := time.Since(start); el > 250*time.Millisecond {
			t.Fatalf("fired after %v; offset change was not applied", el)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timer never fired")
	}
	if tm.Stop() {
		t.Fatal("Stop after fire must return false")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/clock/`
Expected: the build fails — output contains `undefined: New` and `[build failed]`.

- [ ] **Step 3: Write the implementation**

`internal/clock/estimator.go`:

```go
package clock

import (
	"sort"
	"time"
)

// halfSecond corrects for truncation: Kite tick timestamps and HTTP Date headers carry whole
// seconds, so the true instant lies uniformly in [ts, ts+1s); its expected value is ts+0.5 s.
const halfSecond = 500 * time.Millisecond

type tickSample struct {
	at  time.Time // system arrival time
	off time.Duration
}

// estimator keeps offset samples. It is not safe for concurrent use; Real guards it.
type estimator struct {
	window time.Duration // tick samples kept (60 s)
	httpN  int           // HTTP samples kept (5)
	ticks  []tickSample
	http   []time.Duration
}

func (e *estimator) addTick(exchangeTS, arrival time.Time) {
	e.ticks = append(e.ticks, tickSample{at: arrival, off: exchangeTS.Add(halfSecond).Sub(arrival)})
	e.prune(arrival)
}

func (e *estimator) addHTTP(date, sent, received time.Time) {
	mid := sent.Add(received.Sub(sent) / 2)
	e.http = append(e.http, date.Add(halfSecond).Sub(mid))
	if len(e.http) > e.httpN {
		e.http = e.http[len(e.http)-e.httpN:]
	}
}

func (e *estimator) prune(now time.Time) {
	cut := now.Add(-e.window)
	i := 0
	for i < len(e.ticks) && e.ticks[i].at.Before(cut) {
		i++
	}
	e.ticks = e.ticks[i:]
}

// estimate prefers ticks seen in the last window, then the last HTTP Date samples.
func (e *estimator) estimate(now time.Time) (time.Duration, Source, int) {
	e.prune(now)
	if len(e.ticks) > 0 {
		v := make([]time.Duration, len(e.ticks))
		for i, s := range e.ticks {
			v[i] = s.off
		}
		return median(v), SourceTicks, len(v)
	}
	if len(e.http) > 0 {
		v := append([]time.Duration(nil), e.http...)
		return median(v), SourceHTTPDate, len(v)
	}
	return 0, SourceNone, 0
}

func median(v []time.Duration) time.Duration {
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	n := len(v)
	if n%2 == 1 {
		return v[n/2]
	}
	return (v[n/2-1] + v[n/2]) / 2
}
```

`internal/clock/real.go`:

```go
package clock

import (
	"log/slog"
	"sync"
	"time"

	"tradeloom/internal/bus"
)

// TopicOffset carries every significant offset change (ARCHITECTURE §3.4 "clock.offset").
var TopicOffset = bus.NewTopic[Offset]("clock.offset")

// Options configures Real. Zero values take the FR-DATA-004 defaults.
type Options struct {
	Bus         *bus.Bus     // nil: no events
	Log         *slog.Logger // nil: slog.Default()
	System      func() time.Time
	TickWindow  time.Duration // default 60 s
	HTTPSamples int           // default 5
	WarnAbove   time.Duration // default 1 s
	SkewAbove   time.Duration // default 5 s
	ClearBelow  time.Duration // default 2 s
	Rearm       time.Duration // default 100 ms: offset moves larger than this re-arm timers
}

// Real is the production Clock. It also receives offset samples from the ticker manager
// (ObserveTick) and from the Kite HTTP transport (ObserveHTTPDate).
type Real struct {
	o      Options
	mu     sync.Mutex
	est    estimator
	off    Offset
	acked  bool
	timers map[*realTimer]struct{}
}

// New returns a Real clock with zero offset (source none).
func New(o Options) *Real {
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.System == nil {
		o.System = SystemNow
	}
	if o.TickWindow == 0 {
		o.TickWindow = 60 * time.Second
	}
	if o.HTTPSamples == 0 {
		o.HTTPSamples = 5
	}
	if o.WarnAbove == 0 {
		o.WarnAbove = time.Second
	}
	if o.SkewAbove == 0 {
		o.SkewAbove = 5 * time.Second
	}
	if o.ClearBelow == 0 {
		o.ClearBelow = 2 * time.Second
	}
	if o.Rearm == 0 {
		o.Rearm = 100 * time.Millisecond
	}
	return &Real{
		o:      o,
		est:    estimator{window: o.TickWindow, httpN: o.HTTPSamples},
		off:    Offset{Source: SourceNone},
		timers: map[*realTimer]struct{}{},
	}
}

func (r *Real) Now() time.Time {
	r.mu.Lock()
	v := r.off.Value
	r.mu.Unlock()
	return r.o.System().Add(v).In(IST)
}

func (r *Real) System() time.Time { return r.o.System().In(IST) }

func (r *Real) Offset() Offset {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.off
}

// ObserveTick records one exchange timestamp (models.Tick.Timestamp of the clock reference
// token) and its system arrival time.
func (r *Real) ObserveTick(exchangeTS, arrival time.Time) {
	if exchangeTS.IsZero() {
		return
	}
	r.mu.Lock()
	r.est.addTick(exchangeTS, arrival)
	r.mu.Unlock()
	r.recompute()
}

// ObserveHTTPDate records a Kite HTTP Date header with the system times the request was sent
// and the response received.
func (r *Real) ObserveHTTPDate(date, sent, received time.Time) {
	r.mu.Lock()
	r.est.addHTTP(date, sent, received)
	r.mu.Unlock()
	r.recompute()
}

// Acknowledge clears Skewed until the offset next returns below ClearBelow and rises again.
func (r *Real) Acknowledge() {
	r.mu.Lock()
	r.acked = true
	r.mu.Unlock()
	r.recompute()
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func (r *Real) recompute() {
	sys := r.o.System()
	r.mu.Lock()
	v, src, n := r.est.estimate(sys)
	prev := r.off
	a := abs(v)
	if a < r.o.ClearBelow {
		r.acked = false
	}
	skewed := prev.Skewed
	if a > r.o.SkewAbove {
		skewed = true
	}
	if a < r.o.ClearBelow || r.acked {
		skewed = false
	}
	next := Offset{
		Value: v, Millis: v.Milliseconds(), Source: src, Samples: n,
		Warn: a > r.o.WarnAbove, Skewed: skewed, UpdatedAt: sys.Add(v).In(IST),
	}
	r.off = next
	significant := abs(v-prev.Value) > r.o.Rearm || src != prev.Source ||
		next.Warn != prev.Warn || next.Skewed != prev.Skewed
	var ts []*realTimer
	if significant {
		for t := range r.timers {
			ts = append(ts, t)
		}
	}
	r.mu.Unlock()
	if !significant {
		return
	}
	for _, t := range ts {
		t.arm()
	}
	switch {
	case next.Skewed && !prev.Skewed:
		r.o.Log.Error("clock out of sync; live evaluation must suspend", "offset_ms", next.Millis, "source", string(src))
	case next.Warn && !prev.Warn:
		r.o.Log.Warn("clock offset above 1 s", "offset_ms", next.Millis, "source", string(src))
	default:
		r.o.Log.Info("clock offset", "offset_ms", next.Millis, "source", string(src), "samples", n)
	}
	publish(r.o.Bus, TopicOffset, next)
}

func (r *Real) At(target time.Time) Timer {
	t := &realTimer{r: r, target: target, ch: make(chan time.Time, 1)}
	r.mu.Lock()
	r.timers[t] = struct{}{}
	r.mu.Unlock()
	t.arm()
	return t
}

func (r *Real) After(d time.Duration) Timer { return r.At(r.Now().Add(d)) }

func (r *Real) forget(t *realTimer) {
	r.mu.Lock()
	delete(r.timers, t)
	r.mu.Unlock()
}

type realTimer struct {
	r      *Real
	target time.Time
	ch     chan time.Time
	mu     sync.Mutex
	af     *time.Timer
	done   bool
}

func (t *realTimer) C() <-chan time.Time { return t.ch }

// arm (re)schedules the timer for target − Now(); called on creation and on offset moves.
func (t *realTimer) arm() {
	d := t.target.Sub(t.r.Now())
	if d < 0 {
		d = 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done {
		return
	}
	if t.af != nil {
		t.af.Stop()
	}
	t.af = time.AfterFunc(d, t.check)
}

func (t *realTimer) check() {
	now := t.r.Now()
	if now.Before(t.target) { // the offset moved backwards since arming
		t.arm()
		return
	}
	t.mu.Lock()
	if t.done {
		t.mu.Unlock()
		return
	}
	t.done = true
	t.mu.Unlock()
	t.r.forget(t)
	select {
	case t.ch <- now:
	default:
	}
}

func (t *realTimer) Stop() bool {
	t.mu.Lock()
	if t.done {
		t.mu.Unlock()
		return false
	}
	t.done = true
	if t.af != nil {
		t.af.Stop()
	}
	t.mu.Unlock()
	t.r.forget(t)
	return true
}

// publish sends v on t when a bus is configured (tests and tools may run without one).
func publish[T any](b *bus.Bus, t bus.Topic[T], v T) {
	if b != nil {
		bus.Publish(b, t, v)
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/clock/`
Expected: `ok  	tradeloom/internal/clock`; no `DATA RACE`.

- [ ] **Step 5: Commit**

```bash
git add internal/clock/estimator.go \
  internal/clock/real.go \
  internal/clock/real_test.go
git commit -m "$(cat <<'EOF'
feat(clock): offset estimator, skew hysteresis and re-arming real clock

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
