// internal/service/quotes_test.go
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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

// Ruling 73 (1): the previous close comes from OHLC once per token per session; a restored Close is only a fallback.
func TestWatchQuotes_PreviousCloseFetchedPerSession(t *testing.T) {
	db := storetest.Open(t)
	if err := store.NewWindowStateRepo(db).Set(context.Background(), QuoteCacheKey,
		`{"1":{"token":1,"ltp":123,"close":120,"tsMs":1}}`); err != nil {
		t.Fatal(err)
	}
	e := newSvcEnv(t)
	fq := withQuotes(t, e)
	e.svc.d.UiState = store.NewWindowStateRepo(db)
	e.live.err = &ticker.ErrBudget{Used: 9000, Requested: 1, Limit: 9000}
	if _, err := e.svc.WatchQuotes(context.Background(), "watchlist", []uint32{1}); err != nil {
		t.Fatal(err)
	}
	sub := bus.Subscribe(e.bus, TopicPolledTicks, bus.SubOpts[[]TickDTO]{Name: "test.polled", Buffer: 8})
	next := func() []TickDTO {
		t.Helper()
		select {
		case got, ok := <-sub.C():
			if !ok {
				t.Fatal("bus closed")
			}
			return got
		case <-time.After(3 * time.Second):
			t.Fatal("no polled ticks")
		}
		return nil
	}
	e.svc.pollQuotes(context.Background())
	if got := next(); len(got) != 1 || got[0].Close != 98 || got[0].Change != 2 || fq.count("ohlc") != 1 {
		t.Fatalf("fresh close not used: %+v ohlc=%d", got, fq.count("ohlc"))
	}
	e.svc.pollQuotes(context.Background())
	next()
	if fq.count("ohlc") != 1 {
		t.Fatal("OHLC fetched twice in one session")
	}
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanConnect, SessionID: "s2"})
	e.svc.pollQuotes(context.Background())
	next()
	if fq.count("ohlc") != 2 {
		t.Fatal("a new session must fetch the previous close again")
	}
	// OHLC without the token (the cache still holds the restored quote, no event forwarder runs here): the
	// cached close is the fallback.
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanConnect, SessionID: "s3"})
	fq.mu.Lock()
	fq.ohlc = nil
	fq.mu.Unlock()
	e.svc.pollQuotes(context.Background())
	if got := next(); len(got) != 1 || got[0].Close != 120 || got[0].Change != -20 {
		t.Fatalf("cached close fallback: %+v", got)
	}
}

// Ruling 73 (2): a no-data-permission refusal stops polling for that login and resumes on the next one.
func TestWatchQuotes_NoDataPermissionStopsPollingUntilNextLogin(t *testing.T) {
	e := newSvcEnv(t)
	fq := withQuotes(t, e)
	fq.err = kite.APIError("PermissionException", 403, "Insufficient permission for that call.")
	e.live.err = &ticker.ErrBudget{Used: 9000, Requested: 1, Limit: 9000}
	if _, err := e.svc.WatchQuotes(context.Background(), "watchlist", []uint32{1}); err != nil {
		t.Fatal(err)
	}
	calls := func() int { return fq.count("ohlc") + fq.count("ltp") }
	e.svc.pollQuotes(context.Background())
	if calls() != 1 {
		t.Fatalf("calls %d after the refusal, want 1", calls())
	}
	e.svc.pollQuotes(context.Background())
	e.svc.pollQuotes(context.Background())
	if calls() != 1 {
		t.Fatalf("kept polling after a permission refusal: %d calls", calls())
	}
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanConnect, SessionID: "s2"})
	e.svc.pollQuotes(context.Background())
	if calls() != 2 {
		t.Fatalf("a new login must resume polling: %d calls", calls())
	}
}

// Ruling 73 (3): remove → re-add → remove inside the linger window waits 30 s from the LAST removal.
func TestWatchQuotes_LingerRestartsOnSecondRemoval(t *testing.T) {
	e := newSvcEnv(t)
	withQuotes(t, e)
	ctx := context.Background()
	watch := func(toks ...uint32) {
		t.Helper()
		if _, err := e.svc.WatchQuotes(ctx, "watchlist", toks); err != nil {
			t.Fatal(err)
		}
	}
	watch(1, 2)
	watch(2) // t0: 1 removed
	e.clock.Advance(10 * time.Second)
	watch(1, 2) // t0+10: re-added
	e.clock.Advance(10 * time.Second)
	watch(2) // t0+20: removed again
	e.clock.Advance(10 * time.Second)
	time.Sleep(20 * time.Millisecond)
	if !e.live.has(ticker.ConsumerWatchlist, 1) {
		t.Fatal("unsubscribed at t0+30 s, only 10 s after the last removal")
	}
	e.clock.Advance(19 * time.Second)
	time.Sleep(20 * time.Millisecond)
	if !e.live.has(ticker.ConsumerWatchlist, 1) {
		t.Fatal("unsubscribed before t0+50 s")
	}
	e.clock.Advance(time.Second)
	eventually(t, "token 1 unsubscribed at t0+50 s", func() bool { return !e.live.has(ticker.ConsumerWatchlist, 1) })
}

