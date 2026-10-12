> **P0-3 Session & Service Layer Implementation Plan — Task 29.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 29: `Quotes.Watch` — per-consumer live sets, polled fallback and the persisted last-quote cache

**Files:**
- Create: `internal/service/quotes.go`
- Modify: `internal/service/deps.go` (Task 14: `QuoteReader`, one `Deps` field, the poll state on `Service`)
- Test: `internal/service/quotes_test.go`

**Interfaces:**
- Consumes: Task 14 `LiveSubscriber`, the `live`/`liveMu` want-sets on `Service`; Task 21 `TopicPolledTicks`, `LastQuotes`, `rememberTicks`, `FeedState`, `startEvents`/`advanceUntilEvent` (tests); Task 28 `store.WindowStateRepo`; P0-2 `ticker.ConsumerChart/ConsumerWatchlist/ConsumerWidget`, `ticker.ModeQuote`, `*ticker.ErrBudget`, `kite.ReadClient.LTP` / `OHLC` (`kite.QuoteLTP`, `kite.QuoteOHLC`), `kite.IsNoDataPermission` (Task 9); `feed.ErrNotRunning` (Task 6); `clock.Timer`.
- Produces:
  ```go
  type QuoteReader interface { LTP(ctx context.Context, ids ...string) (kite.QuoteLTP, error); OHLC(ctx context.Context, ids ...string) (kite.QuoteOHLC, error) } // *kite.Client
  // Deps gains: Quotes QuoteReader
  type WatchResult struct{ Live []uint32 `json:"live"`; Polled []uint32 `json:"polled"`; Snapshot []TickDTO `json:"snapshot"` } // TS service.WatchResult
  const UnsubscribeLinger = 30 * time.Second; const QuotePollInterval = 5 * time.Second; const QuoteCacheFlush = 60 * time.Second
  const QuoteCacheKey = "quotes.last" // window_state key: JSON map token → TickDTO
  func (s *Service) WatchQuotes(ctx context.Context, consumer string, tokens []uint32) (WatchResult, error) // invalid_consumer, subscription_failed
  func (s *Service) RunQuotes(ctx context.Context) // poll loop + cache flush; started by App.start, flushes on exit
  ```

`WatchQuotes(consumer, tokens)` replaces that consumer's whole set (`chart`, `watchlist`, `widget` → `ticker.ConsumerChart/ConsumerWatchlist/ConsumerWidget`; the ticker owner id is the consumer name, so consumers never unsubscribe each other). New tokens are subscribed at once; removed tokens are unsubscribed 30 s later unless re-added (FR-WATCH-006 AC1). Tokens the feed cannot carry fall back to REST polling: a `*ticker.ErrBudget` refusal, `feed.ErrNotRunning` (no login yet — the feed remembers the subscription and these tokens leave the poll set once `feed.state` is `live`), or DataPlan `none` (no WebSocket at all, C-01). `RunQuotes` polls every 5 s on the app clock while the session is ready: `OHLC` once per token for the previous close, `LTP` every round, published on `TopicPolledTicks` with `polled: true`, which Task 21 merges into `tick.batch`. The result's `Snapshot` is the last quote per token from the in-memory cache. That cache is persisted to `window_state` key `quotes.last` (at most once per 60 s while quotes change, and on shutdown) so the index ticker strip shows the last known values right after a restart (FR-SHELL-004 AC5); restored entries carry `polled: false` and their original `tsMs` until a live or polled tick replaces them.

- [ ] **Step 1: Write the failing tests**

```go
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
	go func() {
		defer close(done)
		e.svc.RunQuotes(ctx)
	}()
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/service/ -run 'TestWatchQuotes' -v`
Expected: FAIL to compile with `undefined: (*Service).WatchQuotes`, `e.svc.d.Quotes undefined`.

- [ ] **Step 3: Extend `deps.go`**

Add the interface and the `Deps` field (after `UiState`):

```go
// QuoteReader is *kite.Client's quote half (REST fallback for tokens the feed cannot carry).
type QuoteReader interface {
	LTP(ctx context.Context, ids ...string) (kite.QuoteLTP, error)
	OHLC(ctx context.Context, ids ...string) (kite.QuoteOHLC, error)
}
```

```go
	Quotes QuoteReader // *kite.Client (Task 29)
```

and to `Service` (after `lastFeed`):

```go
	quotesOnce sync.Once                            // loads the persisted quote cache
	pollMu     sync.Mutex
	polled     map[ticker.Consumer]map[uint32]bool // tokens on the REST path, per consumer
	deferred   map[uint32]bool                     // subscribed while the feed was stopped; polled until feed.state is live
```

initialised in `New` with `polled: map[ticker.Consumer]map[uint32]bool{}, deferred: map[uint32]bool{}`.

- [ ] **Step 4: Write the implementation**

