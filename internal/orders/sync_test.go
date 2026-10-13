// internal/orders/sync_test.go
package orders

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/ratelimit"
	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/store"
	"tradeloom/internal/store/storetest"
)

type fakeReader struct {
	mu          sync.Mutex
	orders      kite.Orders
	ordersErr   error
	positions   kite.Positions
	holdings    kite.Holdings
	holdingsErr error
	ltp         kite.QuoteLTP
	ltpErr      error
	calls       map[string]int
	lastPrio    ratelimit.Priority
}

func (f *fakeReader) Orders(ctx context.Context) (kite.Orders, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls["orders"]++
	f.lastPrio = ratelimit.PriorityFrom(ctx)
	return append(kite.Orders(nil), f.orders...), f.ordersErr
}
func (f *fakeReader) Positions(context.Context) (kite.Positions, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls["positions"]++
	return f.positions, nil
}
func (f *fakeReader) Holdings(context.Context) (kite.Holdings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls["holdings"]++
	if f.holdingsErr != nil {
		return nil, f.holdingsErr
	}
	return append(kite.Holdings(nil), f.holdings...), nil
}
func (f *fakeReader) LTP(context.Context, ...string) (kite.QuoteLTP, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls["ltp"]++
	return f.ltp, f.ltpErr
}
func (f *fakeReader) count(k string) int       { f.mu.Lock(); defer f.mu.Unlock(); return f.calls[k] }
func (f *fakeReader) set(fn func(*fakeReader)) { f.mu.Lock(); defer f.mu.Unlock(); fn(f) }

type fakeLive struct {
	mu       sync.Mutex
	subs     map[uint32]bool
	subCalls int
	releases int
	subErr   error
	owners   map[string]bool
	delay    time.Duration // widens the window between a resubscribe's diff and its feed call
	entered  chan struct{} // when set, Subscribe signals here and then waits on gate
	gate     chan struct{}
}

func (f *fakeLive) pause() {
	f.mu.Lock()
	d := f.delay
	f.mu.Unlock()
	if d > 0 {
		time.Sleep(d)
	}
	runtime.Gosched()
}

func (f *fakeLive) Subscribe(_ ticker.Consumer, owner string, toks []uint32, _ ticker.Mode) error {
	f.pause()
	f.mu.Lock()
	entered, gate := f.entered, f.gate
	f.mu.Unlock()
	if entered != nil {
		entered <- struct{}{}
		<-gate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subCalls++
	if f.subErr != nil {
		return f.subErr
	}
	f.owners[owner] = true
	for _, t := range toks {
		f.subs[t] = true
	}
	return nil
}
func (f *fakeLive) Unsubscribe(_ ticker.Consumer, _ string, toks []uint32) {
	f.pause()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range toks {
		delete(f.subs, t)
	}
}
func (f *fakeLive) Release(_ ticker.Consumer, _ string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releases++
	f.subs = map[uint32]bool{}
}
func (f *fakeLive) calls() int    { f.mu.Lock(); defer f.mu.Unlock(); return f.subCalls }
func (f *fakeLive) released() int { f.mu.Lock(); defer f.mu.Unlock(); return f.releases }
func (f *fakeLive) snapshot() map[uint32]bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[uint32]bool{}
	for k, v := range f.subs {
		out[k] = v
	}
	return out
}

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}
func (l *lockedBuf) String() string { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }

