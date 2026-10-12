> **P0-2 Time & Market Data Implementation Plan — Task 17.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 17: Ticker manager: budget, consumers, modes, reconnect, fan-out, clock feed

**Files:**
- Create: `internal/kite/ticker/types.go`
- Create: `internal/kite/ticker/conn.go`
- Create: `internal/kite/ticker/manager.go`
- Test: `internal/kite/ticker/manager_test.go`

**Interfaces:**
- Consumes: Task 1/2: `clock.Clock`, `clock.Fake`, `clock.Sleep`, `(*clock.Real).ObserveTick` (as `TickObserver`). Task 7: `kite.TopicOrderUpdate`. P0-1: `bus.*`, `logging.Go`.
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
type Mode uint8
const (
	ModeLTP   Mode = 1
	ModeQuote Mode = 2
	ModeFull  Mode = 3 // the only mode carrying the exchange timestamp (clock samples)
)
func (m Mode) String() string
type Consumer uint8
const (
	ConsumerClock Consumer = iota // the clock reference token (manager-owned)
	ConsumerDeployment
	ConsumerLiveScanner
	ConsumerChart
	ConsumerWatchlist
	ConsumerWidget
	ConsumerRecorder
)
func (c Consumer) String() string
type Tick struct {
	Token         uint32    `json:"token"`
	Mode          string    `json:"mode"`
	LTP           float64   `json:"ltp"`
	Change        float64   `json:"change"` // LTP − previous close (Kite net_change)
	Open          float64   `json:"open"`
	High          float64   `json:"high"`
	Low           float64   `json:"low"`
	Close         float64   `json:"close"`  // previous session close
	Volume        uint32    `json:"volume"` // cumulative day volume
	LastQty       uint32    `json:"lastQty"`
	OI            uint32    `json:"oi"`
	ExchangeTS    time.Time `json:"exchangeTs"`    // zero unless full mode
	LastTradeTime time.Time `json:"lastTradeTime"` // zero unless full mode (equities)
	Received      time.Time `json:"received"`      // app clock at arrival
	IsIndex       bool      `json:"isIndex"`
}
type ErrBudget struct {
	Used, Requested, Limit int
	ByConsumer             map[string]int
}
func (e *ErrBudget) Error() string
type FeedState struct {
	State       string    `json:"state"`
	Attempt     int       `json:"attempt"`
	NextRetry   time.Time `json:"nextRetry"`
	LastTick    time.Time `json:"lastTick"`
	Connections int       `json:"connections"`
	Tokens      int       `json:"tokens"`
}
type Usage struct {
	Used        int            `json:"used"`
	Limit       int            `json:"limit"`
	Connections int            `json:"connections"`
	ByConsumer  map[string]int `json:"byConsumer"`
	RawDrops    uint64         `json:"rawDrops"`
}
var (
	// TopicTicks is "tick.batch": latest tick per token, at most every Coalesce interval.
	TopicTicks = bus.NewTopic[[]Tick]("tick.batch")
	// TopicFeed is "feed.state".
	TopicFeed = bus.NewTopic[FeedState]("feed.state")
)
type Events struct {
	OnConnect     func()
	OnTick        func(models.Tick)
	OnOrderUpdate func(kiteconnect.Order)
	OnError       func(error)
}
type Conn interface {
	Serve(ctx context.Context)
	Subscribe(tokens []uint32) error
	Unsubscribe(tokens []uint32) error
	SetMode(m Mode, tokens []uint32) error
	Stop()
}
type Dialer func(ev Events) Conn
type TickObserver interface {
	ObserveTick(exchangeTS, arrival time.Time)
}
type Options struct {
	Dial        Dialer // required
	Clock       clock.Clock
	Bus         *bus.Bus
	Log         *slog.Logger
	PerConn     int                                 // [3000] tokens per connection (A07 §A6)
	MaxConns    int                                 // [3] connections per API key
	ClockRef    uint32                              // token subscribed in full mode as the clock source (NIFTY 50); 0 = none
	Observer    TickObserver                        // receives ClockRef timestamps
	Coalesce    time.Duration                       // [100 ms] tick.batch interval (S0-6 confirms)
	BaseDelay   time.Duration                       // [2 s] first reconnect delay
	MaxDelay    time.Duration                       // [60 s]
	StableAfter time.Duration                       // [30 s] a connection up this long resets the attempt counter
	MaxAttempts int                                 // [50] consecutive failures before Offline
	Jitter      func(d time.Duration) time.Duration // [±20 %]
}
type Manager struct{ /* unexported fields */ }
func New(o Options) *Manager
func (m *Manager) Start(ctx context.Context) error
func (m *Manager) Stop()
func (m *Manager) Reconnect()
func (m *Manager) Subscribe(c Consumer, owner string, tokens []uint32, mode Mode) error
func (m *Manager) Unsubscribe(c Consumer, owner string, tokens []uint32)
func (m *Manager) Release(c Consumer, owner string)
func (m *Manager) Usage() Usage
func (m *Manager) Raw(buffer int) (<-chan Tick, func())
```

C-07 and ARCHITECTURE §5.6: up to three connections, sticky first-fit assignment, union of consumer modes, all-or-nothing budget refusal with a per-consumer breakdown (never evicting), own reconnect with exponential back-off and full resubscribe, `feed.state` events, 100 ms coalesced `tick.batch`, raw per-tick stream for the builder and live checks, exchange timestamps of the clock reference token (subscribed in full mode — the only mode carrying them) fed to the app clock.

- [ ] **Step 1: Write the failing tests**

`internal/kite/ticker/manager_test.go`:

```go
package ticker

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
	"github.com/zerodha/gokiteconnect/v4/models"

	"tradeloom/internal/bus"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
)

