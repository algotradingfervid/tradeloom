package backfill

import (
	"context"
	"errors"
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
	defer bus.Unsubscribe(r.bus, sub)
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

// ---- Fix round 1 (Ruling 51) ----

// spyJobs wraps a JobStore: it counts calls, records the call order shared with spyStore, and can
// fail FinishWindow / SplitWindow.
type spyJobs struct {
	candles.JobStore
	order       *callOrder
	best        atomic.Int32
	claims      atomic.Int32
	finishCalls atomic.Int32
	splitCalls  atomic.Int32
	finishFails atomic.Int32 // FinishWindow fails while > 0 (decremented per failure); -1 = always
	splitFails  atomic.Int32
}

type callOrder struct {
	mu    sync.Mutex
	calls []string
}

func (o *callOrder) add(s string) {
	if o != nil {
		o.mu.Lock()
		o.calls = append(o.calls, s)
		o.mu.Unlock()
	}
}

func failNow(n *atomic.Int32) bool {
	for {
		v := n.Load()
		if v == 0 {
			return false
		}
		if v < 0 || n.CompareAndSwap(v, v-1) {
			return true
		}
	}
}

var errDisk = errors.New("disk I/O error")

func (s *spyJobs) ActiveWindows(ctx context.Context, token uint32, iv candles.Interval) ([]candles.Window, error) {
	s.order.add("ActiveWindows")
	return s.JobStore.ActiveWindows(ctx, token, iv)
}
func (s *spyJobs) BestQueuedPriority(ctx context.Context) (int, bool, error) {
	s.best.Add(1)
	return s.JobStore.BestQueuedPriority(ctx)
}
func (s *spyJobs) ClaimNext(ctx context.Context) (candles.Window, bool, error) {
	s.claims.Add(1)
	return s.JobStore.ClaimNext(ctx)
}
func (s *spyJobs) FinishWindow(ctx context.Context, jobID string, seq int, st candles.WindowStatus, attempts int, msg string) error {
	s.finishCalls.Add(1)
	if failNow(&s.finishFails) {
		return errDisk
	}
	return s.JobStore.FinishWindow(ctx, jobID, seq, st, attempts, msg)
}
func (s *spyJobs) SplitWindow(ctx context.Context, w candles.Window, parts []candles.Range) error {
	s.splitCalls.Add(1)
	if failNow(&s.splitFails) {
		return errDisk
	}
	return s.JobStore.SplitWindow(ctx, w, parts)
}

type spyStore struct {
	candles.Store
	order *callOrder
}

func (s *spyStore) Missing(ctx context.Context, token uint32, iv candles.Interval, from, to time.Time) ([]candles.Range, error) {
	s.order.add("Missing")
	return s.Store.Missing(ctx, token, iv, from, to)
}

// spy rebuilds the rig's Backfill over spying stores (before Start).
func (r *rig) spy(workers int) (*spyJobs, *callOrder) {
	o := &callOrder{}
	j := &spyJobs{JobStore: r.store, order: o}
	r.bf = New(Options{Store: &spyStore{Store: r.store, order: o}, Jobs: j, Fetcher: r.fake, Clock: r.clk, Bus: r.bus, Workers: workers})
	return j, o
}

// advance moves the fake clock forward until ctx ends (clock-based back-offs and limiter windows).
func advance(ctx context.Context, clk *clock.Fake, step time.Duration) {
	go func() {
		for ctx.Err() == nil {
			clk.Advance(step)
			time.Sleep(2 * time.Millisecond)
		}
	}()
}

// terminalEvents collects progress events for id until the bus has been quiet for idle.
func terminalEvents(sub *bus.Sub[Progress], id string, idle time.Duration) (n int) {
	for {
		select {
		case p := <-sub.C():
			if p.JobID == id && p.State != "running" {
				n++
			}
		case <-time.After(idle):
			return n
		}
	}
}

// Ruling 51 (1): a limiter that refuses entry must not make the workers spin.
func TestStoppedLimiterDoesNotBusyLoop(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 18, 0), 3, "")
	spy, _ := r.spy(3)
	lim := ratelimit.New(r.clk, ratelimit.Options{})
	lctx, lcancel := context.WithCancel(context.Background())
	lim.Run(lctx)
	lcancel() // the limiter stops; the backfill ctx stays alive
	deadline := time.Now().Add(2 * time.Second)
	for lim.Acquire(context.Background(), ratelimit.ClassHistorical, ratelimit.Interactive) == nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	r.fake.Limiter = lim
	if _, err := r.bf.Submit(context.Background(), Job{Token: 8, Interval: candles.Day, From: ist(2026, 1, 1, 0, 0), To: ist(2026, 2, 1, 0, 0)}); err != nil {
		t.Fatal(err)
	}
	if err := r.bf.Start(r.ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if n := r.fake.Calls("Historical"); n != 0 {
		t.Fatalf("historical calls = %d, want 0", n)
	}
	if n, c := spy.best.Load(), spy.claims.Load(); n > 20 || c > 20 {
		t.Fatalf("workers spin: %d BestQueuedPriority, %d ClaimNext calls in 300 ms", n, c)
	}
}

