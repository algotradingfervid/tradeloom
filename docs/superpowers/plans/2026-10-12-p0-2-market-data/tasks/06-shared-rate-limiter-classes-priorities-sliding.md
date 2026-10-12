> **P0-2 Time & Market Data Implementation Plan — Task 6.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 6: Shared rate limiter: classes, priorities, sliding window, 429 back-off, pause

**Files:**
- Create: `internal/kite/ratelimit/types.go`
- Create: `internal/kite/ratelimit/sched.go`
- Create: `internal/kite/ratelimit/limiter.go`
- Test: `internal/kite/ratelimit/sched_test.go`
- Test: `internal/kite/ratelimit/limiter_test.go`

**Interfaces:**
- Consumes: Task 1: `clock.Clock`, `clock.Fake`, `clock.Sleep`. P0-1: `bus.*`, `logging.Go`.
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
type Class uint8
const (
	ClassQuote      Class = iota // /quote, /quote/ltp, /quote/ohlc — 1 req/s
	ClassHistorical              // /instruments/historical — 3 req/s
	ClassOrders                  // order placement — 10 req/s at Kite; app default 8 (FR-ORD-006)
	ClassOther                   // everything else — 10 req/s
	numClasses
)
func (c Class) String() string
type Priority uint8
const (
	Interactive Priority = iota
	Deployment
	LiveScanner
	Backtest
	Recorder
	Library
	NumPriorities
)
func (p Priority) String() string
var DefaultRates = map[Class]int{ClassQuote: 1, ClassHistorical: 3, ClassOrders: 8, ClassOther: 10}
func WithPriority(ctx context.Context, p Priority) context.Context
func PriorityFrom(ctx context.Context) Priority
var TopicUsage = bus.NewTopic[Usage]("limits.usage")
type ClassUsage struct {
	Class        string             `json:"class"`
	RatePerSec   int                `json:"ratePerSec"`
	Queued       [NumPriorities]int `json:"queued"` // by Priority
	HTTP429      int64              `json:"http429"`
	BackoffUntil time.Time          `json:"backoffUntil"`
}
type Usage struct {
	At          time.Time    `json:"at"`
	Paused      bool         `json:"paused"`
	PauseReason string       `json:"pauseReason"`
	Classes     []ClassUsage `json:"classes"`
}
type Options struct {
	Rates  map[Class]int // nil: DefaultRates
	Margin time.Duration // added to the 1 s window; default 20 ms
	Bus    *bus.Bus
	Log    *slog.Logger
}
type Limiter struct{ /* unexported fields */ }
func New(clk clock.Clock, o Options) *Limiter
func (l *Limiter) Run(ctx context.Context)
func (l *Limiter) Acquire(ctx context.Context, c Class, p Priority) error
func (l *Limiter) Do(ctx context.Context, c Class, p Priority, fn func() error) error
func (l *Limiter) Backoff(c Class)
func (l *Limiter) Success(c Class)
func (l *Limiter) PauseAll(reason string)
func (l *Limiter) Resume()
func (l *Limiter) Snapshot() Usage
```

FR-DATA-003 AC1 (interactive before queued backtests; never more than 3 historical requests in any 1-second window), AC2 (429 back-off on one class only, counter visible), AC4 (all queues pause). The admission logic (`classState.step`) is pure and tested with explicit times; the `Limiter` wraps it with one goroutine per class and a 1 Hz `limits.usage` event.

- [ ] **Step 1: Write the failing tests**

`internal/kite/ratelimit/sched_test.go`:

```go
package ratelimit

import (
	"container/heap"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)

func push(c *classState, p Priority, seq uint64) *waiter {
	w := &waiter{prio: p, seq: seq, ready: make(chan struct{})}
	heap.Push(&c.q, w)
	return w
}

