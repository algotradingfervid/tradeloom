> **P0-2 Time & Market Data Implementation Plan — Task 10.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 10: Candle model, intervals, ranges and session-anchored resampling

**Files:**
- Create: `internal/candles/interval.go`
- Create: `internal/candles/candle.go`
- Create: `internal/candles/ranges.go`
- Create: `internal/candles/store.go`
- Create: `internal/candles/jobs.go`
- Create: `internal/candles/resample.go`
- Test: `internal/candles/candles_test.go`

**Interfaces:**
- Consumes: Task 1: `clock.IST`, `clock.BucketStart`, `clock.MidnightIST`. Task 3/4: `calendar.Date`, `calendar.DateOf`, `calendar.NewDefault`, `(*Service).Session`, `MergeOverrides`, `ParseFile` (tests).
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
type Interval string
const (
	Minute   Interval = "minute"
	Minute3  Interval = "3minute"
	Minute5  Interval = "5minute"
	Minute10 Interval = "10minute"
	Minute15 Interval = "15minute"
	Minute30 Interval = "30minute"
	Minute60 Interval = "60minute"
	Day      Interval = "day"
	Week     Interval = "week"
	Month    Interval = "month"
)
var Native = []Interval{Minute, Minute3, Minute5, Minute10, Minute15, Minute30, Minute60, Day}
func ParseInterval(s string) (Interval, error)
func (iv Interval) IsNative() bool
func (iv Interval) Table() string
func (iv Interval) Minutes() int
func (iv Interval) IsIntraday() bool
func (iv Interval) Duration() time.Duration
func (iv Interval) Base() Interval
type Source uint8
const (
	SourceHistorical Source = 0
	SourceTick       Source = 1
	SourceManual     Source = 2 // FR-CHART-006 manual refresh
)
type Candle struct {
	Start    time.Time `json:"t"` // candle start, exchange time, IST
	Open     float64   `json:"o"`
	High     float64   `json:"h"`
	Low      float64   `json:"l"`
	Close    float64   `json:"c"`
	Volume   int64     `json:"v"`
	OI       int64     `json:"oi"`
	HasOI    bool      `json:"hasOi"`
	Source   Source    `json:"src"`
	Complete bool      `json:"complete"` // false: tick-built or still forming; replaced by historical
}
func ToPaise(v float64) int64
func FromPaise(p int64) float64
type Range struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}
type Coverage struct {
	Token    uint32   `json:"token"`
	Interval Interval `json:"interval"`
	Ranges   []Range  `json:"ranges"`
}
func Merge(rs []Range) []Range
func Subtract(want Range, have []Range) []Range
func Split(r Range, max time.Duration) []Range
type Store interface {
	// Read returns stored candles of a native interval with Start in [from, to), ascending.
	Read(ctx context.Context, token uint32, iv Interval, from, to time.Time) ([]Candle, error)
	// ReadResampled returns week/month/non-native intervals built from the base interval,
	// anchored per the calendar (FR-DATA-002).
	ReadResampled(ctx context.Context, token uint32, target Interval, from, to time.Time) ([]Candle, error)
	// Upsert writes candles idempotently on (token, start) and updates monthly checksums. A
	// tick-built candle never overwrites a complete historical one. Coverage is unchanged.
	Upsert(ctx context.Context, token uint32, iv Interval, cs []Candle, src Source) error
	// UpsertCovered is Upsert plus marking window as fetched, in one transaction (backfill).
	UpsertCovered(ctx context.Context, token uint32, iv Interval, window Range, cs []Candle, src Source) error
	// Missing returns the parts of [from, to) not covered by fetched ranges.
	Missing(ctx context.Context, token uint32, iv Interval, from, to time.Time) ([]Range, error)
	// Coverage returns the merged fetched ranges.
	Coverage(ctx context.Context, token uint32, iv Interval) (Coverage, error)
	// Checksum returns the hex SHA-256 of the complete candles of (token, iv, month yyyymm).
	Checksum(ctx context.Context, token uint32, iv Interval, month int) (string, error)
	// Stats returns sizes per interval and the configured budget.
	Stats(ctx context.Context) (Stats, error)
	// SetBudget stores the size budget in bytes (0 = none).
	SetBudget(ctx context.Context, bytes int64) error
	// AddRunRef records that backtest run runID used (token, iv, r) (purge guard).
	AddRunRef(ctx context.Context, runID string, token uint32, iv Interval, r Range) error
	// Purge deletes candles of rule.Interval starting before rule.Before. If stored backtest runs
	// reference that data and are not all listed in confirmRuns, nothing is deleted and
	// *RunsReferencedError names them (FR-DATA-002 AC3).
	Purge(ctx context.Context, rule PurgeRule, confirmRuns []string) error
	// Clear deletes every candle, coverage range, checksum and backfill job, with the same guard.
	Clear(ctx context.Context, confirmRuns []string) error
}
type Stats struct {
	Intervals   []IntervalStats `json:"intervals"`
	TotalBytes  int64           `json:"totalBytes"`
	BudgetBytes int64           `json:"budgetBytes"`
}
type IntervalStats struct {
	Interval Interval `json:"interval"`
	Rows     int64    `json:"rows"`
	Bytes    int64    `json:"bytes"`
}
type PurgeRule struct {
	Interval Interval  `json:"interval"`
	Before   time.Time `json:"before"`
}
type RunsReferencedError struct{ Runs []string }
func (e *RunsReferencedError) Error() string
type WindowStatus string
const (
	WindowQueued  WindowStatus = "queued"
	WindowRunning WindowStatus = "running"
	WindowDone    WindowStatus = "done"
	WindowFailed  WindowStatus = "failed"
)
type Job struct {
	ID         string
	Token      uint32
	Interval   Interval
	From, To   time.Time
	Requester  string
	Priority   int // ratelimit.Priority value
	OI         bool
	Status     string // queued | done | failed
	CreatedAt  time.Time
	FinishedAt time.Time
	Error      string
}
type Window struct {
	JobID    string
	Seq      int
	Token    uint32
	Interval Interval
	From, To time.Time
	Priority int
	OI       bool
	Status   WindowStatus
	Attempts int
	Error    string
}
type JobProgress struct {
	Done, Failed, Total int
}
type JobStore interface {
	CreateJob(ctx context.Context, j Job, ws []Window) error
	// ActiveWindows returns queued/running windows of (token, iv) from any job.
	ActiveWindows(ctx context.Context, token uint32, iv Interval) ([]Window, error)
	// RaisePriority lowers the priority value of active windows of (token, iv) overlapping r to p.
	RaisePriority(ctx context.Context, token uint32, iv Interval, r Range, p int) error
	// BestQueuedPriority returns the smallest priority value among queued windows.
	BestQueuedPriority(ctx context.Context) (int, bool, error)
	// ClaimNext marks the best queued window (priority, then job created_at, then seq) running.
	ClaimNext(ctx context.Context) (Window, bool, error)
	// FinishWindow sets a window's terminal or re-queued status; attempts is the new count.
	FinishWindow(ctx context.Context, jobID string, seq int, st WindowStatus, attempts int, errMsg string) error
	// SplitWindow replaces a window by parts (new seqs appended), all queued.
	SplitWindow(ctx context.Context, w Window, parts []Range) error
	// ResetRunning re-queues windows left running by a crash; returns how many.
	ResetRunning(ctx context.Context) (int, error)
	// Progress counts windows of job id plus active windows of other jobs overlapping its range.
	Progress(ctx context.Context, id string) (JobProgress, error)
	// FinishJob marks a job done or failed.
	FinishJob(ctx context.Context, id, status, errMsg string, at time.Time) error
	// OpenJobs returns jobs whose status is queued (for progress and waiters after restart).
	OpenJobs(ctx context.Context) ([]Job, error)
}
type SessionFunc func(d calendar.Date) (start, end time.Time, ok bool)
func ResampleIntraday(cs []Candle, targetMin int, session SessionFunc) []Candle
func DailyFromIntraday(cs []Candle) []Candle
func ResampleWeekly(cs []Candle) []Candle
func ResampleMonthly(cs []Candle) []Candle
```

FR-DATA-002: eight native Kite intervals stored, everything else resampled on read (weeks Mon-based, months per calendar month, intraday anchored at each session start, cut at its end). AC5 golden: week = Monday open, max high, min low, Friday close, Σ volume; Muhurat candles belong to their date. `Store` and `JobStore` are the swap points the SQLite implementation (Tasks 11–12) satisfies.

- [ ] **Step 1: Write the failing tests**

`internal/candles/candles_test.go`:

```go
package candles