// Ruling 51 (2): an input error that is not Kite's window-size limit fails the window at once.
func TestOtherInputErrorFailsWithoutHalving(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 18, 0), 1, "")
	r.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) {
		return nil, &kite.Err{Kind: kite.KindInput, Code: 400, Type: "InputException", Message: "invalid token"}
	}
	_ = r.bf.Start(r.ctx)
	wctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := r.bf.Ensure(wctx, Job{Token: 9, Interval: candles.Day, From: ist(2020, 1, 1, 0, 0), To: ist(2026, 1, 1, 0, 0)})
	if !errors.Is(err, ErrFailed) {
		t.Fatalf("Ensure = %v, want ErrFailed", err)
	}
	if n := r.fake.Calls("Historical"); n != 2 { // 2 windows (2000 d each), each tried once
		t.Fatalf("calls = %d, want 2 (one per window, no halving)", n)
	}
}

func TestInvalidTokenInputErrorIsOneCall(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 18, 0), 1, "")
	r.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) {
		return nil, &kite.Err{Kind: kite.KindInput, Code: 400, Type: "InputException", Message: "invalid token"}
	}
	_ = r.bf.Start(r.ctx)
	wctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id, err := r.bf.Submit(wctx, Job{Token: 10, Interval: candles.Minute, From: ist(2026, 6, 1, 0, 0), To: ist(2026, 7, 31, 0, 0)})
	if err != nil {
		t.Fatal(err)
	}
	p, err := r.bf.Wait(wctx, id)
	if err != nil || p.State != "failed" {
		t.Fatalf("%+v %v, want failed", p, err)
	}
	if n := r.fake.Calls("Historical"); n != 1 {
		t.Fatalf("calls = %d, want exactly 1", n)
	}
}

// Ruling 51 (3): a FinishWindow that keeps failing is retried briefly, then the job fails in memory.
func TestPersistentFinishWindowFailureFailsJobInMemory(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 18, 0), 1, "")
	spy, _ := r.spy(1)
	spy.finishFails.Store(-1)
	sub := bus.Subscribe(r.bus, TopicProgress, bus.SubOpts[Progress]{Buffer: 64})
	defer bus.Unsubscribe(r.bus, sub)
	r.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) { return nil, nil }
	_ = r.bf.Start(r.ctx)
	wctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	advance(wctx, r.clk, 100*time.Millisecond)
	id, err := r.bf.Submit(wctx, Job{Token: 11, Interval: candles.Day, From: ist(2026, 1, 1, 0, 0), To: ist(2026, 2, 1, 0, 0)})
	if err != nil {
		t.Fatal(err)
	}
	p, err := r.bf.Wait(wctx, id)
	if err != nil || p.State != "failed" {
		t.Fatalf("%+v %v, want failed", p, err)
	}
	if n := spy.finishCalls.Load(); n != 3 {
		t.Fatalf("FinishWindow tries = %d, want 3", n)
	}
	if n := terminalEvents(sub, id, 200*time.Millisecond); n != 1 {
		t.Fatalf("terminal events = %d, want 1", n)
	}
}

func TestTransientFinishWindowFailureIsRetried(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 18, 0), 1, "")
	spy, _ := r.spy(1)
	spy.finishFails.Store(2)
	r.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) { return nil, nil }
	_ = r.bf.Start(r.ctx)
	wctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	advance(wctx, r.clk, 100*time.Millisecond)
	if err := r.bf.Ensure(wctx, Job{Token: 12, Interval: candles.Day, From: ist(2026, 1, 1, 0, 0), To: ist(2026, 2, 1, 0, 0)}); err != nil {
		t.Fatal(err)
	}
	if n := spy.finishCalls.Load(); n != 3 {
		t.Fatalf("FinishWindow tries = %d, want 2 failures + 1 success", n)
	}
}

