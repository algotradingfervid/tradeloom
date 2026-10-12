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

// drain returns every event the subscription can deliver right now without blocking. bus.Sub.C()
// is unbuffered (fed by a pump goroutine), so len(C()) is always 0; callers poll drain inside
// eventually so pump latency cannot lose events.
func drain[T any](s *bus.Sub[T]) []T {
	var out []T
	for {
		select {
		case v, ok := <-s.C():
			if !ok {
				return out
			}
			out = append(out, v)
		default:
			return out
		}
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
	ok := func() bool { return len(states) >= 2 && fmt.Sprint(states[len(states)-2:]) == "[reconnecting live]" }
	deadline := time.Now().Add(2 * time.Second)
	for !ok() && time.Now().Before(deadline) {
		for _, fs := range drain(feed) {
			states = append(states, fs.State)
		}
		time.Sleep(time.Millisecond)
	}
	if !ok() {
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
			for _, v := range drain(feed) {
				fs = v
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
		for _, v := range drain(feed) {
			if v.State == "offline" {
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
			for _, v := range drain(feed) {
				fs = v
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

func TestStopDuringReconnectWaitDoesNotRedial(t *testing.T) {
	r := newRig(t, Options{})
	r.d.get(0).Stop()
	if !r.clk.WaitTimers(2, 2*time.Second) { // flush loop + reconnect delay
		t.Fatal("no reconnect timer")
	}
	r.m.Stop()
	r.clk.Advance(time.Hour)
	time.Sleep(20 * time.Millisecond)
	if n := r.d.n(); n != 1 {
		t.Fatalf("stopped manager dialled again: %d dials", n)
	}
}

func TestReconnectWhileLiveDoesNotArmALaterRetry(t *testing.T) {
	r := newRig(t, Options{MaxAttempts: 1})
	feed := bus.Subscribe(r.bus, TopicFeed, bus.SubOpts[FeedState]{Buffer: 64})
	eventually(t, "connect", func() bool { return r.m.Usage().Connections == 1 })
	r.m.Reconnect() // user clicks "Retry now" while the feed is fine
	r.d.mu.Lock()
	r.d.failNext = 10
	r.d.mu.Unlock()
	r.d.get(0).Stop()
	var fs FeedState
	eventually(t, "reconnecting", func() bool {
		for _, v := range drain(feed) {
			fs = v
		}
		return fs.State == "reconnecting"
	})
	r.clk.Advance(fs.NextRetry.Sub(r.clk.Now()))
	eventually(t, "offline", func() bool {
		for _, v := range drain(feed) {
			if v.State == "offline" {
				return true
			}
		}
		return false
	})
	time.Sleep(20 * time.Millisecond)
	if n := r.d.n(); n != 2 { // the first connection + one back-off retry
		t.Fatalf("offline connection redialled without a retry: %d dials, want 2", n)
	}
}

func TestReconnectWakesEveryOfflineConnection(t *testing.T) {
	r := newRig(t, Options{PerConn: 1, MaxConns: 2, MaxAttempts: 1})
	feed := bus.Subscribe(r.bus, TopicFeed, bus.SubOpts[FeedState]{Buffer: 64})
	_ = r.m.Subscribe(ConsumerWatchlist, "w", []uint32{1, 2}, ModeQuote)
	eventually(t, "both live", func() bool {
		var fs FeedState
		for _, v := range drain(feed) {
			fs = v
		}
		return r.d.n() == 2 && fs.State == "live" && fs.Connections == 2
	})
	r.d.mu.Lock()
	r.d.failNext = 10
	r.d.mu.Unlock()
	r.d.get(0).Stop()
	r.d.get(1).Stop()
	if !r.clk.WaitTimers(3, 2*time.Second) { // flush loop + two reconnect delays
		t.Fatal("no reconnect timers")
	}
	r.clk.Advance(time.Minute)
	eventually(t, "both offline", func() bool { return r.d.n() == 4 && r.clk.PendingTimers() == 1 })
	time.Sleep(20 * time.Millisecond)
	r.d.mu.Lock()
	r.d.failNext = 0
	r.d.mu.Unlock()
	r.m.Reconnect()
	eventually(t, "both redialled", func() bool { return r.d.n() == 6 })
}

func TestSubscribeRejectsInvalidMode(t *testing.T) {
	r := newRig(t, Options{})
	for _, md := range []Mode{0, 4} {
		if err := r.m.Subscribe(ConsumerChart, "c", []uint32{1}, md); err == nil {
			t.Fatalf("mode %d accepted", md)
		}
	}
	if u := r.m.Usage(); u.Used != 0 {
		t.Fatalf("usage after refused modes %+v", u)
	}
}

func TestRawCancelClosesStream(t *testing.T) {
	r := newRig(t, Options{})
	raw, cancel := r.m.Raw(4)
	cancel()
	cancel() // idempotent
	select {
	case _, ok := <-raw:
		if ok {
			t.Fatal("raw stream delivered after cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("raw stream not closed by cancel")
	}
	r.d.get(0).ev.OnTick(models.Tick{InstrumentToken: 1, LastPrice: 1}) // no send on closed channel
}

// waitGoroutines waits (real time) until every Manager goroutine has returned.
func waitGoroutines(t *testing.T, m *Manager) {
	t.Helper()
	done := make(chan struct{})
	go func() { m.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("manager goroutines still running after Stop")
	}
}

func TestStopEndsGoroutinesTimersAndEvents(t *testing.T) {
	// conn 0 goes Offline (waits on Reconnect), conn 1 sits in a reconnect back-off.
	r := newRig(t, Options{PerConn: 1, MaxConns: 2, MaxAttempts: 1})
	feed := bus.Subscribe(r.bus, TopicFeed, bus.SubOpts[FeedState]{Buffer: 256})
	_ = r.m.Subscribe(ConsumerWatchlist, "w", []uint32{1, 2}, ModeQuote)
	eventually(t, "two dials", func() bool { return r.d.n() == 2 })
	r.d.mu.Lock()
	r.d.failNext = 1
	r.d.mu.Unlock()
	r.d.get(0).Stop()
	if !r.clk.WaitTimers(2, 2*time.Second) {
		t.Fatal("no reconnect timer")
	}
	r.clk.Advance(2 * time.Second) // conn 0 redial fails -> Offline
	eventually(t, "conn 0 offline", func() bool { return r.d.n() == 3 && r.clk.PendingTimers() == 1 })
	r.d.get(1).Stop() // conn 1 drops -> reconnect timer
	if !r.clk.WaitTimers(2, 2*time.Second) {
		t.Fatal("no reconnect timer for conn 1")
	}
	time.Sleep(10 * time.Millisecond)

	r.m.Stop()
	eventually(t, "stopped event", func() bool {
		for _, v := range drain(feed) {
			if v.State == "stopped" {
				return true
			}
		}
		return false
	})
	waitGoroutines(t, r.m)
	if n := r.clk.PendingTimers(); n != 0 {
		t.Fatalf("%d timers still armed after Stop", n)
	}
	r.m.Reconnect() // must not revive the Offline connection
	r.clk.Advance(time.Hour)
	var after []string
	deadline := time.Now().Add(50 * time.Millisecond)
	for time.Now().Before(deadline) {
		for _, v := range drain(feed) {
			after = append(after, v.State)
		}
		time.Sleep(time.Millisecond)
	}
	if len(after) != 0 {
		t.Fatalf("feed events after stopped: %v", after)
	}
	if n := r.d.n(); n != 3 {
		t.Fatalf("dialled after Stop: %d dials", n)
	}
}

// gatedConn connects only when its gate opens, and records whether Stop came after the connect.
type gatedConn struct {
	ev    Events
	gate  chan struct{}
	mu    sync.Mutex
	conn  bool
	after bool
}

func (g *gatedConn) Serve(ctx context.Context) {
	<-g.gate
	g.mu.Lock()
	g.conn = true
	g.mu.Unlock()
	g.ev.OnConnect()
	<-ctx.Done()
}
func (g *gatedConn) Subscribe([]uint32) error     { return nil }
func (g *gatedConn) Unsubscribe([]uint32) error   { return nil }
func (g *gatedConn) SetMode(Mode, []uint32) error { return nil }
func (g *gatedConn) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.conn {
		g.after = true
	}
}

func TestConnectAfterStopClosesTheSocket(t *testing.T) {
	g := &gatedConn{gate: make(chan struct{})}
	dialed := make(chan struct{})
	m := New(Options{Clock: clock.NewFake(time.Date(2026, 10, 12, 10, 0, 0, 0, clock.IST)),
		Dial: func(ev Events) Conn { g.ev = ev; close(dialed); return g }})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	<-dialed
	eventually(t, "serving", func() bool { m.mu.Lock(); defer m.mu.Unlock(); return m.slots[0].conn != nil })
	m.Stop()      // the dial is still in progress: this Stop cannot close it
	close(g.gate) // the socket connects after Stop
	eventually(t, "late connection stopped", func() bool { g.mu.Lock(); defer g.mu.Unlock(); return g.after })
	waitGoroutines(t, m)
}

func TestStartPublishesConnectingFirst(t *testing.T) {
	d := &dialer{}
	b := bus.New(nil)
	feed := bus.Subscribe(b, TopicFeed, bus.SubOpts[FeedState]{Buffer: 16})
	clk := clock.NewFake(time.Date(2026, 10, 12, 10, 0, 0, 0, clock.IST))
	m := New(Options{Dial: d.dial, Clock: clk, Bus: b})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	var states []string
	eventually(t, "two feed events", func() bool {
		for _, v := range drain(feed) {
			states = append(states, v.State)
		}
		return len(states) >= 2
	})
	if states[0] != "connecting" || states[1] != "live" {
		t.Fatalf("feed states %v, want [connecting live ...]", states)
	}
}

func TestStartLaunchesEverySlotWithTokensOnce(t *testing.T) {
	d := &dialer{}
	clk := clock.NewFake(time.Date(2026, 10, 12, 10, 0, 0, 0, clock.IST))
	m := New(Options{Dial: d.dial, Clock: clk, PerConn: 1, MaxConns: 3})
	if err := m.Subscribe(ConsumerChart, "c", []uint32{1, 2, 3}, ModeQuote); err != nil { // before Start
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	eventually(t, "three connections", func() bool { return d.n() == 3 })
	if err := m.Start(ctx); err == nil {
		t.Fatal("second Start accepted")
	}
	time.Sleep(20 * time.Millisecond)
	if n := d.n(); n != 3 {
		t.Fatalf("second Start dialled: %d dials", n)
	}
	m.Stop()
	waitGoroutines(t, m)
}

func TestFailedStartLaunchesNothing(t *testing.T) {
	d := &dialer{}
	clk := clock.NewFake(time.Date(2026, 10, 12, 10, 0, 0, 0, clock.IST))
	m := New(Options{Dial: d.dial, Clock: clk, PerConn: 1, MaxConns: 1, ClockRef: 256265})
	if err := m.Subscribe(ConsumerChart, "c", []uint32{1}, ModeQuote); err != nil { // budget full
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var eb *ErrBudget
	if err := m.Start(ctx); !errors.As(err, &eb) {
		t.Fatalf("Start err = %v, want *ErrBudget for the clock reference", err)
	}
	time.Sleep(20 * time.Millisecond)
	if n, p := d.n(), clk.PendingTimers(); n != 0 || p != 0 {
		t.Fatalf("failed Start launched work: %d dials, %d timers", n, p)
	}
	waitGoroutines(t, m)
}

func TestEmptiedOwnerLeavesNoUsageRow(t *testing.T) {
	r := newRig(t, Options{})
	_ = r.m.Subscribe(ConsumerChart, "c", []uint32{1, 2}, ModeQuote)
	r.m.Unsubscribe(ConsumerChart, "c", []uint32{1, 2})
	if u := r.m.Usage(); len(u.ByConsumer) != 0 || u.Used != 0 {
		t.Fatalf("usage %+v, want no rows", u)
	}
}