// fakeConn records writes; Serve connects (unless failing) and blocks until dropped.
type fakeConn struct {
	ev   Events
	fail bool
	mu   sync.Mutex
	ops  []string
	drop chan struct{}
}

func (f *fakeConn) Serve(ctx context.Context) {
	if f.fail {
		return
	}
	f.ev.OnConnect()
	select {
	case <-ctx.Done():
	case <-f.drop:
	}
}
func (f *fakeConn) log(op string, ts []uint32) error {
	s := append([]uint32(nil), ts...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	f.mu.Lock()
	f.ops = append(f.ops, fmt.Sprint(op, s))
	f.mu.Unlock()
	return nil
}
func (f *fakeConn) Subscribe(ts []uint32) error       { return f.log("sub", ts) }
func (f *fakeConn) Unsubscribe(ts []uint32) error     { return f.log("unsub", ts) }
func (f *fakeConn) SetMode(m Mode, ts []uint32) error { return f.log(m.String(), ts) }
func (f *fakeConn) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	select {
	case <-f.drop:
	default:
		close(f.drop)
	}
}
func (f *fakeConn) Ops() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.ops...)
}

type dialer struct {
	mu       sync.Mutex
	conns    []*fakeConn
	failNext int
}

func (d *dialer) dial(ev Events) Conn {
	d.mu.Lock()
	defer d.mu.Unlock()
	c := &fakeConn{ev: ev, drop: make(chan struct{})}
	if d.failNext > 0 {
		d.failNext--
		c.fail = true
	}
	d.conns = append(d.conns, c)
	return c
}

func (d *dialer) n() int              { d.mu.Lock(); defer d.mu.Unlock(); return len(d.conns) }
func (d *dialer) get(i int) *fakeConn { d.mu.Lock(); defer d.mu.Unlock(); return d.conns[i] }

func eventually(t *testing.T, what string, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !f() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

type rig struct {
	m   *Manager
	d   *dialer
	clk *clock.Fake
	bus *bus.Bus
}

func newRig(t *testing.T, o Options) *rig {
	t.Helper()
	r := &rig{d: &dialer{}, clk: clock.NewFake(time.Date(2026, 10, 12, 10, 0, 0, 0, clock.IST)), bus: bus.New(nil)}
	o.Dial, o.Clock, o.Bus = r.d.dial, r.clk, r.bus
	if o.Jitter == nil {
		o.Jitter = func(d time.Duration) time.Duration { return d }
	}
	r.m = New(o)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := r.m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	eventually(t, "first connection", func() bool { return r.d.n() >= 1 })
	return r
}

func TestBudgetRefusesWithBreakdownAndNeverEvicts(t *testing.T) { // C-07
	r := newRig(t, Options{PerConn: 3, MaxConns: 2})
	if err := r.m.Subscribe(ConsumerDeployment, "d1", []uint32{1, 2, 3, 4}, ModeFull); err != nil {
		t.Fatal(err)
	}
	eventually(t, "second connection", func() bool { return r.d.n() == 2 })
	if err := r.m.Subscribe(ConsumerWatchlist, "w1", []uint32{5, 6}, ModeQuote); err != nil {
		t.Fatal(err)
	}
	err := r.m.Subscribe(ConsumerDeployment, "d2", []uint32{7}, ModeFull)
	var eb *ErrBudget
	if !errors.As(err, &eb) || eb.Used != 6 || eb.Requested != 1 || eb.Limit != 6 || eb.ByConsumer["deployment"] != 4 || eb.ByConsumer["watchlist"] != 2 {
		t.Fatalf("err = %v", err)
	}
	if u := r.m.Usage(); u.Used != 6 || u.Connections != 2 {
		t.Fatalf("usage after refusal %+v", u)
	}
	// Re-subscribing tokens already in the set costs nothing.
	if err := r.m.Subscribe(ConsumerChart, "c1", []uint32{1, 5}, ModeQuote); err != nil {
		t.Fatalf("overlapping subscribe refused: %v", err)
	}
}

func TestModeUnionAndDeltas(t *testing.T) {
	r := newRig(t, Options{})
	c := r.d.get(0)
	eventually(t, "connect", func() bool { return r.m.Usage().Connections == 1 })
	_ = r.m.Subscribe(ConsumerChart, "chart", []uint32{10}, ModeQuote)
	_ = r.m.Subscribe(ConsumerDeployment, "dep", []uint32{10, 11}, ModeFull)
	r.m.Release(ConsumerDeployment, "dep")
	r.m.Unsubscribe(ConsumerChart, "chart", []uint32{10})
	want := []string{"sub[10]", "quote[10]", "sub[11]", "full[10 11]", "unsub[11]", "quote[10]", "unsub[10]"}
	got := c.Ops()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("ops = %v\nwant %v", got, want)
	}
}