// Ruling 73 (4): a failed subscription leaves the consumer's set as it was, so a retry subscribes again.
func TestWatchQuotes_SubscriptionFailedKeepsPreviousSet(t *testing.T) {
	e := newSvcEnv(t)
	withQuotes(t, e)
	ctx := context.Background()
	if _, err := e.svc.WatchQuotes(ctx, "watchlist", []uint32{1}); err != nil {
		t.Fatal(err)
	}
	e.live.err = errors.New("ticker write failed")
	if _, err := e.svc.WatchQuotes(ctx, "watchlist", []uint32{2}); refusedCode(t, err) != "subscription_failed" {
		t.Fatal(err)
	}
	e.live.err = nil
	e.clock.Advance(UnsubscribeLinger + time.Second)
	time.Sleep(20 * time.Millisecond)
	if !e.live.has(ticker.ConsumerWatchlist, 1) {
		t.Fatal("a refused change must not schedule the old set's removal")
	}
	res, err := e.svc.WatchQuotes(ctx, "watchlist", []uint32{1, 2})
	if err != nil || len(res.Live) != 2 || !e.live.has(ticker.ConsumerWatchlist, 2) {
		t.Fatalf("retry did not subscribe token 2: %+v %v", res, err)
	}
}

// Ruling 73 (5): pending linger goroutines end (and release their timers) when RunQuotes exits.
func TestWatchQuotes_LingerStopsWithRunQuotes(t *testing.T) {
	e := newSvcEnv(t)
	withQuotes(t, e)
	stop := runQuotes(t, e)
	ctx := context.Background()
	e.svc.WatchQuotes(ctx, "watchlist", []uint32{1})
	e.svc.WatchQuotes(ctx, "widget", []uint32{2})
	e.svc.WatchQuotes(ctx, "watchlist", nil)
	e.svc.WatchQuotes(ctx, "widget", nil)
	if n := e.clock.PendingTimers(); n != 4 {
		t.Fatalf("pending timers %d, want 2 (RunQuotes) + 2 (linger)", n)
	}
	stop()
	eventually(t, "linger timers released", func() bool { return e.clock.PendingTimers() == 0 })
	if !e.live.has(ticker.ConsumerWatchlist, 1) {
		t.Fatal("shutdown must not unsubscribe")
	}
}

// chunkQuotes answers the first LTP chunk and fails the second one.
type chunkQuotes struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (f *chunkQuotes) LTP(_ context.Context, ids ...string) (kite.QuoteLTP, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.calls > 1 {
		return nil, f.err
	}
	m := map[string]map[string]any{}
	for i, id := range ids {
		m[id] = map[string]any{"instrument_token": 1000 + i, "last_price": 10}
	}
	raw, _ := json.Marshal(m)
	var q kite.QuoteLTP
	if err := json.Unmarshal(raw, &q); err != nil {
		panic(err)
	}
	return q, nil
}
func (f *chunkQuotes) OHLC(context.Context, ...string) (kite.QuoteOHLC, error) { return nil, nil }

// lockedBuf is a goroutine-safe log sink.
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

// Ruling 73 (9, 10): a failed LTP chunk still publishes the chunks that succeeded; a cancelled poll is not a
// warning; tokens missing from the instrument master are logged at Debug.
func TestWatchQuotes_PartialChunksAndLogging(t *testing.T) {
	e := newSvcEnv(t)
	var logs lockedBuf
	e.svc.d.Log = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cq := &chunkQuotes{err: errors.New("network down")}
	e.svc.d.Quotes = cq
	toks := make([]uint32, 0, ltpChunk+2)
	for i := 0; i < ltpChunk+1; i++ {
		tok := uint32(1000 + i)
		e.instr.m[tok] = instruments.Instrument{Token: tok, Exchange: "NSE", Tradingsymbol: fmt.Sprintf("S%04d", i)}
		toks = append(toks, tok)
	}
	toks = append(toks, 99999) // not in the master
	e.live.err = &ticker.ErrBudget{Used: 9000, Requested: 1, Limit: 9000}
	if _, err := e.svc.WatchQuotes(context.Background(), "watchlist", toks); err != nil {
		t.Fatal(err)
	}
	sub := bus.Subscribe(e.bus, TopicPolledTicks, bus.SubOpts[[]TickDTO]{Name: "test.polled", Buffer: 8})
	e.svc.pollQuotes(context.Background())
	select {
	case got, ok := <-sub.C():
		if !ok {
			t.Fatal("bus closed")
		}
		if len(got) != ltpChunk {
			t.Fatalf("published %d quotes, want the %d of the chunk that succeeded", len(got), ltpChunk)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the successful chunk was not published")
	}
	out := logs.String()
	if !strings.Contains(out, "level=WARN") || !strings.Contains(out, "quote LTP poll failed") {
		t.Fatalf("a real failure must warn: %s", out)
	}
	if !strings.Contains(out, "level=DEBUG") || !strings.Contains(out, "99999") {
		t.Fatalf("missing-token debug log absent: %s", out)
	}

	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	cq.mu.Lock()
	cq.calls, cq.err = 1, context.Canceled
	cq.mu.Unlock()
	before := strings.Count(logs.String(), "level=WARN")
	e.svc.pollQuotes(cctx)
	if after := strings.Count(logs.String(), "level=WARN"); after != before {
		t.Fatalf("a cancelled poll logged a warning: %s", logs.String())
	}
}
