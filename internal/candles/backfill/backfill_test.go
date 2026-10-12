package backfill

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/candles/sqlite"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/mock"
	"tradeloom/internal/kite/ratelimit"
)

func ist(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, clock.IST)
}

type rig struct {
	store *sqlite.Store
	fake  *mock.Fake
	clk   *clock.Fake
	bus   *bus.Bus
	bf    *Backfill
	path  string
	ctx   context.Context // cancelled before the store closes, so workers stop first
}

func newRig(t *testing.T, now time.Time, workers int, path string) *rig {
	t.Helper()
	if path == "" {
		path = filepath.Join(t.TempDir(), "candles.db")
	}
	cal, _ := calendar.NewDefault()
	fc := clock.NewFake(now)
	st, err := sqlite.Open(context.Background(), path, sqlite.Options{Clock: fc, Calendar: cal})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	r := &rig{store: st, fake: mock.NewFake(), clk: fc, bus: bus.New(nil), path: path, ctx: ctx}
	r.bf = New(Options{Store: st, Jobs: st, Fetcher: r.fake, Clock: fc, Bus: r.bus, Workers: workers})
	t.Cleanup(func() { cancel(); r.bf.WaitStopped() }) // runs before the store closes
	return r
}

// candlesFor answers a request with one candle per interval step inside [From, To].
func candlesFor(req kite.HistoricalRequest, step time.Duration) []kite.HistoricalData {
	var out []kite.HistoricalData
	for t := req.From; !t.After(req.To); t = t.Add(step) {
		out = append(out, kite.HistoricalData{Date: kite.Time{Time: t}, Open: 100, High: 101, Low: 99, Close: 100.5, Volume: 10})
	}
	return out
}

func overlaps(a, b kite.HistoricalRequest) bool { return a.From.Before(b.To) && b.From.Before(a.To) }

func TestWindowSizesPerInterval(t *testing.T) {
	r := candles.Range{From: ist(2020, 1, 1, 0, 0), To: ist(2026, 1, 1, 0, 0)}
	cases := map[candles.Interval]int{candles.Minute: 37, candles.Minute5: 22, candles.Minute15: 11, candles.Minute60: 6, candles.Day: 2}
	for iv, n := range cases {
		if got := len(plan([]candles.Range{r}, iv)); got != n {
			t.Errorf("%s: %d windows, want %d", iv, got, n)
		}
	}
	if parts, ok := halve(candles.Range{From: ist(2026, 1, 1, 0, 0), To: ist(2026, 3, 2, 0, 0)}); !ok || parts[0].To != ist(2026, 1, 31, 0, 0) {
		t.Fatalf("halve = %v %v", parts, ok)
	}
}

func TestSameWindowFromTwoConsumersIsFetchedOnce(t *testing.T) { // FR-DATA-002 AC1
	r := newRig(t, ist(2026, 10, 12, 18, 0), 3, "")
	r.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) {
		return candlesFor(req, 24*time.Hour), nil
	}
	ctx := context.Background()
	j := Job{Token: 408065, Interval: candles.Minute15, From: ist(2026, 3, 26, 0, 0), To: ist(2026, 10, 12, 0, 0)} // 200 days
	j.Requester, j.Priority = "chart", ratelimit.Interactive
	a, err := r.bf.Submit(ctx, j)
	if err != nil {
		t.Fatal(err)
	}
	j.Requester, j.Priority = "backtest:R1", ratelimit.Backtest
	b, err := r.bf.Submit(ctx, j)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.bf.Start(r.ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{a, b} {
		wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		p, err := r.bf.Wait(wctx, id)
		cancel()
		if err != nil || p.State != "done" {
			t.Fatalf("job %s: %+v %v", id, p, err)
		}
	}
	if n := r.fake.Calls("Historical"); n != 1 {
		t.Fatalf("historical requests = %d, want exactly 1", n)
	}
	got, _ := r.store.Read(ctx, 408065, candles.Minute15, j.From, j.To)
	if len(got) == 0 {
		t.Fatal("consumers see no candles")
	}
}