type syncEnv struct {
	sync   *Sync
	reader *fakeReader
	live   *fakeLive
	clock  *clock.Fake
	bus    *bus.Bus
	repo   *store.OrdersCacheRepo
	logs   *lockedBuf
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

func newSyncEnv(t *testing.T, plan kite.DataPlan) *syncEnv {
	t.Helper()
	return newSyncEnvWith(t, plan, nil)
}

func newSyncEnvWith(t *testing.T, plan kite.DataPlan, opt func(*SyncDeps)) *syncEnv {
	t.Helper()
	repo := store.NewOrdersCacheRepo(storetest.Open(t)) // registers temp-dir cleanup first…
	ctx, cancel := context.WithCancel(context.Background())
	e := &syncEnv{
		reader: &fakeReader{calls: map[string]int{}},
		live:   &fakeLive{subs: map[uint32]bool{}, owners: map[string]bool{}},
		clock:  clock.NewFake(time.Date(2026, 10, 12, 10, 0, 0, 0, clock.IST)),
		bus:    bus.New(nil),
		repo:   repo,
		logs:   &lockedBuf{},
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	// …so this cleanup (LIFO) stops Run before the database directory is removed.
	t.Cleanup(func() {
		cancel()
		select {
		case <-e.done:
		case <-time.After(3 * time.Second):
		}
	})
	d := SyncDeps{Read: e.reader, Cache: e.repo, Bus: e.bus, Clock: e.clock, Live: e.live,
		Log: slog.New(slog.NewTextHandler(e.logs, nil))}
	if opt != nil {
		opt(&d)
	}
	e.sync = NewSync(d)
	e.sync.SetDataPlan(plan)
	return e
}

func (e *syncEnv) start() {
	go func() {
		defer close(e.done)
		e.sync.Run(e.ctx)
	}()
}

func waitFor[T any](t *testing.T, sub *bus.Sub[T], what string, pred func(T) bool) T {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case v := <-sub.C():
			if pred(v) {
				return v
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

// advanceUntil steps the fake clock until pred holds on a published value.
func advanceUntil[T any](t *testing.T, e *syncEnv, sub *bus.Sub[T], step time.Duration, what string, pred func(T) bool) T {
	t.Helper()
	for i := 0; i < 400; i++ {
		for drained := false; !drained; {
			select {
			case v := <-sub.C():
				if pred(v) {
					return v
				}
			default:
				drained = true
			}
		}
		e.clock.Advance(step)
		time.Sleep(3 * time.Millisecond)
	}
	t.Fatalf("never observed %s", what)
	var zero T
	return zero
}

const orderOpen = `{"order_id":"230","status":"OPEN","tradingsymbol":"INFY","exchange":"NSE","transaction_type":"BUY","product":"CNC","quantity":1,"filled_quantity":0,"order_timestamp":"2026-10-12 10:00:05","exchange_update_timestamp":"2026-10-12 10:00:05"}`
const orderDone = `{"order_id":"230","status":"COMPLETE","tradingsymbol":"INFY","exchange":"NSE","transaction_type":"BUY","product":"CNC","quantity":1,"filled_quantity":1,"order_timestamp":"2026-10-12 10:00:05","exchange_update_timestamp":"2026-10-12 10:00:10"}`

// FR-ORD-014 AC1 and FR-ORD-001 AC2: an order placed on Kite web appears within 30 s with no ticker at all.
func TestSync_OrderPlacedOnKiteWebAppearsWithin30s(t *testing.T) {
	e := newSyncEnv(t, kite.PlanConnect)
	sub := bus.Subscribe(e.bus, TopicBook, bus.SubOpts[BookSnapshot]{Name: "t", Buffer: 64})
	e.start()
	waitFor(t, sub, "first fresh book", func(b BookSnapshot) bool { return !b.Stale && len(b.Orders) == 0 })
	e.reader.set(func(f *fakeReader) { f.orders = mustJSON[kite.Orders](t, "["+orderOpen+"]") })
	e.clock.Advance(PollInterval)
	b := waitFor(t, sub, "order 230", func(b BookSnapshot) bool { return len(b.Orders) == 1 })
	if b.Orders[0].OrderID != "230" || b.Counts.Open != 1 {
		t.Fatalf("%+v", b)
	}
}

// FR-ORD-014: ticker order updates apply immediately; a stale update never overrides a newer state.
func TestSync_OrderUpdatesImmediateAndOutOfOrderIgnored(t *testing.T) {
	e := newSyncEnv(t, kite.PlanConnect)
	sub := bus.Subscribe(e.bus, TopicBook, bus.SubOpts[BookSnapshot]{Name: "t", Buffer: 64})
	e.start()
	waitFor(t, sub, "fresh book", func(b BookSnapshot) bool { return !b.Stale })
	bus.Publish(e.bus, kite.TopicOrderUpdate, mustJSON[kite.Order](t, orderDone))
	waitFor(t, sub, "complete", func(b BookSnapshot) bool { return len(b.Orders) == 1 && b.Orders[0].Status == "COMPLETE" })
	bus.Publish(e.bus, kite.TopicOrderUpdate, mustJSON[kite.Order](t, orderOpen))
	bus.Publish(e.bus, kite.TopicOrderUpdate, mustJSON[kite.Order](t, `{"order_id":"231","status":"OPEN","order_timestamp":"2026-10-12 10:01:00"}`))
	b := waitFor(t, sub, "order 231", func(b BookSnapshot) bool { return len(b.Orders) == 2 })
	for _, r := range b.Orders {
		if r.OrderID == "230" && r.Status != "COMPLETE" {
			t.Fatal("stale OPEN update overrode COMPLETE")
		}
	}
}

// FR-HOME-002 (refetch on CNC order updates) and FR-ORD-014 reconciliation.
func TestSync_CNCFillRefetchesHoldingsAndPositions(t *testing.T) {
	e := newSyncEnv(t, kite.PlanConnect)
	sub := bus.Subscribe(e.bus, TopicHoldings, bus.SubOpts[HoldingsSnapshot]{Name: "t", Buffer: 64})
	e.start()
	waitFor(t, sub, "first holdings", func(HoldingsSnapshot) bool { return true })
	bus.Publish(e.bus, kite.TopicOrderUpdate, mustJSON[kite.Order](t, orderDone))
	advanceUntil(t, e, sub, 500*time.Millisecond, "holdings refetch", func(HoldingsSnapshot) bool {
		return e.reader.count("holdings") >= 2 && e.reader.count("positions") >= 2
	})
}

func TestSync_PollReconcilesAndLogs(t *testing.T) {
	e := newSyncEnv(t, kite.PlanConnect)
	e.reader.set(func(f *fakeReader) { f.orders = mustJSON[kite.Orders](t, "["+orderOpen+"]") })
	sub := bus.Subscribe(e.bus, TopicBook, bus.SubOpts[BookSnapshot]{Name: "t", Buffer: 64})
	e.start()
	waitFor(t, sub, "open", func(b BookSnapshot) bool { return !b.Stale && len(b.Orders) == 1 })
	e.reader.set(func(f *fakeReader) { f.orders = mustJSON[kite.Orders](t, "["+orderDone+"]") })
	e.clock.Advance(PollInterval)
	waitFor(t, sub, "complete via poll", func(b BookSnapshot) bool { return len(b.Orders) == 1 && b.Orders[0].Status == "COMPLETE" })
	if !strings.Contains(e.logs.String(), "order state reconciled with Kite") {
		t.Fatalf("logs: %s", e.logs.String())
	}
}

const twoHoldings = `[
 {"tradingsymbol":"A","exchange":"NSE","instrument_token":1,"quantity":10,"average_price":100,"last_price":110,"close_price":105},
 {"tradingsymbol":"B","exchange":"NSE","instrument_token":2,"quantity":5,"average_price":200,"last_price":190,"close_price":200}]`

// FR-HOME-002 AC2: a tick updates the summary (published within 250 ms of fake time); tokens are subscribed.
func TestSync_TickUpdatesHoldingsSummary(t *testing.T) {
	e := newSyncEnv(t, kite.PlanConnect)
	e.reader.set(func(f *fakeReader) { f.holdings = mustJSON[kite.Holdings](t, twoHoldings) })
	sub := bus.Subscribe(e.bus, TopicHoldings, bus.SubOpts[HoldingsSnapshot]{Name: "t", Buffer: 64})
	e.start()
	waitFor(t, sub, "holdings", func(h HoldingsSnapshot) bool { return h.Summary.CurrentValuePaise == 205000 })
	if subs := e.live.snapshot(); !subs[1] || !subs[2] {
		t.Fatal("holding tokens not subscribed")
	}
	bus.Publish(e.bus, ticker.TopicTicks, []ticker.Tick{{Token: 1, LTP: 120}})
	advanceUntil(t, e, sub, 50*time.Millisecond, "summary 2150.00", func(h HoldingsSnapshot) bool {
		return h.Summary.CurrentValuePaise == 215000
	})
}

func TestSync_HoldingsRefreshEveryFiveMinutes(t *testing.T) {
	e := newSyncEnv(t, kite.PlanConnect)
	sub := bus.Subscribe(e.bus, TopicHoldings, bus.SubOpts[HoldingsSnapshot]{Name: "t", Buffer: 256})
	e.start()
	waitFor(t, sub, "holdings", func(HoldingsSnapshot) bool { return true })
	advanceUntil(t, e, sub, time.Minute, "second holdings fetch", func(HoldingsSnapshot) bool { return e.reader.count("holdings") >= 2 })
}

// Review Focus 4 / C-01: Personal plan polls /quote/ltp every 60 s and stops on a permission error.
func TestSync_PersonalPlan_PollsLTPAndStopsOnPermissionError(t *testing.T) {
	e := newSyncEnv(t, kite.PlanNone)
	e.reader.set(func(f *fakeReader) {
		f.holdings = mustJSON[kite.Holdings](t, `[{"tradingsymbol":"A","exchange":"NSE","instrument_token":1,"quantity":10,"average_price":100,"last_price":110,"close_price":105}]`)
		f.ltp = kite.QuoteLTP{"NSE:A": {InstrumentToken: 1, LastPrice: 130}}
	})
	sub := bus.Subscribe(e.bus, TopicHoldings, bus.SubOpts[HoldingsSnapshot]{Name: "t", Buffer: 256})
	e.start()
	advanceUntil(t, e, sub, 10*time.Second, "polled LTP 130", func(h HoldingsSnapshot) bool { return h.Summary.CurrentValuePaise == 130000 })
	if e.live.calls() != 0 {
		t.Fatal("no ticker subscriptions without a data plan")
	}
	e.reader.set(func(f *fakeReader) {
		f.ltpErr = kite.APIError("PermissionException", 403, "Insufficient permission for that call.")
	})
	before := e.reader.count("ltp")
	advanceUntil(t, e, sub, 10*time.Second, "one failing poll", func(HoldingsSnapshot) bool { return e.reader.count("ltp") > before })
	after := e.reader.count("ltp")
	for i := 0; i < 30; i++ {
		e.clock.Advance(10 * time.Second)
		time.Sleep(2 * time.Millisecond)
	}
	if e.reader.count("ltp") != after {
		t.Fatalf("LTP polled %d more times after a permission error", e.reader.count("ltp")-after)
	}
}

// NFR-14 / offline start: the cached book is shown stale when Kite is unreachable.
func TestSync_ShowsCachedBookWhenKiteUnreachable(t *testing.T) {
	e := newSyncEnv(t, kite.PlanConnect)
	if err := e.repo.ReplaceOrders(context.Background(), []store.CachedOrder{{KiteOrderID: "230", Status: "OPEN", Body: []byte(orderOpen)}}, 5); err != nil {
		t.Fatal(err)
	}
	e.reader.set(func(f *fakeReader) { f.ordersErr = kite.APIError("NetworkException", 503, "Request failed.") })
	sub := bus.Subscribe(e.bus, TopicBook, bus.SubOpts[BookSnapshot]{Name: "t", Buffer: 64})
	e.start()
	b := waitFor(t, sub, "cached book", func(b BookSnapshot) bool { return b.Stale && len(b.Orders) == 1 })
	if b.AsOfMs != 5 {
		t.Fatalf("asOf %d", b.AsOfMs)
	}
}

// --- Fix round 1 (Ruling 62) ---

// stop cancels Run and waits for it to return.
func (e *syncEnv) stop(t *testing.T) {
	t.Helper()
	e.cancel()
	select {
	case <-e.done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return")
	}
}

func subscribedOf(s *Sync) map[uint32]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[uint32]bool{}
	for k, v := range s.subscribed {
		out[k] = v
	}
	return out
}

func ltpOf(s *Sync, tok uint32) (float64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.ltp[tok]
	return v, ok
}

const onePosition = `{"net":[{"tradingsymbol":"C","exchange":"NSE","instrument_token":3,"product":"MIS","quantity":1,"average_price":50,"last_price":50}],"day":[]}`

// Ruling 62 (1): concurrent resubscribes leave the feed equal to s.subscribed; Run's exit empties both.
func TestSync_ConcurrentRefreshAndPlanChange_SubscriptionsConsistent(t *testing.T) {
	e := newSyncEnv(t, kite.PlanConnect)
	e.reader.set(func(f *fakeReader) {
		f.holdings = mustJSON[kite.Holdings](t, twoHoldings)
		f.positions = mustJSON[kite.Positions](t, onePosition)
	})
	sub := bus.Subscribe(e.bus, TopicHoldings, bus.SubOpts[HoldingsSnapshot]{Name: "t", Buffer: 64})
	e.start()
	waitFor(t, sub, "holdings", func(h HoldingsSnapshot) bool { return h.Summary.CurrentValuePaise == 205000 })
	e.live.mu.Lock()
	e.live.delay = 200 * time.Microsecond
	e.live.mu.Unlock()
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 150; i++ {
				if (i+g)%2 == 0 {
					e.sync.SetDataPlan(kite.PlanNone)
				} else {
					e.sync.SetDataPlan(kite.PlanConnect)
				}
				_ = e.sync.Refresh(e.ctx, ScopeHoldings|ScopePositions)
			}
		}(g)
	}
	wg.Wait()
	if got, want := e.live.snapshot(), subscribedOf(e.sync); !reflect.DeepEqual(got, want) {
		t.Fatalf("feed %v != sync %v", got, want)
	}
	e.stop(t)
	if got, want := e.live.snapshot(), subscribedOf(e.sync); len(got) != 0 || len(want) != 0 {
		t.Fatalf("after Run: feed %v sync %v", got, want)
	}
}

// Ruling 62 (2): a Subscribe failure leaves the tokens unsubscribed so a later poll retries them.
func TestSync_SubscribeErrorRetriedOnNextPoll(t *testing.T) {
	e := newSyncEnv(t, kite.PlanConnect)
	e.reader.set(func(f *fakeReader) { f.holdings = mustJSON[kite.Holdings](t, twoHoldings) })
	e.live.mu.Lock()
	e.live.subErr = errors.New("budget exhausted")
	e.live.mu.Unlock()
	sub := bus.Subscribe(e.bus, TopicPositions, bus.SubOpts[PositionsSnapshot]{Name: "t", Buffer: 256})
	hsub := bus.Subscribe(e.bus, TopicHoldings, bus.SubOpts[HoldingsSnapshot]{Name: "h", Buffer: 256})
	e.start()
	waitFor(t, hsub, "fresh holdings", func(h HoldingsSnapshot) bool { return !h.Stale })
	if got := subscribedOf(e.sync); len(got) != 0 {
		t.Fatalf("failed subscription recorded: %v", got)
	}
	if !strings.Contains(e.logs.String(), "subscribing portfolio tokens failed") {
		t.Fatalf("logs: %s", e.logs.String())
	}
	e.live.mu.Lock()
	e.live.subErr = nil
	e.live.mu.Unlock()
	advanceUntil(t, e, sub, PollInterval, "retried subscription", func(PositionsSnapshot) bool {
		s := e.live.snapshot()
		return s[1] && s[2]
	})
}

// Ruling 62 (2): the feed's "not running" answer means "remembered": recorded, no Warn.
func TestSync_SubscribeNotRunningIsRecorded(t *testing.T) {
	notRunning := errors.New("feed: not running")
	e := newSyncEnvWith(t, kite.PlanConnect, func(d *SyncDeps) { d.LiveNotRunning = notRunning })
	e.reader.set(func(f *fakeReader) { f.holdings = mustJSON[kite.Holdings](t, twoHoldings) })
	e.live.mu.Lock()
	e.live.subErr = fmt.Errorf("wrapped: %w", notRunning)
	e.live.mu.Unlock()
	sub := bus.Subscribe(e.bus, TopicHoldings, bus.SubOpts[HoldingsSnapshot]{Name: "t", Buffer: 64})
	e.start()
	waitFor(t, sub, "holdings", func(h HoldingsSnapshot) bool { return !h.Stale })
	if got := subscribedOf(e.sync); !got[1] || !got[2] {
		t.Fatalf("not-running subscription not recorded: %v", got)
	}
	if strings.Contains(e.logs.String(), "subscribing portfolio tokens failed") {
		t.Fatalf("warned on not-running: %s", e.logs.String())
	}
}

// Ruling 62 (3): Release is called when Run exits, even with nothing subscribed.
func TestSync_ReleaseOnRunExit(t *testing.T) {
	e := newSyncEnv(t, kite.PlanConnect)
	sub := bus.Subscribe(e.bus, TopicBook, bus.SubOpts[BookSnapshot]{Name: "t", Buffer: 64})
	e.start()
	waitFor(t, sub, "fresh book", func(b BookSnapshot) bool { return !b.Stale })
	e.stop(t)
	if e.live.released() == 0 {
		t.Fatal("Release not called on Run exit")
	}
}

// Ruling 62 (10): a closed bus ends Run (no spin on zero values) and releases the subscriptions.
func TestSync_ClosedBusEndsRun(t *testing.T) {
	e := newSyncEnv(t, kite.PlanConnect)
	e.reader.set(func(f *fakeReader) { f.holdings = mustJSON[kite.Holdings](t, twoHoldings) })
	sub := bus.Subscribe(e.bus, TopicHoldings, bus.SubOpts[HoldingsSnapshot]{Name: "t", Buffer: 64})
	e.start()
	waitFor(t, sub, "holdings", func(h HoldingsSnapshot) bool { return !h.Stale })
	e.bus.Close()
	select {
	case <-e.done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run kept running on a closed bus")
	}
	if e.live.released() == 0 || len(e.live.snapshot()) != 0 || len(subscribedOf(e.sync)) != 0 {
		t.Fatalf("subscriptions not released: released=%d feed=%v", e.live.released(), e.live.snapshot())
	}
	if n := len(e.bus.Stats()); n != 0 {
		t.Fatalf("%d bus subscriptions left", n)
	}
}

// Ruling 62 (4): holdings are stale until the first successful fetch of a run.
func TestSync_HoldingsStaleUntilFirstFetch(t *testing.T) {
	e := newSyncEnv(t, kite.PlanConnect)
	if !e.sync.Holdings().Stale {
		t.Fatal("holdings fresh before any fetch")
	}
	sub := bus.Subscribe(e.bus, TopicHoldings, bus.SubOpts[HoldingsSnapshot]{Name: "t", Buffer: 64})
	e.start()
	waitFor(t, sub, "fresh holdings", func(h HoldingsSnapshot) bool { return !h.Stale })
	if e.sync.Holdings().Stale {
		t.Fatal("holdings stale after a successful fetch")
	}
}

// Ruling 62 (6): a zero tick never becomes an LTP.
func TestSync_ZeroTickIgnored(t *testing.T) {
	e := newSyncEnv(t, kite.PlanConnect)
	e.reader.set(func(f *fakeReader) { f.holdings = mustJSON[kite.Holdings](t, twoHoldings) })
	sub := bus.Subscribe(e.bus, TopicHoldings, bus.SubOpts[HoldingsSnapshot]{Name: "t", Buffer: 64})
	e.start()
	waitFor(t, sub, "holdings", func(h HoldingsSnapshot) bool { return h.Summary.CurrentValuePaise == 205000 })
	bus.Publish(e.bus, ticker.TopicTicks, []ticker.Tick{{Token: 1, LTP: 0}, {Token: 2, LTP: 180}})
	advanceUntil(t, e, sub, 50*time.Millisecond, "summary 2000.00", func(h HoldingsSnapshot) bool {
		return h.Summary.CurrentValuePaise == 200000
	})
	if v, ok := ltpOf(e.sync, 1); ok {
		t.Fatalf("zero tick stored as LTP %v", v)
	}
}

// Ruling 62 (6): a zero polled LTP never becomes an LTP.
func TestSync_ZeroPolledLTPIgnored(t *testing.T) {
	e := newSyncEnv(t, kite.PlanNone)
	e.reader.set(func(f *fakeReader) {
		f.holdings = mustJSON[kite.Holdings](t, `[{"tradingsymbol":"A","exchange":"NSE","instrument_token":1,"quantity":10,"average_price":100,"last_price":110,"close_price":105}]`)
		f.ltp = kite.QuoteLTP{"NSE:A": {InstrumentToken: 1, LastPrice: 0}}
	})
	sub := bus.Subscribe(e.bus, TopicHoldings, bus.SubOpts[HoldingsSnapshot]{Name: "t", Buffer: 256})
	e.start()
	advanceUntil(t, e, sub, 10*time.Second, "an LTP poll", func(HoldingsSnapshot) bool { return e.reader.count("ltp") >= 1 })
	if v, ok := ltpOf(e.sync, 1); ok {
		t.Fatalf("zero polled LTP stored as %v", v)
	}
}

// Ruling 62: a position that closes (quantity 0) loses its live subscription.
func TestSync_ClosedPositionUnsubscribed(t *testing.T) {
	e := newSyncEnv(t, kite.PlanConnect)
	e.reader.set(func(f *fakeReader) { f.positions = mustJSON[kite.Positions](t, onePosition) })
	sub := bus.Subscribe(e.bus, TopicPositions, bus.SubOpts[PositionsSnapshot]{Name: "t", Buffer: 64})
	e.start()
	waitFor(t, sub, "fresh positions", func(p PositionsSnapshot) bool { return !p.Stale && len(p.Net) == 1 })
	if !e.live.snapshot()[3] {
		t.Fatal("open position not subscribed")
	}
	e.reader.set(func(f *fakeReader) {
		f.positions = mustJSON[kite.Positions](t, `{"net":[{"tradingsymbol":"C","exchange":"NSE","instrument_token":3,"product":"MIS","quantity":0,"average_price":50,"last_price":51,"pnl":1}],"day":[]}`)
	})
	if err := e.sync.Refresh(e.ctx, ScopePositions); err != nil {
		t.Fatal(err)
	}
	if e.live.snapshot()[3] || subscribedOf(e.sync)[3] {
		t.Fatal("closed position still subscribed")
	}
}

// FR-ORD-014: the exchange-timestamp rule alone (both updates non-terminal).
func TestSync_EarlierExchangeTimestampIgnored(t *testing.T) {
	const later = `{"order_id":"230","status":"OPEN","tradingsymbol":"INFY","exchange":"NSE","transaction_type":"BUY","product":"MIS","quantity":2,"filled_quantity":1,"order_timestamp":"2026-10-12 10:00:05","exchange_update_timestamp":"2026-10-12 10:00:10"}`
	const earlier = `{"order_id":"230","status":"OPEN","tradingsymbol":"INFY","exchange":"NSE","transaction_type":"BUY","product":"MIS","quantity":2,"filled_quantity":0,"order_timestamp":"2026-10-12 10:00:05","exchange_update_timestamp":"2026-10-12 10:00:05"}`
	if newer(mustJSON[kite.Order](t, later), mustJSON[kite.Order](t, earlier)) {
		t.Fatal("newer() accepted an earlier exchange timestamp")
	}
	e := newSyncEnv(t, kite.PlanConnect)
	sub := bus.Subscribe(e.bus, TopicBook, bus.SubOpts[BookSnapshot]{Name: "t", Buffer: 64})
	e.start()
	waitFor(t, sub, "fresh book", func(b BookSnapshot) bool { return !b.Stale })
	bus.Publish(e.bus, kite.TopicOrderUpdate, mustJSON[kite.Order](t, later))
	waitFor(t, sub, "partial fill", func(b BookSnapshot) bool { return len(b.Orders) == 1 && b.Orders[0].FilledQty == 1 })
	bus.Publish(e.bus, kite.TopicOrderUpdate, mustJSON[kite.Order](t, earlier))
	bus.Publish(e.bus, kite.TopicOrderUpdate, mustJSON[kite.Order](t, `{"order_id":"231","status":"OPEN","order_timestamp":"2026-10-12 10:01:00"}`))
	b := waitFor(t, sub, "order 231", func(b BookSnapshot) bool { return len(b.Orders) == 2 })
	for _, r := range b.Orders {
		if r.OrderID == "230" && r.FilledQty != 1 {
			t.Fatal("earlier exchange update overrode a later one")
		}
	}
}

// Ruling 62 (7): a network failure marks the order book stale (NFR-14 offline).
func TestSync_NetworkFailureMarksBookStale(t *testing.T) {
	e := newSyncEnv(t, kite.PlanConnect)
	e.reader.set(func(f *fakeReader) { f.orders = mustJSON[kite.Orders](t, "["+orderOpen+"]") })
	sub := bus.Subscribe(e.bus, TopicBook, bus.SubOpts[BookSnapshot]{Name: "t", Buffer: 64})
	e.start()
	waitFor(t, sub, "fresh book", func(b BookSnapshot) bool { return !b.Stale && len(b.Orders) == 1 })
	e.reader.set(func(f *fakeReader) { f.ordersErr = kite.APIError("NetworkException", 503, "Request failed.") })
	e.clock.Advance(PollInterval)
	waitFor(t, sub, "stale book", func(b BookSnapshot) bool { return b.Stale && len(b.Orders) == 1 })
	if !e.sync.Book().Stale {
		t.Fatal("Book() not stale after a network failure")
	}
}

// Ruling 62 (8): background polls queue below interactive requests; Refresh keeps the caller's priority.
func TestSync_BackgroundPollsUseLowerPriority(t *testing.T) {
	e := newSyncEnv(t, kite.PlanConnect)
	sub := bus.Subscribe(e.bus, TopicBook, bus.SubOpts[BookSnapshot]{Name: "t", Buffer: 64})
	e.start()
	waitFor(t, sub, "fresh book", func(b BookSnapshot) bool { return !b.Stale })
	before := e.reader.count("orders")
	e.clock.Advance(PollInterval)
	advanceUntil(t, e, sub, time.Millisecond, "poll", func(BookSnapshot) bool { return e.reader.count("orders") > before })
	e.reader.mu.Lock()
	got := e.reader.lastPrio
	e.reader.mu.Unlock()
	if got != pollPriority || got == ratelimit.Interactive {
		t.Fatalf("poll priority %v", got)
	}
	if err := e.sync.Refresh(context.Background(), ScopeOrders); err != nil {
		t.Fatal(err)
	}
	e.reader.mu.Lock()
	got = e.reader.lastPrio
	e.reader.mu.Unlock()
	if got != ratelimit.Interactive {
		t.Fatalf("Refresh priority %v", got)
	}
}

// Ruling 62 (1), deterministic: a drop computed while an add is still in the feed call must not overtake it.
func TestSync_DropWaitsForInFlightSubscribe(t *testing.T) {
	e := newSyncEnv(t, kite.PlanNone)
	e.reader.set(func(f *fakeReader) { f.holdings = mustJSON[kite.Holdings](t, twoHoldings) })
	sub := bus.Subscribe(e.bus, TopicHoldings, bus.SubOpts[HoldingsSnapshot]{Name: "t", Buffer: 64})
	e.start()
	waitFor(t, sub, "holdings", func(h HoldingsSnapshot) bool { return !h.Stale })
	entered, gate := make(chan struct{}), make(chan struct{})
	e.live.mu.Lock()
	e.live.entered, e.live.gate = entered, gate
	e.live.mu.Unlock()
	addDone, dropDone := make(chan struct{}), make(chan struct{})
	go func() { defer close(addDone); e.sync.SetDataPlan(kite.PlanConnect) }()
	<-entered // the add is inside Live.Subscribe
	go func() { defer close(dropDone); e.sync.SetDataPlan(kite.PlanNone) }()
	select { // without subMu the drop finishes here, before the add reaches the feed
	case <-dropDone:
	case <-time.After(50 * time.Millisecond):
	}
	close(gate)
	<-addDone
	<-dropDone
	if got, want := e.live.snapshot(), subscribedOf(e.sync); !reflect.DeepEqual(got, want) {
		t.Fatalf("feed %v != sync %v", got, want)
	}
}

// Task 20 fix round 1 (Ruling 65): a network failure marks holdings stale, like orders and positions.
func TestSync_NetworkFailureMarksHoldingsStale(t *testing.T) {
	e := newSyncEnv(t, kite.PlanConnect)
	e.reader.set(func(f *fakeReader) { f.holdings = mustJSON[kite.Holdings](t, twoHoldings) })
	sub := bus.Subscribe(e.bus, TopicHoldings, bus.SubOpts[HoldingsSnapshot]{Name: "t", Buffer: 64})
	e.start()
	waitFor(t, sub, "fresh holdings", func(h HoldingsSnapshot) bool { return !h.Stale && len(h.Rows) == 2 })
	e.reader.set(func(f *fakeReader) { f.holdingsErr = kite.APIError("NetworkException", 503, "Request failed.") })
	if err := e.sync.Refresh(context.Background(), ScopeHoldings); err == nil {
		t.Fatal("expected the refresh error")
	}
	waitFor(t, sub, "stale holdings", func(h HoldingsSnapshot) bool { return h.Stale && len(h.Rows) == 2 })
	if !e.sync.Holdings().Stale {
		t.Fatal("Holdings() not stale after a network failure")
	}
}
