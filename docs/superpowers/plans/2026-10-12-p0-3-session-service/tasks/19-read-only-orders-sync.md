> **P0-3 Session & Service Layer Implementation Plan — Task 19.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 19: Read-only `orders.Sync`

**Files:**
- Create: `internal/orders/sync.go`
- Test: `internal/orders/sync_test.go`

**Interfaces:**
- Consumes: Tasks 12 (builders, `Scope`) and 18; P0-2 `kite.TopicOrderUpdate` ("order.update", `kite.Order`), `ticker.TopicTicks` ("tick.batch", `[]ticker.Tick{Token, LTP, …}`), `ticker.ConsumerWidget`, `ticker.ModeQuote`, `kite.Order/Orders/Position/Positions/Holding/Holdings/QuoteLTP` (verified: `QuoteLTP map[string]struct{InstrumentToken int; LastPrice float64}`), `clock.Timer`; P0-1 `bus.SubOpts[T]`, `bus.Unsubscribe`; `kite.IsNoDataPermission` (Task 9); `(*feed.Feed).Subscribe/Unsubscribe/Release` (Task 6) through `LiveTokens`.
- Produces:
  ```go
  type Reader interface{ Orders(ctx context.Context) (kite.Orders, error); Positions(ctx context.Context) (kite.Positions, error)
      Holdings(ctx context.Context) (kite.Holdings, error); LTP(ctx context.Context, ids ...string) (kite.QuoteLTP, error) } // *kite.Client
  type LiveTokens interface{ Subscribe(c ticker.Consumer, owner string, tokens []uint32, m ticker.Mode) error; Unsubscribe(c ticker.Consumer, owner string, tokens []uint32); Release(c ticker.Consumer, owner string) } // *feed.Feed
  const PortfolioOwner = "portfolio" // ticker owner id of this Sync's ConsumerWidget subscriptions
  var TopicBook bus.Topic[BookSnapshot]           // "orders.changed"
  var TopicPositions bus.Topic[PositionsSnapshot] // "positions.changed"
  var TopicHoldings bus.Topic[HoldingsSnapshot]   // "holdings.changed"
  const PollInterval = 30 * time.Second; const HoldingsInterval = 5 * time.Minute; const LTPPollInterval = 60 * time.Second
  type SyncDeps struct{ Read Reader; Cache *store.OrdersCacheRepo; Bus *bus.Bus; Clock clock.Clock; Live LiveTokens; Log *slog.Logger }
  func NewSync(d SyncDeps) *Sync
  func (s *Sync) Run(ctx context.Context) // session-scoped (started/stopped by service.Run)
  func (s *Sync) SetDataPlan(p kite.DataPlan)
  func (s *Sync) Refresh(ctx context.Context, sc Scope) error
  func (s *Sync) Book() BookSnapshot; func (s *Sync) Positions() PositionsSnapshot; func (s *Sync) Holdings() HoldingsSnapshot
  ```

Behaviour:
- On start: publish the cached order book and positions (`Stale=true`), then fetch everything.
- **Orders and positions poll every 30 s while the session is ready, whatever the feed state.** This is a superset of FR-ORD-014's "while orders are open". It guarantees FR-ORD-001 AC2 and FR-ORD-014 AC1 even when an order update was missed, at a cost of 2 requests per 30 s against a 10 req/s budget.
- Holdings are fetched every 5 minutes, and 2 s after an order update fills a CNC order. Positions are fetched 2 s after any fill.
- Ticker order updates apply immediately. An update that is older than the cached state (terminal → non-terminal, or an earlier `exchange_update_timestamp`) is ignored.
- A poll replaces the local state wholesale (Kite wins). When it changes state learned from updates, an INFO log `order state reconciled with Kite` records the count.
- Holdings and net-position tokens are subscribed for live LTP (`ConsumerWidget`, `quote`) on a Connect plan. Snapshots are re-published at most every 250 ms.
- On the Personal plan, LTP is polled through `GET /quote/ltp` every 60 s (C-01). A no-data permission error switches the polling off for the session.

- [ ] **Step 1: Write the failing tests**