import (
	"strings"
	"testing"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/clock"
)

func ist(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, clock.IST)
}

func TestIntervals(t *testing.T) {
	for _, s := range []string{"minute", "3minute", "60minute", "day", "week", "month", "120minute", "45minute"} {
		if _, err := ParseInterval(s); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
	for _, s := range []string{"hour", "0minute", "2day"} {
		if _, err := ParseInterval(s); err == nil {
			t.Errorf("%s accepted", s)
		}
	}
	if Minute15.Table() != "candles_15m" || Week.IsNative() || Interval("120minute").Base() != Minute60 ||
		Interval("45minute").Base() != Minute15 || Month.Base() != Day || Minute.Minutes() != 1 {
		t.Fatal("interval metadata wrong")
	}
	if ToPaise(2450.05) != 245005 || FromPaise(245005) != 2450.05 {
		t.Fatal("paise conversion")
	}
}

func TestRangesMergeSubtractSplit(t *testing.T) {
	r := func(a, b int) Range { return Range{ist(2026, 1, a, 0, 0), ist(2026, 1, b, 0, 0)} }
	m := Merge([]Range{r(5, 7), r(1, 3), r(3, 4), r(6, 9)})
	if len(m) != 2 || m[0] != r(1, 4) || m[1] != r(5, 9) {
		t.Fatalf("merge = %v", m)
	}
	miss := Subtract(r(1, 20), []Range{r(3, 5), r(8, 10), r(9, 12)})
	if len(miss) != 3 || miss[0] != r(1, 3) || miss[1] != r(5, 8) || miss[2] != r(12, 20) {
		t.Fatalf("subtract = %v", miss)
	}
	if got := Subtract(r(3, 4), []Range{r(1, 10)}); len(got) != 0 {
		t.Fatalf("fully covered = %v", got)
	}
	sp := Split(r(1, 11), 4*24*time.Hour)
	if len(sp) != 3 || sp[2] != r(9, 11) {
		t.Fatalf("split = %v", sp)
	}
}

func day(d int, o, h, l, c float64, v int64) Candle {
	return Candle{Start: ist(2026, 10, d, 0, 0), Open: o, High: h, Low: l, Close: c, Volume: v, Complete: true}
}

func TestWeeklyGolden(t *testing.T) { // FR-DATA-002 AC5 part 1
	week := []Candle{
		day(5, 100, 110, 95, 105, 1000), // Mon
		day(6, 105, 120, 101, 118, 2000),
		day(7, 118, 119, 90, 92, 1500),
		day(8, 92, 99, 91, 98, 1200),
		day(9, 98, 104, 97, 103, 800), // Fri
	}
	got := ResampleWeekly(week)
	want := Candle{Start: ist(2026, 10, 5, 0, 0), Open: 100, High: 120, Low: 90, Close: 103, Volume: 6500, Complete: true}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("week = %+v, want %+v", got, want)
	}
	m := ResampleMonthly(append(week, day(12, 103, 106, 100, 101, 10)))
	if len(m) != 1 || m[0].Start != ist(2026, 10, 1, 0, 0) || m[0].Close != 101 {
		t.Fatalf("month = %+v", m)
	}
}

