// internal/service/quotes_test.go
package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/instruments"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/feed"
	"tradeloom/internal/kite/ratelimit"
	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/store"
	"tradeloom/internal/store/storetest"
)

type fakeQuotes struct {
	mu    sync.Mutex
	ltp   kite.QuoteLTP
	ohlc  kite.QuoteOHLC
	err   error
	calls map[string]int
	ids   []string
}

func (f *fakeQuotes) LTP(_ context.Context, ids ...string) (kite.QuoteLTP, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls["ltp"]++
	f.ids = append([]string(nil), ids...)
	return f.ltp, f.err
}
func (f *fakeQuotes) OHLC(_ context.Context, ids ...string) (kite.QuoteOHLC, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls["ohlc"]++
	return f.ohlc, f.err
}
func (f *fakeQuotes) count(k string) int { f.mu.Lock(); defer f.mu.Unlock(); return f.calls[k] }

func withQuotes(t *testing.T, e *svcEnv) *fakeQuotes {
	t.Helper()
	fq := &fakeQuotes{calls: map[string]int{}}
	if err := json.Unmarshal([]byte(`{"NSE:SBIN":{"instrument_token":1,"last_price":100}}`), &fq.ltp); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"NSE:SBIN":{"instrument_token":1,"last_price":100,"ohlc":{"open":99,"high":101,"low":97,"close":98}}}`), &fq.ohlc); err != nil {
		t.Fatal(err)
	}
	e.instr.m[1] = instruments.Instrument{Token: 1, Exchange: "NSE", Tradingsymbol: "SBIN", DisplayName: "SBIN"}
	e.svc.d.Quotes = fq
	return fq
}

// runQuotes starts RunQuotes and returns a stop function that waits for its exit (the cache flush).
func runQuotes(t *testing.T, e *svcEnv) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	armed := e.clock.PendingTimers()
	go func() {
		defer close(done)
		e.svc.RunQuotes(ctx)
	}()
	// RunQuotes arms its poll and flush timers on the app clock; an Advance before that would be lost.
	if !e.clock.WaitTimers(armed+2, 3*time.Second) {
		t.Fatal("RunQuotes did not arm its timers")
	}
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("RunQuotes did not stop")
		}
	}
}

// FR-WATCH-006 AC1 (backend): A-only tokens are unsubscribed 30 s after a switch, B's immediately subscribed.
func TestWatchQuotes_LingerThenUnsubscribe(t *testing.T) {
	e := newSvcEnv(t)
	withQuotes(t, e)
	ctx := context.Background()
	res, err := e.svc.WatchQuotes(ctx, "watchlist", []uint32{1, 2})
	if err != nil || len(res.Live) != 2 || len(res.Polled) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
	if _, err := e.svc.WatchQuotes(ctx, "watchlist", []uint32{2, 3}); err != nil {
		t.Fatal(err)
	}
	if !e.live.has(ticker.ConsumerWatchlist, 3) || !e.live.has(ticker.ConsumerWatchlist, 1) {
		t.Fatal("3 must be subscribed now and 1 must linger")
	}
	e.clock.Advance(29 * time.Second)
	time.Sleep(20 * time.Millisecond)
	if !e.live.has(ticker.ConsumerWatchlist, 1) {
		t.Fatal("unsubscribed before 30 s")
	}
	e.clock.Advance(time.Second)
	eventually(t, "token 1 unsubscribed", func() bool { return !e.live.has(ticker.ConsumerWatchlist, 1) })
	e.svc.WatchQuotes(ctx, "watchlist", []uint32{3})
	e.svc.WatchQuotes(ctx, "watchlist", []uint32{2, 3}) // 2 re-added within the linger window
	e.clock.Advance(31 * time.Second)
	time.Sleep(20 * time.Millisecond)
	if !e.live.has(ticker.ConsumerWatchlist, 2) {
		t.Fatal("re-added token was unsubscribed")
	}
	if _, err := e.svc.WatchQuotes(ctx, "scanner", nil); refusedCode(t, err) != "invalid_consumer" {
		t.Fatal(err)
	}
	e.live.err = &ticker.ErrBudget{Used: 9000, Requested: 1, Limit: 9000}
	res, err = e.svc.WatchQuotes(ctx, "chart", []uint32{9})
	if err != nil || len(res.Polled) != 1 || res.Polled[0] != 9 || len(res.Live) != 0 {
		t.Fatalf("over budget must poll: %+v %v", res, err)
	}
	e.live.err = feed.ErrNotRunning // before login: remembered by the feed, polled meanwhile
	res, err = e.svc.WatchQuotes(ctx, "widget", []uint32{10})
	if err != nil || len(res.Polled) != 1 || res.Polled[0] != 10 {
		t.Fatalf("not running must poll: %+v %v", res, err)
	}
	e.live.err = nil
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanNone})
	res, err = e.svc.WatchQuotes(ctx, "chart", []uint32{11})
	if err != nil || len(res.Polled) != 1 || e.live.has(ticker.ConsumerChart, 11) {
		t.Fatalf("personal plan must poll without subscribing: %+v %v", res, err)
	}
}

// C-01 / FR-ACCT-007: polled tokens produce tick.batch entries with polled: true every 5 s on the app clock.
func TestWatchQuotes_PolledFallbackPublishesTicks(t *testing.T) {
	e := newSvcEnv(t)
	fq := withQuotes(t, e)
	ch := startEvents(t, e)
	stop := runQuotes(t, e)
	defer stop()
	ctx := context.Background()
	e.live.err = &ticker.ErrBudget{Used: 9000, Requested: 1, Limit: 9000}
	if res, err := e.svc.WatchQuotes(ctx, "watchlist", []uint32{1}); err != nil || len(res.Polled) != 1 || len(res.Snapshot) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
	e.clock.Advance(QuotePollInterval)
	eventually(t, "one poll", func() bool { return fq.count("ltp") == 1 && fq.count("ohlc") == 1 })
	got := advanceUntilEvent(t, e, ch, EvTickBatch).payload.([]TickDTO)
	if len(got) != 1 || got[0].Token != 1 || got[0].LTP != 100 || got[0].Close != 98 || got[0].Change != 2 || !got[0].Polled || got[0].TsMs == 0 {
		t.Fatalf("%+v", got)
	}
	fq.mu.Lock()
	ids := fq.ids
	fq.mu.Unlock()
	if len(ids) != 1 || ids[0] != "NSE:SBIN" {
		t.Fatalf("ids %v", ids)
	}
	e.clock.Advance(QuotePollInterval)
	eventually(t, "second poll", func() bool { return fq.count("ltp") >= 2 })
	if fq.count("ohlc") != 1 {
		t.Fatal("the previous close is fetched once per token, not every round")
	}
	res, err := e.svc.WatchQuotes(ctx, "watchlist", []uint32{1})
	if err != nil || len(res.Snapshot) != 1 || res.Snapshot[0].LTP != 100 || !res.Snapshot[0].Polled {
		t.Fatalf("snapshot %+v %v", res, err)
	}
	e.session.setState(kite.SessionState{State: kite.StateExpired})
	time.Sleep(20 * time.Millisecond)
	before := fq.count("ltp")
	e.clock.Advance(QuotePollInterval)
	time.Sleep(20 * time.Millisecond)
	if fq.count("ltp") != before {
		t.Fatal("polled while not logged in")
	}
}

// FR-SHELL-004 AC5: the last quotes survive a restart through window_state key quotes.last.
func TestWatchQuotes_SnapshotSurvivesRestart(t *testing.T) {
	db := storetest.Open(t)
	e := newSvcEnv(t)
	withQuotes(t, e)
	e.svc.d.UiState = store.NewWindowStateRepo(db)
	ch := startEvents(t, e)
	stop := runQuotes(t, e)
	ctx := context.Background()
	if _, err := e.svc.WatchQuotes(ctx, "widget", []uint32{1}); err != nil {
		t.Fatal(err)
	}
	bus.Publish(e.bus, ticker.TopicTicks, []ticker.Tick{{Token: 1, LTP: 123, Close: 120, ExchangeTS: ist(2026, 10, 12, 10, 0)}})
	time.Sleep(20 * time.Millisecond)
	advanceUntilEvent(t, e, ch, EvTickBatch)
	stop() // flushes the cache
	raw, err := store.NewWindowStateRepo(db).Get(ctx, QuoteCacheKey)
	if err != nil || raw == "" {
		t.Fatalf("cache not persisted: %q %v", raw, err)
	}

	e2 := newSvcEnv(t)
	withQuotes(t, e2)
	e2.svc.d.UiState = store.NewWindowStateRepo(db)
	res, err := e2.svc.WatchQuotes(ctx, "widget", []uint32{1, 2})
	if err != nil || len(res.Snapshot) != 1 || res.Snapshot[0].Token != 1 || res.Snapshot[0].LTP != 123 || res.Snapshot[0].Close != 120 ||
		res.Snapshot[0].Polled || res.Snapshot[0].TsMs != ist(2026, 10, 12, 10, 0).UnixMilli() {
		t.Fatalf("restored snapshot %+v %v", res, err)
	}
}

type prioQuotes struct {
	fakeQuotes
	prio []ratelimit.Priority
}

func (f *prioQuotes) LTP(ctx context.Context, ids ...string) (kite.QuoteLTP, error) {
	f.mu.Lock()
	f.prio = append(f.prio, ratelimit.PriorityFrom(ctx))
	f.mu.Unlock()
	return f.fakeQuotes.LTP(ctx, ids...)
}

// Ruling 62 pattern: background polls queue behind interactive requests, and a zero LTP is never published.
func TestWatchQuotes_PollIsBackgroundAndSkipsZeroLTP(t *testing.T) {
	e := newSvcEnv(t)
	base := withQuotes(t, e)
	fq := &prioQuotes{fakeQuotes: fakeQuotes{calls: map[string]int{}, ohlc: base.ohlc}}
	if err := json.Unmarshal([]byte(`{"NSE:SBIN":{"instrument_token":1,"last_price":0},"NSE:INFY":{"instrument_token":2,"last_price":50}}`), &fq.ltp); err != nil {
		t.Fatal(err)
	}
	e.instr.m[2] = instruments.Instrument{Token: 2, Exchange: "NSE", Tradingsymbol: "INFY"}
	e.svc.d.Quotes = fq
	sub := bus.Subscribe(e.bus, TopicPolledTicks, bus.SubOpts[[]TickDTO]{Name: "test.polled", Buffer: 8})
	stop := runQuotes(t, e)
	defer stop()
	e.live.err = &ticker.ErrBudget{Used: 9000, Requested: 2, Limit: 9000}
	if _, err := e.svc.WatchQuotes(context.Background(), "watchlist", []uint32{1, 2}); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(QuotePollInterval)
	select {
	case got, ok := <-sub.C():
		if !ok {
			t.Fatal("bus closed")
		}
		if len(got) != 1 || got[0].Token != 2 || got[0].LTP != 50 {
			t.Fatalf("zero LTP must be skipped: %+v", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no polled ticks")
	}
	fq.mu.Lock()
	defer fq.mu.Unlock()
	if len(fq.prio) != 1 || fq.prio[0] == ratelimit.Interactive {
		t.Fatalf("poll priority %v, want a background priority", fq.prio)
	}
}