func TestOverlappingRequestsProduceNonOverlappingWindows(t *testing.T) { // FR-DATA-003 AC5
	r := newRig(t, ist(2026, 10, 12, 18, 0), 3, "")
	r.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) { return nil, nil }
	ctx := context.Background()
	first, err := r.bf.Submit(ctx, Job{Token: 1, Interval: candles.Minute, From: ist(2026, 1, 1, 0, 0), To: ist(2026, 6, 1, 0, 0), Priority: ratelimit.Backtest})
	if err != nil {
		t.Fatal(err)
	}
	id, err := r.bf.Submit(ctx, Job{Token: 1, Interval: candles.Minute, From: ist(2026, 4, 1, 0, 0), To: ist(2026, 9, 1, 0, 0), Priority: ratelimit.Interactive})
	if err != nil {
		t.Fatal(err)
	}
	_ = r.bf.Start(r.ctx)
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, j := range []string{first, id} {
		if _, err := r.bf.Wait(wctx, j); err != nil {
			t.Fatal(err)
		}
	}
	reqs := r.fake.HistoricalRequests()
	for i := range reqs {
		for k := i + 1; k < len(reqs); k++ {
			if overlaps(reqs[i], reqs[k]) {
				t.Fatalf("requests %d and %d overlap: %v–%v / %v–%v", i, k, reqs[i].From, reqs[i].To, reqs[k].From, reqs[k].To)
			}
		}
	}
	if len(reqs) != 5 { // Jan–Jun: 3 windows (60 d); Jun–Sep remainder: 2 windows
		t.Fatalf("requests = %d, want 5", len(reqs))
	}
}

func TestResumeFetchesOnlyRemainingWindows(t *testing.T) { // FR-DATA-003 AC3
	path := filepath.Join(t.TempDir(), "candles.db")
	r1 := newRig(t, ist(2026, 10, 12, 18, 0), 1, path)
	ctx1, kill := context.WithCancel(context.Background())
	var calls atomic.Int32
	r1.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) {
		if calls.Add(1) == 26 {
			kill() // the process dies while the 26th window is in flight
			return nil, context.Canceled
		}
		return nil, nil
	}
	from := ist(2026, 10, 12, 0, 0).Add(-40 * 60 * 24 * time.Hour)
	id, err := r1.bf.Submit(ctx1, Job{Token: 2, Interval: candles.Minute, From: from, To: ist(2026, 10, 12, 0, 0), Priority: ratelimit.Backtest})
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := r1.store.Progress(context.Background(), id); p.Total != 40 {
		t.Fatalf("windows = %d, want 40", p.Total)
	}
	_ = r1.bf.Start(ctx1)
	<-ctx1.Done()
	r1.bf.WaitStopped()
	if p, _ := r1.store.Progress(context.Background(), id); p.Done != 25 {
		t.Fatalf("done before kill = %d, want 25", p.Done)
	}
	r1.store.Close()

	r2 := newRig(t, ist(2026, 10, 12, 18, 5), 3, path)
	r2.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) { return nil, nil }
	ctx := context.Background()
	if err := r2.bf.Start(r2.ctx); err != nil {
		t.Fatal(err)
	}
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if p, err := r2.bf.Wait(wctx, id); err != nil || p.State != "done" {
		t.Fatalf("%+v %v", p, err)
	}
	if n := r2.fake.Calls("Historical"); n != 15 {
		t.Fatalf("after restart fetched %d windows, want 15", n)
	}
}

func TestHistoricalReplacesTickBuiltCandle(t *testing.T) { // FR-DATA-002 AC2
	r := newRig(t, ist(2026, 10, 12, 18, 0), 1, "")
	ctx := context.Background()
	at := ist(2026, 10, 12, 10, 0)
	_ = r.store.Upsert(ctx, 3, candles.Minute5, []candles.Candle{{Start: at, Open: 1, High: 2, Low: 1, Close: 2, Volume: 5}}, candles.SourceTick)
	r.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) {
		return []kite.HistoricalData{{Date: kite.Time{Time: at}, Open: 1.05, High: 2.5, Low: 0.95, Close: 2.2, Volume: 7}}, nil
	}
	_ = r.bf.Start(r.ctx)
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := r.bf.Ensure(wctx, Job{Token: 3, Interval: candles.Minute5, From: ist(2026, 10, 12, 0, 0), To: ist(2026, 10, 13, 0, 0)}); err != nil {
		t.Fatal(err)
	}
	got, _ := r.store.Read(ctx, 3, candles.Minute5, at, at.Add(5*time.Minute))
	if len(got) != 1 || got[0].Close != 2.2 || !got[0].Complete || got[0].Source != candles.SourceHistorical {
		t.Fatalf("stored %+v", got)
	}
}

