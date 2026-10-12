// internal/service/chart_test.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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
	token  uint32
	iv     candles.Interval
	window candles.Range
	cs     []candles.Candle
	src    candles.Source
}

type fakeUpserter struct {
	mu   sync.Mutex
	rows []upsert
}

func (f *fakeUpserter) UpsertCovered(_ context.Context, token uint32, iv candles.Interval, window candles.Range, cs []candles.Candle, src candles.Source) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, upsert{token, iv, window, cs, src})
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
	if err != nil || !from.Equal(ist(2026, 10, 9, 0, 0)) { // Ruling 66: day candles are stored at 00:00 IST
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
	// Deviation: newSvcEnv's clock is 10:00, when the 09:15 hourly candle is still forming; at 10:30 the
	// first row has ended and the second (10:15) is forming, as the assertions below expect.
	e.clock.Set(ist(2026, 10, 12, 10, 30))
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
	// Ruling 66 (4): coverage stops at the forming 10:15 candle, as in backfill.store.
	if !row.window.From.Equal(ist(2026, 10, 12, 9, 15)) || !row.window.To.Equal(ist(2026, 10, 12, 10, 15)) {
		t.Fatalf("covered window %v → %v", row.window.From, row.window.To)
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

// alsoOnNSEFinder returns its results as given, so the search's AlsoOnNSE flag can be asserted.
type alsoOnNSEFinder struct {
	fakeInstruments
	res []instruments.Result
}

func (f *alsoOnNSEFinder) Search(string, int) []instruments.Result { return f.res }

// P0-3 ledger carry-forward (FR-WATCH-008): a BSE hit carries "also on NSE" to the UI.
func TestSearchInstruments_CarriesAlsoOnNSE(t *testing.T) {
	e := newSvcEnv(t)
	withChartDeps(t, e)
	e.svc.d.Instruments = &alsoOnNSEFinder{res: []instruments.Result{
		{Instrument: instruments.Instrument{Token: 7795, Exchange: "BSE", Tag: instruments.TagBSE, Tradingsymbol: "SBIN"}, AlsoOnNSE: true},
		{Instrument: instruments.Instrument{Token: 7796, Exchange: "BSE", Tag: instruments.TagBSE, Tradingsymbol: "BSEONLY"}},
	}}
	got, err := e.svc.SearchInstruments(context.Background(), "sbin", nil, 10)
	if err != nil || len(got) != 2 || !got[0].AlsoOnNSE || got[1].AlsoOnNSE {
		t.Fatalf("%+v %v", got, err)
	}
	b, err := json.Marshal(got[0])
	if err != nil || !strings.Contains(string(b), `"alsoOnNse":true`) {
		t.Fatalf("json %s %v", b, err)
	}
}

// A subscription failure that is neither a budget refusal nor "feed not running" is not reported as a budget problem.
func TestOpenLive_OtherSubscribeErrorIsInternal(t *testing.T) {
	e := newSvcEnv(t)
	_, _, tr := withChartDeps(t, e)
	e.live.err = errors.New("ticker: closed")
	if _, err := e.svc.OpenLive(context.Background(), 9, "minute"); refusedCode(t, err) != "internal" {
		t.Fatal(err)
	}
	if n, _ := tr.counts(9, candles.Minute); n != 0 {
		t.Fatalf("tracked %d times after a failed subscribe", n)
	}
}

// Ruling 66 (2): before the continuous open the day of `to` does not count.
func TestCandles_TradingDaysPreOpen(t *testing.T) {
	e := newSvcEnv(t)
	withChartDeps(t, e)
	rc := &rangeCandles{}
	e.svc.d.Candles = rc
	e.clock.Set(ist(2026, 10, 12, 8, 0)) // Monday, trading day, before 09:15
	for _, c := range []struct {
		iv   string
		n    int
		want time.Time
	}{
		{"60minute", 1, ist(2026, 10, 9, 9, 15)},
		{"60minute", 3, ist(2026, 10, 7, 9, 15)},
		{"day", 1, ist(2026, 10, 9, 0, 0)},
		{"day", 3, ist(2026, 10, 7, 0, 0)},
	} {
		if _, err := e.svc.Candles(context.Background(), CandlesRequest{Token: 256265, Interval: c.iv, TradingDays: c.n}); err != nil {
			t.Fatal(err)
		}
		rc.mu.Lock()
		from := rc.from
		rc.mu.Unlock()
		if !from.Equal(c.want) {
			t.Errorf("%s × %d at 08:00: from %v, want %v", c.iv, c.n, from, c.want)
		}
	}
}

// Ruling 66 (2): a calendar gap is surfaced, never served as fewer trading days.
func TestCandles_TradingDaysCalendarGapRefused(t *testing.T) {
	e := newSvcEnv(t)
	withChartDeps(t, e)
	var hol []calendar.Entry
	for d := calendar.DateOf(ist(2026, 9, 1, 12, 0)); d.Before(calendar.DateOf(ist(2026, 10, 12, 12, 0))); d = d.AddDays(1) {
		hol = append(hol, calendar.Entry{Date: d, Exchange: calendar.NSE, Type: calendar.DayHoliday, Source: "user"})
	}
	e.svc.d.Calendar.SetOverrides(hol)
	if _, err := e.svc.Candles(context.Background(), CandlesRequest{Token: 256265, Interval: "day", TradingDays: 2}); refusedCode(t, err) != "invalid_range" {
		t.Fatal(err)
	}
	if len(e.backfill.jobs) != 0 {
		t.Fatal("submitted a backfill for a range the calendar could not resolve")
	}
}

// Ruling 66 (3c): without the instrument the OI need is unknown, so nothing is submitted.
func TestCandles_UnknownTokenSubmitsNothing(t *testing.T) {
	e := newSvcEnv(t)
	withChartDeps(t, e)
	e.candles.missing = []candles.Range{{From: ist(2026, 10, 5, 9, 15), To: ist(2026, 10, 6, 15, 30)}}
	res, err := e.svc.Candles(context.Background(), req())
	if err != nil || res.Complete || res.BackfillJobID != "" || len(e.backfill.jobs) != 0 {
		t.Fatalf("%+v %v jobs %d", res, err, len(e.backfill.jobs))
	}
}

// Ruling 66 (5): missing wiring is an explicit internal refusal, not a panic.
func TestRefreshCandles_NilDepsRefused(t *testing.T) {
	e := newSvcEnv(t)
	withChartDeps(t, e)
	rr := RefreshRequest{Token: 256265, Interval: "minute", FromMs: ist(2026, 10, 12, 9, 15).UnixMilli(), ToMs: ist(2026, 10, 12, 10, 0).UnixMilli()}
	e.svc.d.Historical = nil
	if _, err := e.svc.RefreshCandles(context.Background(), rr); refusedCode(t, err) != "internal" {
		t.Fatal(err)
	}
	withChartDeps(t, e)
	e.svc.d.CandleWriter = nil
	if _, err := e.svc.RefreshCandles(context.Background(), rr); refusedCode(t, err) != "internal" {
		t.Fatal(err)
	}
}

// loadedFinder is a master that reports its size (*instruments.Master.Len).
type loadedFinder struct{ fakeInstruments }

func (f *loadedFinder) Len() int { return len(f.m) }

// Ruling 66 (7): with a loaded master, a token it does not list is refused.
func TestOpenLive_UnknownTokenWithLoadedMaster(t *testing.T) {
	e := newSvcEnv(t)
	_, _, tr := withChartDeps(t, e)
	lf := &loadedFinder{fakeInstruments{m: map[uint32]instruments.Instrument{256265: {Token: 256265, Exchange: "NSE", Tradingsymbol: "NIFTY 50"}}}}
	e.svc.d.Instruments = lf
	if _, err := e.svc.OpenLive(context.Background(), 99, "minute"); refusedCode(t, err) != "not_found" {
		t.Fatal(err)
	}
	if n, _ := tr.counts(99, candles.Minute); n != 0 || e.live.has(ticker.ConsumerChart, 99) {
		t.Fatal("unknown token was subscribed or tracked")
	}
	if _, err := e.svc.OpenLive(context.Background(), 256265, "minute"); err != nil {
		t.Fatal(err)
	}
}

// The production master must expose Len, or OpenLive's unknown-token check would never apply.
var _ masterSize = (*instruments.Master)(nil)