```go
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
func (f *fakeReader) count(k string) int { f.mu.Lock(); defer f.mu.Unlock(); return f.calls[k] }
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

func (l *lockedBuf) Write(p []byte) (int, error) { l.mu.Lock(); defer l.mu.Unlock(); return l.b.Write(p) }
func (l *lockedBuf) String() string              { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }

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
	e.reader.set(func(f *fakeReader) { f.ltpErr = kite.APIError("PermissionException", 403, "Insufficient permission for that call.") })
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/orders/ -run 'TestSync' -v`
Expected: FAIL to compile with `undefined: NewSync`.

- [ ] **Step 3: Write the implementation**

```go
// internal/orders/sync.go
package orders

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/store"
)

type Reader interface {
	Orders(ctx context.Context) (kite.Orders, error)
	Positions(ctx context.Context) (kite.Positions, error)
	Holdings(ctx context.Context) (kite.Holdings, error)
	LTP(ctx context.Context, ids ...string) (kite.QuoteLTP, error)
}

// LiveTokens is *feed.Feed (P0-2 ticker owners: this Sync subscribes as ConsumerWidget / PortfolioOwner).
type LiveTokens interface {
	Subscribe(c ticker.Consumer, owner string, tokens []uint32, m ticker.Mode) error
	Unsubscribe(c ticker.Consumer, owner string, tokens []uint32)
	Release(c ticker.Consumer, owner string)
}

var (
	TopicBook      = bus.NewTopic[BookSnapshot]("orders.changed")
	TopicPositions = bus.NewTopic[PositionsSnapshot]("positions.changed")
	TopicHoldings  = bus.NewTopic[HoldingsSnapshot]("holdings.changed")
)

const (
	PollInterval     = 30 * time.Second
	HoldingsInterval = 5 * time.Minute
	LTPPollInterval  = 60 * time.Second
	PortfolioOwner   = "portfolio"
	publishDelay     = 250 * time.Millisecond
	settleDelay      = 2 * time.Second
	ltpChunk         = 500
)

// timerC returns a timer's channel, or a nil channel (never ready) for an unarmed timer.
func timerC(t clock.Timer) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C()
}

func stopTimer(t clock.Timer) {
	if t != nil {
		t.Stop()
	}
}

type SyncDeps struct {
	Read  Reader
	Cache *store.OrdersCacheRepo
	Bus   *bus.Bus
	Clock clock.Clock
	Live  LiveTokens
	Log   *slog.Logger
}

// Sync is the read-only part of internal/orders for P0 (ARCH §12 row 5d). It never writes to Kite.
type Sync struct {
	d SyncDeps

	mu          sync.Mutex
	orders      map[string]kite.Order
	pos         kite.Positions
	hold        kite.Holdings
	ltp         map[uint32]float64
	ordersAt    int64
	posAt       int64
	holdAt      int64
	ordersStale bool
	posStale    bool
	plan        kite.DataPlan
	ltpOff      bool
	subscribed  map[uint32]bool
}

func NewSync(d SyncDeps) *Sync {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Sync{d: d, orders: map[string]kite.Order{}, ltp: map[uint32]float64{}, subscribed: map[uint32]bool{}, plan: kite.PlanUnknown}
}

func (s *Sync) SetDataPlan(p kite.DataPlan) {
	s.mu.Lock()
	s.plan = p
	if p != kite.PlanNone {
		s.ltpOff = false
	}
	s.mu.Unlock()
	s.resubscribe()
}

func (s *Sync) orderList() []kite.Order {
	out := make([]kite.Order, 0, len(s.orders))
	for _, o := range s.orders {
		out = append(out, o)
	}
	return out
}

func (s *Sync) Book() BookSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return BuildBook(s.orderList(), s.ordersAt, s.ordersStale)
}

func (s *Sync) Positions() PositionsSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return BuildPositions(s.pos, s.ltp, s.posAt, s.posStale)
}

func (s *Sync) Holdings() HoldingsSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return BuildHoldings(s.hold, s.ltp, s.holdAt, false)
}

func (s *Sync) Refresh(ctx context.Context, sc Scope) error { return s.refresh(ctx, sc) }

func (s *Sync) Run(ctx context.Context) {
	upd := bus.Subscribe(s.d.Bus, kite.TopicOrderUpdate, bus.SubOpts[kite.Order]{Name: "orders.sync.updates", Buffer: 256})
	ticks := bus.Subscribe(s.d.Bus, ticker.TopicTicks, bus.SubOpts[[]ticker.Tick]{Name: "orders.sync.ticks", Buffer: 16})
	defer bus.Unsubscribe(s.d.Bus, upd)
	defer bus.Unsubscribe(s.d.Bus, ticks)
	defer s.unsubscribeAll()

	s.loadCache(ctx)
	clk := s.d.Clock
	poll := clk.At(clk.Now().Add(PollInterval))
	hold := clk.At(clk.Now().Add(HoldingsInterval))
	ltpT := clk.At(clk.Now().Add(LTPPollInterval))
	var (
		pub, settle clock.Timer // nil while not armed
		settleScope Scope
	)
	defer func() {
		for _, t := range []clock.Timer{poll, hold, ltpT, pub, settle} {
			stopTimer(t)
		}
	}()
	s.logErr(s.refresh(ctx, ScopeAll))
	for {
		select {
		case <-ctx.Done():
			return
		case o := <-upd.C():
			sc, book, changed := s.applyUpdate(o)
			if !changed {
				continue
			}
			if sc != 0 {
				settleScope |= sc
				if settle == nil {
					settle = clk.At(clk.Now().Add(settleDelay))
				}
			}
			s.persistOrder(ctx, o)
			bus.Publish(s.d.Bus, TopicBook, book)
		case batch := <-ticks.C():
			if s.applyTicks(batch) && pub == nil {
				pub = clk.At(clk.Now().Add(publishDelay))
			}
		case <-timerC(pub):
			pub = nil
			s.publishLive()
		case <-timerC(settle):
			settle = nil
			sc := settleScope
			settleScope = 0
			s.logErr(s.refresh(ctx, sc))
		case <-poll.C():
			poll = clk.At(clk.Now().Add(PollInterval))
			s.logErr(s.refresh(ctx, ScopeOrders|ScopePositions))
		case <-hold.C():
			hold = clk.At(clk.Now().Add(HoldingsInterval))
			s.logErr(s.refresh(ctx, ScopeHoldings))
		case <-ltpT.C():
			ltpT = clk.At(clk.Now().Add(LTPPollInterval))
			s.pollLTP(ctx)
		}
	}
}

func (s *Sync) logErr(err error) {
	if err != nil {
		s.d.Log.Warn("portfolio refresh failed", "err", err)
	}
}

func (s *Sync) loadCache(ctx context.Context) {
	cos, err := s.d.Cache.ListOrders(ctx)
	if err != nil {
		s.d.Log.Warn("reading the orders cache failed", "err", err)
	}
	cps, err := s.d.Cache.ListPositions(ctx)
	if err != nil {
		s.d.Log.Warn("reading the positions cache failed", "err", err)
	}
	oAt, pAt, _ := s.d.Cache.LastUpdated(ctx)
	m := map[string]kite.Order{}
	for _, c := range cos {
		var o kite.Order
		if json.Unmarshal(c.Body, &o) == nil && o.OrderID != "" {
			m[o.OrderID] = o
		}
	}
	var pos kite.Positions
	for _, c := range cps {
		var p kite.Position
		if json.Unmarshal(c.Body, &p) != nil {
			continue
		}
		if strings.HasPrefix(c.K, "day|") {
			pos.Day = append(pos.Day, p)
		} else {
			pos.Net = append(pos.Net, p)
		}
	}
	s.mu.Lock()
	s.orders, s.ordersAt, s.ordersStale = m, oAt, true
	s.pos, s.posAt, s.posStale = pos, pAt, true
	book := BuildBook(s.orderList(), s.ordersAt, true)
	ps := BuildPositions(s.pos, s.ltp, s.posAt, true)
	s.mu.Unlock()
	bus.Publish(s.d.Bus, TopicBook, book)
	bus.Publish(s.d.Bus, TopicPositions, ps)
}

func (s *Sync) refresh(ctx context.Context, sc Scope) error {
	var errs []error
	if sc&ScopeOrders != 0 {
		if os, err := s.d.Read.Orders(ctx); err != nil {
			errs = append(errs, fmt.Errorf("orders: %w", err))
		} else {
			s.reconcileOrders(ctx, os)
		}
	}
	if sc&ScopePositions != 0 {
		if ps, err := s.d.Read.Positions(ctx); err != nil {
			errs = append(errs, fmt.Errorf("positions: %w", err))
		} else {
			s.setPositions(ctx, ps)
		}
	}
	if sc&ScopeHoldings != 0 {
		if hs, err := s.d.Read.Holdings(ctx); err != nil {
			errs = append(errs, fmt.Errorf("holdings: %w", err))
		} else {
			s.setHoldings(hs)
		}
	}
	return errors.Join(errs...)
}

// reconcileOrders replaces local state with Kite's order list (Kite wins) and logs differences (FR-ORD-014).
func (s *Sync) reconcileOrders(ctx context.Context, os kite.Orders) {
	now := s.d.Clock.Now().UnixMilli()
	next := make(map[string]kite.Order, len(os))
	s.mu.Lock()
	changed := 0
	for _, o := range os {
		next[o.OrderID] = o
		if p, ok := s.orders[o.OrderID]; !ok || p.Status != o.Status || p.FilledQuantity != o.FilledQuantity {
			changed++
		}
	}
	for id := range s.orders {
		if _, ok := next[id]; !ok {
			changed++
		}
	}
	hadLive := !s.ordersStale && s.ordersAt != 0
	s.orders, s.ordersAt, s.ordersStale = next, now, false
	book := BuildBook(s.orderList(), now, false)
	s.mu.Unlock()
	if hadLive && changed > 0 {
		s.d.Log.Info("order state reconciled with Kite", "changed", changed)
	}
	rows := make([]store.CachedOrder, 0, len(os))
	for _, o := range os {
		b, _ := json.Marshal(o)
		rows = append(rows, store.CachedOrder{KiteOrderID: o.OrderID, Tag: o.Tag, Status: o.Status, Body: b})
	}
	if err := s.d.Cache.ReplaceOrders(ctx, rows, now); err != nil {
		s.d.Log.Warn("writing the orders cache failed", "err", err)
	}
	bus.Publish(s.d.Bus, TopicBook, book)
}

func newer(prev, next kite.Order) bool {
	if terminal(prev.Status) && !terminal(next.Status) {
		return false
	}
	pt, nt := prev.ExchangeUpdateTimestamp.Time, next.ExchangeUpdateTimestamp.Time
	if !pt.IsZero() && !nt.IsZero() && nt.Before(pt) {
		return false
	}
	return true
}

func (s *Sync) applyUpdate(o kite.Order) (Scope, BookSnapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, ok := s.orders[o.OrderID]
	if ok && !newer(prev, o) {
		return 0, BookSnapshot{}, false
	}
	s.orders[o.OrderID] = o
	var sc Scope
	if strings.EqualFold(o.Status, statusComplete) || o.FilledQuantity > prev.FilledQuantity {
		sc |= ScopePositions
		if strings.EqualFold(o.Product, "CNC") {
			sc |= ScopeHoldings
		}
	}
	return sc, BuildBook(s.orderList(), s.ordersAt, s.ordersStale), true
}

func (s *Sync) persistOrder(ctx context.Context, o kite.Order) {
	b, _ := json.Marshal(o)
	if err := s.d.Cache.UpsertOrder(ctx, store.CachedOrder{KiteOrderID: o.OrderID, Tag: o.Tag, Status: o.Status, Body: b}, s.d.Clock.Now().UnixMilli()); err != nil {
		s.d.Log.Warn("writing an order update to the cache failed", "err", err)
	}
}

func (s *Sync) setPositions(ctx context.Context, ps kite.Positions) {
	now := s.d.Clock.Now().UnixMilli()
	s.mu.Lock()
	s.pos, s.posAt, s.posStale = ps, now, false
	for _, p := range append(append([]kite.Position{}, ps.Net...), ps.Day...) {
		delete(s.ltp, p.InstrumentToken)
	}
	snap := BuildPositions(s.pos, s.ltp, now, false)
	s.mu.Unlock()
	var rows []store.CachedPosition
	for kind, list := range map[string][]kite.Position{"net": ps.Net, "day": ps.Day} {
		for _, p := range list {
			b, _ := json.Marshal(p)
			rows = append(rows, store.CachedPosition{K: positionKey(kind, p), Body: b})
		}
	}
	if err := s.d.Cache.ReplacePositions(ctx, rows, now); err != nil {
		s.d.Log.Warn("writing the positions cache failed", "err", err)
	}
	s.resubscribe()
	bus.Publish(s.d.Bus, TopicPositions, snap)
}

func (s *Sync) setHoldings(hs kite.Holdings) {
	now := s.d.Clock.Now().UnixMilli()
	s.mu.Lock()
	s.hold, s.holdAt = hs, now
	for _, h := range hs {
		delete(s.ltp, h.InstrumentToken)
	}
	snap := BuildHoldings(s.hold, s.ltp, now, false)
	s.mu.Unlock()
	s.resubscribe()
	bus.Publish(s.d.Bus, TopicHoldings, snap)
}

// resubscribe keeps the ticker subscriptions equal to holdings ∪ net positions on a Connect plan.
func (s *Sync) resubscribe() {
	s.mu.Lock()
	want := map[uint32]bool{}
	if s.plan == kite.PlanConnect && s.d.Live != nil {
		for _, h := range s.hold {
			want[h.InstrumentToken] = true
		}
		for _, p := range s.pos.Net {
			want[p.InstrumentToken] = true
		}
	}
	var add, drop []uint32
	for t := range want {
		if !s.subscribed[t] {
			add = append(add, t)
		}
	}
	for t := range s.subscribed {
		if !want[t] {
			drop = append(drop, t)
		}
	}
	s.subscribed = want
	s.mu.Unlock()
	sort.Slice(add, func(i, j int) bool { return add[i] < add[j] })
	if len(add) > 0 {
		if err := s.d.Live.Subscribe(ticker.ConsumerWidget, PortfolioOwner, add, ticker.ModeQuote); err != nil {
			s.d.Log.Warn("subscribing portfolio tokens failed", "err", err)
		}
	}
	if len(drop) > 0 {
		s.d.Live.Unsubscribe(ticker.ConsumerWidget, PortfolioOwner, drop)
	}
}

// unsubscribeAll releases this Sync's whole ticker ownership (session ended).
func (s *Sync) unsubscribeAll() {
	s.mu.Lock()
	had := len(s.subscribed) > 0
	s.subscribed = map[uint32]bool{}
	s.mu.Unlock()
	if had && s.d.Live != nil {
		s.d.Live.Release(ticker.ConsumerWidget, PortfolioOwner)
	}
}

func (s *Sync) applyTicks(batch []ticker.Tick) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for _, t := range batch {
		if s.subscribed[t.Token] {
			s.ltp[t.Token] = t.LTP
			changed = true
		}
	}
	return changed
}

func (s *Sync) publishLive() {
	s.mu.Lock()
	ps := BuildPositions(s.pos, s.ltp, s.posAt, s.posStale)
	hs := BuildHoldings(s.hold, s.ltp, s.holdAt, false)
	s.mu.Unlock()
	bus.Publish(s.d.Bus, TopicPositions, ps)
	bus.Publish(s.d.Bus, TopicHoldings, hs)
}

// pollLTP is the Personal-plan fallback (FR-HOME-002 data dependency, C-01).
func (s *Sync) pollLTP(ctx context.Context) {
	s.mu.Lock()
	off := s.plan != kite.PlanNone || s.ltpOff || len(s.hold) == 0
	ids := make([]string, 0, len(s.hold))
	for _, h := range s.hold {
		ids = append(ids, h.Exchange+":"+h.Tradingsymbol)
	}
	s.mu.Unlock()
	if off {
		return
	}
	for i := 0; i < len(ids); i += ltpChunk {
		end := i + ltpChunk
		if end > len(ids) {
			end = len(ids)
		}
		q, err := s.d.Read.LTP(ctx, ids[i:end]...)
		if err != nil {
			if kite.IsNoDataPermission(err) {
				s.mu.Lock()
				s.ltpOff = true
				s.mu.Unlock()
				s.d.Log.Info("LTP polling is not available on this API plan; holdings use Kite's last price, refreshed every 5 minutes")
			} else {
				s.d.Log.Warn("LTP poll failed", "err", err)
			}
			return
		}
		s.mu.Lock()
		for _, v := range q {
			s.ltp[uint32(v.InstrumentToken)] = v.LastPrice
		}
		s.mu.Unlock()
	}
	s.publishLive()
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/orders/ -v`
Expected: PASS (all builder and sync tests; `-race` clean).

- [ ] **Step 5: Commit**

```bash
git add internal/orders/sync.go internal/orders/sync_test.go
git commit -m "feat(orders): read-only Sync with polling, order updates and LTP overlay

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