func TestFormingCandleStoredIncompleteAndNotCovered(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 11, 2).Add(30*time.Second), 1, "")
	ctx := context.Background()
	r.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) {
		var out []kite.HistoricalData
		for m := ist(2026, 10, 12, 9, 15); !m.After(ist(2026, 10, 12, 11, 2)); m = m.Add(time.Minute) {
			out = append(out, kite.HistoricalData{Date: kite.Time{Time: m}, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1})
		}
		return out, nil
	}
	_ = r.bf.Start(r.ctx)
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := r.bf.Ensure(wctx, Job{Token: 4, Interval: candles.Minute, From: ist(2026, 10, 12, 9, 15), To: ist(2026, 10, 12, 15, 30)}); err != nil {
		t.Fatal(err)
	}
	last, _ := r.store.Read(ctx, 4, candles.Minute, ist(2026, 10, 12, 11, 1), ist(2026, 10, 12, 11, 3))
	if len(last) != 2 || !last[0].Complete || last[1].Complete {
		t.Fatalf("11:01 must be complete and 11:02 forming: %+v", last)
	}
	miss, _ := r.store.Missing(ctx, 4, candles.Minute, ist(2026, 10, 12, 9, 15), ist(2026, 10, 12, 15, 30))
	if len(miss) != 1 || !miss[0].From.Equal(ist(2026, 10, 12, 11, 2)) {
		t.Fatalf("missing = %+v, want from 11:02 (the forming candle is fetched again)", miss)
	}
}

func TestTokenExceptionMidBackfillRequeuesWindow(t *testing.T) { // Review Focus 3
	r := newRig(t, ist(2026, 10, 12, 18, 0), 1, "")
	lim := ratelimit.New(r.clk, ratelimit.Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lim.Run(ctx)
	r.fake.Limiter = lim
	var calls atomic.Int32
	r.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) {
		if calls.Add(1) == 2 {
			lim.PauseAll("session expired") // what kite.Client does on TokenException
			return nil, &kite.Err{Kind: kite.KindSession, Op: "Historical", Code: 403, Type: "TokenException", Message: "expired"}
		}
		return nil, nil
	}
	id, err := r.bf.Submit(ctx, Job{Token: 5, Interval: candles.Minute, From: ist(2026, 6, 1, 0, 0), To: ist(2026, 10, 1, 0, 0), Priority: ratelimit.Backtest})
	if err != nil {
		t.Fatal(err)
	}
	_ = r.bf.Start(r.ctx)
	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	p, _ := r.store.Progress(ctx, id)
	if p.Failed != 0 || p.Done != 1 || p.Total != 3 {
		t.Fatalf("after TokenException progress = %+v, want 1 done, 0 failed", p)
	}
	lim.Resume()
	wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
	defer wcancel()
	go func() { // the limiter's 3/s window needs app-clock time to pass
		for wctx.Err() == nil {
			r.clk.Advance(400 * time.Millisecond)
			time.Sleep(5 * time.Millisecond)
		}
	}()
	if pr, err := r.bf.Wait(wctx, id); err != nil || pr.State != "done" || pr.Done != 3 {
		t.Fatalf("%+v %v", pr, err)
	}
}

func TestWindowTooLargeIsHalved(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 18, 0), 1, "")
	var mu sync.Mutex
	first := true
	r.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) {
		mu.Lock()
		defer mu.Unlock()
		if first {
			first = false
			return nil, &kite.Err{Kind: kite.KindInput, Code: 400, Type: "InputException", Message: "interval exceeds max limit: 30 days"}
		}
		return nil, nil
	}
	ctx := context.Background()
	_ = r.bf.Start(r.ctx)
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := r.bf.Ensure(wctx, Job{Token: 6, Interval: candles.Minute, From: ist(2026, 6, 1, 0, 0), To: ist(2026, 7, 31, 0, 0)}); err != nil {
		t.Fatal(err)
	}
	if n := r.fake.Calls("Historical"); n != 3 {
		t.Fatalf("calls = %d, want 1 failed + 2 halves", n)
	}
}