func TestStickyAssignment(t *testing.T) {
	r := newRig(t, Options{PerConn: 2, MaxConns: 3})
	_ = r.m.Subscribe(ConsumerWatchlist, "w", []uint32{1, 2, 3}, ModeQuote) // 1,2 → conn0; 3 → conn1
	eventually(t, "two connections", func() bool { return r.d.n() == 2 })
	r.m.Unsubscribe(ConsumerWatchlist, "w", []uint32{1})
	_ = r.m.Subscribe(ConsumerWatchlist, "w", []uint32{4}, ModeQuote) // fills conn0's free place
	r.m.mu.Lock()
	a := map[uint32]int{2: r.m.assign[2], 3: r.m.assign[3], 4: r.m.assign[4]}
	r.m.mu.Unlock()
	if a[2] != 0 || a[3] != 1 || a[4] != 0 {
		t.Fatalf("assignment %v", a)
	}
}

func TestReconnectResubscribesAllTokensWithModes(t *testing.T) { // NFR-09
	r := newRig(t, Options{})
	feed := bus.Subscribe(r.bus, TopicFeed, bus.SubOpts[FeedState]{Buffer: 64})
	_ = r.m.Subscribe(ConsumerChart, "c", []uint32{5, 6}, ModeQuote)
	_ = r.m.Subscribe(ConsumerDeployment, "d", []uint32{7}, ModeFull)
	r.d.get(0).Stop()                        // the network drops
	if !r.clk.WaitTimers(2, 2*time.Second) { // flush loop + reconnect delay
		t.Fatal("no reconnect timer")
	}
	r.clk.Advance(2 * time.Second)
	eventually(t, "redial", func() bool { return r.d.n() == 2 })
	eventually(t, "resubscribe", func() bool { return len(r.d.get(1).Ops()) == 3 })
	if got := fmt.Sprint(r.d.get(1).Ops()); got != "[sub[5 6 7] quote[5 6] full[7]]" {
		t.Fatalf("resubscribe ops %s", got)
	}
	var states []string
	for len(feed.C()) > 0 {
		states = append(states, (<-feed.C()).State)
	}
	if fmt.Sprint(states[len(states)-2:]) != "[reconnecting live]" {
		t.Fatalf("feed states %v", states)
	}
}

func TestReconnectStormBacksOffAndGoesOffline(t *testing.T) { // Review Focus 5
	r := newRig(t, Options{MaxAttempts: 4})
	feed := bus.Subscribe(r.bus, TopicFeed, bus.SubOpts[FeedState]{Buffer: 64})
	r.d.mu.Lock()
	r.d.failNext = 10
	r.d.mu.Unlock()
	r.d.get(0).Stop()
	var delays []time.Duration
	for i := 0; i < 4; i++ {
		var fs FeedState
		eventually(t, "reconnecting state", func() bool {
			for len(feed.C()) > 0 {
				fs = <-feed.C()
			}
			return fs.State == "reconnecting" && fs.Attempt == i+1
		})
		delays = append(delays, fs.NextRetry.Sub(r.clk.Now()))
		r.clk.Advance(fs.NextRetry.Sub(r.clk.Now()))
	}
	if fmt.Sprint(delays) != "[2s 4s 8s 16s]" {
		t.Fatalf("delays %v, want doubling from 2 s", delays)
	}
	eventually(t, "offline", func() bool {
		for len(feed.C()) > 0 {
			if (<-feed.C()).State == "offline" {
				return true
			}
		}
		return false
	})
	dials := r.d.n()
	r.clk.Advance(time.Hour)
	time.Sleep(20 * time.Millisecond)
	if r.d.n() != dials {
		t.Fatal("offline connection kept dialling")
	}
	r.d.mu.Lock()
	r.d.failNext = 0
	r.d.mu.Unlock()
	r.m.Reconnect()
	eventually(t, "manual reconnect", func() bool { return r.d.n() == dials+1 })
}

func TestQuickReconnectsDoNotResetBackoff(t *testing.T) {
	r := newRig(t, Options{})
	feed := bus.Subscribe(r.bus, TopicFeed, bus.SubOpts[FeedState]{Buffer: 64})
	last := func() FeedState {
		var fs FeedState
		eventually(t, "reconnecting", func() bool {
			for len(feed.C()) > 0 {
				fs = <-feed.C()
			}
			return fs.State == "reconnecting"
		})
		return fs
	}
	r.d.get(0).Stop()
	fs := last()
	r.clk.Advance(fs.NextRetry.Sub(r.clk.Now()))
	eventually(t, "second conn", func() bool { return r.d.n() == 2 })
	r.clk.Advance(5 * time.Second) // up for 5 s only
	r.d.get(1).Stop()
	if fs = last(); fs.Attempt != 2 {
		t.Fatalf("attempt after a 5 s connection = %d, want 2", fs.Attempt)
	}
	r.clk.Advance(fs.NextRetry.Sub(r.clk.Now()))
	eventually(t, "third conn", func() bool { return r.d.n() == 3 })
	r.clk.Advance(31 * time.Second) // stable
	r.d.get(2).Stop()
	if fs = last(); fs.Attempt != 1 {
		t.Fatalf("attempt after a 31 s connection = %d, want 1", fs.Attempt)
	}
}