```go
// internal/service/quotes.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/feed"
	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/logging"
)

const (
	quotesStage       = "quotes"
	UnsubscribeLinger = 30 * time.Second
	QuotePollInterval = 5 * time.Second
	QuoteCacheFlush   = 60 * time.Second
	QuoteCacheKey     = "quotes.last"
	ltpChunk          = 500
)

var liveConsumers = map[string]ticker.Consumer{
	"chart":     ticker.ConsumerChart,
	"watchlist": ticker.ConsumerWatchlist,
	"widget":    ticker.ConsumerWidget,
}

// WatchResult tells the UI which tokens are live, which are polled, and the last known quote of each.
type WatchResult struct {
	Live     []uint32  `json:"live"`
	Polled   []uint32  `json:"polled"`
	Snapshot []TickDTO `json:"snapshot"`
}

// WatchQuotes replaces a UI consumer's quote set (FR-WATCH-006): new tokens now, removed tokens after
// UnsubscribeLinger; tokens the feed cannot carry are polled by RunQuotes.
func (s *Service) WatchQuotes(_ context.Context, consumer string, tokens []uint32) (WatchResult, error) {
	c, ok := liveConsumers[consumer]
	if !ok {
		return WatchResult{}, refused(quotesStage, "invalid_consumer", "Unknown quote consumer "+consumer+".")
	}
	s.quotesOnce.Do(s.loadQuoteCache)
	want := make(map[uint32]bool, len(tokens))
	ordered := make([]uint32, 0, len(tokens))
	for _, t := range tokens {
		if !want[t] {
			want[t] = true
			ordered = append(ordered, t)
		}
	}
	s.liveMu.Lock()
	cur := s.live[c]
	var add, removed []uint32
	for _, t := range ordered {
		if !cur[t] {
			add = append(add, t)
		}
	}
	for t := range cur {
		if !want[t] {
			removed = append(removed, t)
		}
	}
	s.live[c] = want
	s.liveMu.Unlock()
	sort.Slice(add, func(i, j int) bool { return add[i] < add[j] })

	var toPoll []uint32
	deferred := false
	if len(add) > 0 {
		if s.d.Session.State().DataPlan == kite.PlanNone {
			toPoll = add // C-01: no WebSocket on a Personal plan
		} else {
			err := s.d.Live.Subscribe(c, consumer, add, ticker.ModeQuote)
			var budget *ticker.ErrBudget
			switch {
			case err == nil:
			case errors.Is(err, feed.ErrNotRunning):
				toPoll, deferred = add, true
			case errors.As(err, &budget):
				toPoll = add
				s.d.Log.Warn("quote subscription over budget; polling instead", "consumer", consumer, "tokens", len(add), "err", err)
			default:
				return WatchResult{}, refused(quotesStage, "subscription_failed", err.Error())
			}
		}
	}

	s.pollMu.Lock()
	set := s.polled[c]
	if set == nil {
		set = map[uint32]bool{}
		s.polled[c] = set
	}
	for _, t := range add {
		delete(set, t)
	}
	for _, t := range toPoll {
		set[t] = true
		if deferred {
			s.deferred[t] = true
		}
	}
	res := WatchResult{Live: []uint32{}, Polled: []uint32{}}
	for _, t := range ordered {
		if set[t] {
			res.Polled = append(res.Polled, t)
		} else {
			res.Live = append(res.Live, t)
		}
	}
	s.pollMu.Unlock()
	res.Snapshot = s.LastQuotes(ordered)

	if len(removed) > 0 {
		linger := s.d.Clock.At(s.d.Clock.Now().Add(UnsubscribeLinger))
		logging.Go("service.quotes.linger", func() {
			<-linger.C()
			s.liveMu.Lock()
			var drop []uint32
			for _, t := range removed {
				if !s.live[c][t] {
					drop = append(drop, t)
				}
			}
			s.liveMu.Unlock()
			if len(drop) == 0 {
				return
			}
			s.pollMu.Lock()
			for _, t := range drop {
				delete(s.polled[c], t)
			}
			s.pollMu.Unlock()
			s.d.Live.Unsubscribe(c, consumer, drop)
		})
	}
	return res, nil
}

// RunQuotes polls the REST quote endpoints for polled tokens every QuotePollInterval and flushes the
// last-quote cache every QuoteCacheFlush and on exit.
func (s *Service) RunQuotes(ctx context.Context) {
	s.quotesOnce.Do(s.loadQuoteCache)
	clk := s.d.Clock
	poll := clk.At(clk.Now().Add(QuotePollInterval))
	flush := clk.At(clk.Now().Add(QuoteCacheFlush))
	defer func() {
		poll.Stop()
		flush.Stop()
		s.flushQuoteCache(context.Background())
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-poll.C():
			poll = clk.At(clk.Now().Add(QuotePollInterval))
			s.pollQuotes(ctx)
		case <-flush.C():
			flush = clk.At(clk.Now().Add(QuoteCacheFlush))
			s.flushQuoteCache(ctx)
		}
	}
}

// polledTokens is the union of every consumer's polled set; deferred tokens leave it once the feed is live.
func (s *Service) polledTokens() []uint32 {
	feedLive := s.FeedState().State == "live"
	s.pollMu.Lock()
	defer s.pollMu.Unlock()
	union := map[uint32]bool{}
	for _, set := range s.polled {
		for t := range set {
			if feedLive && s.deferred[t] {
				delete(set, t)
				delete(s.deferred, t)
				continue
			}
			union[t] = true
		}
	}
	out := make([]uint32, 0, len(union))
	for t := range union {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (s *Service) pollQuotes(ctx context.Context) {
	if s.d.Quotes == nil || s.d.Session.State().State != kite.StateReady {
		return
	}
	tokens := s.polledTokens()
	if len(tokens) == 0 {
		return
	}
	ids := make([]string, 0, len(tokens))
	byID := map[string]uint32{}
	for _, t := range tokens {
		in, ok := s.d.Instruments.ByToken(t)
		if !ok {
			continue
		}
		id := in.Exchange + ":" + in.Tradingsymbol
		ids = append(ids, id)
		byID[id] = t
	}
	if len(ids) == 0 {
		return
	}
	cached := map[uint32]TickDTO{}
	for _, q := range s.LastQuotes(tokens) {
		cached[q.Token] = q
	}
	closes := map[uint32]float64{}
	var needClose []string
	for _, id := range ids {
		if cached[byID[id]].Close == 0 {
			needClose = append(needClose, id)
		}
	}
	for i := 0; i < len(needClose); i += ltpChunk {
		end := min(i+ltpChunk, len(needClose))
		q, err := s.d.Quotes.OHLC(ctx, needClose[i:end]...)
		if err != nil {
			s.d.Log.Warn("quote OHLC poll failed", "err", err)
			break
		}
		for _, v := range q {
			closes[uint32(v.InstrumentToken)] = v.OHLC.Close
		}
	}
	now := s.d.Clock.Now().UnixMilli()
	var out []TickDTO
	for i := 0; i < len(ids); i += ltpChunk {
		end := min(i+ltpChunk, len(ids))
		q, err := s.d.Quotes.LTP(ctx, ids[i:end]...)
		if err != nil {
			if kite.IsNoDataPermission(err) {
				s.d.Log.Info("quote polling is not permitted on this API plan")
			} else {
				s.d.Log.Warn("quote LTP poll failed", "err", err)
			}
			return
		}
		for _, v := range q {
			tok := uint32(v.InstrumentToken)
			d := TickDTO{Token: tok, LTP: v.LastPrice, Close: closes[tok], TsMs: now, Polled: true}
			if d.Close == 0 {
				d.Close = cached[tok].Close
			}
			if d.Close > 0 {
				d.Change = d.LTP - d.Close
				d.ChangePct = d.Change / d.Close * 100
			}
			out = append(out, d)
		}
	}
	if len(out) > 0 {
		bus.Publish(s.d.Bus, TopicPolledTicks, out)
	}
}

// loadQuoteCache seeds the in-memory cache from window_state (entries marked as not polled).
func (s *Service) loadQuoteCache() {
	if s.d.UiState == nil {
		return
	}
	raw, err := s.d.UiState.Get(context.Background(), QuoteCacheKey)
	if err != nil || raw == "" {
		return
	}
	var m map[uint32]TickDTO
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		s.d.Log.Warn("ignoring the saved quote cache", "err", err)
		return
	}
	s.quoteMu.Lock()
	for tok, q := range m {
		if _, ok := s.lastQuote[tok]; !ok {
			q.Polled = false
			s.lastQuote[tok] = q
		}
	}
	s.quoteMu.Unlock()
}

// flushQuoteCache writes the cache to window_state when it changed since the last flush.
func (s *Service) flushQuoteCache(ctx context.Context) {
	if s.d.UiState == nil {
		return
	}
	s.quoteMu.Lock()
	if !s.quoteDirty {
		s.quoteMu.Unlock()
		return
	}
	snapshot := make(map[uint32]TickDTO, len(s.lastQuote))
	for k, v := range s.lastQuote {
		snapshot[k] = v
	}
	s.quoteDirty = false
	s.quoteMu.Unlock()
	b, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	if err := s.d.UiState.Set(ctx, QuoteCacheKey, string(b)); err != nil {
		s.d.Log.Warn("saving the quote cache failed", "err", err)
		s.quoteMu.Lock()
		s.quoteDirty = true
		s.quoteMu.Unlock()
	}
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/service/ -run 'TestWatchQuotes|TestForwardTicks' -v`
Expected: PASS (`TestWatchQuotes_LingerThenUnsubscribe`, `TestWatchQuotes_PolledFallbackPublishesTicks`, `TestWatchQuotes_SnapshotSurvivesRestart`, `TestForwardTicks_MergesLiveAndPolledAndCaches`; `-race` clean).

- [ ] **Step 6: Commit**

```bash
git add internal/service/quotes.go internal/service/quotes_test.go internal/service/deps.go
git commit -m "feat(service): Quotes.Watch with per-consumer live sets, REST polling fallback and a persisted last-quote cache

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