func TestMuhuratCandlesBelongToTheirDate(t *testing.T) { // FR-DATA-002 AC5 part 2
	cal, _ := calendar.NewDefault()
	f, _ := calendar.ParseFile(strings.NewReader(calendar.Header+"\n2026-11-08,NSE,special,continuous,18:15,19:15\n"), "test")
	cal.MergeOverrides(f.Entries)
	var mins []Candle
	for i := 0; i < 60; i++ {
		p := 100 + float64(i)
		mins = append(mins, Candle{Start: ist(2026, 11, 8, 18, 15).Add(time.Duration(i) * time.Minute), Open: p, High: p + 1, Low: p - 1, Close: p + 0.5, Volume: 10, Complete: true})
	}
	d := DailyFromIntraday(mins)
	if len(d) != 1 || d[0].Start != ist(2026, 11, 8, 0, 0) || d[0].Volume != 600 || d[0].Open != 100 || d[0].Close != 159.5 {
		t.Fatalf("muhurat day = %+v", d)
	}
	five := ResampleIntraday(mins, 5, func(dt calendar.Date) (time.Time, time.Time, bool) { return cal.Session(calendar.NSE, dt) })
	if len(five) != 12 || five[0].Start.Format("15:04") != "18:15" || five[11].Start.Format("15:04") != "19:10" {
		t.Fatalf("5m starts: first %v last %v n=%d", five[0].Start, five[len(five)-1].Start, len(five))
	}
	w := ResampleWeekly(d)
	if w[0].Start != ist(2026, 11, 2, 0, 0) {
		t.Fatalf("Sunday session week start = %v, want Monday 02 Nov", w[0].Start)
	}
}