func TestCoalescedBatchAndRawStream(t *testing.T) {
	r := newRig(t, Options{})
	batches := bus.Subscribe(r.bus, TopicTicks, bus.SubOpts[[]Tick]{Buffer: 8})
	raw, cancel := r.m.Raw(16)
	defer cancel()
	c := r.d.get(0)
	for i := 0; i < 5; i++ {
		c.ev.OnTick(models.Tick{InstrumentToken: 9, LastPrice: float64(100 + i)})
	}
	c.ev.OnTick(models.Tick{InstrumentToken: 3, LastPrice: 50})
	if len(raw) != 6 {
		t.Fatalf("raw got %d ticks, want every tick", len(raw))
	}
	r.clk.Advance(100 * time.Millisecond)
	var b []Tick
	eventually(t, "batch", func() bool {
		select {
		case b = <-batches.C():
			return true
		default:
			return false
		}
	})
	if len(b) != 2 || b[0].Token != 3 || b[1].Token != 9 || b[1].LTP != 104 {
		t.Fatalf("batch %+v, want latest per token sorted", b)
	}
}

type obs struct {
	mu sync.Mutex
	ts []time.Time
}

func (o *obs) ObserveTick(ex, arrival time.Time) { o.mu.Lock(); o.ts = append(o.ts, ex); o.mu.Unlock() }

func TestClockReferenceIsFullModeAndFeedsObserver(t *testing.T) {
	o := &obs{}
	r := newRig(t, Options{ClockRef: 256265, Observer: o})
	eventually(t, "clock ref subscribed", func() bool { return len(r.d.get(0).Ops()) == 2 })
	if got := fmt.Sprint(r.d.get(0).Ops()); got != "[sub[256265] full[256265]]" {
		t.Fatalf("ops %s", got)
	}
	ex := time.Date(2026, 10, 12, 10, 0, 0, 0, clock.IST)
	r.d.get(0).ev.OnTick(models.Tick{InstrumentToken: 256265, Timestamp: models.Time{Time: ex}})
	r.d.get(0).ev.OnTick(models.Tick{InstrumentToken: 738561, Timestamp: models.Time{Time: ex}})
	if len(o.ts) != 1 || !o.ts[0].Equal(ex) {
		t.Fatalf("observer got %v", o.ts)
	}
}

