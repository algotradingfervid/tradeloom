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
	"tradeloom/internal/kite/ratelimit"
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
	// pollPriority queues background polls behind interactive requests (ARCH §5 priorities).
	pollPriority = ratelimit.Recorder
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
	// LiveNotRunning is the error Live.Subscribe returns when it remembered the tokens but no feed runs
	// (feed.ErrNotRunning); such a subscription counts as recorded. orders must not import feed.
	LiveNotRunning error
}

// Sync is the read-only part of internal/orders for P0 (ARCH §12 row 5d). It never writes to Kite.
type Sync struct {
	d SyncDeps

	runMu sync.Mutex // serialises Run: a new Run waits for the previous Run's cleanup
	subMu sync.Mutex // held across a subscription diff and its Live calls; lock order subMu → mu

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
	holdStale   bool // true until the first successful holdings fetch of the current Run
	ordersLive  bool // a fetch succeeded since the cache was loaded (reconciliation log baseline)
	running     bool // inside Run: only then are ticker subscriptions held
	plan        kite.DataPlan
	ltpOff      bool
	subscribed  map[uint32]bool
}

func NewSync(d SyncDeps) *Sync {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Sync{d: d, orders: map[string]kite.Order{}, ltp: map[uint32]float64{}, subscribed: map[uint32]bool{}, plan: kite.PlanUnknown, holdStale: true}
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
	return BuildHoldings(s.hold, s.ltp, s.holdAt, s.holdStale)
}

func (s *Sync) Refresh(ctx context.Context, sc Scope) error { return s.refresh(ctx, sc) }

func (s *Sync) Run(ctx context.Context) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	s.mu.Lock()
	s.hold, s.holdAt, s.holdStale = nil, 0, true
	s.ltp, s.ltpOff = map[uint32]float64{}, false
	s.running = true
	s.mu.Unlock()

	upd := bus.Subscribe(s.d.Bus, kite.TopicOrderUpdate, bus.SubOpts[kite.Order]{Name: "orders.sync.updates", Buffer: 256})
	ticks := bus.Subscribe(s.d.Bus, ticker.TopicTicks, bus.SubOpts[[]ticker.Tick]{Name: "orders.sync.ticks", Buffer: 16})
	defer bus.Unsubscribe(s.d.Bus, upd)
	defer bus.Unsubscribe(s.d.Bus, ticks)
	defer s.unsubscribeAll()

	s.loadCache(ctx)
	bg := ratelimit.WithPriority(ctx, pollPriority) // background polls queue behind interactive requests
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
		case o, ok := <-upd.C():
			if !ok {
				return // bus closed
			}
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
		case batch, ok := <-ticks.C():
			if !ok {
				return // bus closed
			}
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
			s.logErr(s.refresh(bg, sc))
		case <-poll.C():
			poll = clk.At(clk.Now().Add(PollInterval))
			s.logErr(s.refresh(bg, ScopeOrders|ScopePositions))
		case <-hold.C():
			hold = clk.At(clk.Now().Add(HoldingsInterval))
			s.logErr(s.refresh(bg, ScopeHoldings))
		case <-ltpT.C():
			ltpT = clk.At(clk.Now().Add(LTPPollInterval))
			s.pollLTP(bg)
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
	oAt, pAt, err := s.d.Cache.LastUpdated(ctx)
	if err != nil {
		s.d.Log.Warn("reading the cache sync times failed", "err", err)
	}
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
	s.orders, s.ordersAt, s.ordersStale, s.ordersLive = m, oAt, true, false
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
			s.markStaleIfOffline(err, ScopeOrders)
		} else {
			s.reconcileOrders(ctx, os)
		}
	}
	if sc&ScopePositions != 0 {
		if ps, err := s.d.Read.Positions(ctx); err != nil {
			errs = append(errs, fmt.Errorf("positions: %w", err))
			s.markStaleIfOffline(err, ScopePositions)
		} else {
			s.setPositions(ctx, ps)
		}
	}
	if sc&ScopeHoldings != 0 {
		if hs, err := s.d.Read.Holdings(ctx); err != nil {
			errs = append(errs, fmt.Errorf("holdings: %w", err))
			s.markStaleIfOffline(err, ScopeHoldings)
		} else {
			s.setHoldings(hs)
		}
	}
	return errors.Join(errs...)
}