func TestPersistentSplitWindowFailureFailsJobInMemory(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 18, 0), 1, "")
	spy, _ := r.spy(1)
	spy.splitFails.Store(-1)
	r.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) {
		return nil, &kite.Err{Kind: kite.KindInput, Code: 400, Type: "InputException", Message: "interval exceeds max limit: 30 days"}
	}
	_ = r.bf.Start(r.ctx)
	wctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	advance(wctx, r.clk, 100*time.Millisecond)
	if err := r.bf.Ensure(wctx, Job{Token: 13, Interval: candles.Minute, From: ist(2026, 6, 1, 0, 0), To: ist(2026, 7, 31, 0, 0)}); !errors.Is(err, ErrFailed) {
		t.Fatalf("Ensure = %v, want ErrFailed", err)
	}
	if n := spy.splitCalls.Load(); n != 3 {
		t.Fatalf("SplitWindow tries = %d, want 3", n)
	}
}

// Ruling 51 (4): Submit reads the active windows before the missing ranges.
func TestSubmitReadsActiveWindowsBeforeMissing(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 18, 0), 1, "")
	_, order := r.spy(1)
	if _, err := r.bf.Submit(context.Background(), Job{Token: 14, Interval: candles.Day, From: ist(2026, 1, 1, 0, 0), To: ist(2026, 2, 1, 0, 0)}); err != nil {
		t.Fatal(err)
	}
	order.mu.Lock()
	defer order.mu.Unlock()
	if len(order.calls) < 2 || order.calls[0] != "ActiveWindows" || order.calls[1] != "Missing" {
		t.Fatalf("call order = %v, want ActiveWindows then Missing", order.calls)
	}
}

// Ruling 51 (6): a cancellation error while the worker is alive (a per-call deadline) counts as an
// attempt, so the window fails after 3 tries instead of looping forever.
func TestDeadlineWhileAliveCountsAsAttempt(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 18, 0), 1, "")
	r.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) {
		return nil, context.DeadlineExceeded
	}
	_ = r.bf.Start(r.ctx)
	wctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.bf.Ensure(wctx, Job{Token: 15, Interval: candles.Day, From: ist(2026, 1, 1, 0, 0), To: ist(2026, 2, 1, 0, 0)}); !errors.Is(err, ErrFailed) {
		t.Fatalf("Ensure = %v, want ErrFailed", err)
	}
	if n := r.fake.Calls("Historical"); n != 3 {
		t.Fatalf("calls = %d, want 3", n)
	}
}

// Ruling 51 (7): a waiter whose ctx ends is removed; an unknown job id is an error.
func TestWaitRemovesWaiterAndRejectsUnknownJob(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 18, 0), 1, "")
	id, err := r.bf.Submit(context.Background(), Job{Token: 16, Interval: candles.Day, From: ist(2026, 1, 1, 0, 0), To: ist(2026, 2, 1, 0, 0)})
	if err != nil {
		t.Fatal(err)
	}
	wctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := r.bf.Wait(wctx, id); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait = %v, want deadline (workers not started)", err)
	}
	r.bf.mu.Lock()
	n := len(r.bf.waiters[id])
	r.bf.mu.Unlock()
	if n != 0 {
		t.Fatalf("%d waiters left after ctx ended", n)
	}
	if _, err := r.bf.Wait(context.Background(), "no-such-job"); !errors.Is(err, ErrUnknownJob) {
		t.Fatalf("Wait(unknown) = %v, want ErrUnknownJob", err)
	}
}

// Ruling 51 (8): each job's terminal event is published exactly once.
func TestTerminalEventPublishedOnce(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 18, 0), 1, "")
	sub := bus.Subscribe(r.bus, TopicProgress, bus.SubOpts[Progress]{Buffer: 64})
	defer bus.Unsubscribe(r.bus, sub)
	r.fake.HistoricalFunc = func(req kite.HistoricalRequest) ([]kite.HistoricalData, error) { return nil, nil }
	_ = r.bf.Start(r.ctx)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	j := Job{Token: 17, Interval: candles.Day, From: ist(2026, 1, 1, 0, 0), To: ist(2026, 2, 1, 0, 0)}
	id, err := r.bf.Submit(ctx, j)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ { // later Waits and a second job on the same series re-run update
		if p, err := r.bf.Wait(ctx, id); err != nil || p.State != "done" {
			t.Fatalf("%+v %v", p, err)
		}
	}
	j.From = ist(2025, 12, 1, 0, 0)
	if err := r.bf.Ensure(ctx, j); err != nil {
		t.Fatal(err)
	}
	if n := terminalEvents(sub, id, 200*time.Millisecond); n != 1 {
		t.Fatalf("terminal events for %s = %d, want 1", id, n)
	}
}

// Ruling 51 (9): Start is single-use.
func TestStartIsSingleUse(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 18, 0), 1, "")
	if err := r.bf.Start(r.ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.bf.Start(r.ctx); err == nil {
		t.Fatal("second Start succeeded")
	}
}