func TestOrderUpdatesPublished(t *testing.T) {
	r := newRig(t, Options{})
	sub := bus.Subscribe(r.bus, kite.TopicOrderUpdate, bus.SubOpts[kite.Order]{Buffer: 4})
	r.d.get(0).ev.OnOrderUpdate(kiteconnect.Order{OrderID: "151220000000000", Status: "COMPLETE"})
	if o := <-sub.C(); o.OrderID != "151220000000000" {
		t.Fatalf("order %+v", o)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/kite/ticker/`
Expected: the build fails — output contains `undefined: New` and `[build failed]`.

- [ ] **Step 3: Write the implementation**

`internal/kite/ticker/types.go`:

```go
// Package ticker is the WebSocket subscription manager (C-07, ARCHITECTURE §5.6): up to three
// gokiteconnect tickers × 3000 tokens, consumer priorities, own reconnect with resubscribe,
// mode switching, 100 ms coalesced fan-out on the bus, raw per-tick streams, clock samples.
package ticker

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zerodha/gokiteconnect/v4/models"
	kt "github.com/zerodha/gokiteconnect/v4/ticker"

	"tradeloom/internal/bus"
)

// Mode is a subscription mode; higher includes lower (ltp < quote < full).
type Mode uint8

const (
	ModeLTP   Mode = 1
	ModeQuote Mode = 2
	ModeFull  Mode = 3 // the only mode carrying the exchange timestamp (clock samples)
)

func (m Mode) String() string {
	switch m {
	case ModeLTP:
		return "ltp"
	case ModeQuote:
		return "quote"
	case ModeFull:
		return "full"
	}
	return fmt.Sprintf("mode(%d)", m)
}

func (m Mode) kite() kt.Mode {
	switch m {
	case ModeLTP:
		return kt.ModeLTP
	case ModeFull:
		return kt.ModeFull
	}
	return kt.ModeQuote
}

// Consumer is a class of subscriber, highest priority first.
type Consumer uint8

const (
	ConsumerClock Consumer = iota // the clock reference token (manager-owned)
	ConsumerDeployment
	ConsumerLiveScanner
	ConsumerChart
	ConsumerWatchlist
	ConsumerWidget
	ConsumerRecorder
)

var consumerNames = []string{"clock", "deployment", "live-scanner", "chart", "watchlist", "widget", "recorder"}

func (c Consumer) String() string {
	if int(c) < len(consumerNames) {
		return consumerNames[c]
	}
	return fmt.Sprintf("consumer(%d)", c)
}

// Tick is the app's tick (JSON for the UI bridge). Prices in rupees.
type Tick struct {
	Token         uint32    `json:"token"`
	Mode          string    `json:"mode"`
	LTP           float64   `json:"ltp"`
	Change        float64   `json:"change"` // LTP − previous close (Kite net_change)
	Open          float64   `json:"open"`
	High          float64   `json:"high"`
	Low           float64   `json:"low"`
	Close         float64   `json:"close"`  // previous session close
	Volume        uint32    `json:"volume"` // cumulative day volume
	LastQty       uint32    `json:"lastQty"`
	OI            uint32    `json:"oi"`
	ExchangeTS    time.Time `json:"exchangeTs"`    // zero unless full mode
	LastTradeTime time.Time `json:"lastTradeTime"` // zero unless full mode (equities)
	Received      time.Time `json:"received"`      // app clock at arrival
	IsIndex       bool      `json:"isIndex"`
}

// fromKite converts gokiteconnect's tick.
func fromKite(t models.Tick, received time.Time) Tick {
	return Tick{
		Token: t.InstrumentToken, Mode: t.Mode, LTP: t.LastPrice, Change: t.NetChange,
		Open: t.OHLC.Open, High: t.OHLC.High, Low: t.OHLC.Low, Close: t.OHLC.Close,
		Volume: t.VolumeTraded, LastQty: t.LastTradedQuantity, OI: t.OI,
		ExchangeTS: t.Timestamp.Time, LastTradeTime: t.LastTradeTime.Time, Received: received, IsIndex: t.IsIndex,
	}
}

// ErrBudget refuses a subscription that would exceed the 3 × 3000 token budget (C-07).
type ErrBudget struct {
	Used, Requested, Limit int
	ByConsumer             map[string]int
}

func (e *ErrBudget) Error() string {
	var parts []string
	for k, v := range e.ByConsumer {
		parts = append(parts, fmt.Sprintf("%s %d", k, v))
	}
	sort.Strings(parts)
	return fmt.Sprintf("live-data budget exceeded: %d in use + %d new > %d (%s)", e.Used, e.Requested, e.Limit, strings.Join(parts, ", "))
}

// FeedState is "feed.state" (§3.4): live | connecting | reconnecting | offline | stopped.
type FeedState struct {
	State       string    `json:"state"`
	Attempt     int       `json:"attempt"`
	NextRetry   time.Time `json:"nextRetry"`
	LastTick    time.Time `json:"lastTick"`
	Connections int       `json:"connections"`
	Tokens      int       `json:"tokens"`
}

// Usage is the budget breakdown for Settings → Limits (FR-ACCT-009).
type Usage struct {
	Used        int            `json:"used"`
	Limit       int            `json:"limit"`
	Connections int            `json:"connections"`
	ByConsumer  map[string]int `json:"byConsumer"`
	RawDrops    uint64         `json:"rawDrops"`
}

var (
	// TopicTicks is "tick.batch": latest tick per token, at most every Coalesce interval.
	TopicTicks = bus.NewTopic[[]Tick]("tick.batch")
	// TopicFeed is "feed.state".
	TopicFeed = bus.NewTopic[FeedState]("feed.state")
)
```

`internal/kite/ticker/conn.go`:

```go
package ticker

import (
	"context"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
	"github.com/zerodha/gokiteconnect/v4/models"
)

// Events are the callbacks a Conn invokes (on its own goroutine).
type Events struct {
	OnConnect     func()
	OnTick        func(models.Tick)
	OnOrderUpdate func(kiteconnect.Order)
	OnError       func(error)
}

// Conn is one WebSocket connection. Serve blocks until the connection ends; Subscribe,
// Unsubscribe and SetMode are only called after OnConnect and never concurrently.
type Conn interface {
	Serve(ctx context.Context)
	Subscribe(tokens []uint32) error
	Unsubscribe(tokens []uint32) error
	SetMode(m Mode, tokens []uint32) error
	Stop()
}

// Dialer creates a Conn wired to ev.
type Dialer func(ev Events) Conn
```

`internal/kite/ticker/manager.go`:

```go
package ticker

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
	"github.com/zerodha/gokiteconnect/v4/models"

	"tradeloom/internal/bus"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/logging"
)

// TickObserver receives exchange timestamps of the clock reference token (clock.Real).
type TickObserver interface {
	ObserveTick(exchangeTS, arrival time.Time)
}

