> **P0-3 Session & Service Layer Implementation Plan — Task 16.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 16: Chart data (trading-day ranges, manual refresh, live chart subscriptions) and instrument lookup

**Files:**
- Create: `internal/service/chart.go`
- Modify: `internal/service/deps.go` (Task 14: three narrow interfaces, four `Deps` fields, the live-chart registry on `Service`)
- Test: `internal/service/chart_test.go`

**Interfaces:**
- Consumes: `CandleReader`, `BackfillSubmitter`, `InstrumentFinder`, `LiveSubscriber` (Task 14); P0-2 `candles.ParseInterval`, `candles.Candle{Start, Open, High, Low, Close, Volume, OI, HasOI, Source, Complete}`, `candles.Range{From, To}`, `candles.SourceManual`, `(Interval).Duration`, `(Interval).IsIntraday`; `backfill.Job{Token, Interval, From, To, Requester, Priority, OI}`; `ratelimit.Interactive`; `kite.HistoricalPicker` (`HistoricalPick`), `kite.HistoricalRequest{Token, Interval, From, To, Continuous, OI}`, `kite.HistoricalData`; `ticker.ConsumerChart`, `ticker.ModeQuote`, `ticker.Tick{Token, LTP, Change, Close, Volume, ExchangeTS, Received}`, `*ticker.ErrBudget`; `builder.Event{Token, Interval, Candle, Partial}`, `(*builder.Builder).Track/Untrack`; `calendar.Service.IsTradingDay/PrevTradingDay/Day`, `calendar.DateOf`, `(Date).Midnight`, `(Day).Phase`, `calendar.PhaseContinuous`, `calendar.Exchanges`, `calendar.NSE`, `calendar.NewDefault` (tests); `instruments.Instrument{Token, Exchange, Segment, Tag, Tradingsymbol, Name, Type, Expiry, TickSizePaise, LotSize, DisplayName, Underlying, IsIndex}` + `IsFuture()`, `instruments.Result{Instrument}`, `instruments.Tag`; `feed.ErrNotRunning` (Task 6).
- Produces:
  ```go
  // deps.go additions
  type HistoricalFetcher interface { HistoricalPick(ctx context.Context, prio ratelimit.Priority, pick func() (kite.HistoricalRequest, bool)) (kite.HistoricalRequest, []kite.HistoricalData, bool, error) } // *kite.Client
  type CandleUpserter interface { Upsert(ctx context.Context, token uint32, iv candles.Interval, cs []candles.Candle, src candles.Source) error }                                                    // *sqlite.Store
  type CandleTracker interface { Track(token uint32, ivs ...candles.Interval); Untrack(token uint32, ivs ...candles.Interval) }                                                                      // *builder.Builder
  // Deps gains: Calendar *calendar.Service; Historical HistoricalFetcher; CandleWriter CandleUpserter; Tracker CandleTracker
  // chart.go
  type CandlesRequest struct{ Token uint32; Interval string; FromMs, ToMs int64; TradingDays int } // TS service.CandlesRequest
  type RefreshRequest struct{ Token uint32; Interval string; FromMs, ToMs int64 }                  // TS service.RefreshRequest
  type CandleDTO struct{ TimeMs int64 `json:"t"`; Open, High, Low, Close float64 (o,h,l,c); Volume, OI int64 (v,oi); Complete bool }
  type CandlesResult struct{ Candles []CandleDTO; Complete bool; BackfillJobID string; NeedsDataPlan, Offline bool }
  type InstrumentDTO struct{ Token uint32; Exchange, Segment, Tag, Tradingsymbol, Name, DisplayName, InstrumentType string; ExpiryMs int64; TickSizePaise, LotSize int64; IsIndex, IsFuture bool; Underlying string }
  type TickDTO struct{ Token uint32; LTP, Close, Change, ChangePct float64; Volume uint32; TsMs int64; Polled bool }
  type CandleEventDTO struct{ Token uint32; Interval string; Candle CandleDTO; Partial bool }
  func CandleDTOFrom(c candles.Candle) CandleDTO; func TickDTOFrom(t ticker.Tick) TickDTO; func CandleEventFrom(e builder.Event) CandleEventDTO
  func (s *Service) Candles(ctx context.Context, req CandlesRequest) (CandlesResult, error)
  func (s *Service) RefreshCandles(ctx context.Context, req RefreshRequest) (CandlesResult, error)
  func (s *Service) OpenLive(ctx context.Context, token uint32, interval string) (string, error)
  func (s *Service) CloseLive(ctx context.Context, id string) error
  func (s *Service) SearchInstruments(ctx context.Context, q string, tags []string, limit int) ([]InstrumentDTO, error)
  func (s *Service) InstrumentsByTokens(ctx context.Context, tokens []uint32) ([]InstrumentDTO, error)
  func (s *Service) InstrumentByIdentifier(ctx context.Context, id string) (InstrumentDTO, error)
  ```

`Candles` always returns what the store has. `ToMs` 0 means "now" on the app clock. `TradingDays > 0` replaces `FromMs`: the range starts at the continuous-phase open of the N-th trading day before `ToMs` (the day containing `ToMs` counts as the first when it is a trading day), on the instrument's exchange (`NSE` when the token is unknown). When ranges are missing:
- Not logged in: `Offline=true`, nothing is submitted.
- DataPlan `none`: `NeedsDataPlan=true`, nothing is submitted (FR-ACCT-007 AC1/AC3).
- Otherwise: an `Interactive` backfill job is submitted (`OI` set for futures, from the instrument master). P0-5 waits for `backfill.progress` and asks again.