// markStaleIfOffline flags the order book, positions or holdings stale when Kite could not be reached (NFR-14)
// and publishes the stale snapshot so the UI shows it.
func (s *Sync) markStaleIfOffline(err error, sc Scope) {
	if kite.Classify(err) != kite.KindNetwork {
		return
	}
	s.mu.Lock()
	switch sc {
	case ScopeOrders:
		already := s.ordersStale
		s.ordersStale = true
		book := BuildBook(s.orderList(), s.ordersAt, true)
		s.mu.Unlock()
		if !already {
			bus.Publish(s.d.Bus, TopicBook, book)
		}
	case ScopePositions:
		already := s.posStale
		s.posStale = true
		ps := BuildPositions(s.pos, s.ltp, s.posAt, true)
		s.mu.Unlock()
		if !already {
			bus.Publish(s.d.Bus, TopicPositions, ps)
		}
	case ScopeHoldings:
		already := s.holdStale
		s.holdStale = true
		hs := BuildHoldings(s.hold, s.ltp, s.holdAt, true)
		s.mu.Unlock()
		if !already {
			bus.Publish(s.d.Bus, TopicHoldings, hs)
		}
	default:
		s.mu.Unlock()
	}
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
	hadLive := s.ordersLive
	s.orders, s.ordersAt, s.ordersStale, s.ordersLive = next, now, false, true
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
	s.hold, s.holdAt, s.holdStale = hs, now, false
	for _, h := range hs {
		delete(s.ltp, h.InstrumentToken)
	}
	snap := BuildHoldings(s.hold, s.ltp, now, false)
	s.mu.Unlock()
	s.resubscribe()
	bus.Publish(s.d.Bus, TopicHoldings, snap)
}

// resubscribe keeps the ticker subscriptions equal to holdings ∪ open net positions on a Connect plan
// while Run runs. subMu covers the diff and the Live calls, so concurrent resubscribes reach the feed in
// the order they computed their diffs.
func (s *Sync) resubscribe() {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	s.mu.Lock()
	want := map[uint32]bool{}
	if s.running && s.plan == kite.PlanConnect && s.d.Live != nil {
		for _, h := range s.hold {
			want[h.InstrumentToken] = true
		}
		for _, p := range s.pos.Net {
			if p.Quantity != 0 { // a closed position needs no live price
				want[p.InstrumentToken] = true
			}
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
	if s.d.Live == nil {
		return
	}
	sort.Slice(add, func(i, j int) bool { return add[i] < add[j] })
	if len(add) > 0 {
		err := s.d.Live.Subscribe(ticker.ConsumerWidget, PortfolioOwner, add, ticker.ModeQuote)
		if err != nil && !(s.d.LiveNotRunning != nil && errors.Is(err, s.d.LiveNotRunning)) {
			// Not subscribed: forget the tokens so the next resubscribe (poll) retries them.
			s.mu.Lock()
			for _, t := range add {
				delete(s.subscribed, t)
			}
			s.mu.Unlock()
			s.d.Log.Warn("subscribing portfolio tokens failed", "err", err, "tokens", len(add))
		}
	}
	if len(drop) > 0 {
		s.d.Live.Unsubscribe(ticker.ConsumerWidget, PortfolioOwner, drop)
	}
}

// unsubscribeAll releases this Sync's whole ticker ownership (Run ended), always, so tokens the feed
// remembered while not running are forgotten too.
func (s *Sync) unsubscribeAll() {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	s.mu.Lock()
	s.subscribed = map[uint32]bool{}
	s.running = false
	s.mu.Unlock()
	if s.d.Live != nil {
		s.d.Live.Release(ticker.ConsumerWidget, PortfolioOwner)
	}
}

func (s *Sync) applyTicks(batch []ticker.Tick) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for _, t := range batch {
		if t.LTP > 0 && s.subscribed[t.Token] {
			s.ltp[t.Token] = t.LTP
			changed = true
		}
	}
	return changed
}

func (s *Sync) publishLive() {
	s.mu.Lock()
	ps := BuildPositions(s.pos, s.ltp, s.posAt, s.posStale)
	hs := BuildHoldings(s.hold, s.ltp, s.holdAt, s.holdStale)
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
			if v.LastPrice > 0 {
				s.ltp[uint32(v.InstrumentToken)] = v.LastPrice
			}
		}
		s.mu.Unlock()
	}
	s.publishLive()
}