// Options configures a Manager. Zero values take the defaults in brackets.
type Options struct {
	Dial        Dialer // required
	Clock       clock.Clock
	Bus         *bus.Bus
	Log         *slog.Logger
	PerConn     int                                 // [3000] tokens per connection (A07 §A6)
	MaxConns    int                                 // [3] connections per API key
	ClockRef    uint32                              // token subscribed in full mode as the clock source (NIFTY 50); 0 = none
	Observer    TickObserver                        // receives ClockRef timestamps
	Coalesce    time.Duration                       // [100 ms] tick.batch interval (S0-6 confirms)
	BaseDelay   time.Duration                       // [2 s] first reconnect delay
	MaxDelay    time.Duration                       // [60 s]
	StableAfter time.Duration                       // [30 s] a connection up this long resets the attempt counter
	MaxAttempts int                                 // [50] consecutive failures before Offline
	Jitter      func(d time.Duration) time.Duration // [±20 %]
}

type ownerKey struct {
	c  Consumer
	id string
}

type slot struct {
	idx         int
	started     bool
	conn        Conn
	live        bool
	state       string
	attempt     int
	nextRetry   time.Time
	connectedAt time.Time
	tokens      map[uint32]Mode // assigned (sticky) tokens with their applied mode
}

// Manager owns the connections and the subscription registry.
type Manager struct {
	o       Options
	mu      sync.Mutex
	ctx     context.Context
	stopped bool
	owners  map[ownerKey]map[uint32]Mode
	assign  map[uint32]int
	slots   []*slot
	retry   chan struct{}

	tickMu   sync.Mutex
	latest   map[uint32]Tick
	lastTick time.Time
	raws     map[int]chan Tick
	rawSeq   int
	rawDrops atomic.Uint64
}

// New builds a Manager; Start connects.
func New(o Options) *Manager {
	if o.PerConn == 0 {
		o.PerConn = 3000
	}
	if o.MaxConns == 0 {
		o.MaxConns = 3
	}
	if o.Coalesce == 0 {
		o.Coalesce = 100 * time.Millisecond
	}
	if o.BaseDelay == 0 {
		o.BaseDelay = 2 * time.Second
	}
	if o.MaxDelay == 0 {
		o.MaxDelay = 60 * time.Second
	}
	if o.StableAfter == 0 {
		o.StableAfter = 30 * time.Second
	}
	if o.MaxAttempts == 0 {
		o.MaxAttempts = 50
	}
	if o.Jitter == nil {
		o.Jitter = func(d time.Duration) time.Duration { return time.Duration(float64(d) * (0.8 + 0.4*rand.Float64())) }
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	m := &Manager{o: o, owners: map[ownerKey]map[uint32]Mode{}, assign: map[uint32]int{},
		retry: make(chan struct{}, 1), latest: map[uint32]Tick{}, raws: map[int]chan Tick{}}
	for i := 0; i < o.MaxConns; i++ {
		m.slots = append(m.slots, &slot{idx: i, state: "stopped", tokens: map[uint32]Mode{}})
	}
	return m
}

// Start opens connection 1, subscribes the clock reference, and starts the tick flusher.
// Connections 2 and 3 open only when the previous one is full.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	m.ctx = ctx
	m.mu.Unlock()
	logging.Go("ticker.flush", func() { m.flushLoop(ctx) })
	if m.o.ClockRef != 0 {
		if err := m.Subscribe(ConsumerClock, "clock", []uint32{m.o.ClockRef}, ModeFull); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.startSlot(m.slots[0])
	m.mu.Unlock()
	return nil
}

// Stop closes every connection (logout, shutdown). A Manager serves one session: Stop is final
// and a new Manager is built with the next access token.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
	for _, s := range m.slots {
		if s.conn != nil {
			s.conn.Stop()
		}
		s.live, s.state = false, "stopped"
	}
	m.publishFeedLocked()
}

// Reconnect restarts connections that went Offline (user "Retry now").
func (m *Manager) Reconnect() {
	select {
	case m.retry <- struct{}{}:
	default:
	}
}

// Subscribe adds tokens for (c, owner) at mode. It is all-or-nothing: if the union of all
// subscriptions would exceed MaxConns × PerConn tokens it returns *ErrBudget and changes nothing.
// No subscription is ever evicted to make room.
func (m *Manager) Subscribe(c Consumer, owner string, tokens []uint32, mode Mode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := ownerKey{c, owner}
	newCount := m.countNew(tokens)
	limit := m.o.PerConn * m.o.MaxConns
	if len(m.assign)+newCount > limit {
		return &ErrBudget{Used: len(m.assign), Requested: newCount, Limit: limit, ByConsumer: m.byConsumerLocked()}
	}
	set := m.owners[k]
	if set == nil {
		set = map[uint32]Mode{}
		m.owners[k] = set
	}
	for _, t := range tokens {
		if set[t] < mode {
			set[t] = mode
		}
	}
	m.applyLocked()
	return nil
}

func (m *Manager) countNew(tokens []uint32) int {
	seen := map[uint32]bool{}
	n := 0
	for _, t := range tokens {
		if _, ok := m.assign[t]; ok || seen[t] {
			continue
		}
		seen[t] = true
		n++
	}
	return n
}