func TestSlidingWindowNeverExceedsRate(t *testing.T) {
	c := newClassState(3, time.Second)
	for i := 0; i < 20; i++ {
		push(c, Backtest, uint64(i))
	}
	var grants []time.Time
	now := t0
	for len(grants) < 20 {
		w, wait := c.step(now, false)
		if w != nil {
			grants = append(grants, now)
			continue
		}
		now = now.Add(wait)
	}
	for i := range grants {
		n := 0
		for _, g := range grants {
			if !g.Before(grants[i]) && g.Before(grants[i].Add(time.Second)) {
				n++
			}
		}
		if n > 3 {
			t.Fatalf("window starting %v holds %d grants", grants[i].Sub(t0), n)
		}
	}
	if got := grants[19].Sub(t0); got != 6*time.Second {
		t.Fatalf("20 grants at 3/s should finish at +6s, finished at +%v", got)
	}
}

func TestPriorityThenFIFO(t *testing.T) {
	c := newClassState(3, time.Second)
	for i := 0; i < 10; i++ {
		push(c, Backtest, uint64(i))
	}
	push(c, Library, 100)
	inter := push(c, Interactive, 200)
	w, _ := c.step(t0, false)
	if w != inter {
		t.Fatal("interactive must be granted before queued backtests")
	}
	w, _ = c.step(t0, false)
	if w.prio != Backtest || w.seq != 0 {
		t.Fatalf("then oldest backtest, got %v seq %d", w.prio, w.seq)
	}
}

func TestBackoffDoublesToThirtySeconds(t *testing.T) {
	c := newClassState(3, time.Second)
	want := []time.Duration{1, 2, 4, 8, 16, 30, 30}
	for i, s := range want {
		if d := c.backoff(t0); d != s*time.Second {
			t.Fatalf("backoff %d = %v, want %v", i, d, s*time.Second)
		}
	}
	if c.http429 != 7 {
		t.Fatalf("http429 = %d", c.http429)
	}
}

func TestPausedGrantsNothing(t *testing.T) {
	c := newClassState(3, time.Second)
	push(c, Interactive, 1)
	if w, wait := c.step(t0, true); w != nil || wait != -1 {
		t.Fatal("paused class granted")
	}
}
```

`internal/kite/ratelimit/limiter_test.go`:

```go
package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/clock"
)

type grant struct {
	p  Priority
	at time.Time
}

func waitQueued(t *testing.T, l *Limiter, c Class, p Priority, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if l.Snapshot().Classes[c].Queued[p] == n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("never saw %d queued at %v (snapshot %+v)", n, p, l.Snapshot().Classes[c])
}