`RefreshCandles` (FR-CHART-006) re-fetches one window from Kite at `Interactive` priority through `HistoricalFetcher.HistoricalPick`, upserts it with `candles.SourceManual` and returns the stored range again. It refuses `offline` (no session), `no_data_plan`, and `invalid_range` when the window exceeds Kite's per-request limit for the interval (P0-2 backfill's spans: 60 days for `minute`, 100 for 3–30 minutes, 400 for `60minute`, 2000 for `day`).

`OpenLive(token, interval)` subscribes the token for the chart (`ConsumerChart`, owner = the returned subscription id, quote mode) and asks the candle builder to track (token, interval) if no other open subscription already does. `CloseLive(id)` releases the subscription and untracks when it was the last user of (token, interval). `feed.ErrNotRunning` (no login yet) is not an error: the feed remembers the subscription and applies it at login. A `*ticker.ErrBudget` refusal is reported as `subscription_budget`. The watchlist and widget consumers go through `Quotes.Watch` (Task 29).

`SearchInstruments` returns an empty list for an empty query, filters by P0-2 `instruments.Tag` when `tags` is non-empty, and caps `limit` at 50 (default 20). `InstrumentsByTokens` omits unknown tokens; `InstrumentByIdentifier("NSE:NIFTY 50")` refuses `not_found`.

- [ ] **Step 1: Write the failing tests**