// Unsubscribe removes tokens of (c, owner).
func (m *Manager) Unsubscribe(c Consumer, owner string, tokens []uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if set := m.owners[ownerKey{c, owner}]; set != nil {
		for _, t := range tokens {
			delete(set, t)
		}
	}
	m.applyLocked()
}

// Release removes every token of (c, owner).
func (m *Manager) Release(c Consumer, owner string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.owners, ownerKey{c, owner})
	m.applyLocked()
}

// Usage returns the budget breakdown.
func (m *Manager) Usage() Usage {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, s := range m.slots {
		if s.started {
			n++
		}
	}
	return Usage{Used: len(m.assign), Limit: m.o.PerConn * m.o.MaxConns, Connections: n, ByConsumer: m.byConsumerLocked(), RawDrops: m.rawDrops.Load()}
}

func (m *Manager) byConsumerLocked() map[string]int {
	out := map[string]int{}
	for k, set := range m.owners {
		out[k.c.String()] += len(set)
	}
	return out
}

// desiredLocked is the union of all owners with the highest mode per token.
func (m *Manager) desiredLocked() map[uint32]Mode {
	d := map[uint32]Mode{}
	for _, set := range m.owners {
		for t, md := range set {
			if d[t] < md {
				d[t] = md
			}
		}
	}
	return d
}

// applyLocked reconciles assignments with the desired set and sends deltas to live connections.
func (m *Manager) applyLocked() {
	want := m.desiredLocked()
	type delta struct {
		sub, unsub []uint32
		modes      map[Mode][]uint32
	}
	ds := make([]delta, len(m.slots))
	for i := range ds {
		ds[i].modes = map[Mode][]uint32{}
	}
	for t, i := range m.assign { // removals and mode changes
		md, ok := want[t]
		s := m.slots[i]
		if !ok {
			delete(m.assign, t)
			delete(s.tokens, t)
			ds[i].unsub = append(ds[i].unsub, t)
			continue
		}
		if s.tokens[t] != md {
			s.tokens[t] = md
			ds[i].modes[md] = append(ds[i].modes[md], t)
		}
	}
	var added []uint32
	for t := range want {
		if _, ok := m.assign[t]; !ok {
			added = append(added, t)
		}
	}
	sort.Slice(added, func(i, j int) bool { return added[i] < added[j] })
	for _, t := range added { // sticky first-fit assignment
		for i, s := range m.slots {
			if len(s.tokens) < m.o.PerConn {
				m.assign[t] = i
				s.tokens[t] = want[t]
				ds[i].sub = append(ds[i].sub, t)
				ds[i].modes[want[t]] = append(ds[i].modes[want[t]], t)
				if !s.started && m.ctx != nil {
					m.startSlot(s)
				}
				break
			}
		}
	}
	for i, s := range m.slots {
		if !s.live {
			continue // applied in full on connect
		}
		d := ds[i]
		m.send(s, "unsubscribe", func() error { return s.conn.Unsubscribe(d.unsub) }, len(d.unsub))
		m.send(s, "subscribe", func() error { return s.conn.Subscribe(d.sub) }, len(d.sub))
		for _, md := range []Mode{ModeLTP, ModeQuote, ModeFull} {
			if ts := d.modes[md]; len(ts) > 0 {
				m.send(s, "mode "+md.String(), func() error { return s.conn.SetMode(md, ts) }, len(ts))
			}
		}
	}
}

func (m *Manager) send(s *slot, what string, fn func() error, n int) {
	if n == 0 {
		return
	}
	if err := fn(); err != nil {
		m.o.Log.Warn("ticker write failed; connection will be re-established", "conn", s.idx, "op", what, "err", err)
	}
}

// startSlot launches the connect/reconnect loop of s (caller holds mu, m.ctx set).
func (m *Manager) startSlot(s *slot) {
	if s.started || m.ctx == nil || m.stopped {
		return
	}
	s.started, s.state = true, "connecting"
	ctx := m.ctx
	logging.Go(fmt.Sprintf("ticker.conn.%d", s.idx), func() { m.run(ctx, s) })
}

