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
	ctx     context.Context    // derived in Start; cancelled by Stop
	cancel  context.CancelFunc // ends the flusher and every connection loop
	started bool
	stopped bool
	wg      sync.WaitGroup // the flusher and the connection loops
	owners  map[ownerKey]map[uint32]Mode
	assign  map[uint32]int
	slots   []*slot
	retry   chan struct{} // closed and replaced by Reconnect: wakes every Offline connection at once

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
		retry: make(chan struct{}), latest: map[uint32]Tick{}, raws: map[int]chan Tick{}}
	for i := 0; i < o.MaxConns; i++ {
		m.slots = append(m.slots, &slot{idx: i, state: "stopped", tokens: map[uint32]Mode{}})
	}
	return m
}

// Start opens connection 1 (and every connection that already holds tokens subscribed before
// Start), subscribes the clock reference, and starts the tick flusher. Further connections open
// only when the previous one is full. Start may be called once; if it fails nothing was launched.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started || m.stopped {
		m.mu.Unlock()
		return fmt.Errorf("ticker: Start called on a started or stopped manager")
	}
	m.started = true // claims Start; reset below if the clock reference is refused
	m.mu.Unlock()
	if m.o.ClockRef != 0 { // m.ctx is still nil: this only registers and assigns, launches nothing
		if err := m.Subscribe(ConsumerClock, "clock", []uint32{m.o.ClockRef}, ModeFull); err != nil {
			m.mu.Lock()
			m.started = false
			m.mu.Unlock()
			return err
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped { // Stop ran while the clock reference was being registered
		return fmt.Errorf("ticker: manager stopped during Start")
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	runCtx := m.ctx
	m.goLocked("ticker.flush", func() { m.flushLoop(runCtx) })
	for i, s := range m.slots {
		if i == 0 || len(s.tokens) > 0 {
			m.startSlot(s)
		}
	}
	return nil
}

// goLocked starts a tracked goroutine (caller holds mu).
func (m *Manager) goLocked(name string, fn func()) {
	m.wg.Add(1)
	logging.Go(name, func() {
		defer m.wg.Done()
		fn()
	})
}

// Stop closes every connection (logout, shutdown). A Manager serves one session: Stop is final
// and a new Manager is built with the next access token.
// It cancels the flusher and every connection loop; none publishes after the "stopped" event.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped {
		return
	}
	m.stopped = true
	if m.cancel != nil {
		m.cancel()
	}
	for _, s := range m.slots {
		if s.conn != nil {
			s.conn.Stop()
		}
		s.live, s.state = false, "stopped"
	}
	m.publishFeedLocked()
}

// Reconnect restarts connections that went Offline (user "Retry now").
// It wakes every Offline connection; a call while none is Offline has no later effect.
func (m *Manager) Reconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()
	close(m.retry)
	m.retry = make(chan struct{})
}

// Subscribe adds tokens for (c, owner) at mode. It is all-or-nothing: if the union of all
// subscriptions would exceed MaxConns × PerConn tokens it returns *ErrBudget and changes nothing.
// No subscription is ever evicted to make room.
func (m *Manager) Subscribe(c Consumer, owner string, tokens []uint32, mode Mode) error {
	if mode < ModeLTP || mode > ModeFull {
		return fmt.Errorf("ticker: invalid subscription mode %d", mode)
	}
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
		if len(set) == 0 {
			delete(m.owners, ownerKey{c, owner})
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

// startSlot launches the connect/reconnect loop of s and publishes "connecting" (caller holds mu).
// Before Start (m.ctx nil) it does nothing; Start launches every slot that holds tokens.
func (m *Manager) startSlot(s *slot) {
	if s.started || m.ctx == nil || m.stopped {
		return
	}
	s.started, s.state = true, "connecting"
	ctx := m.ctx
	m.goLocked(fmt.Sprintf("ticker.conn.%d", s.idx), func() { m.run(ctx, s) })
	m.publishFeedLocked()
}

func (m *Manager) run(ctx context.Context, s *slot) {
	for ctx.Err() == nil {
		m.mu.Lock()
		stopped := m.stopped
		m.mu.Unlock()
		if stopped {
			return
		}
		var c Conn
		c = m.o.Dial(Events{
			OnConnect:     func() { m.onConnect(s, c) },
			OnTick:        m.onTick,
			OnOrderUpdate: func(o kiteconnect.Order) { publish(m.o.Bus, kite.TopicOrderUpdate, o) },
			OnError:       func(err error) { m.o.Log.Info("ticker error", "conn", s.idx, "err", err) },
		})
		m.mu.Lock()
		if m.stopped { // Stop ran while this goroutine waited or dialled: never serve after Stop
			m.mu.Unlock()
			c.Stop()
			return
		}
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
			retry := m.retry
			m.publishFeedLocked()
			m.mu.Unlock()
			m.o.Log.Warn("ticker offline", "conn", s.idx, "attempts", att-1)
			select {
			case <-ctx.Done():
				return
			case <-retry:
				m.mu.Lock()
				if ctx.Err() != nil || m.stopped { // Reconnect raced with Stop: stay stopped, publish nothing
					m.mu.Unlock()
					return
				}
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
	if m.stopped { // connected after Stop: close it so no live socket outlives the manager
		c.Stop()
		return
	}
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
// and a cancel function. A full buffer drops ticks and counts them in Usage.RawDrops. The channel
// is closed when cancel is called (cancel is idempotent); it is not closed by Stop.
func (m *Manager) Raw(buffer int) (<-chan Tick, func()) {
	ch := make(chan Tick, buffer)
	m.tickMu.Lock()
	id := m.rawSeq
	m.rawSeq++
	m.raws[id] = ch
	m.tickMu.Unlock()
	return ch, func() {
		m.tickMu.Lock()
		defer m.tickMu.Unlock()
		if _, ok := m.raws[id]; ok { // sends happen under tickMu, so closing here is safe
			delete(m.raws, id)
			close(ch)
		}
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