```go
// internal/service/chart_test.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/candles/builder"
	"tradeloom/internal/instruments"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/feed"
	"tradeloom/internal/kite/ratelimit"
	"tradeloom/internal/kite/ticker"
)

// rangeCandles records the range the service asked for.
type rangeCandles struct {
	fakeCandles
	mu       sync.Mutex
	from, to time.Time
}

func (r *rangeCandles) Read(ctx context.Context, tok uint32, iv candles.Interval, from, to time.Time) ([]candles.Candle, error) {
	r.mu.Lock()
	r.from, r.to = from, to
	r.mu.Unlock()
	return r.fakeCandles.Read(ctx, tok, iv, from, to)
}

type fakeHistorical struct {
	mu   sync.Mutex
	reqs []kite.HistoricalRequest
	prio ratelimit.Priority
	data []kite.HistoricalData
	err  error
}

func (f *fakeHistorical) HistoricalPick(_ context.Context, prio ratelimit.Priority, pick func() (kite.HistoricalRequest, bool)) (kite.HistoricalRequest, []kite.HistoricalData, bool, error) {
	req, ok := pick()
	if !ok {
		return req, nil, false, nil
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, req)
	f.prio = prio
	f.mu.Unlock()
	return req, f.data, true, f.err
}

type upsert struct {
	token uint32
	iv    candles.Interval
	cs    []candles.Candle
	src   candles.Source
}

type fakeUpserter struct {
	mu   sync.Mutex
	rows []upsert
}

func (f *fakeUpserter) Upsert(_ context.Context, token uint32, iv candles.Interval, cs []candles.Candle, src candles.Source) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, upsert{token, iv, cs, src})
	return nil
}

type trackKey struct {
	token uint32
	iv    candles.Interval
}

type fakeTracker struct {
	mu      sync.Mutex
	tracks  map[trackKey]int
	untrack map[trackKey]int
}

func (f *fakeTracker) Track(token uint32, ivs ...candles.Interval) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, iv := range ivs {
		f.tracks[trackKey{token, iv}]++
	}
}
func (f *fakeTracker) Untrack(token uint32, ivs ...candles.Interval) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, iv := range ivs {
		f.untrack[trackKey{token, iv}]++
	}
}
func (f *fakeTracker) counts(token uint32, iv candles.Interval) (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tracks[trackKey{token, iv}], f.untrack[trackKey{token, iv}]
}

func mustCalendar(t *testing.T) *calendar.Service {
	t.Helper()
	cal, err := calendar.NewDefault()
	if err != nil {
		t.Fatal(err)
	}
	return cal
}

// withChartDeps installs the Task 16 dependencies on an env built by newSvcEnv.
func withChartDeps(t *testing.T, e *svcEnv) (*fakeHistorical, *fakeUpserter, *fakeTracker) {
	t.Helper()
	h, u, tr := &fakeHistorical{}, &fakeUpserter{}, &fakeTracker{tracks: map[trackKey]int{}, untrack: map[trackKey]int{}}
	e.svc.d.Calendar = mustCalendar(t)
	e.svc.d.Historical = h
	e.svc.d.CandleWriter = u
	e.svc.d.Tracker = tr
	return h, u, tr
}

func req() CandlesRequest {
	return CandlesRequest{Token: 256265, Interval: "60minute", FromMs: ist(2026, 10, 1, 9, 15).UnixMilli(), ToMs: ist(2026, 10, 12, 15, 30).UnixMilli()}
}

func TestCandles_CompleteFromStore(t *testing.T) {
	e := newSvcEnv(t)
	withChartDeps(t, e)
	e.candles.have = []candles.Candle{{Start: ist(2026, 10, 1, 9, 15), Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 10, Complete: true}}
	res, err := e.svc.Candles(context.Background(), req())
	if err != nil || !res.Complete || len(res.Candles) != 1 || res.Candles[0].TimeMs != ist(2026, 10, 1, 9, 15).UnixMilli() || len(e.backfill.jobs) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
}

// P0-5 asks for history as N trading days ending now: 5 trading days before Mon 12 Oct 2026 start on Tue 6 Oct 09:15 IST.
func TestCandles_TradingDaysResolvesFromCalendar(t *testing.T) {
	e := newSvcEnv(t)
	withChartDeps(t, e)
	rc := &rangeCandles{}
	e.svc.d.Candles = rc
	res, err := e.svc.Candles(context.Background(), CandlesRequest{Token: 256265, Interval: "60minute", TradingDays: 5})
	if err != nil || !res.Complete {
		t.Fatalf("%+v %v", res, err)
	}
	rc.mu.Lock()
	from, to := rc.from, rc.to
	rc.mu.Unlock()
	if !from.Equal(ist(2026, 10, 6, 9, 15)) || !to.Equal(e.clock.Now()) {
		t.Fatalf("range %v → %v", from, to)
	}
	// ToMs on a Sunday counts back from the previous trading day.
	_, err = e.svc.Candles(context.Background(), CandlesRequest{Token: 256265, Interval: "day", TradingDays: 1, ToMs: ist(2026, 10, 11, 12, 0).UnixMilli()})
	rc.mu.Lock()
	from = rc.from
	rc.mu.Unlock()
	if err != nil || !from.Equal(ist(2026, 10, 9, 9, 15)) {
		t.Fatalf("sunday range from %v err %v", from, err)
	}
}

func TestCandles_MissingSubmitsInteractiveBackfill(t *testing.T) {
	e := newSvcEnv(t)
	withChartDeps(t, e)
	e.instr.m[256265] = instruments.Instrument{Token: 256265, Exchange: "NSE", Tradingsymbol: "NIFTY 50", Type: "EQ", IsIndex: true}
	e.instr.m[12345] = instruments.Instrument{Token: 12345, Exchange: "NFO", Tradingsymbol: "NIFTY26OCTFUT", Type: "FUT", Expiry: ist(2026, 10, 29, 0, 0)}
	e.candles.missing = []candles.Range{{From: ist(2026, 10, 5, 9, 15), To: ist(2026, 10, 6, 15, 30)}}
	res, err := e.svc.Candles(context.Background(), req())
	if err != nil || res.Complete || res.BackfillJobID != "job-1" {
		t.Fatalf("%+v %v", res, err)
	}
	if j := e.backfill.jobs[0]; j.Token != 256265 || j.Priority != ratelimit.Interactive || j.Requester != "chart" || j.OI {
		t.Fatalf("job %+v", j)
	}
	r := req()
	r.Token = 12345
	if _, err := e.svc.Candles(context.Background(), r); err != nil || !e.backfill.jobs[1].OI {
		t.Fatalf("futures must request open interest: %+v %v", e.backfill.jobs[1], err)
	}
}

// FR-ACCT-007 AC1/AC3 and Review Focus 4.
func TestCandles_PersonalPlan_ServesCacheAndNeedsDataPlan(t *testing.T) {
	e := newSvcEnv(t)
	withChartDeps(t, e)
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanNone})
	e.candles.have = []candles.Candle{{Start: ist(2026, 10, 1, 9, 15), Close: 1, Complete: true}}
	e.candles.missing = []candles.Range{{From: ist(2026, 10, 5, 9, 15), To: ist(2026, 10, 6, 15, 30)}}
	res, err := e.svc.Candles(context.Background(), req())
	if err != nil || !res.NeedsDataPlan || len(res.Candles) != 1 || len(e.backfill.jobs) != 0 {
		t.Fatalf("%+v %v jobs %d", res, err, len(e.backfill.jobs))
	}
}

func TestCandles_LoggedOutServesCacheOffline(t *testing.T) {
	e := newSvcEnv(t)
	withChartDeps(t, e)
	e.session.setState(kite.SessionState{State: kite.StateExpired})
	e.candles.missing = []candles.Range{{}}
	res, err := e.svc.Candles(context.Background(), req())
	if err != nil || !res.Offline || len(e.backfill.jobs) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestCandles_RejectsBadInput(t *testing.T) {
	e := newSvcEnv(t)
	withChartDeps(t, e)
	r := req()
	r.Interval = "7minute"
	if _, err := e.svc.Candles(context.Background(), r); refusedCode(t, err) != "invalid_interval" {
		t.Fatal(err)
	}
	r = req()
	r.FromMs, r.ToMs = r.ToMs, r.FromMs
	if _, err := e.svc.Candles(context.Background(), r); refusedCode(t, err) != "invalid_range" {
		t.Fatal(err)
	}
}

func TestDTOs_FromP02Types(t *testing.T) {
	tk := TickDTOFrom(ticker.Tick{Token: 7, LTP: 102, Close: 100, Change: 2, Volume: 55, ExchangeTS: ist(2026, 10, 12, 10, 0)})
	if tk.Token != 7 || tk.LTP != 102 || tk.Close != 100 || tk.Change != 2 || tk.ChangePct != 2 || tk.Volume != 55 || tk.TsMs != ist(2026, 10, 12, 10, 0).UnixMilli() || tk.Polled {
		t.Fatalf("%+v", tk)
	}
	if rx := TickDTOFrom(ticker.Tick{Token: 8, LTP: 1, Received: ist(2026, 10, 12, 10, 1)}); rx.TsMs != ist(2026, 10, 12, 10, 1).UnixMilli() || rx.ChangePct != 0 {
		t.Fatalf("received fallback %+v", rx)
	}
	ev := CandleEventFrom(builder.Event{Token: 7, Interval: candles.Minute5, Candle: candles.Candle{Start: ist(2026, 10, 12, 10, 0), Close: 3, OI: 9}, Partial: true})
	if ev.Token != 7 || ev.Interval != "5minute" || ev.Candle.TimeMs != ist(2026, 10, 12, 10, 0).UnixMilli() || ev.Candle.OI != 9 || !ev.Partial {
		t.Fatalf("%+v", ev)
	}
	in := instrumentDTO(instruments.Instrument{Token: 12345, Exchange: "NFO", Segment: "NFO-FUT", Tag: instruments.TagNFOFut, Tradingsymbol: "NIFTY26OCTFUT", Name: "NIFTY",
		Type: "FUT", Expiry: ist(2026, 10, 29, 0, 0), TickSizePaise: 5, LotSize: 25, DisplayName: "NIFTY OCT FUT", Underlying: "NIFTY"})
	if in.Tag != "NFO-FUT" || in.InstrumentType != "FUT" || in.ExpiryMs != ist(2026, 10, 29, 0, 0).UnixMilli() || in.LotSize != 25 || !in.IsFuture || in.Underlying != "NIFTY" {
		t.Fatalf("%+v", in)
	}
}

// FR-CHART-006: a manual refresh re-fetches the window at Interactive priority and upserts it as source manual.
func TestRefreshCandles_FetchesAndUpsertsManual(t *testing.T) {
	e := newSvcEnv(t)
	h, u, _ := withChartDeps(t, e)
	e.instr.m[12345] = instruments.Instrument{Token: 12345, Exchange: "NFO", Tradingsymbol: "NIFTY26OCTFUT", Type: "FUT", Expiry: ist(2026, 10, 29, 0, 0)}
	if err := json.Unmarshal([]byte(`[{"date":"2026-10-12T09:15:00+0530","open":1,"high":2,"low":0.5,"close":1.5,"volume":10,"oi":7},
		{"date":"2026-10-12T10:15:00+0530","open":1.5,"high":3,"low":1,"close":2,"volume":12,"oi":8}]`), &h.data); err != nil {
		t.Fatal(err)
	}
	e.candles.have = []candles.Candle{{Start: ist(2026, 10, 12, 9, 15), Close: 1.5, Complete: true}, {Start: ist(2026, 10, 12, 10, 15), Close: 2}}
	rr := RefreshRequest{Token: 12345, Interval: "60minute", FromMs: ist(2026, 10, 12, 9, 15).UnixMilli(), ToMs: ist(2026, 10, 12, 11, 0).UnixMilli()}
	res, err := e.svc.RefreshCandles(context.Background(), rr)
	if err != nil || len(res.Candles) != 2 || !res.Complete {
		t.Fatalf("%+v %v", res, err)
	}
	h.mu.Lock()
	hreq, prio := h.reqs[0], h.prio
	h.mu.Unlock()
	if prio != ratelimit.Interactive || hreq.Token != 12345 || hreq.Interval != "60minute" || !hreq.OI || hreq.Continuous {
		t.Fatalf("request %+v prio %v", hreq, prio)
	}
	u.mu.Lock()
	row := u.rows[0]
	u.mu.Unlock()
	if row.src != candles.SourceManual || row.iv != candles.Minute60 || len(row.cs) != 2 || !row.cs[0].Start.Equal(ist(2026, 10, 12, 9, 15)) ||
		row.cs[0].Volume != 10 || row.cs[0].OI != 7 || !row.cs[0].HasOI || !row.cs[0].Complete || row.cs[1].Complete {
		t.Fatalf("upsert %+v", row)
	}
}

func TestRefreshCandles_Refusals(t *testing.T) {
	e := newSvcEnv(t)
	h, _, _ := withChartDeps(t, e)
	rr := RefreshRequest{Token: 256265, Interval: "minute", FromMs: ist(2026, 10, 12, 9, 15).UnixMilli(), ToMs: ist(2026, 10, 12, 11, 0).UnixMilli()}
	e.session.setState(kite.SessionState{State: kite.StateLoggedOut})
	if _, err := e.svc.RefreshCandles(context.Background(), rr); refusedCode(t, err) != "offline" {
		t.Fatal(err)
	}
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanNone})
	if _, err := e.svc.RefreshCandles(context.Background(), rr); refusedCode(t, err) != "no_data_plan" {
		t.Fatal(err)
	}
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanConnect})
	wide := rr
	wide.FromMs = ist(2026, 7, 1, 9, 15).UnixMilli()
	if _, err := e.svc.RefreshCandles(context.Background(), wide); refusedCode(t, err) != "invalid_range" {
		t.Fatal(err)
	}
	h.err = errors.New("historical: 503")
	if _, err := e.svc.RefreshCandles(context.Background(), rr); refusedCode(t, err) != "refresh_failed" {
		t.Fatal(err)
	}
}

// FR-CHART-003 live candles: the builder tracks (token, interval) once per open chart, and stops with the last one.
func TestOpenCloseLive_TracksBuilderOnce(t *testing.T) {
	e := newSvcEnv(t)
	_, _, tr := withChartDeps(t, e)
	ctx := context.Background()
	id1, err := e.svc.OpenLive(ctx, 256265, "5minute")
	if err != nil || id1 == "" {
		t.Fatalf("%q %v", id1, err)
	}
	id2, err := e.svc.OpenLive(ctx, 256265, "5minute")
	if err != nil || id2 == id1 {
		t.Fatalf("%q %v", id2, err)
	}
	if n, _ := tr.counts(256265, candles.Minute5); n != 1 {
		t.Fatalf("tracked %d times", n)
	}
	if !e.live.has(ticker.ConsumerChart, 256265) {
		t.Fatal("token not subscribed for the chart")
	}
	if err := e.svc.CloseLive(ctx, id1); err != nil {
		t.Fatal(err)
	}
	if _, un := tr.counts(256265, candles.Minute5); un != 0 {
		t.Fatal("untracked while another chart still shows the interval")
	}
	if err := e.svc.CloseLive(ctx, id2); err != nil {
		t.Fatal(err)
	}
	if _, un := tr.counts(256265, candles.Minute5); un != 1 || e.live.has(ticker.ConsumerChart, 256265) {
		t.Fatalf("untracked %d, subscribed %v", un, e.live.has(ticker.ConsumerChart, 256265))
	}
	if err := e.svc.CloseLive(ctx, id2); refusedCode(t, err) != "not_found" {
		t.Fatal(err)
	}
	if _, err := e.svc.OpenLive(ctx, 256265, "7minute"); refusedCode(t, err) != "invalid_interval" {
		t.Fatal(err)
	}
	e.live.err = &ticker.ErrBudget{Used: 9000, Requested: 1, Limit: 9000}
	if _, err := e.svc.OpenLive(ctx, 9, "minute"); refusedCode(t, err) != "subscription_budget" {
		t.Fatal(err)
	}
	e.live.err = feed.ErrNotRunning // before login the feed remembers the subscription; not an error for the UI
	if _, err := e.svc.OpenLive(ctx, 10, "minute"); err != nil {
		t.Fatal(err)
	}
	e.live.err = nil
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanNone})
	if _, err := e.svc.OpenLive(ctx, 11, "minute"); refusedCode(t, err) != "no_data_plan" {
		t.Fatal(err)
	}
}

func TestInstruments_SearchTagsByTokensByIdentifier(t *testing.T) {
	e := newSvcEnv(t)
	withChartDeps(t, e)
	e.instr.m[779521] = instruments.Instrument{Token: 779521, Exchange: "NSE", Tag: instruments.TagNSE, Tradingsymbol: "SBIN", DisplayName: "SBIN"}
	e.instr.m[7795] = instruments.Instrument{Token: 7795, Exchange: "BSE", Tag: instruments.TagBSE, Tradingsymbol: "SBIN", DisplayName: "SBIN"}
	ctx := context.Background()
	all, err := e.svc.SearchInstruments(ctx, "SBIN", nil, 10)
	if err != nil || len(all) != 2 {
		t.Fatalf("%+v %v", all, err)
	}
	bse, err := e.svc.SearchInstruments(ctx, "SBIN", []string{"BSE"}, 10)
	if err != nil || len(bse) != 1 || bse[0].Exchange != "BSE" || bse[0].Tag != "BSE" {
		t.Fatalf("%+v %v", bse, err)
	}
	if none, err := e.svc.SearchInstruments(ctx, "   ", nil, 10); err != nil || len(none) != 0 {
		t.Fatalf("empty query %+v %v", none, err)
	}
	by, err := e.svc.InstrumentsByTokens(ctx, []uint32{779521, 99})
	if err != nil || len(by) != 1 || by[0].Token != 779521 {
		t.Fatalf("%+v %v", by, err)
	}
	in, err := e.svc.InstrumentByIdentifier(ctx, "NSE:SBIN")
	if err != nil || in.Token != 779521 {
		t.Fatalf("%+v %v", in, err)
	}
	if _, err := e.svc.InstrumentByIdentifier(ctx, "NSE:NOPE"); refusedCode(t, err) != "not_found" {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/service/ -run 'TestCandles|TestDTOs|TestRefreshCandles|TestOpenCloseLive|TestInstruments_' -v`