func (m *Manager) run(ctx context.Context, s *slot) {
	for ctx.Err() == nil {
		var c Conn
		c = m.o.Dial(Events{
			OnConnect:     func() { m.onConnect(s, c) },
			OnTick:        m.onTick,
			OnOrderUpdate: func(o kiteconnect.Order) { publish(m.o.Bus, kite.TopicOrderUpdate, o) },
			OnError:       func(err error) { m.o.Log.Info("ticker error", "conn", s.idx, "err", err) },
		})
		m.mu.Lock()
		s.conn = c
		m.mu.Unlock()
		c.Serve(ctx)
		now := m.o.Clock.Now()
		m.mu.Lock()
		if ctx.Err() != nil || m.stopped {
			m.mu.Unlock()
			return
		}
		if !s.connectedAt.IsZero() && now.Sub(s.connectedAt) >= m.o.StableAfter {
			s.attempt = 0
		}
		s.live, s.connectedAt = false, time.Time{}
		s.attempt++
		att := s.attempt
		if att > m.o.MaxAttempts {
			s.state = "offline"
			m.publishFeedLocked()
			m.mu.Unlock()
			m.o.Log.Warn("ticker offline", "conn", s.idx, "attempts", att-1)
			select {
			case <-ctx.Done():
				return
			case <-m.retry:
				m.mu.Lock()
				s.attempt = 0
				s.state = "connecting"
				m.publishFeedLocked()
				m.mu.Unlock()
				continue
			}
		}
		delay := m.o.BaseDelay << (att - 1)
		if delay > m.o.MaxDelay || delay <= 0 {
			delay = m.o.MaxDelay
		}
		delay = m.o.Jitter(delay)
		s.state, s.nextRetry = "reconnecting", now.Add(delay)
		m.publishFeedLocked()
		m.mu.Unlock()
		m.o.Log.Info("ticker reconnecting", "conn", s.idx, "attempt", att, "delay", delay.String())
		if clock.Sleep(ctx, m.o.Clock, delay) != nil {
			return
		}
	}
}

// onConnect re-applies the slot's whole subscription on the fresh connection (resubscribe).
func (m *Manager) onConnect(s *slot, c Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s.live, s.state, s.connectedAt = true, "live", m.o.Clock.Now()
	all := make([]uint32, 0, len(s.tokens))
	modes := map[Mode][]uint32{}
	for t, md := range s.tokens {
		all = append(all, t)
		modes[md] = append(modes[md], t)
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	m.send(s, "subscribe", func() error { return c.Subscribe(all) }, len(all))
	for _, md := range []Mode{ModeLTP, ModeQuote, ModeFull} {
		if ts := modes[md]; len(ts) > 0 {
			sort.Slice(ts, func(i, j int) bool { return ts[i] < ts[j] })
			m.send(s, "mode "+md.String(), func() error { return c.SetMode(md, ts) }, len(ts))
		}
	}
	m.o.Log.Info("ticker connected", "conn", s.idx, "tokens", len(all))
	m.publishFeedLocked()
}

func (m *Manager) publishFeedLocked() {
	fs := FeedState{State: "stopped", Tokens: len(m.assign)}
	rank := map[string]int{"stopped": 0, "live": 1, "connecting": 2, "reconnecting": 3, "offline": 4}
	for _, s := range m.slots {
		if !s.started {
			continue
		}
		fs.Connections++
		if rank[s.state] > rank[fs.State] {
			fs.State = s.state
			fs.Attempt, fs.NextRetry = s.attempt, s.nextRetry
		}
	}
	m.tickMu.Lock()
	fs.LastTick = m.lastTick
	m.tickMu.Unlock()
	publish(m.o.Bus, TopicFeed, fs)
}

func (m *Manager) onTick(kt models.Tick) {
	t := fromKite(kt, m.o.Clock.Now())
	if t.Token == m.o.ClockRef && m.o.Observer != nil && !t.ExchangeTS.IsZero() {
		m.o.Observer.ObserveTick(t.ExchangeTS, m.o.Clock.System())
	}
	m.tickMu.Lock()
	m.latest[t.Token] = t
	m.lastTick = t.Received
	for _, ch := range m.raws {
		select {
		case ch <- t:
		default:
			m.rawDrops.Add(1)
		}
	}
	m.tickMu.Unlock()
}

// Raw returns a stream of every tick (all tokens) for the candle builder and live SL/TP checks,
// and a cancel function. A full buffer drops ticks and counts them in Usage.RawDrops.
func (m *Manager) Raw(buffer int) (<-chan Tick, func()) {
	ch := make(chan Tick, buffer)
	m.tickMu.Lock()
	id := m.rawSeq
	m.rawSeq++
	m.raws[id] = ch
	m.tickMu.Unlock()
	return ch, func() {
		m.tickMu.Lock()
		delete(m.raws, id)
		m.tickMu.Unlock()
	}
}

func (m *Manager) flushLoop(ctx context.Context) {
	for {
		if clock.Sleep(ctx, m.o.Clock, m.o.Coalesce) != nil {
			return
		}
		m.tickMu.Lock()
		if len(m.latest) == 0 {
			m.tickMu.Unlock()
			continue
		}
		batch := make([]Tick, 0, len(m.latest))
		for _, t := range m.latest {
			batch = append(batch, t)
		}
		m.latest = map[uint32]Tick{}
		m.tickMu.Unlock()
		sort.Slice(batch, func(i, j int) bool { return batch[i].Token < batch[j].Token })
		publish(m.o.Bus, TopicTicks, batch)
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

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/kite/ticker/`
Expected: `ok  	tradeloom/internal/kite/ticker`; no `DATA RACE`.

- [ ] **Step 5: Commit**

```bash
git add internal/kite/ticker/types.go \
  internal/kite/ticker/conn.go \
  internal/kite/ticker/manager.go \
  internal/kite/ticker/manager_test.go
git commit -m "$(cat <<'EOF'
feat(kite/ticker): subscription manager with budget, reconnect, coalescing and clock feed

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