func TestSixtyMinuteBucketsAnchorAt0915AndCutAtClose(t *testing.T) {
	cal, _ := calendar.NewDefault()
	var mins []Candle
	for tm := ist(2026, 10, 12, 9, 15); tm.Before(ist(2026, 10, 12, 15, 30)); tm = tm.Add(time.Minute) {
		mins = append(mins, Candle{Start: tm, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, Complete: true})
	}
	h := ResampleIntraday(mins, 60, func(d calendar.Date) (time.Time, time.Time, bool) { return cal.Session(calendar.NSE, d) })
	if len(h) != 7 || h[0].Start.Format("15:04") != "09:15" || h[6].Start.Format("15:04") != "15:15" || h[6].Volume != 15 {
		t.Fatalf("60m = n%d first %s last %s vol %d", len(h), h[0].Start.Format("15:04"), h[6].Start.Format("15:04"), h[6].Volume)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/candles/`
Expected: the build fails — output contains `undefined: ParseInterval` and `[build failed]`.

- [ ] **Step 3: Write the implementation**

`internal/candles/interval.go`:

```go
// Package candles defines the candle model, the Store interface (the OQ-41 swap point,
// ARCHITECTURE §4.3), coverage ranges and pure resampling. The SQLite implementation is in
// candles/sqlite, backfill in candles/backfill, tick building in candles/builder.
package candles

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Interval uses Kite's interval names; "week", "month" and "<N>minute" (N not native) are
// resampled on read and never stored (FR-DATA-002).
type Interval string

const (
	Minute   Interval = "minute"
	Minute3  Interval = "3minute"
	Minute5  Interval = "5minute"
	Minute10 Interval = "10minute"
	Minute15 Interval = "15minute"
	Minute30 Interval = "30minute"
	Minute60 Interval = "60minute"
	Day      Interval = "day"
	Week     Interval = "week"
	Month    Interval = "month"
)

// Native lists the eight stored intervals, smallest first.
var Native = []Interval{Minute, Minute3, Minute5, Minute10, Minute15, Minute30, Minute60, Day}

var tables = map[Interval]string{
	Minute: "candles_1m", Minute3: "candles_3m", Minute5: "candles_5m", Minute10: "candles_10m",
	Minute15: "candles_15m", Minute30: "candles_30m", Minute60: "candles_60m", Day: "candles_1d",
}

// ParseInterval validates s.
func ParseInterval(s string) (Interval, error) {
	iv := Interval(s)
	if iv == Day || iv == Week || iv == Month || iv == Minute {
		return iv, nil
	}
	if n, ok := strings.CutSuffix(s, "minute"); ok {
		if m, err := strconv.Atoi(n); err == nil && m >= 1 && m <= 375 {
			if m == 1 {
				return Minute, nil
			}
			return iv, nil
		}
	}
	return "", fmt.Errorf("candles: unknown interval %q", s)
}

// IsNative reports whether iv is stored.
func (iv Interval) IsNative() bool { _, ok := tables[iv]; return ok }

// Table is the SQLite table of a native interval ("" otherwise).
func (iv Interval) Table() string { return tables[iv] }

// Minutes is the length of an intraday interval; 0 for day, week, month.
func (iv Interval) Minutes() int {
	if iv == Minute {
		return 1
	}
	if n, ok := strings.CutSuffix(string(iv), "minute"); ok {
		m, _ := strconv.Atoi(n)
		return m
	}
	return 0
}

// IsIntraday reports whether iv is a minute-based interval.
func (iv Interval) IsIntraday() bool { return iv.Minutes() > 0 }

// Duration is the nominal length (intraday only; 24 h for day; 0 for week/month).
func (iv Interval) Duration() time.Duration {
	if iv == Day {
		return 24 * time.Hour
	}
	return time.Duration(iv.Minutes()) * time.Minute
}

// Base returns the native interval a non-native interval is resampled from: the largest native
// intraday interval dividing it, or Day for week/month.
func (iv Interval) Base() Interval {
	if iv == Week || iv == Month {
		return Day
	}
	m := iv.Minutes()
	for i := len(Native) - 2; i >= 0; i-- {
		if n := Native[i].Minutes(); n > 0 && m%n == 0 {
			return Native[i]
		}
	}
	return Minute
}
```

`internal/candles/candle.go`:

```go
package candles

import (
	"math"
	"time"
)

// Source of a stored candle (§4.3 column src).
type Source uint8

const (
	SourceHistorical Source = 0
	SourceTick       Source = 1
	SourceManual     Source = 2 // FR-CHART-006 manual refresh
)

// Candle is what the engine and the chart see. Prices are rupees (stored as integer paise).
type Candle struct {
	Start    time.Time `json:"t"` // candle start, exchange time, IST
	Open     float64   `json:"o"`
	High     float64   `json:"h"`
	Low      float64   `json:"l"`
	Close    float64   `json:"c"`
	Volume   int64     `json:"v"`
	OI       int64     `json:"oi"`
	HasOI    bool      `json:"hasOi"`
	Source   Source    `json:"src"`
	Complete bool      `json:"complete"` // false: tick-built or still forming; replaced by historical
}

// ToPaise converts rupees to integer paise (exact for Kite's 0.05 ticks).
func ToPaise(v float64) int64 { return int64(math.Round(v * 100)) }

// FromPaise converts integer paise to rupees.
func FromPaise(p int64) float64 { return float64(p) / 100 }

// Range is a half-open time range [From, To).
type Range struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// Coverage is the merged, non-overlapping set of fetched ranges of (Token, Interval).
type Coverage struct {
	Token    uint32   `json:"token"`
	Interval Interval `json:"interval"`
	Ranges   []Range  `json:"ranges"`
}
```

`internal/candles/ranges.go`:

```go
package candles

import (
	"sort"
	"time"
)

// Merge returns rs sorted, with overlapping or touching ranges joined and empty ones dropped.
func Merge(rs []Range) []Range {
	var in []Range
	for _, r := range rs {
		if r.To.After(r.From) {
			in = append(in, r)
		}
	}
	sort.Slice(in, func(i, j int) bool { return in[i].From.Before(in[j].From) })
	var out []Range
	for _, r := range in {
		if n := len(out); n > 0 && !r.From.After(out[n-1].To) {
			if r.To.After(out[n-1].To) {
				out[n-1].To = r.To
			}
			continue
		}
		out = append(out, r)
	}
	return out
}

// Subtract returns the parts of want not covered by have (have need not be merged).
func Subtract(want Range, have []Range) []Range {
	if !want.To.After(want.From) {
		return nil
	}
	cur := want.From
	var out []Range
	for _, h := range Merge(have) {
		if !h.To.After(cur) {
			continue
		}
		if !h.From.Before(want.To) {
			break
		}
		if h.From.After(cur) {
			out = append(out, Range{cur, h.From})
		}
		cur = h.To
		if !cur.Before(want.To) {
			return out
		}
	}
	return append(out, Range{cur, want.To})
}

// Split cuts r into consecutive windows of at most max.
func Split(r Range, max time.Duration) []Range {
	var out []Range
	for f := r.From; f.Before(r.To); f = f.Add(max) {
		t := f.Add(max)
		if t.After(r.To) {
			t = r.To
		}
		out = append(out, Range{f, t})
	}
	return out
}
```

`internal/candles/store.go`:

```go
package candles

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Store is the candle store swap point (§4.3). candles/sqlite implements it; a colfile
// implementation would replace it if S0-2 fails.
type Store interface {
	// Read returns stored candles of a native interval with Start in [from, to), ascending.
	Read(ctx context.Context, token uint32, iv Interval, from, to time.Time) ([]Candle, error)
	// ReadResampled returns week/month/non-native intervals built from the base interval,
	// anchored per the calendar (FR-DATA-002).
	ReadResampled(ctx context.Context, token uint32, target Interval, from, to time.Time) ([]Candle, error)
	// Upsert writes candles idempotently on (token, start) and updates monthly checksums. A
	// tick-built candle never overwrites a complete historical one. Coverage is unchanged.
	Upsert(ctx context.Context, token uint32, iv Interval, cs []Candle, src Source) error
	// UpsertCovered is Upsert plus marking window as fetched, in one transaction (backfill).
	UpsertCovered(ctx context.Context, token uint32, iv Interval, window Range, cs []Candle, src Source) error
	// Missing returns the parts of [from, to) not covered by fetched ranges.
	Missing(ctx context.Context, token uint32, iv Interval, from, to time.Time) ([]Range, error)
	// Coverage returns the merged fetched ranges.
	Coverage(ctx context.Context, token uint32, iv Interval) (Coverage, error)
	// Checksum returns the hex SHA-256 of the complete candles of (token, iv, month yyyymm).
	Checksum(ctx context.Context, token uint32, iv Interval, month int) (string, error)
	// Stats returns sizes per interval and the configured budget.
	Stats(ctx context.Context) (Stats, error)
	// SetBudget stores the size budget in bytes (0 = none).
	SetBudget(ctx context.Context, bytes int64) error
	// AddRunRef records that backtest run runID used (token, iv, r) (purge guard).
	AddRunRef(ctx context.Context, runID string, token uint32, iv Interval, r Range) error
	// Purge deletes candles of rule.Interval starting before rule.Before. If stored backtest runs
	// reference that data and are not all listed in confirmRuns, nothing is deleted and
	// *RunsReferencedError names them (FR-DATA-002 AC3).
	Purge(ctx context.Context, rule PurgeRule, confirmRuns []string) error
	// Clear deletes every candle, coverage range, checksum and backfill job, with the same guard.
	Clear(ctx context.Context, confirmRuns []string) error
}

// Stats is shown in Settings → Data.
type Stats struct {
	Intervals   []IntervalStats `json:"intervals"`
	TotalBytes  int64           `json:"totalBytes"`
	BudgetBytes int64           `json:"budgetBytes"`
}

// IntervalStats is the size of one interval's table.
type IntervalStats struct {
	Interval Interval `json:"interval"`
	Rows     int64    `json:"rows"`
	Bytes    int64    `json:"bytes"`
}

// PurgeRule removes Interval candles that start before Before.
type PurgeRule struct {
	Interval Interval  `json:"interval"`
	Before   time.Time `json:"before"`
}

// RunsReferencedError lists the backtest runs a purge would affect.
type RunsReferencedError struct{ Runs []string }

func (e *RunsReferencedError) Error() string {
	return fmt.Sprintf("candles: purge would remove data used by backtest runs: %s", strings.Join(e.Runs, ", "))
}
```

`internal/candles/jobs.go`:

```go
package candles

import (
	"context"
	"time"
)

// WindowStatus is a backfill window's state (§4.3 backfill_windows.status).
type WindowStatus string

const (
	WindowQueued  WindowStatus = "queued"
	WindowRunning WindowStatus = "running"
	WindowDone    WindowStatus = "done"
	WindowFailed  WindowStatus = "failed"
)

// Job is a backfill request persisted in backfill_jobs.
type Job struct {
	ID         string
	Token      uint32
	Interval   Interval
	From, To   time.Time
	Requester  string
	Priority   int // ratelimit.Priority value
	OI         bool
	Status     string // queued | done | failed
	CreatedAt  time.Time
	FinishedAt time.Time
	Error      string
}

// Window is one Kite-sized fetch persisted in backfill_windows.
type Window struct {
	JobID    string
	Seq      int
	Token    uint32
	Interval Interval
	From, To time.Time
	Priority int
	OI       bool
	Status   WindowStatus
	Attempts int
	Error    string
}

// JobProgress counts the windows a job depends on: its own plus other jobs' active windows it
// reused (FR-DATA-003 AC5).
type JobProgress struct {
	Done, Failed, Total int
}

// JobStore persists backfill state in the candle DB (AD-23). candles/sqlite implements it.
type JobStore interface {
	CreateJob(ctx context.Context, j Job, ws []Window) error
	// ActiveWindows returns queued/running windows of (token, iv) from any job.
	ActiveWindows(ctx context.Context, token uint32, iv Interval) ([]Window, error)
	// RaisePriority lowers the priority value of active windows of (token, iv) overlapping r to p.
	RaisePriority(ctx context.Context, token uint32, iv Interval, r Range, p int) error
	// BestQueuedPriority returns the smallest priority value among queued windows.
	BestQueuedPriority(ctx context.Context) (int, bool, error)
	// ClaimNext marks the best queued window (priority, then job created_at, then seq) running.
	ClaimNext(ctx context.Context) (Window, bool, error)
	// FinishWindow sets a window's terminal or re-queued status; attempts is the new count.
	FinishWindow(ctx context.Context, jobID string, seq int, st WindowStatus, attempts int, errMsg string) error
	// SplitWindow replaces a window by parts (new seqs appended), all queued.
	SplitWindow(ctx context.Context, w Window, parts []Range) error
	// ResetRunning re-queues windows left running by a crash; returns how many.
	ResetRunning(ctx context.Context) (int, error)
	// Progress counts windows of job id plus active windows of other jobs overlapping its range.
	Progress(ctx context.Context, id string) (JobProgress, error)
	// FinishJob marks a job done or failed.
	FinishJob(ctx context.Context, id, status, errMsg string, at time.Time) error
	// OpenJobs returns jobs whose status is queued (for progress and waiters after restart).
	OpenJobs(ctx context.Context) ([]Job, error)
}
```

`internal/candles/resample.go`:

```go
package candles

import (
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/clock"
)

// SessionFunc returns the continuous session of IST date d for one instrument's exchange.
type SessionFunc func(d calendar.Date) (start, end time.Time, ok bool)

type agg struct {
	c   Candle
	set bool
}

func (a *agg) add(c Candle) {
	if !a.set {
		a.c = c
		a.set = true
		return
	}
	if c.High > a.c.High {
		a.c.High = c.High
	}
	if c.Low < a.c.Low {
		a.c.Low = c.Low
	}
	a.c.Close = c.Close
	a.c.Volume += c.Volume
	if c.HasOI {
		a.c.OI, a.c.HasOI = c.OI, true
	}
	a.c.Complete = a.c.Complete && c.Complete
	if c.Source != a.c.Source {
		a.c.Source = SourceTick
	}
}

func (a *agg) flush(start time.Time, out []Candle) []Candle {
	if !a.set {
		return out
	}
	a.c.Start = start
	out = append(out, a.c)
	a.set = false
	return out
}

// ResampleIntraday aggregates ascending intraday candles into targetMin-minute buckets anchored
// at each day's session start (FR-DATA-002, FR-BT-034); the last bucket of a session is cut at the
// session end (e.g. 60-minute: 09:15, 10:15, …, 15:15–15:30). Candles outside the session drop.
func ResampleIntraday(cs []Candle, targetMin int, session SessionFunc) []Candle {
	step := time.Duration(targetMin) * time.Minute
	var out []Candle
	var a agg
	var bucket time.Time
	var day calendar.Date
	var start, end time.Time
	var ok bool
	for _, c := range cs {
		d := calendar.DateOf(c.Start)
		if d != day {
			day = d
			start, end, ok = session(d)
		}
		if !ok || c.Start.Before(start) || !c.Start.Before(end) {
			continue
		}
		b := clock.BucketStart(c.Start, start, step)
		if a.set && !b.Equal(bucket) {
			out = a.flush(bucket, out)
		}
		bucket = b
		a.add(c)
	}
	return a.flush(bucket, out)
}

// DailyFromIntraday aggregates intraday candles per IST calendar date; a special session (e.g.
// an evening Muhurat session) therefore belongs to its own date's day candle (FR-DATA-002 AC5).
func DailyFromIntraday(cs []Candle) []Candle {
	return byPeriod(cs, func(t time.Time) time.Time { return clock.MidnightIST(t) })
}

// ResampleWeekly aggregates daily candles into Monday-starting weeks (Start = Monday 00:00 IST);
// a weekend special session joins the week of the preceding Monday.
func ResampleWeekly(cs []Candle) []Candle {
	return byPeriod(cs, func(t time.Time) time.Time {
		m := clock.MidnightIST(t)
		off := (int(m.Weekday()) + 6) % 7 // days since Monday
		return m.AddDate(0, 0, -off)
	})
}

// ResampleMonthly aggregates daily candles per calendar month (Start = 1st, 00:00 IST).
func ResampleMonthly(cs []Candle) []Candle {
	return byPeriod(cs, func(t time.Time) time.Time {
		y, m, _ := t.In(clock.IST).Date()
		return time.Date(y, m, 1, 0, 0, 0, 0, clock.IST)
	})
}

func byPeriod(cs []Candle, key func(time.Time) time.Time) []Candle {
	var out []Candle
	var a agg
	var cur time.Time
	for _, c := range cs {
		k := key(c.Start)
		if a.set && !k.Equal(cur) {
			out = a.flush(cur, out)
		}
		cur = k
		a.add(c)
	}
	return a.flush(cur, out)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/candles/`
Expected: `ok  	tradeloom/internal/candles`; no `DATA RACE`.

- [ ] **Step 5: Commit**

```bash
git add internal/candles/interval.go \
  internal/candles/candle.go \
  internal/candles/ranges.go \
  internal/candles/store.go \
  internal/candles/jobs.go \
  internal/candles/resample.go \
  internal/candles/candles_test.go
git commit -m "$(cat <<'EOF'
feat(candles): candle model, store interfaces, range algebra and resampling

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