Expected: FAIL to compile with `undefined: CandlesRequest`, `e.svc.d.Calendar undefined`, `e.svc.d.Historical undefined`.

- [ ] **Step 3: Extend `internal/service/deps.go`**

Add the three interfaces, the four `Deps` fields and the live-chart registry. The complete additions:

```go
// internal/service/deps.go — additions of Task 16 (imports: add "tradeloom/internal/calendar")

// HistoricalFetcher is *kite.Client (kite.HistoricalPicker): the manual refresh fetches one window at Interactive priority.
type HistoricalFetcher interface {
	HistoricalPick(ctx context.Context, prio ratelimit.Priority, pick func() (kite.HistoricalRequest, bool)) (kite.HistoricalRequest, []kite.HistoricalData, bool, error)
}

// CandleUpserter is the write half of candles.Store (*sqlite.Store).
type CandleUpserter interface {
	Upsert(ctx context.Context, token uint32, iv candles.Interval, cs []candles.Candle, src candles.Source) error
}

// CandleTracker is *builder.Builder: forming/closed candles are built only for tracked (token, interval) pairs.
type CandleTracker interface {
	Track(token uint32, ivs ...candles.Interval)
	Untrack(token uint32, ivs ...candles.Interval)
}

// liveChart is one open chart subscription (OpenLive).
type liveChart struct {
	token uint32
	iv    candles.Interval
}
```