// FR-DATA-003 AC1: 10 backtest requests queued, then one interactive: the interactive request is
// served before the next backtest request and no 1-second window has more than 3 historical grants.
func TestInteractiveOvertakesQueuedBacktestsAndRateHolds(t *testing.T) {
	fc := clock.NewFake(time.Date(2026, 10, 12, 10, 0, 0, 0, clock.IST))
	l := New(fc, Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.Run(ctx)

	var mu sync.Mutex
	var grants []grant
	var wg sync.WaitGroup
	acquire := func(p Priority) {
		defer wg.Done()
		if err := l.Acquire(ctx, ClassHistorical, p); err != nil {
			t.Error(err)
			return
		}
		mu.Lock()
		grants = append(grants, grant{p, fc.Now()})
		mu.Unlock()
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go acquire(Backtest)
	}
	waitQueued(t, l, ClassHistorical, Backtest, 7) // three granted at once, seven wait
	wg.Add(1)
	go acquire(Interactive)
	waitQueued(t, l, ClassHistorical, Interactive, 1)
	for i := 0; i < 10; i++ {
		fc.Advance(250 * time.Millisecond)
		time.Sleep(5 * time.Millisecond)
	}
	for l.Snapshot().Classes[ClassHistorical].Queued[Backtest] > 0 {
		fc.Advance(250 * time.Millisecond)
		time.Sleep(5 * time.Millisecond)
	}
	wg.Wait()

	if len(grants) != 11 {
		t.Fatalf("grants = %d", len(grants))
	}
	// Goroutines append in scheduling order, so compare grant instants: exactly the initial
	// burst of three backtests may be granted strictly before the interactive request.
	var interAt time.Time
	for _, g := range grants {
		if g.p == Interactive {
			interAt = g.at
		}
	}
	before := 0
	for _, g := range grants {
		if g.p == Backtest && g.at.Before(interAt) {
			before++
		}
	}
	if before != 3 {
		t.Fatalf("%d backtest grants before the interactive one, want 3 (the initial burst)", before)
	}
	for i := range grants {
		n := 0
		for _, g := range grants {
			if !g.at.Before(grants[i].at) && g.at.Before(grants[i].at.Add(time.Second)) {
				n++
			}
		}
		if n > 3 {
			t.Fatalf("1-second window from grant %d holds %d historical grants", i, n)
		}
	}
}

// FR-DATA-003 AC2: a 429 on historical delays the next historical request ≥ 1 s; quote and
// orders are unaffected; the 429 counter is visible.
func TestBackoffIsolatedToClass(t *testing.T) {
	fc := clock.NewFake(time.Date(2026, 10, 12, 10, 0, 0, 0, clock.IST))
	l := New(fc, Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.Run(ctx)

	l.Backoff(ClassHistorical)
	start := fc.Now()
	if err := l.Acquire(ctx, ClassQuote, Interactive); err != nil {
		t.Fatal(err)
	}
	if err := l.Acquire(ctx, ClassOrders, Interactive); err != nil {
		t.Fatal(err)
	}
	if !fc.Now().Equal(start) {
		t.Fatal("quote/orders waited on a historical back-off")
	}
	done := make(chan time.Time, 1)
	go func() {
		if err := l.Acquire(ctx, ClassHistorical, Interactive); err == nil {
			done <- fc.Now()
		}
	}()
	waitQueued(t, l, ClassHistorical, Interactive, 1)
	fc.Advance(999 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("historical granted before 1 s back-off elapsed")
	case <-time.After(20 * time.Millisecond):
	}
	fc.Advance(time.Millisecond)
	select {
	case at := <-done:
		if at.Sub(start) < time.Second {
			t.Fatalf("granted after %v", at.Sub(start))
		}
	case <-time.After(time.Second):
		t.Fatal("historical never granted after back-off")
	}
	if got := l.Snapshot().Classes[ClassHistorical].HTTP429; got != 1 {
		t.Fatalf("http429 = %d", got)
	}
}

// FR-DATA-003 AC4 (limiter side): PauseAll stops every class until Resume.
func TestPauseAllAndResume(t *testing.T) {
	fc := clock.NewFake(time.Date(2026, 10, 12, 10, 0, 0, 0, clock.IST))
	l := New(fc, Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.Run(ctx)
	l.PauseAll("session expired")
	got := make(chan Class, 4)
	for c := Class(0); c < numClasses; c++ {
		c := c
		go func() {
			if l.Acquire(ctx, c, Interactive) == nil {
				got <- c
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	if len(got) != 0 {
		t.Fatal("a class granted while paused")
	}
	if u := l.Snapshot(); !u.Paused || u.PauseReason != "session expired" {
		t.Fatalf("snapshot %+v", u)
	}
	l.Resume()
	for i := 0; i < int(numClasses); i++ {
		select {
		case <-got:
		case <-time.After(time.Second):
			t.Fatal("class not resumed")
		}
	}
}

func TestCancelledWaiterLeavesQueue(t *testing.T) {
	fc := clock.NewFake(time.Date(2026, 10, 12, 10, 0, 0, 0, clock.IST))
	l := New(fc, Options{})
	l.PauseAll("test")
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- l.Acquire(ctx, ClassQuote, Backtest) }()
	waitQueued(t, l, ClassQuote, Backtest, 1)
	cancel()
	if err := <-errc; err != context.Canceled {
		t.Fatalf("err = %v", err)
	}
	if n := l.Snapshot().Classes[ClassQuote].Queued[Backtest]; n != 0 {
		t.Fatalf("queued after cancel = %d", n)
	}
}

func TestPriorityContext(t *testing.T) {
	if PriorityFrom(context.Background()) != Interactive {
		t.Fatal("default must be Interactive")
	}
	if PriorityFrom(WithPriority(context.Background(), Backtest)) != Backtest {
		t.Fatal("WithPriority lost")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/kite/ratelimit/`
Expected: the build fails — output contains `undefined: Limiter` and `[build failed]`.

- [ ] **Step 3: Write the implementation**

`internal/kite/ratelimit/types.go`:

```go
// Package ratelimit is the shared Kite rate limiter (FR-DATA-003, ARCHITECTURE §5.4): one
// sliding-window admission per endpoint class, priority queues with FIFO inside a priority,
// per-class 429 back-off, global pause on session expiry, and 1 Hz usage events.
package ratelimit

import (
	"context"
	"fmt"
)

// Class is a Kite endpoint class with its own rate (A07 §A3).
type Class uint8

const (
	ClassQuote      Class = iota // /quote, /quote/ltp, /quote/ohlc — 1 req/s
	ClassHistorical              // /instruments/historical — 3 req/s
	ClassOrders                  // order placement — 10 req/s at Kite; app default 8 (FR-ORD-006)
	ClassOther                   // everything else — 10 req/s
	numClasses
)

var classNames = [numClasses]string{"quote", "historical", "orders", "other"}

func (c Class) String() string {
	if c < numClasses {
		return classNames[c]
	}
	return fmt.Sprintf("class(%d)", c)
}

// Priority orders requests inside a class; lower value is served first (FR-DATA-003).
type Priority uint8

const (
	Interactive Priority = iota
	Deployment
	LiveScanner
	Backtest
	Recorder
	Library
	NumPriorities
)

var prioNames = [NumPriorities]string{"interactive", "deployment", "live-scanner", "backtest", "recorder", "library"}

func (p Priority) String() string {
	if p < NumPriorities {
		return prioNames[p]
	}
	return fmt.Sprintf("priority(%d)", p)
}

// DefaultRates are requests per second per class.
var DefaultRates = map[Class]int{ClassQuote: 1, ClassHistorical: 3, ClassOrders: 8, ClassOther: 10}

type ctxKey struct{}

// WithPriority tags ctx so kite.Client calls made with it queue at p.
func WithPriority(ctx context.Context, p Priority) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// PriorityFrom returns the priority carried by ctx; Interactive when none.
func PriorityFrom(ctx context.Context) Priority {
	if p, ok := ctx.Value(ctxKey{}).(Priority); ok {
		return p
	}
	return Interactive
}
```

`internal/kite/ratelimit/sched.go`:

```go
package ratelimit

import (
	"container/heap"
	"time"
)

// waiter is one queued request.
type waiter struct {
	prio  Priority
	seq   uint64
	ready chan struct{}
	index int // heap index; -1 once popped or removed
}

type waitHeap []*waiter

func (h waitHeap) Len() int { return len(h) }
func (h waitHeap) Less(i, j int) bool {
	if h[i].prio != h[j].prio {
		return h[i].prio < h[j].prio
	}
	return h[i].seq < h[j].seq
}
func (h waitHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i]; h[i].index = i; h[j].index = j }
func (h *waitHeap) Push(x any)   { w := x.(*waiter); w.index = len(*h); *h = append(*h, w) }
func (h *waitHeap) Pop() any {
	old := *h
	n := len(old)
	w := old[n-1]
	old[n-1] = nil
	w.index = -1
	*h = old[:n-1]
	return w
}

// classState is the admission state of one class. It holds no lock and no goroutine; the
// Limiter guards it and drives it, and tests drive it directly with explicit times.
type classState struct {
	rate         int
	span         time.Duration
	grants       []time.Time // ring of the last `rate` grant times
	next         int
	count        int
	q            waitHeap
	backoffUntil time.Time
	backoffN     int
	http429      int64
	wake         chan struct{}
}

func newClassState(rate int, span time.Duration) *classState {
	return &classState{rate: rate, span: span, grants: make([]time.Time, rate), wake: make(chan struct{}, 1)}
}

// nextAllowed is the earliest instant a new grant keeps every span-long window at ≤ rate grants
// and respects the 429 back-off.
func (c *classState) nextAllowed() time.Time {
	t := c.backoffUntil
	if c.count == c.rate {
		if a := c.grants[c.next].Add(c.span); a.After(t) {
			t = a
		}
	}
	return t
}

// step grants the head waiter if allowed at now. It returns the granted waiter (nil if none) and,
// when the head must wait, how long.
func (c *classState) step(now time.Time, paused bool) (*waiter, time.Duration) {
	if paused || c.q.Len() == 0 {
		return nil, -1
	}
	if na := c.nextAllowed(); na.After(now) {
		return nil, na.Sub(now)
	}
	w := heap.Pop(&c.q).(*waiter)
	c.grants[c.next] = now
	c.next = (c.next + 1) % c.rate
	if c.count < c.rate {
		c.count++
	}
	return w, 0
}

// backoff applies the FR-DATA-003 429 policy: 1 s × 2ⁿ, capped at 30 s.
func (c *classState) backoff(now time.Time) time.Duration {
	d := time.Second << c.backoffN
	if d > 30*time.Second || d <= 0 {
		d = 30 * time.Second
	} else {
		c.backoffN++
	}
	c.backoffUntil = now.Add(d)
	c.http429++
	return d
}

func (c *classState) poke() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}
```

`internal/kite/ratelimit/limiter.go`:

```go
package ratelimit

import (
	"container/heap"
	"context"
	"log/slog"
	"sync"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/clock"
	"tradeloom/internal/logging"
)

// TopicUsage carries the 1 Hz usage snapshot (ARCHITECTURE §3.4 "limits.usage").
var TopicUsage = bus.NewTopic[Usage]("limits.usage")

// ClassUsage is one class's queue and back-off state.
type ClassUsage struct {
	Class        string             `json:"class"`
	RatePerSec   int                `json:"ratePerSec"`
	Queued       [NumPriorities]int `json:"queued"` // by Priority
	HTTP429      int64              `json:"http429"`
	BackoffUntil time.Time          `json:"backoffUntil"`
}

// Usage is the snapshot shown in Settings → Limits (FR-ACCT-009).
type Usage struct {
	At          time.Time    `json:"at"`
	Paused      bool         `json:"paused"`
	PauseReason string       `json:"pauseReason"`
	Classes     []ClassUsage `json:"classes"`
}

// Options configures a Limiter.
type Options struct {
	Rates  map[Class]int // nil: DefaultRates
	Margin time.Duration // added to the 1 s window; default 20 ms
	Bus    *bus.Bus
	Log    *slog.Logger
}

// Limiter is shared by every Kite HTTP call.
type Limiter struct {
	clk    clock.Clock
	o      Options
	mu     sync.Mutex
	cls    [numClasses]*classState
	seq    uint64
	paused bool
	reason string
}

// New builds a Limiter; call Run to start its schedulers.
func New(clk clock.Clock, o Options) *Limiter {
	if o.Rates == nil {
		o.Rates = DefaultRates
	}
	if o.Margin == 0 {
		o.Margin = 20 * time.Millisecond
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	l := &Limiter{clk: clk, o: o}
	for c := Class(0); c < numClasses; c++ {
		r := o.Rates[c]
		if r <= 0 {
			r = DefaultRates[c]
		}
		l.cls[c] = newClassState(r, time.Second+o.Margin)
	}
	return l
}

// Run starts one scheduler goroutine per class and the 1 Hz usage publisher; they stop with ctx.
func (l *Limiter) Run(ctx context.Context) {
	for c := Class(0); c < numClasses; c++ {
		c := c
		logging.Go("ratelimit."+c.String(), func() { l.loop(ctx, c) })
	}
	logging.Go("ratelimit.usage", func() {
		for {
			if err := clock.Sleep(ctx, l.clk, time.Second); err != nil {
				return
			}
			publish(l.o.Bus, TopicUsage, l.Snapshot())
		}
	})
}

// Acquire blocks until class c grants a slot to a request of priority p, or ctx ends.
func (l *Limiter) Acquire(ctx context.Context, c Class, p Priority) error {
	if p >= NumPriorities {
		p = NumPriorities - 1
	}
	w := &waiter{prio: p, ready: make(chan struct{})}
	cs := l.cls[c]
	l.mu.Lock()
	l.seq++
	w.seq = l.seq
	heap.Push(&cs.q, w)
	l.mu.Unlock()
	cs.poke()
	select {
	case <-w.ready:
		return nil
	case <-ctx.Done():
		l.mu.Lock()
		if w.index >= 0 {
			heap.Remove(&cs.q, w.index)
		}
		l.mu.Unlock()
		return ctx.Err()
	}
}

// Do acquires a slot and runs fn.
func (l *Limiter) Do(ctx context.Context, c Class, p Priority, fn func() error) error {
	if err := l.Acquire(ctx, c, p); err != nil {
		return err
	}
	return fn()
}

// Backoff records an HTTP 429 on class c and delays only that class (FR-DATA-003 AC2).
func (l *Limiter) Backoff(c Class) {
	l.mu.Lock()
	d := l.cls[c].backoff(l.clk.Now())
	n := l.cls[c].http429
	l.mu.Unlock()
	l.o.Log.Info("kite rate limited", "class", c.String(), "backoff", d.String(), "http429_total", n)
}

// Success resets class c's back-off exponent after a successful response.
func (l *Limiter) Success(c Class) {
	l.mu.Lock()
	l.cls[c].backoffN = 0
	l.mu.Unlock()
}

// PauseAll stops every class from granting (TokenException, FR-DATA-003 AC4).
func (l *Limiter) PauseAll(reason string) {
	l.mu.Lock()
	already := l.paused
	l.paused, l.reason = true, reason
	l.mu.Unlock()
	if !already {
		l.o.Log.Info("kite queues paused", "reason", reason)
	}
}

// Resume lets every class grant again (after login).
func (l *Limiter) Resume() {
	l.mu.Lock()
	l.paused, l.reason = false, ""
	l.mu.Unlock()
	for _, cs := range l.cls {
		cs.poke()
	}
	l.o.Log.Info("kite queues resumed")
}

// Snapshot returns current usage.
func (l *Limiter) Snapshot() Usage {
	l.mu.Lock()
	defer l.mu.Unlock()
	u := Usage{At: l.clk.Now(), Paused: l.paused, PauseReason: l.reason}
	for c := Class(0); c < numClasses; c++ {
		cs := l.cls[c]
		cu := ClassUsage{Class: c.String(), RatePerSec: cs.rate, HTTP429: cs.http429, BackoffUntil: cs.backoffUntil}
		for _, w := range cs.q {
			cu.Queued[w.prio]++
		}
		u.Classes = append(u.Classes, cu)
	}
	return u
}

func (l *Limiter) loop(ctx context.Context, c Class) {
	cs := l.cls[c]
	for {
		l.mu.Lock()
		w, wait := cs.step(l.clk.Now(), l.paused)
		l.mu.Unlock()
		if w != nil {
			close(w.ready)
			continue
		}
		var tc <-chan time.Time
		var t clock.Timer
		if wait > 0 {
			t = l.clk.After(wait)
			tc = t.C()
		}
		select {
		case <-ctx.Done():
			if t != nil {
				t.Stop()
			}
			return
		case <-cs.wake:
		case <-tc:
		}
		if t != nil {
			t.Stop()
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

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/kite/ratelimit/`
Expected: `ok  	tradeloom/internal/kite/ratelimit`; no `DATA RACE`.

- [ ] **Step 5: Commit**

```bash
git add internal/kite/ratelimit/types.go \
  internal/kite/ratelimit/sched.go \
  internal/kite/ratelimit/limiter.go \
  internal/kite/ratelimit/sched_test.go \
  internal/kite/ratelimit/limiter_test.go
git commit -m "$(cat <<'EOF'
feat(kite/ratelimit): per-class sliding-window limiter with priorities, back-off and pause

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