func TestProgressEventsReachSubscribers(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 18, 0), 1, "")
	sub := bus.Subscribe(r.bus, TopicProgress, bus.SubOpts[Progress]{Buffer: 64})
	r.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) {
		r.clk.Advance(time.Second)
		return nil, nil
	}
	ctx := context.Background()
	_ = r.bf.Start(r.ctx)
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := r.bf.Ensure(wctx, Job{Token: 7, Interval: candles.Minute, From: ist(2026, 1, 1, 0, 0), To: ist(2026, 7, 1, 0, 0)}); err != nil {
		t.Fatal(err)
	}
	var last Progress
	var sawETA bool
	// The bus delivers on its own goroutine through an unbuffered channel (len is always 0), so
	// read until the terminal event arrives.
	drain := time.After(2 * time.Second)
	for last.State != "done" && last.State != "failed" {
		select {
		case last = <-sub.C():
			if last.State == "running" && last.Done >= 2 && last.ETA > 0 {
				sawETA = true
			}
		case <-drain:
			t.Fatalf("no terminal progress event; last %+v", last)
		}
	}
	if last.State != "done" || last.Done != last.Total || last.Total != 4 || !sawETA {
		t.Fatalf("last progress %+v, sawETA %v", last, sawETA)
	}
}

// FR-DATA-003 AC1 through backfill: with 10 backtest jobs waiting for historical slots, a chart
// job submitted later is the next request sent.
func TestInteractiveJobOvertakesQueuedBacktests(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 18, 0), 3, "")
	lim := ratelimit.New(r.clk, ratelimit.Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lim.Run(ctx)
	r.fake.Limiter = lim
	r.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) { return nil, nil }
	for tok := uint32(100); tok < 110; tok++ {
		if _, err := r.bf.Submit(ctx, Job{Token: tok, Interval: candles.Day, From: ist(2026, 1, 1, 0, 0), To: ist(2026, 2, 1, 0, 0), Priority: ratelimit.Backtest}); err != nil {
			t.Fatal(err)
		}
	}
	_ = r.bf.Start(r.ctx)
	deadline := time.Now().Add(2 * time.Second)
	for (r.fake.Calls("Historical") < 3 || lim.Snapshot().Classes[ratelimit.ClassHistorical].Queued[ratelimit.Backtest] < 3) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if n := r.fake.Calls("Historical"); n != 3 {
		t.Fatalf("initial burst = %d, want 3", n)
	}
	chart, err := r.bf.Submit(ctx, Job{Token: 999, Interval: candles.Day, From: ist(2026, 1, 1, 0, 0), To: ist(2026, 2, 1, 0, 0), Requester: "chart", Priority: ratelimit.Interactive})
	if err != nil {
		t.Fatal(err)
	}
	r.clk.Advance(1100 * time.Millisecond)
	wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
	defer wcancel()
	if _, err := r.bf.Wait(wctx, chart); err != nil {
		t.Fatal(err)
	}
	if req := r.fake.HistoricalRequests()[3]; req.Token != 999 {
		t.Fatalf("4th request token = %d, want the chart's 999", req.Token)
	}
}

func TestDailyCheckForCalendarInference(t *testing.T) { // FR-DATA-001 AC5 (data side)
	r := newRig(t, ist(2026, 10, 14, 16, 5), 1, "")
	r.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) {
		if req.From.Day() == 13 { // a NIFTY 50 day candle exists for 13 Oct only
			return []kite.HistoricalData{{Date: kite.Time{Time: ist(2026, 10, 13, 0, 0)}, Open: 1, High: 1, Low: 1, Close: 1}}, nil
		}
		return nil, nil
	}
	_ = r.bf.Start(r.ctx)
	check := r.bf.DailyCheck(256265)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if ok, err := check(ctx, calendar.Date{Year: 2026, Month: 10, Day: 13}); err != nil || !ok {
		t.Fatalf("13 Oct: %v %v", ok, err)
	}
	if ok, err := check(ctx, calendar.Date{Year: 2026, Month: 10, Day: 14}); err != nil || ok {
		t.Fatalf("14 Oct: %v %v", ok, err)
	}
}