In `Deps`, after `Live LiveSubscriber`, add:

```go
	Calendar     *calendar.Service // trading-day arithmetic, market phase (Task 21), calendar settings (Task 31)
	Historical   HistoricalFetcher // *kite.Client
	CandleWriter CandleUpserter    // *sqlite.Store
	Tracker      CandleTracker     // *builder.Builder
```

In `Service`, after the `liveMu`/`live` fields, add:

```go
	chartMu    sync.Mutex
	liveCharts map[string]liveChart // subscription id → (token, interval)
```

and in `New` initialise it: `return &Service{d: d, live: map[ticker.Consumer]map[uint32]bool{}, liveCharts: map[string]liveChart{}}`. The import block gains `"tradeloom/internal/kite/ratelimit"` and `"tradeloom/internal/calendar"`.

- [ ] **Step 4: Write the implementation**

```go
// internal/service/chart.go
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/candles/backfill"
	"tradeloom/internal/candles/builder"
	"tradeloom/internal/clock"
	"tradeloom/internal/instruments"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/feed"
	"tradeloom/internal/kite/ratelimit"
	"tradeloom/internal/kite/ticker"
)

const (
	chartStage       = "chart"
	instrumentsStage = "instruments"
	maxSearchLimit   = 50
	defaultSearch    = 20
)

// maxRefreshSpan mirrors Kite's per-request historical limits (P0-2 backfill windows).
var maxRefreshSpan = map[candles.Interval]time.Duration{
	candles.Minute:   60 * 24 * time.Hour,
	candles.Minute3:  100 * 24 * time.Hour,
	candles.Minute5:  100 * 24 * time.Hour,
	candles.Minute10: 100 * 24 * time.Hour,
	candles.Minute15: 100 * 24 * time.Hour,
	candles.Minute30: 100 * 24 * time.Hour,
	candles.Minute60: 400 * 24 * time.Hour,
	candles.Day:      2000 * 24 * time.Hour,
}

type CandlesRequest struct {
	Token       uint32 `json:"token"`
	Interval    string `json:"interval"`
	FromMs      int64  `json:"fromMs"`
	ToMs        int64  `json:"toMs"`        // 0 = now
	TradingDays int    `json:"tradingDays"` // > 0: FromMs is resolved from the calendar
}

type RefreshRequest struct {
	Token    uint32 `json:"token"`
	Interval string `json:"interval"`
	FromMs   int64  `json:"fromMs"`
	ToMs     int64  `json:"toMs"`
}

type CandleDTO struct {
	TimeMs   int64   `json:"t"`
	Open     float64 `json:"o"`
	High     float64 `json:"h"`
	Low      float64 `json:"l"`
	Close    float64 `json:"c"`
	Volume   int64   `json:"v"`
	OI       int64   `json:"oi"`
	Complete bool    `json:"complete"`
}

type CandlesResult struct {
	Candles       []CandleDTO `json:"candles"`
	Complete      bool        `json:"complete"`
	BackfillJobID string      `json:"backfillJobId"`
	NeedsDataPlan bool        `json:"needsDataPlan"`
	Offline       bool        `json:"offline"`
}

// InstrumentDTO is instruments.Instrument with the expiry as Unix ms (§8.4).
type InstrumentDTO struct {
	Token          uint32 `json:"token"`
	Exchange       string `json:"exchange"`
	Segment        string `json:"segment"`
	Tag            string `json:"tag"`
	Tradingsymbol  string `json:"tradingsymbol"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	InstrumentType string `json:"instrumentType"`
	ExpiryMs       int64  `json:"expiryMs"`
	TickSizePaise  int64  `json:"tickSizePaise"`
	LotSize        int64  `json:"lotSize"`
	IsIndex        bool   `json:"isIndex"`
	IsFuture       bool   `json:"isFuture"`
	Underlying     string `json:"underlying"`
}

