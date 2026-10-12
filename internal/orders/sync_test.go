// internal/orders/sync_test.go
package orders

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/store"
	"tradeloom/internal/store/storetest"
)

type fakeReader struct {
	mu        sync.Mutex
	orders    kite.Orders
	ordersErr error
	positions kite.Positions
	holdings  kite.Holdings
	ltp       kite.QuoteLTP
	ltpErr    error
	calls     map[string]int
}

func (f *fakeReader) Orders(context.Context) (kite.Orders, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls["orders"]++
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
	owners   map[string]bool
}

func (f *fakeLive) Subscribe(_ ticker.Consumer, owner string, toks []uint32, _ ticker.Mode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subCalls++
	f.owners[owner] = true
	for _, t := range toks {
		f.subs[t] = true
	}
	return nil
}
func (f *fakeLive) Unsubscribe(_ ticker.Consumer, _ string, toks []uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range toks {
		delete(f.subs, t)
	}
}
func (f *fakeLive) Release(_ ticker.Consumer, owner string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.owners[owner] {
		f.subs = map[uint32]bool{}
	}
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
	done   chan struct{}
}

func newSyncEnv(t *testing.T, plan kite.DataPlan) *syncEnv {
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
	e.sync = NewSync(SyncDeps{Read: e.reader, Cache: e.repo, Bus: e.bus, Clock: e.clock, Live: e.live,
		Log: slog.New(slog.NewTextHandler(e.logs, nil))})
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
	e.live.mu.Lock()
	subscribed := e.live.subs[1] && e.live.subs[2]
	e.live.mu.Unlock()
	if !subscribed {
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
	if e.live.subCalls != 0 {
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
