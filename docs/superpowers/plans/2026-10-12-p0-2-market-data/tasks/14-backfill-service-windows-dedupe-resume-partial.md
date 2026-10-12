> **P0-2 Time & Market Data Implementation Plan — Task 14.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 14: Backfill service: windows, dedupe, resume, partial candles, progress

**Files:**
- Create: `internal/candles/backfill/windows.go`
- Create: `internal/candles/backfill/backfill.go`
- Test: `internal/candles/backfill/backfill_test.go`

**Interfaces:**
- Consumes: Task 1: `clock.Clock`, `clock.Fake`. Task 3: `calendar.Date`, `calendar.DailyCandleCheck` (Task 5). Task 6: `ratelimit.Priority`, `ratelimit.New`, `Limiter.PauseAll/Resume/Snapshot`. Task 7: `kite.HistoricalPicker`, `kite.HistoricalRequest`, `kite.HistoricalData`, `kite.Time`, `kite.Err`, `kite.Classify`, kinds. Task 8: `mock.NewFake`, `Fake.Limiter`, `Fake.HistoricalFunc`. Tasks 10–12: `candles.Store`, `candles.JobStore`, `sqlite.Open`. P0-1: `bus.*`, `logging.Go`.
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
var MaxWindow = map[candles.Interval]time.Duration{
	candles.Minute:   60 * day,
	candles.Minute3:  100 * day,
	candles.Minute5:  100 * day,
	candles.Minute10: 100 * day,
	candles.Minute15: 200 * day,
	candles.Minute30: 200 * day,
	candles.Minute60: 400 * day,
	candles.Day:      2000 * day,
}
type Job struct {
	Token     uint32
	Interval  candles.Interval
	From, To  time.Time
	Requester string // "chart", "backtest:<run>", "calendar", …
	Priority  ratelimit.Priority
	OI        bool // futures: fetch open interest (oi=1)
}
type Progress struct {
	JobID    string        `json:"jobId"`
	Token    uint32        `json:"token"`
	Interval string        `json:"interval"`
	Done     int           `json:"done"`
	Failed   int           `json:"failed"`
	Total    int           `json:"total"`
	ETA      time.Duration `json:"etaNs"`
	State    string        `json:"state"` // running | done | failed
}
var TopicProgress = bus.NewTopic[Progress]("backfill.progress")
var ErrFailed = errors.New("backfill: some windows failed")
type Options struct {
	Store   candles.Store
	Jobs    candles.JobStore
	Fetcher kite.HistoricalPicker
	Clock   clock.Clock
	Bus     *bus.Bus
	Log     *slog.Logger
	Workers int // concurrent fetchers; default 3 (= historical rate)
}
type Backfill struct{ /* unexported fields */ }
func New(o Options) *Backfill
func (b *Backfill) Start(ctx context.Context) error
func (b *Backfill) WaitStopped()
func (b *Backfill) Submit(ctx context.Context, j Job) (string, error)
func (b *Backfill) Wait(ctx context.Context, id string) (Progress, error)
func (b *Backfill) Ensure(ctx context.Context, j Job) error
func (b *Backfill) DailyCheck(token uint32) calendar.DailyCandleCheck
```

FR-DATA-003 backfill: split missing ranges into Kite windows, persist per-window state, reuse and raise the priority of overlapping queued windows (AC5, FR-DATA-002 AC1), choose the window when the historical slot is granted (AC1), resume without refetching done windows (AC3), halve on a window error, re-queue on `TokenException`, store Kite's forming candle incomplete and leave it "missing" (OQ-35), publish `backfill.progress` with ETA from the last 20 windows. `DailyCheck` supplies the calendar inference rule (FR-DATA-001 AC5).

- [ ] **Step 0: Add the UUID dependency**

Run: `GOWORK=off go get github.com/google/uuid@v1.6.0`
Expected: `go.mod` lists `github.com/google/uuid v1.6.0` as a direct requirement (job ids are UUIDv7, §4.2 convention).

- [ ] **Step 1: Write the failing tests**

`internal/candles/backfill/backfill_test.go`:

```go
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
	for len(sub.C()) > 0 {
		last = <-sub.C()
		if last.State == "running" && last.Done >= 2 && last.ETA > 0 {
			sawETA = true
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/candles/backfill/`
Expected: the build fails — output contains `undefined: New` and `[build failed]`.

- [ ] **Step 3: Write the implementation**

`internal/candles/backfill/windows.go`:

```go
// Package backfill is the historical backfill service (FR-DATA-003, ARCHITECTURE §5.5): jobs are
// split into Kite-sized windows, persisted in the candle DB, fetched through the shared limiter,
// resumed after restart, de-duplicated across consumers, and reported on the bus.
package backfill

import (
	"time"

	"tradeloom/internal/candles"
)

const day = 24 * time.Hour

// MaxWindow is Kite's maximum days per historical request per interval (A07 §A4).
var MaxWindow = map[candles.Interval]time.Duration{
	candles.Minute:   60 * day,
	candles.Minute3:  100 * day,
	candles.Minute5:  100 * day,
	candles.Minute10: 100 * day,
	candles.Minute15: 200 * day,
	candles.Minute30: 200 * day,
	candles.Minute60: 400 * day,
	candles.Day:      2000 * day,
}

// plan cuts each range into windows of at most MaxWindow[iv].
func plan(rs []candles.Range, iv candles.Interval) []candles.Range {
	var out []candles.Range
	for _, r := range rs {
		out = append(out, candles.Split(r, MaxWindow[iv])...)
	}
	return out
}

// halve splits a window in two at a whole-day boundary; ok=false when it is one day or less.
func halve(r candles.Range) ([]candles.Range, bool) {
	span := r.To.Sub(r.From)
	if span <= day {
		return nil, false
	}
	mid := r.From.Add((span / 2).Truncate(day))
	if !mid.After(r.From) {
		mid = r.From.Add(day)
	}
	return []candles.Range{{From: r.From, To: mid}, {From: mid, To: r.To}}, true
}
```

`internal/candles/backfill/backfill.go`:

```go
package backfill

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"tradeloom/internal/bus"
	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/ratelimit"
	"tradeloom/internal/logging"
)

// Job is a backfill request.
type Job struct {
	Token     uint32
	Interval  candles.Interval
	From, To  time.Time
	Requester string // "chart", "backtest:<run>", "calendar", …
	Priority  ratelimit.Priority
	OI        bool // futures: fetch open interest (oi=1)
}

// Progress is published on TopicProgress after every window (FR-DATA-003 "reports progress").
type Progress struct {
	JobID    string        `json:"jobId"`
	Token    uint32        `json:"token"`
	Interval string        `json:"interval"`
	Done     int           `json:"done"`
	Failed   int           `json:"failed"`
	Total    int           `json:"total"`
	ETA      time.Duration `json:"etaNs"`
	State    string        `json:"state"` // running | done | failed
}

// TopicProgress is "backfill.progress" (§3.4).
var TopicProgress = bus.NewTopic[Progress]("backfill.progress")

// ErrFailed is returned by Ensure when some windows failed after their retries.
var ErrFailed = errors.New("backfill: some windows failed")

// Options configures Backfill.
type Options struct {
	Store   candles.Store
	Jobs    candles.JobStore
	Fetcher kite.HistoricalPicker
	Clock   clock.Clock
	Bus     *bus.Bus
	Log     *slog.Logger
	Workers int // concurrent fetchers; default 3 (= historical rate)
}

// Backfill is the service.
type Backfill struct {
	o       Options
	submit  sync.Mutex // serialises Submit so overlapping requests see each other's windows
	mu      sync.Mutex
	waiters map[string][]chan Progress
	recent  []time.Time // completion times of the last 20 windows (ETA)
	wake    chan struct{}
	workers sync.WaitGroup
}

// New builds the service; call Start to run workers.
func New(o Options) *Backfill {
	if o.Workers == 0 {
		o.Workers = 3
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	return &Backfill{o: o, waiters: map[string][]chan Progress{}, wake: make(chan struct{}, o.Workers)}
}

// Start re-queues windows a crash left running (done windows are never refetched, FR-DATA-003
// AC3) and starts the workers.
func (b *Backfill) Start(ctx context.Context) error {
	n, err := b.o.Jobs.ResetRunning(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		b.o.Log.Info("backfill resumed", "requeued_windows", n)
	}
	for i := 0; i < b.o.Workers; i++ {
		b.workers.Add(1)
		logging.Go(fmt.Sprintf("backfill.worker.%d", i), func() {
			defer b.workers.Done()
			b.worker(ctx)
		})
	}
	b.pokeAll()
	return nil
}

// WaitStopped blocks until every worker has returned after Start's ctx ended (shutdown: call it
// before closing the candle DB). It returns at once if Start was never called.
func (b *Backfill) WaitStopped() { b.workers.Wait() }

// poke wakes one idle worker.
func (b *Backfill) poke() {
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

func (b *Backfill) pokeAll() {
	for i := 0; i < b.o.Workers; i++ {
		b.poke()
	}
}

// Submit records a job for the parts of [From, To) not yet covered and not already queued by
// another job, reusing (and raising the priority of) overlapping queued windows.
func (b *Backfill) Submit(ctx context.Context, j Job) (string, error) {
	if !j.Interval.IsNative() {
		return "", fmt.Errorf("backfill: %q is not a Kite interval", j.Interval)
	}
	now := b.o.Clock.Now()
	if j.To.After(now) {
		j.To = now
	}
	b.submit.Lock()
	defer b.submit.Unlock()
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	missing, err := b.o.Store.Missing(ctx, j.Token, j.Interval, j.From, j.To)
	if err != nil {
		return "", err
	}
	active, err := b.o.Jobs.ActiveWindows(ctx, j.Token, j.Interval)
	if err != nil {
		return "", err
	}
	var ws []candles.Window
	var activeRanges []candles.Range
	for _, a := range active {
		activeRanges = append(activeRanges, candles.Range{From: a.From, To: a.To})
	}
	for _, m := range missing {
		for _, a := range active {
			if a.From.Before(m.To) && a.To.After(m.From) {
				ws = append(ws, a)
			}
		}
	}
	seq := 0
	for _, m := range missing {
		for _, r := range plan(candles.Subtract(m, activeRanges), j.Interval) {
			ws = append(ws, candles.Window{JobID: id.String(), Seq: seq, From: r.From, To: r.To, Priority: int(j.Priority)})
			seq++
		}
	}
	job := candles.Job{ID: id.String(), Token: j.Token, Interval: j.Interval, From: j.From, To: j.To,
		Requester: j.Requester, Priority: int(j.Priority), OI: j.OI, CreatedAt: now}
	if err := b.o.Jobs.CreateJob(ctx, job, ws); err != nil {
		return "", err
	}
	for _, m := range missing {
		if err := b.o.Jobs.RaisePriority(ctx, j.Token, j.Interval, m, int(j.Priority)); err != nil {
			return "", err
		}
	}
	b.o.Log.Info("backfill job", "id", job.ID, "token", j.Token, "interval", string(j.Interval),
		"requester", j.Requester, "priority", j.Priority.String(), "windows", seq, "reused", len(ws)-seq)
	b.update(ctx, job.ID)
	b.pokeAll()
	return job.ID, nil
}

// Wait blocks until job id is done or failed, or ctx ends.
func (b *Backfill) Wait(ctx context.Context, id string) (Progress, error) {
	ch := make(chan Progress, 1)
	b.mu.Lock()
	b.waiters[id] = append(b.waiters[id], ch)
	b.mu.Unlock()
	b.update(ctx, id) // may already be complete
	select {
	case p := <-ch:
		return p, nil
	case <-ctx.Done():
		return Progress{}, ctx.Err()
	}
}

// Ensure submits j and waits for it; ErrFailed when any window failed.
func (b *Backfill) Ensure(ctx context.Context, j Job) error {
	id, err := b.Submit(ctx, j)
	if err != nil {
		return err
	}
	p, err := b.Wait(ctx, id)
	if err != nil {
		return err
	}
	if p.State == "failed" {
		return ErrFailed
	}
	return nil
}

// DailyCheck returns the calendar inference check (FR-DATA-001): fetch token's day candle for d
// at interactive priority and report whether one exists.
func (b *Backfill) DailyCheck(token uint32) calendar.DailyCandleCheck {
	return func(ctx context.Context, d calendar.Date) (bool, error) {
		from := d.Midnight()
		if err := b.Ensure(ctx, Job{Token: token, Interval: candles.Day, From: from, To: from.AddDate(0, 0, 1), Requester: "calendar", Priority: ratelimit.Interactive}); err != nil {
			return false, err
		}
		cs, err := b.o.Store.Read(ctx, token, candles.Day, from, from.AddDate(0, 0, 1))
		return len(cs) > 0, err
	}
}

func (b *Backfill) worker(ctx context.Context) {
	for ctx.Err() == nil {
		p, ok, err := b.o.Jobs.BestQueuedPriority(ctx)
		if err != nil || !ok {
			select {
			case <-ctx.Done():
				return
			case <-b.wake:
				continue
			}
		}
		var w candles.Window
		_, data, picked, ferr := b.o.Fetcher.HistoricalPick(ctx, ratelimit.Priority(p), func() (kite.HistoricalRequest, bool) {
			cw, ok, err := b.o.Jobs.ClaimNext(ctx)
			if err != nil || !ok {
				return kite.HistoricalRequest{}, false
			}
			w = cw
			b.poke() // let an idle worker queue for the next slot
			// Kite's to is inclusive; windows are half-open.
			return kite.HistoricalRequest{Token: int(cw.Token), Interval: string(cw.Interval), From: cw.From, To: cw.To.Add(-time.Second), OI: cw.OI}, true
		})
		if !picked {
			continue
		}
		b.finish(context.WithoutCancel(ctx), w, data, ferr)
		b.poke() // other workers may be idle while windows remain
	}
}

func (b *Backfill) finish(ctx context.Context, w candles.Window, data []kite.HistoricalData, ferr error) {
	if ferr == nil {
		ferr = b.store(ctx, w, data)
	}
	if ferr == nil {
		_ = b.o.Jobs.FinishWindow(ctx, w.JobID, w.Seq, candles.WindowDone, w.Attempts+1, "")
		b.mu.Lock()
		b.recent = append(b.recent, b.o.Clock.Now())
		if len(b.recent) > 20 {
			b.recent = b.recent[len(b.recent)-20:]
		}
		b.mu.Unlock()
	} else {
		switch kind := kite.Classify(ferr); {
		case kind == kite.KindCanceled || kind == kite.KindSession:
			// shutdown or expired session: back to the queue untouched; the limiter is paused
			// until login, so nothing is fetched meanwhile.
			_ = b.o.Jobs.FinishWindow(ctx, w.JobID, w.Seq, candles.WindowQueued, w.Attempts, "")
		case kind == kite.KindPermission:
			_ = b.o.Jobs.FinishWindow(ctx, w.JobID, w.Seq, candles.WindowFailed, w.Attempts+1, "historical data is not available on this Kite plan")
		case kind == kite.KindInput:
			if parts, ok := halve(candles.Range{From: w.From, To: w.To}); ok {
				b.o.Log.Info("backfill window too large; halving", "job", w.JobID, "seq", w.Seq, "err", ferr.Error())
				_ = b.o.Jobs.SplitWindow(ctx, w, parts)
				break
			}
			_ = b.o.Jobs.FinishWindow(ctx, w.JobID, w.Seq, candles.WindowFailed, w.Attempts+1, ferr.Error())
		default:
			st := candles.WindowQueued
			if w.Attempts+1 >= 3 {
				st = candles.WindowFailed
			}
			_ = b.o.Jobs.FinishWindow(ctx, w.JobID, w.Seq, st, w.Attempts+1, ferr.Error())
		}
	}
	b.updateAll(ctx, w.Token, w.Interval)
}

// store writes a fetched window. Candles that have not ended on the app clock (Kite may return the
// forming candle, OQ-35) are stored incomplete, and coverage stops before the first of them so
// it is fetched again later.
func (b *Backfill) store(ctx context.Context, w candles.Window, data []kite.HistoricalData) error {
	now := b.o.Clock.Now()
	dur := w.Interval.Duration()
	var firstOpen *time.Time
	cs := make([]candles.Candle, 0, len(data))
	for _, d := range data {
		start := d.Date.Time.In(clock.IST)
		complete := !start.Add(dur).After(now)
		if !complete && firstOpen == nil {
			firstOpen = &start
		}
		cs = append(cs, candles.Candle{Start: start, Open: d.Open, High: d.High, Low: d.Low, Close: d.Close,
			Volume: int64(d.Volume), OI: int64(d.OI), HasOI: w.OI, Source: candles.SourceHistorical, Complete: complete})
	}
	coverTo := w.To
	if firstOpen != nil { // Kite returned the forming candle: cover up to its start
		if firstOpen.Before(coverTo) {
			coverTo = *firstOpen
		}
	} else if lim := now.Add(-dur).Add(time.Second); lim.Before(coverTo) {
		coverTo = lim // a candle starting at or after lim cannot have ended yet
	}
	if coverTo.Before(w.From) {
		coverTo = w.From
	}
	return b.o.Store.UpsertCovered(ctx, w.Token, w.Interval, candles.Range{From: w.From, To: coverTo}, cs, candles.SourceHistorical)
}

func (b *Backfill) updateAll(ctx context.Context, token uint32, iv candles.Interval) {
	jobs, err := b.o.Jobs.OpenJobs(ctx)
	if err != nil {
		b.o.Log.Error("backfill: list jobs", "err", err)
		return
	}
	for _, j := range jobs {
		if j.Token == token && j.Interval == iv {
			b.update(ctx, j.ID)
		}
	}
}

// update recomputes and publishes job id's progress, closing the job and waking waiters when
// every window it depends on is done or failed.
func (b *Backfill) update(ctx context.Context, id string) {
	pr, err := b.o.Jobs.Progress(ctx, id)
	if err != nil {
		return
	}
	p := Progress{JobID: id, Done: pr.Done, Failed: pr.Failed, Total: pr.Total, State: "running"}
	jobs, _ := b.o.Jobs.OpenJobs(ctx)
	open := false
	for _, j := range jobs {
		if j.ID == id {
			p.Token, p.Interval, open = j.Token, string(j.Interval), true
		}
	}
	if pr.Done+pr.Failed == pr.Total {
		p.State = "done"
		if pr.Failed > 0 {
			p.State = "failed"
		}
		if open {
			_ = b.o.Jobs.FinishJob(ctx, id, p.State, "", b.o.Clock.Now())
		}
	}
	b.mu.Lock()
	if n := len(b.recent); n >= 2 {
		if span := b.recent[n-1].Sub(b.recent[0]); span > 0 {
			rate := float64(n-1) / span.Seconds()
			p.ETA = time.Duration(float64(pr.Total-pr.Done-pr.Failed) / rate * float64(time.Second))
		}
	}
	var ws []chan Progress
	if p.State != "running" {
		ws = b.waiters[id]
		delete(b.waiters, id)
	}
	b.mu.Unlock()
	for _, ch := range ws {
		ch <- p
	}
	if open || p.State != "running" {
		publish(b.o.Bus, TopicProgress, p)
	}
}

// publish sends v on t when a bus is configured (tests and tools may run without one).
func publish[T any](b *bus.Bus, t bus.Topic[T], v T) {
	if b != nil {
		bus.Publish(b, t, v)
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/candles/backfill/`
Expected: `ok  	tradeloom/internal/candles/backfill`; no `DATA RACE`.

- [ ] **Step 5: Commit**

```bash
git add internal/candles/backfill/windows.go \
  internal/candles/backfill/backfill.go \
  internal/candles/backfill/backfill_test.go
git commit -m "$(cat <<'EOF'
feat(candles/backfill): resumable, de-duplicated historical backfill with progress events

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