// TickDTO is ticker.Tick reduced to what the UI shows; Close is the previous session close.
type TickDTO struct {
	Token     uint32  `json:"token"`
	LTP       float64 `json:"ltp"`
	Close     float64 `json:"close"`
	Change    float64 `json:"change"`
	ChangePct float64 `json:"changePct"`
	Volume    uint32  `json:"volume"`
	TsMs      int64   `json:"tsMs"`
	Polled    bool    `json:"polled"` // true when produced by REST polling instead of the WebSocket feed
}

type CandleEventDTO struct {
	Token    uint32    `json:"token"`
	Interval string    `json:"interval"`
	Candle   CandleDTO `json:"candle"`
	Partial  bool      `json:"partial"`
}

func CandleDTOFrom(c candles.Candle) CandleDTO {
	return CandleDTO{TimeMs: c.Start.UnixMilli(), Open: c.Open, High: c.High, Low: c.Low, Close: c.Close, Volume: c.Volume, OI: c.OI, Complete: c.Complete}
}

// TickDTOFrom stamps the tick with its exchange timestamp (full mode) or the app-clock arrival.
func TickDTOFrom(t ticker.Tick) TickDTO {
	d := TickDTO{Token: t.Token, LTP: t.LTP, Close: t.Close, Change: t.Change, Volume: t.Volume}
	switch {
	case !t.ExchangeTS.IsZero():
		d.TsMs = t.ExchangeTS.UnixMilli()
	case !t.Received.IsZero():
		d.TsMs = t.Received.UnixMilli()
	}
	if t.Close > 0 {
		d.ChangePct = (t.LTP - t.Close) / t.Close * 100
	}
	return d
}

func CandleEventFrom(e builder.Event) CandleEventDTO {
	return CandleEventDTO{Token: e.Token, Interval: string(e.Interval), Candle: CandleDTOFrom(e.Candle), Partial: e.Partial}
}

func instrumentDTO(in instruments.Instrument) InstrumentDTO {
	d := InstrumentDTO{Token: in.Token, Exchange: in.Exchange, Segment: in.Segment, Tag: string(in.Tag), Tradingsymbol: in.Tradingsymbol, Name: in.Name,
		DisplayName: in.DisplayName, InstrumentType: in.Type, TickSizePaise: in.TickSizePaise, LotSize: in.LotSize, IsIndex: in.IsIndex,
		IsFuture: in.IsFuture(), Underlying: in.Underlying}
	if !in.Expiry.IsZero() {
		d.ExpiryMs = in.Expiry.UnixMilli()
	}
	return d
}

// exchangeOf returns the calendar exchange of a token (NSE when unknown).
func (s *Service) exchangeOf(token uint32) calendar.Exchange {
	if in, ok := s.d.Instruments.ByToken(token); ok {
		for _, ex := range calendar.Exchanges {
			if string(ex) == in.Exchange {
				return ex
			}
		}
	}
	return calendar.NSE
}

// resolveRange turns a request into [from, to): ToMs 0 is now; TradingDays > 0 starts at the continuous open
// of the N-th trading day before to (the day of to counts when it is a trading day).
func (s *Service) resolveRange(token uint32, fromMs, toMs int64, tradingDays int) (time.Time, time.Time, error) {
	to := s.d.Clock.Now().In(clock.IST)
	if toMs > 0 {
		to = time.UnixMilli(toMs).In(clock.IST)
	}
	if tradingDays <= 0 {
		from := time.UnixMilli(fromMs).In(clock.IST)
		if !to.After(from) {
			return time.Time{}, time.Time{}, refused(chartStage, "invalid_range", "The start of the range must be before its end.")
		}
		return from, to, nil
	}
	if s.d.Calendar == nil {
		return time.Time{}, time.Time{}, refused(chartStage, "invalid_range", "Trading-day ranges need the market calendar.")
	}
	ex := s.exchangeOf(token)
	d := calendar.DateOf(to)
	if !s.d.Calendar.IsTradingDay(ex, d) {
		p, ok := s.d.Calendar.PrevTradingDay(ex, d)
		if !ok {
			return time.Time{}, time.Time{}, refused(chartStage, "invalid_range", "No trading day found before the end of the range.")
		}
		d = p
	}
	for i := 1; i < tradingDays; i++ {
		p, ok := s.d.Calendar.PrevTradingDay(ex, d)
		if !ok {
			break
		}
		d = p
	}
	from := d.Midnight()
	if ph, ok := s.d.Calendar.Day(ex, d).Phase(calendar.PhaseContinuous); ok {
		from = ph.Start
	}
	return from, to, nil
}

func (s *Service) readRange(ctx context.Context, token uint32, iv candles.Interval, from, to time.Time) (CandlesResult, error) {
	have, err := s.d.Candles.Read(ctx, token, iv, from, to)
	if err != nil {
		return CandlesResult{}, asRefused(chartStage, err)
	}
	missing, err := s.d.Candles.Missing(ctx, token, iv, from, to)
	if err != nil {
		return CandlesResult{}, asRefused(chartStage, err)
	}
	res := CandlesResult{Candles: make([]CandleDTO, 0, len(have)), Complete: len(missing) == 0}
	for _, c := range have {
		res.Candles = append(res.Candles, CandleDTOFrom(c))
	}
	return res, nil
}

// Candles serves the store and requests a backfill for gaps (FR-CHART-003 data path, FR-ACCT-007 degraded mode).
func (s *Service) Candles(ctx context.Context, req CandlesRequest) (CandlesResult, error) {
	iv, err := candles.ParseInterval(req.Interval)
	if err != nil {
		return CandlesResult{}, refused(chartStage, "invalid_interval", "Unsupported timeframe "+req.Interval+".")
	}
	from, to, err := s.resolveRange(req.Token, req.FromMs, req.ToMs, req.TradingDays)
	if err != nil {
		return CandlesResult{}, err
	}
	res, err := s.readRange(ctx, req.Token, iv, from, to)
	if err != nil || res.Complete {
		return res, err
	}
	st := s.d.Session.State()
	switch {
	case st.State != kite.StateReady:
		res.Offline = true
	case st.DataPlan == kite.PlanNone:
		res.NeedsDataPlan = true
	default:
		in, known := s.d.Instruments.ByToken(req.Token)
		job := backfill.Job{Token: req.Token, Interval: iv, From: from, To: to, Requester: "chart", Priority: ratelimit.Interactive, OI: known && in.IsFuture()}
		id, err := s.d.Backfill.Submit(ctx, job)
		if err != nil {
			return res, asRefused(chartStage, err)
		}
		res.BackfillJobID = id
	}
	return res, nil
}

// candlesFromHistorical converts Kite rows to store candles marked as a manual fetch.
func candlesFromHistorical(hs []kite.HistoricalData, iv candles.Interval, now time.Time) []candles.Candle {
	out := make([]candles.Candle, 0, len(hs))
	for _, h := range hs {
		start := h.Date.Time
		out = append(out, candles.Candle{Start: start, Open: h.Open, High: h.High, Low: h.Low, Close: h.Close, Volume: int64(h.Volume),
			OI: int64(h.OI), HasOI: h.OI != 0, Source: candles.SourceManual, Complete: !start.Add(iv.Duration()).After(now)})
	}
	return out
}

// RefreshCandles re-fetches one window from Kite at Interactive priority and upserts it as source manual (FR-CHART-006).
func (s *Service) RefreshCandles(ctx context.Context, req RefreshRequest) (CandlesResult, error) {
	iv, err := candles.ParseInterval(req.Interval)
	if err != nil {
		return CandlesResult{}, refused(chartStage, "invalid_interval", "Unsupported timeframe "+req.Interval+".")
	}
	from, to, err := s.resolveRange(req.Token, req.FromMs, req.ToMs, 0)
	if err != nil {
		return CandlesResult{}, err
	}
	if span := maxRefreshSpan[iv]; span > 0 && to.Sub(from) > span {
		return CandlesResult{}, refused(chartStage, "invalid_range", "Refresh a smaller range: Kite serves at most "+span.String()+" of "+req.Interval+" candles per request.")
	}
	st := s.d.Session.State()
	switch {
	case st.State != kite.StateReady:
		return CandlesResult{}, refused(chartStage, "offline", "Log in to Kite to refresh candles.")
	case st.DataPlan == kite.PlanNone:
		return CandlesResult{}, refused(chartStage, "no_data_plan", "This API key has no market-data permission; cached candles only.")
	}
	in, known := s.d.Instruments.ByToken(req.Token)
	hreq := kite.HistoricalRequest{Token: int(req.Token), Interval: string(iv), From: from, To: to, OI: known && in.IsFuture()}
	_, rows, ok, err := s.d.Historical.HistoricalPick(ctx, ratelimit.Interactive, func() (kite.HistoricalRequest, bool) { return hreq, true })
	if err != nil {
		return CandlesResult{}, refused(chartStage, "refresh_failed", "Kite did not return candles: "+err.Error())
	}
	if ok {
		cs := candlesFromHistorical(rows, iv, s.d.Clock.Now())
		if err := s.d.CandleWriter.Upsert(ctx, req.Token, iv, cs, candles.SourceManual); err != nil {
			return CandlesResult{}, asRefused(chartStage, err)
		}
		s.d.Log.Info("candles refreshed manually", "token", req.Token, "interval", iv, "rows", len(cs))
	}
	return s.readRange(ctx, req.Token, iv, from, to)
}

// OpenLive subscribes a chart to live ticks and candle building for (token, interval); returns the subscription id.
func (s *Service) OpenLive(_ context.Context, token uint32, interval string) (string, error) {
	iv, err := candles.ParseInterval(interval)
	if err != nil {
		return "", refused(chartStage, "invalid_interval", "Unsupported timeframe "+interval+".")
	}
	if s.d.Session.State().DataPlan == kite.PlanNone {
		return "", refused(chartStage, "no_data_plan", "This API key has no market-data permission; live candles are unavailable.")
	}
	id := s.d.NewID()
	if err := s.d.Live.Subscribe(ticker.ConsumerChart, id, []uint32{token}, ticker.ModeQuote); err != nil && !errors.Is(err, feed.ErrNotRunning) {
		return "", refused(chartStage, "subscription_budget", err.Error())
	}
	s.chartMu.Lock()
	first := !s.trackedLocked(token, iv)
	s.liveCharts[id] = liveChart{token: token, iv: iv}
	s.chartMu.Unlock()
	if first && s.d.Tracker != nil {
		s.d.Tracker.Track(token, iv)
	}
	return id, nil
}

func (s *Service) trackedLocked(token uint32, iv candles.Interval) bool {
	for _, lc := range s.liveCharts {
		if lc.token == token && lc.iv == iv {
			return true
		}
	}
	return false
}

// CloseLive releases a chart subscription; the builder stops tracking when no open chart shows (token, interval).
func (s *Service) CloseLive(_ context.Context, id string) error {
	s.chartMu.Lock()
	lc, ok := s.liveCharts[id]
	if !ok {
		s.chartMu.Unlock()
		return refused(chartStage, "not_found", "Unknown live subscription "+id+".")
	}
	delete(s.liveCharts, id)
	last := !s.trackedLocked(lc.token, lc.iv)
	s.chartMu.Unlock()
	s.d.Live.Release(ticker.ConsumerChart, id)
	if last && s.d.Tracker != nil {
		s.d.Tracker.Untrack(lc.token, lc.iv)
	}
	return nil
}

// SearchInstruments is the symbol search (FR-WATCH-008 ranking is P0-2's); tags filter by instruments.Tag.
func (s *Service) SearchInstruments(_ context.Context, q string, tags []string, limit int) ([]InstrumentDTO, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return []InstrumentDTO{}, nil
	}
	if limit <= 0 || limit > maxSearchLimit {
		limit = defaultSearch
	}
	want := map[instruments.Tag]bool{}
	for _, t := range tags {
		want[instruments.Tag(t)] = true
	}
	fetch := limit
	if len(want) > 0 {
		fetch = limit * 4
	}
	out := make([]InstrumentDTO, 0, limit)
	for _, r := range s.d.Instruments.Search(q, fetch) {
		if len(want) > 0 && !want[r.Instrument.Tag] {
			continue
		}
		out = append(out, instrumentDTO(r.Instrument))
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

// InstrumentsByTokens resolves tokens; unknown tokens are omitted.
func (s *Service) InstrumentsByTokens(_ context.Context, tokens []uint32) ([]InstrumentDTO, error) {
	out := make([]InstrumentDTO, 0, len(tokens))
	for _, t := range tokens {
		if in, ok := s.d.Instruments.ByToken(t); ok {
			out = append(out, instrumentDTO(in))
		}
	}
	return out, nil
}

// InstrumentByIdentifier resolves "EXCHANGE:TRADINGSYMBOL".
func (s *Service) InstrumentByIdentifier(_ context.Context, id string) (InstrumentDTO, error) {
	in, ok := s.d.Instruments.ByID(strings.TrimSpace(id))
	if !ok {
		return InstrumentDTO{}, refused(instrumentsStage, "not_found", "No instrument "+id+" in the instrument list.")
	}
	return instrumentDTO(in), nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/service/ -run 'TestCandles|TestDTOs|TestRefreshCandles|TestOpenCloseLive|TestInstruments_' -v`
Expected: PASS (11 tests: `TestCandles_CompleteFromStore`, `TestCandles_TradingDaysResolvesFromCalendar`, `TestCandles_MissingSubmitsInteractiveBackfill`, `TestCandles_PersonalPlan_ServesCacheAndNeedsDataPlan`, `TestCandles_LoggedOutServesCacheOffline`, `TestCandles_RejectsBadInput`, `TestDTOs_FromP02Types`, `TestRefreshCandles_FetchesAndUpsertsManual`, `TestRefreshCandles_Refusals`, `TestOpenCloseLive_TracksBuilderOnce`, `TestInstruments_SearchTagsByTokensByIdentifier`; `-race` clean).

- [ ] **Step 6: Commit**

```bash
git add internal/service/chart.go internal/service/chart_test.go internal/service/deps.go
git commit -m "feat(service): chart candles with trading-day ranges, manual refresh, live chart subscriptions and instrument lookup

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
