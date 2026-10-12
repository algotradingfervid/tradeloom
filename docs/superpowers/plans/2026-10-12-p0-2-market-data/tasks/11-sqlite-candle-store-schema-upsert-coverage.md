> **P0-2 Time & Market Data Implementation Plan — Task 11.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 11: SQLite candle store: schema, upsert, coverage, checksums, resample, purge

**Files:**
- Create: `internal/candles/sqlite/migrations/0001_init.sql`
- Create: `internal/candles/sqlite/open.go`
- Create: `internal/candles/sqlite/store.go`
- Create: `internal/candles/sqlite/admin.go`
- Create: `internal/candles/sqlite/resampled.go`
- Test: `internal/candles/sqlite/testutil_test.go`
- Test: `internal/candles/sqlite/store_test.go`
- Test: `internal/candles/sqlite/admin_test.go`

**Interfaces:**
- Consumes: Task 1: `clock.Clock`, `clock.Fake`. Task 4: `calendar.Service.Session`. Task 10: every `candles` type and the resample functions. P0-1: `store.NewMigrator(fs.FS) (*store.Migrator, error)`, `(*store.Migrator).Latest`, `(*store.Migrator).Apply`; `modernc.org/sqlite` v1.59.0 in `go.mod`.
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
type Options struct {
	Clock clock.Clock
	// Sessions returns the continuous session of token on date d (anchors for resampling).
	// nil: the NSE session from Calendar.
	Sessions func(token uint32, d calendar.Date) (start, end time.Time, ok bool)
	Calendar *calendar.Service // used when Sessions is nil
}
type Store struct{ /* unexported fields */ }
func Open(ctx context.Context, path string, o Options) (*Store, error)
func (s *Store) Close() error
func (s *Store) PageSize(ctx context.Context) (int, error)
func (s *Store) Read(ctx context.Context, token uint32, iv candles.Interval, from, to time.Time) ([]candles.Candle, error)
func (s *Store) Upsert(ctx context.Context, token uint32, iv candles.Interval, cs []candles.Candle, src candles.Source) error
func (s *Store) UpsertCovered(ctx context.Context, token uint32, iv candles.Interval, window candles.Range, cs []candles.Candle, src candles.Source) error
func (s *Store) Checksum(ctx context.Context, token uint32, iv candles.Interval, month int) (string, error)
func (s *Store) Coverage(ctx context.Context, token uint32, iv candles.Interval) (candles.Coverage, error)
func (s *Store) Missing(ctx context.Context, token uint32, iv candles.Interval, from, to time.Time) ([]candles.Range, error)
func (s *Store) Stats(ctx context.Context) (candles.Stats, error)
func (s *Store) SetBudget(ctx context.Context, bytes int64) error
func (s *Store) AddRunRef(ctx context.Context, runID string, token uint32, iv candles.Interval, r candles.Range) error
func (s *Store) Purge(ctx context.Context, rule candles.PurgeRule, confirmRuns []string) error
func (s *Store) Clear(ctx context.Context, confirmRuns []string) error
func (s *Store) ReadResampled(ctx context.Context, token uint32, target candles.Interval, from, to time.Time) ([]candles.Candle, error)
```

ARCHITECTURE §4.3 schema (integer paise, `WITHOUT ROWID`, `page_size` 8192, WAL, `_txlock=immediate` so concurrent writers wait instead of failing on upgrade). FR-DATA-002 AC2 (store side: historical replaces tick-built, never the reverse), AC3 (purge refuses referenced runs until confirmed), AC6 (checksum of exactly one month changes). Coverage is written only by `UpsertCovered`, so a tick-built candle never marks a range as fetched. `backfill_windows` carries its own `priority` and `backfill_links` records reused windows (additions to §4.3 needed for FR-DATA-003 AC1/AC5).

- [ ] **Step 1: Write the failing tests**

`internal/candles/sqlite/testutil_test.go`:

```go
package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/clock"
)

func ist(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, clock.IST)
}

func openTest(t *testing.T, now time.Time) (*Store, *clock.Fake) {
	t.Helper()
	cal, err := calendar.NewDefault()
	if err != nil {
		t.Fatal(err)
	}
	fc := clock.NewFake(now)
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "candles.db"), Options{Clock: fc, Calendar: cal})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, fc
}

func minute(start time.Time, i int, p float64, complete bool) candles.Candle {
	return candles.Candle{Start: start.Add(time.Duration(i) * time.Minute), Open: p, High: p + 1, Low: p - 1, Close: p + 0.05, Volume: int64(100 + i), Complete: complete}
}
```

`internal/candles/sqlite/store_test.go`:

```go
package sqlite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"tradeloom/internal/candles"
)

func TestNewFileUses8KPagesAndWAL(t *testing.T) {
	s, _ := openTest(t, ist(2026, 10, 12, 10, 0))
	if n, err := s.PageSize(context.Background()); err != nil || n != 8192 {
		t.Fatalf("page_size = %d %v", n, err)
	}
	var mode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("journal_mode = %q %v", mode, err)
	}
}

func TestUpsertReadRoundTripInPaise(t *testing.T) {
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	day := ist(2026, 10, 12, 9, 15)
	in := []candles.Candle{
		{Start: day, Open: 2450.05, High: 2451.1, Low: 2449.95, Close: 2450.5, Volume: 1200, Complete: true},
		{Start: day.Add(time.Minute), Open: 2450.5, High: 2452, Low: 2450, Close: 2451.35, Volume: 900, OI: 15000, HasOI: true, Complete: true},
	}
	if err := s.Upsert(ctx, 738561, candles.Minute, in, candles.SourceHistorical); err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert(ctx, 738561, candles.Minute, in, candles.SourceHistorical); err != nil { // idempotent
		t.Fatal(err)
	}
	got, err := s.Read(ctx, 738561, candles.Minute, day, day.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != in[0] || got[1] != in[1] {
		t.Fatalf("read %+v", got)
	}
	if _, err := s.Read(ctx, 1, candles.Week, day, day); err == nil {
		t.Fatal("Read of a non-native interval must fail")
	}
}

func TestHistoricalReplacesTickBuiltButNotViceVersa(t *testing.T) { // FR-DATA-002 AC2 (store side)
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	at := ist(2026, 10, 12, 10, 0)
	tick := candles.Candle{Start: at, Open: 100, High: 101, Low: 99, Close: 100.5, Volume: 10}
	if err := s.Upsert(ctx, 1, candles.Minute5, []candles.Candle{tick}, candles.SourceTick); err != nil {
		t.Fatal(err)
	}
	hist := candles.Candle{Start: at, Open: 100.05, High: 101.5, Low: 98.9, Close: 100.4, Volume: 12, Complete: true}
	if err := s.Upsert(ctx, 1, candles.Minute5, []candles.Candle{hist}, candles.SourceHistorical); err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert(ctx, 1, candles.Minute5, []candles.Candle{tick}, candles.SourceTick); err != nil { // late tick write
		t.Fatal(err)
	}
	got, _ := s.Read(ctx, 1, candles.Minute5, at, at.Add(5*time.Minute))
	if len(got) != 1 || got[0].Close != 100.4 || !got[0].Complete || got[0].Source != candles.SourceHistorical {
		t.Fatalf("stored %+v, want the historical candle", got)
	}
}

func TestCoverageAndMissing(t *testing.T) {
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	r := func(a, b int) candles.Range {
		return candles.Range{From: ist(2026, 1, a, 0, 0), To: ist(2026, 1, b, 0, 0)}
	}
	for _, w := range []candles.Range{r(1, 5), r(10, 15), r(5, 8)} {
		if err := s.UpsertCovered(ctx, 7, candles.Day, w, nil, candles.SourceHistorical); err != nil {
			t.Fatal(err)
		}
	}
	cov, _ := s.Coverage(ctx, 7, candles.Day)
	if len(cov.Ranges) != 2 || cov.Ranges[0] != r(1, 8) || cov.Ranges[1] != r(10, 15) {
		t.Fatalf("coverage %+v", cov.Ranges)
	}
	miss, _ := s.Missing(ctx, 7, candles.Day, ist(2026, 1, 3, 0, 0), ist(2026, 1, 20, 0, 0))
	if len(miss) != 2 || miss[0] != r(8, 10) || miss[1] != r(15, 20) {
		t.Fatalf("missing %+v", miss)
	}
	if err := s.Upsert(ctx, 7, candles.Day, []candles.Candle{{Start: ist(2026, 1, 9, 0, 0), Complete: true}}, candles.SourceTick); err != nil {
		t.Fatal(err)
	}
	if miss2, _ := s.Missing(ctx, 7, candles.Day, ist(2026, 1, 3, 0, 0), ist(2026, 1, 20, 0, 0)); len(miss2) != 2 {
		t.Fatal("plain Upsert must not change coverage")
	}
}

func TestChecksumChangesOnlyForTouchedMonth(t *testing.T) { // FR-DATA-002 AC6
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	var sep, oct []candles.Candle
	for i := 0; i < 30; i++ {
		sep = append(sep, minute(ist(2026, 9, 30, 9, 15), i, 100, true))
		oct = append(oct, minute(ist(2026, 10, 1, 9, 15), i, 200, true))
	}
	for _, tok := range []uint32{1, 2} {
		if err := s.Upsert(ctx, tok, candles.Minute, append(sep, oct...), candles.SourceHistorical); err != nil {
			t.Fatal(err)
		}
	}
	before := map[string]string{}
	for _, tok := range []uint32{1, 2} {
		for _, m := range []int{202609, 202610} {
			sum, _ := s.Checksum(ctx, tok, candles.Minute, m)
			if sum == "" {
				t.Fatalf("no checksum for %d/%d", tok, m)
			}
			before[fmt.Sprint(tok, "/", m)] = sum
		}
	}
	if err := s.Upsert(ctx, 1, candles.Minute, []candles.Candle{minute(ist(2026, 10, 1, 9, 15), 30, 201, true)}, candles.SourceHistorical); err != nil {
		t.Fatal(err)
	}
	for _, tok := range []uint32{1, 2} {
		for _, m := range []int{202609, 202610} {
			sum, _ := s.Checksum(ctx, tok, candles.Minute, m)
			changed := sum != before[fmt.Sprint(tok, "/", m)]
			if want := tok == 1 && m == 202610; changed != want {
				t.Fatalf("token %d month %d changed=%v, want %v", tok, m, changed, want)
			}
		}
	}
	// Incomplete (tick-built) rows are not part of the checksum.
	sum0, _ := s.Checksum(ctx, 2, candles.Minute, 202610)
	if err := s.Upsert(ctx, 2, candles.Minute, []candles.Candle{minute(ist(2026, 10, 1, 9, 15), 40, 1, false)}, candles.SourceTick); err != nil {
		t.Fatal(err)
	}
	if sum1, _ := s.Checksum(ctx, 2, candles.Minute, 202610); sum1 != sum0 {
		t.Fatal("tick-built candle changed the checksum")
	}
}
```

`internal/candles/sqlite/admin_test.go`:

```go
package sqlite

import (
	"context"
	"errors"
	"testing"

	"tradeloom/internal/candles"
)

func TestPurgeRefusesReferencedRunsUntilConfirmed(t *testing.T) { // FR-DATA-002 AC3
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	old := ist(2024, 3, 4, 9, 15)
	cs := []candles.Candle{minute(old, 0, 100, true), minute(ist(2026, 10, 9, 9, 15), 0, 300, true)}
	if err := s.UpsertCovered(ctx, 5, candles.Minute, candles.Range{From: old, To: ist(2026, 10, 10, 0, 0)}, cs, candles.SourceHistorical); err != nil {
		t.Fatal(err)
	}
	if err := s.AddRunRef(ctx, "R", 5, candles.Minute, candles.Range{From: old, To: ist(2024, 3, 5, 0, 0)}); err != nil {
		t.Fatal(err)
	}
	rule := candles.PurgeRule{Interval: candles.Minute, Before: ist(2024, 10, 12, 0, 0)}
	err := s.Purge(ctx, rule, nil)
	var re *candles.RunsReferencedError
	if !errors.As(err, &re) || len(re.Runs) != 1 || re.Runs[0] != "R" {
		t.Fatalf("err = %v, want RunsReferencedError{R}", err)
	}
	if got, _ := s.Read(ctx, 5, candles.Minute, old, old.AddDate(0, 0, 1)); len(got) != 1 {
		t.Fatal("refused purge deleted data")
	}
	if err := s.Purge(ctx, rule, []string{"R"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Read(ctx, 5, candles.Minute, old, old.AddDate(0, 0, 1)); len(got) != 0 {
		t.Fatal("confirmed purge kept old data")
	}
	if got, _ := s.Read(ctx, 5, candles.Minute, ist(2026, 10, 9, 0, 0), ist(2026, 10, 10, 0, 0)); len(got) != 1 {
		t.Fatal("purge removed recent data")
	}
	if sum, _ := s.Checksum(ctx, 5, candles.Minute, 202403); sum != "" {
		t.Fatal("purged month kept its checksum")
	}
	miss, _ := s.Missing(ctx, 5, candles.Minute, ist(2024, 1, 1, 0, 0), ist(2024, 10, 1, 0, 0))
	if len(miss) != 1 || !miss[0].To.Equal(ist(2024, 10, 1, 0, 0)) {
		t.Fatalf("purged range still covered: %+v", miss)
	}
}

func TestClearAndStatsAndBudget(t *testing.T) {
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	var cs []candles.Candle
	for i := 0; i < 500; i++ {
		cs = append(cs, minute(ist(2026, 10, 9, 9, 15), i, 100, true))
	}
	if err := s.Upsert(ctx, 9, candles.Minute, cs, candles.SourceHistorical); err != nil {
		t.Fatal(err)
	}
	if err := s.SetBudget(ctx, 5<<30); err != nil {
		t.Fatal(err)
	}
	st, err := s.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Intervals[0].Interval != candles.Minute || st.Intervals[0].Rows != 500 || st.Intervals[0].Bytes == 0 || st.BudgetBytes != 5<<30 {
		t.Fatalf("stats %+v", st)
	}
	_ = s.AddRunRef(ctx, "R2", 9, candles.Day, candles.Range{From: cs[0].Start, To: cs[1].Start})
	if err := s.Clear(ctx, nil); err == nil {
		t.Fatal("Clear ignored a referenced run")
	}
	if err := s.Clear(ctx, []string{"R2"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Read(ctx, 9, candles.Minute, cs[0].Start, cs[499].Start.Add(1)); len(got) != 0 {
		t.Fatal("Clear left candles")
	}
}

func TestReadResampledWeekAndIncompleteTail(t *testing.T) {
	ctx := context.Background()
	s, fc := openTest(t, ist(2026, 10, 14, 12, 0)) // Wednesday of the current week
	var days []candles.Candle
	for d := 5; d <= 14; d++ {
		if dt := ist(2026, 10, d, 0, 0); dt.Weekday() >= 1 && dt.Weekday() <= 5 {
			days = append(days, candles.Candle{Start: dt, Open: float64(d), High: float64(d) + 1, Low: float64(d) - 1, Close: float64(d), Volume: 1, Complete: true})
		}
	}
	if err := s.Upsert(ctx, 3, candles.Day, days, candles.SourceHistorical); err != nil {
		t.Fatal(err)
	}
	w, err := s.ReadResampled(ctx, 3, candles.Week, ist(2026, 10, 1, 0, 0), ist(2026, 10, 20, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(w) != 2 || !w[0].Complete || w[1].Complete || w[0].Open != 5 || w[0].Close != 9 || w[0].Volume != 5 {
		t.Fatalf("weeks %+v", w)
	}
	fc.Set(ist(2026, 10, 19, 0, 0))
	w, _ = s.ReadResampled(ctx, 3, candles.Week, ist(2026, 10, 1, 0, 0), ist(2026, 10, 20, 0, 0))
	if !w[1].Complete {
		t.Fatal("week ending before now must be complete")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/candles/sqlite/`
Expected: the build fails — output contains `undefined: Open` and `[build failed]`.

- [ ] **Step 3: Write the implementation**

`internal/candles/sqlite/migrations/0001_init.sql`:

```sql
-- candles/candles.db schema v1 (ARCHITECTURE §4.3). Prices in integer paise, ts = unix seconds
-- of the candle start (exchange time). One WITHOUT ROWID table per native interval.
CREATE TABLE candles_1m  (token INTEGER NOT NULL, ts INTEGER NOT NULL, o INTEGER NOT NULL, h INTEGER NOT NULL, l INTEGER NOT NULL, c INTEGER NOT NULL, v INTEGER NOT NULL, oi INTEGER, src INTEGER NOT NULL, complete INTEGER NOT NULL, PRIMARY KEY(token, ts)) WITHOUT ROWID;
CREATE TABLE candles_3m  (token INTEGER NOT NULL, ts INTEGER NOT NULL, o INTEGER NOT NULL, h INTEGER NOT NULL, l INTEGER NOT NULL, c INTEGER NOT NULL, v INTEGER NOT NULL, oi INTEGER, src INTEGER NOT NULL, complete INTEGER NOT NULL, PRIMARY KEY(token, ts)) WITHOUT ROWID;
CREATE TABLE candles_5m  (token INTEGER NOT NULL, ts INTEGER NOT NULL, o INTEGER NOT NULL, h INTEGER NOT NULL, l INTEGER NOT NULL, c INTEGER NOT NULL, v INTEGER NOT NULL, oi INTEGER, src INTEGER NOT NULL, complete INTEGER NOT NULL, PRIMARY KEY(token, ts)) WITHOUT ROWID;
CREATE TABLE candles_10m (token INTEGER NOT NULL, ts INTEGER NOT NULL, o INTEGER NOT NULL, h INTEGER NOT NULL, l INTEGER NOT NULL, c INTEGER NOT NULL, v INTEGER NOT NULL, oi INTEGER, src INTEGER NOT NULL, complete INTEGER NOT NULL, PRIMARY KEY(token, ts)) WITHOUT ROWID;
CREATE TABLE candles_15m (token INTEGER NOT NULL, ts INTEGER NOT NULL, o INTEGER NOT NULL, h INTEGER NOT NULL, l INTEGER NOT NULL, c INTEGER NOT NULL, v INTEGER NOT NULL, oi INTEGER, src INTEGER NOT NULL, complete INTEGER NOT NULL, PRIMARY KEY(token, ts)) WITHOUT ROWID;
CREATE TABLE candles_30m (token INTEGER NOT NULL, ts INTEGER NOT NULL, o INTEGER NOT NULL, h INTEGER NOT NULL, l INTEGER NOT NULL, c INTEGER NOT NULL, v INTEGER NOT NULL, oi INTEGER, src INTEGER NOT NULL, complete INTEGER NOT NULL, PRIMARY KEY(token, ts)) WITHOUT ROWID;
CREATE TABLE candles_60m (token INTEGER NOT NULL, ts INTEGER NOT NULL, o INTEGER NOT NULL, h INTEGER NOT NULL, l INTEGER NOT NULL, c INTEGER NOT NULL, v INTEGER NOT NULL, oi INTEGER, src INTEGER NOT NULL, complete INTEGER NOT NULL, PRIMARY KEY(token, ts)) WITHOUT ROWID;
CREATE TABLE candles_1d  (token INTEGER NOT NULL, ts INTEGER NOT NULL, o INTEGER NOT NULL, h INTEGER NOT NULL, l INTEGER NOT NULL, c INTEGER NOT NULL, v INTEGER NOT NULL, oi INTEGER, src INTEGER NOT NULL, complete INTEGER NOT NULL, PRIMARY KEY(token, ts)) WITHOUT ROWID;
CREATE TABLE coverage (token INTEGER NOT NULL, interval TEXT NOT NULL, from_ts INTEGER NOT NULL, to_ts INTEGER NOT NULL, PRIMARY KEY(token, interval, from_ts));
CREATE TABLE checksums (token INTEGER NOT NULL, interval TEXT NOT NULL, month INTEGER NOT NULL, sha256 TEXT NOT NULL, rows INTEGER NOT NULL, updated_at INTEGER NOT NULL, PRIMARY KEY(token, interval, month));
CREATE TABLE backfill_jobs (id TEXT PRIMARY KEY, token INTEGER NOT NULL, interval TEXT NOT NULL, from_ts INTEGER NOT NULL, to_ts INTEGER NOT NULL, requester TEXT NOT NULL, priority INTEGER NOT NULL, oi INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL, created_at INTEGER NOT NULL, finished_at INTEGER, error TEXT);
-- priority on windows (not only jobs) so an interactive request can raise just the windows it reuses
CREATE TABLE backfill_windows (job_id TEXT NOT NULL REFERENCES backfill_jobs(id) ON DELETE CASCADE, seq INTEGER NOT NULL, from_ts INTEGER NOT NULL, to_ts INTEGER NOT NULL, priority INTEGER NOT NULL, status TEXT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, error TEXT, PRIMARY KEY(job_id, seq));
CREATE INDEX ix_bfw_status ON backfill_windows(status, priority);
-- windows of other jobs a job reused instead of fetching again (FR-DATA-003 AC5)
CREATE TABLE backfill_links (job_id TEXT NOT NULL REFERENCES backfill_jobs(id) ON DELETE CASCADE, dep_job_id TEXT NOT NULL, dep_seq INTEGER NOT NULL, PRIMARY KEY(job_id, dep_job_id, dep_seq));
CREATE TABLE run_refs (run_id TEXT NOT NULL, token INTEGER NOT NULL, interval TEXT NOT NULL, from_ts INTEGER NOT NULL, to_ts INTEGER NOT NULL);
CREATE INDEX ix_run_refs ON run_refs(interval, from_ts);
CREATE TABLE meta (k TEXT PRIMARY KEY, v TEXT NOT NULL);
INSERT INTO meta(k, v) VALUES ('budget_bytes', '0');
```

`internal/candles/sqlite/open.go`:

```go
// Package sqlite implements candles.Store and candles.JobStore on candles/candles.db
// (modernc.org/sqlite). It is one of the two packages allowed to import database/sql (rule 4).
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"time"

	_ "modernc.org/sqlite"

	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/clock"
	"tradeloom/internal/store"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Options configures Open.
type Options struct {
	Clock clock.Clock
	// Sessions returns the continuous session of token on date d (anchors for resampling).
	// nil: the NSE session from Calendar.
	Sessions func(token uint32, d calendar.Date) (start, end time.Time, ok bool)
	Calendar *calendar.Service // used when Sessions is nil
}

// Store is the SQLite candle store.
type Store struct {
	db  *sql.DB
	clk clock.Clock
	ses func(token uint32, d calendar.Date) (time.Time, time.Time, bool)
}

var _ candles.Store = (*Store)(nil)

const dsnPragmas = "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)" +
	"&_pragma=foreign_keys(1)&_pragma=temp_store(MEMORY)&_pragma=mmap_size(1073741824)&_txlock=immediate"

// Open opens (creating if needed) the candle DB at path and migrates it. A new file gets
// page_size 8192, which SQLite only honours before the first table exists and before WAL.
func Open(ctx context.Context, path string, o Options) (*Store, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		boot, err := sql.Open("sqlite", "file:"+path)
		if err != nil {
			return nil, err
		}
		_, err = boot.ExecContext(ctx, "PRAGMA page_size=8192; VACUUM;")
		boot.Close()
		if err != nil {
			return nil, fmt.Errorf("candles: init page size: %w", err)
		}
	}
	db, err := sql.Open("sqlite", "file:"+path+dsnPragmas)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("candles: migrate: %w", err)
	}
	s := &Store{db: db, clk: o.Clock, ses: o.Sessions}
	if s.ses == nil {
		cal := o.Calendar
		s.ses = func(_ uint32, d calendar.Date) (time.Time, time.Time, bool) { return cal.Session(calendar.NSE, d) }
	}
	return s, nil
}

// migrate runs P0-1's migration runner over candles.db's own embedded migrations (§4.4).
func migrate(ctx context.Context, db *sql.DB) error {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	m, err := store.NewMigrator(sub)
	if err != nil {
		return err
	}
	var cur int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&cur); err != nil {
		return err
	}
	if cur > m.Latest() {
		return fmt.Errorf("candles.db schema %d is newer than this version supports (%d)", cur, m.Latest())
	}
	return m.Apply(ctx, db, cur)
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// PageSize reports PRAGMA page_size (tests, S0-2).
func (s *Store) PageSize(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&n)
	return n, err
}
```

`internal/candles/sqlite/store.go`:

```go
package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"tradeloom/internal/candles"
	"tradeloom/internal/clock"
)

func table(iv candles.Interval) (string, error) {
	t := iv.Table()
	if t == "" {
		return "", fmt.Errorf("candles: %q is not a stored interval", iv)
	}
	return t, nil
}

func (s *Store) Read(ctx context.Context, token uint32, iv candles.Interval, from, to time.Time) ([]candles.Candle, error) {
	t, err := table(iv)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT ts, o, h, l, c, v, oi, src, complete FROM `+t+` WHERE token=? AND ts>=? AND ts<? ORDER BY ts`,
		token, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []candles.Candle
	for rows.Next() {
		var ts, o, h, l, c, v int64
		var oi sql.NullInt64
		var src, complete int
		if err := rows.Scan(&ts, &o, &h, &l, &c, &v, &oi, &src, &complete); err != nil {
			return nil, err
		}
		out = append(out, candles.Candle{
			Start: time.Unix(ts, 0).In(clock.IST), Open: candles.FromPaise(o), High: candles.FromPaise(h),
			Low: candles.FromPaise(l), Close: candles.FromPaise(c), Volume: v, OI: oi.Int64, HasOI: oi.Valid,
			Source: candles.Source(src), Complete: complete == 1,
		})
	}
	return out, rows.Err()
}

func (s *Store) Upsert(ctx context.Context, token uint32, iv candles.Interval, cs []candles.Candle, src candles.Source) error {
	return s.write(ctx, token, iv, nil, cs, src)
}

func (s *Store) UpsertCovered(ctx context.Context, token uint32, iv candles.Interval, window candles.Range, cs []candles.Candle, src candles.Source) error {
	return s.write(ctx, token, iv, &window, cs, src)
}

func (s *Store) write(ctx context.Context, token uint32, iv candles.Interval, window *candles.Range, cs []candles.Candle, src candles.Source) error {
	t, err := table(iv)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	st, err := tx.PrepareContext(ctx, `INSERT INTO `+t+`(token, ts, o, h, l, c, v, oi, src, complete) VALUES(?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(token, ts) DO UPDATE SET o=excluded.o, h=excluded.h, l=excluded.l, c=excluded.c, v=excluded.v,
		oi=excluded.oi, src=excluded.src, complete=excluded.complete
		WHERE NOT (excluded.src = 1 AND `+t+`.complete = 1)`)
	if err != nil {
		return err
	}
	defer st.Close()
	months := map[int]bool{}
	for _, c := range cs {
		var oi any
		if c.HasOI {
			oi = c.OI
		}
		complete := 0
		if c.Complete {
			complete = 1
			months[monthOf(c.Start)] = true
		}
		if _, err := st.ExecContext(ctx, token, c.Start.Unix(), candles.ToPaise(c.Open), candles.ToPaise(c.High),
			candles.ToPaise(c.Low), candles.ToPaise(c.Close), c.Volume, oi, int(src), complete); err != nil {
			return err
		}
	}
	if window != nil {
		if err := addCoverage(ctx, tx, token, iv, *window); err != nil {
			return err
		}
	}
	ms := make([]int, 0, len(months))
	for m := range months {
		ms = append(ms, m)
	}
	sort.Ints(ms)
	for _, m := range ms {
		if err := recomputeChecksum(ctx, tx, t, token, iv, m, s.now()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) now() time.Time {
	if s.clk == nil {
		return time.Unix(0, 0)
	}
	return s.clk.Now()
}

func monthOf(t time.Time) int {
	y, m, _ := t.In(clock.IST).Date()
	return y*100 + int(m)
}

func monthBounds(month int) (time.Time, time.Time) {
	start := time.Date(month/100, time.Month(month%100), 1, 0, 0, 0, 0, clock.IST)
	return start, start.AddDate(0, 1, 0)
}

// recomputeChecksum hashes the month's complete rows in ts order: per row ts, o, h, l, c, v, oi
// (oi NULL as -1) as big-endian int64 (FR-DATA-002 AC6, FR-BT-044).
func recomputeChecksum(ctx context.Context, tx *sql.Tx, t string, token uint32, iv candles.Interval, month int, now time.Time) error {
	from, to := monthBounds(month)
	rows, err := tx.QueryContext(ctx, `SELECT ts, o, h, l, c, v, oi FROM `+t+` WHERE token=? AND ts>=? AND ts<? AND complete=1 ORDER BY ts`,
		token, from.Unix(), to.Unix())
	if err != nil {
		return err
	}
	h := sha256.New()
	var buf [8]byte
	n := 0
	for rows.Next() {
		var f [6]int64
		var oi sql.NullInt64
		if err := rows.Scan(&f[0], &f[1], &f[2], &f[3], &f[4], &f[5], &oi); err != nil {
			rows.Close()
			return err
		}
		o := int64(-1)
		if oi.Valid {
			o = oi.Int64
		}
		for _, v := range append(f[:], o) {
			binary.BigEndian.PutUint64(buf[:], uint64(v))
			h.Write(buf[:])
		}
		n++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if n == 0 {
		_, err = tx.ExecContext(ctx, `DELETE FROM checksums WHERE token=? AND interval=? AND month=?`, token, string(iv), month)
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO checksums(token, interval, month, sha256, rows, updated_at) VALUES(?,?,?,?,?,?)
		ON CONFLICT(token, interval, month) DO UPDATE SET sha256=excluded.sha256, rows=excluded.rows, updated_at=excluded.updated_at`,
		token, string(iv), month, hex.EncodeToString(h.Sum(nil)), n, now.UnixMilli())
	return err
}

func (s *Store) Checksum(ctx context.Context, token uint32, iv candles.Interval, month int) (string, error) {
	var sum string
	err := s.db.QueryRowContext(ctx, `SELECT sha256 FROM checksums WHERE token=? AND interval=? AND month=?`, token, string(iv), month).Scan(&sum)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return sum, err
}

type querier interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
}

func coverageRanges(ctx context.Context, q querier, token uint32, iv candles.Interval) ([]candles.Range, error) {
	rows, err := q.QueryContext(ctx, `SELECT from_ts, to_ts FROM coverage WHERE token=? AND interval=? ORDER BY from_ts`, token, string(iv))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []candles.Range
	for rows.Next() {
		var f, t int64
		if err := rows.Scan(&f, &t); err != nil {
			return nil, err
		}
		out = append(out, candles.Range{From: time.Unix(f, 0).In(clock.IST), To: time.Unix(t, 0).In(clock.IST)})
	}
	return out, rows.Err()
}

func writeCoverage(ctx context.Context, tx *sql.Tx, token uint32, iv candles.Interval, rs []candles.Range) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM coverage WHERE token=? AND interval=?`, token, string(iv)); err != nil {
		return err
	}
	for _, r := range candles.Merge(rs) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO coverage(token, interval, from_ts, to_ts) VALUES(?,?,?,?)`,
			token, string(iv), r.From.Unix(), r.To.Unix()); err != nil {
			return err
		}
	}
	return nil
}

func addCoverage(ctx context.Context, tx *sql.Tx, token uint32, iv candles.Interval, r candles.Range) error {
	have, err := coverageRanges(ctx, tx, token, iv)
	if err != nil {
		return err
	}
	return writeCoverage(ctx, tx, token, iv, append(have, r))
}

func (s *Store) Coverage(ctx context.Context, token uint32, iv candles.Interval) (candles.Coverage, error) {
	rs, err := coverageRanges(ctx, s.db, token, iv)
	return candles.Coverage{Token: token, Interval: iv, Ranges: rs}, err
}

func (s *Store) Missing(ctx context.Context, token uint32, iv candles.Interval, from, to time.Time) ([]candles.Range, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT from_ts, to_ts FROM coverage WHERE token=? AND interval=? AND to_ts>? AND from_ts<? ORDER BY from_ts`,
		token, string(iv), from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var have []candles.Range
	for rows.Next() {
		var f, t int64
		if err := rows.Scan(&f, &t); err != nil {
			return nil, err
		}
		have = append(have, candles.Range{From: time.Unix(f, 0).In(clock.IST), To: time.Unix(t, 0).In(clock.IST)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return candles.Subtract(candles.Range{From: from.In(clock.IST), To: to.In(clock.IST)}, have), nil
}
```

`internal/candles/sqlite/admin.go`:

```go
package sqlite

import (
	"context"
	"sort"
	"strconv"
	"time"

	"tradeloom/internal/candles"
	"tradeloom/internal/clock"
)

func (s *Store) Stats(ctx context.Context) (candles.Stats, error) {
	var st candles.Stats
	sizes := map[string][2]int64{}
	// Candle tables are WITHOUT ROWID (index B-trees): every cell, interior or leaf, is one row.
	rows, err := s.db.QueryContext(ctx, `SELECT name, SUM(pgsize), SUM(ncell) FROM dbstat GROUP BY name`)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var name string
		var b, n int64
		if err := rows.Scan(&name, &b, &n); err != nil {
			rows.Close()
			return st, err
		}
		sizes[name] = [2]int64{b, n}
		st.TotalBytes += b
	}
	rows.Close()
	for _, iv := range candles.Native {
		v := sizes[iv.Table()]
		st.Intervals = append(st.Intervals, candles.IntervalStats{Interval: iv, Bytes: v[0], Rows: v[1]})
	}
	var b string
	if err := s.db.QueryRowContext(ctx, `SELECT v FROM meta WHERE k='budget_bytes'`).Scan(&b); err != nil {
		return st, err
	}
	st.BudgetBytes, _ = strconv.ParseInt(b, 10, 64)
	return st, nil
}

func (s *Store) SetBudget(ctx context.Context, bytes int64) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO meta(k, v) VALUES('budget_bytes', ?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, strconv.FormatInt(bytes, 10))
	return err
}

func (s *Store) AddRunRef(ctx context.Context, runID string, token uint32, iv candles.Interval, r candles.Range) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO run_refs(run_id, token, interval, from_ts, to_ts) VALUES(?,?,?,?,?)`,
		runID, token, string(iv), r.From.Unix(), r.To.Unix())
	return err
}

func blockedRuns(ctx context.Context, q querier, where string, args []any, confirm []string) ([]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT DISTINCT run_id FROM run_refs `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ok := map[string]bool{}
	for _, r := range confirm {
		ok[r] = true
	}
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if !ok[id] {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out, rows.Err()
}

// Purge deletes rule.Interval candles before the first day of rule.Before's month (purges work
// in whole months so monthly checksums stay exact).
func (s *Store) Purge(ctx context.Context, rule candles.PurgeRule, confirmRuns []string) error {
	t, err := table(rule.Interval)
	if err != nil {
		return err
	}
	y, m, _ := rule.Before.In(clock.IST).Date()
	cut := time.Date(y, m, 1, 0, 0, 0, 0, clock.IST)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	blocked, err := blockedRuns(ctx, tx, `WHERE interval=? AND from_ts<?`, []any{string(rule.Interval), cut.Unix()}, confirmRuns)
	if err != nil {
		return err
	}
	if len(blocked) > 0 {
		return &candles.RunsReferencedError{Runs: blocked}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+t+` WHERE ts<?`, cut.Unix()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM checksums WHERE interval=? AND month<?`, string(rule.Interval), y*100+int(m)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM coverage WHERE interval=? AND to_ts<=?`, string(rule.Interval), cut.Unix()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE coverage SET from_ts=? WHERE interval=? AND from_ts<?`, cut.Unix(), string(rule.Interval), cut.Unix()); err != nil {
		return err
	}
	return tx.Commit()
}

// Clear empties the candle store and backfill history (Q-A2 decision), with the run guard.
func (s *Store) Clear(ctx context.Context, confirmRuns []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	blocked, err := blockedRuns(ctx, tx, ``, nil, confirmRuns)
	if err != nil {
		return err
	}
	if len(blocked) > 0 {
		return &candles.RunsReferencedError{Runs: blocked}
	}
	for _, iv := range candles.Native {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+iv.Table()); err != nil {
			return err
		}
	}
	for _, q := range []string{`DELETE FROM coverage`, `DELETE FROM checksums`, `DELETE FROM backfill_links`, `DELETE FROM backfill_windows`, `DELETE FROM backfill_jobs`} {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return tx.Commit()
}
```

`internal/candles/sqlite/resampled.go`:

```go
package sqlite

import (
	"context"
	"fmt"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
)

// ReadResampled builds week/month from day candles and non-native minute intervals from their
// base interval, anchored at each session start. The last period is marked incomplete while it
// has not ended on the app clock.
func (s *Store) ReadResampled(ctx context.Context, token uint32, target candles.Interval, from, to time.Time) ([]candles.Candle, error) {
	if target.IsNative() {
		return s.Read(ctx, token, target, from, to)
	}
	base := target.Base()
	src, err := s.Read(ctx, token, base, from, to)
	if err != nil {
		return nil, err
	}
	var out []candles.Candle
	var periodEnd func(c candles.Candle) time.Time
	switch {
	case target == candles.Week:
		out = candles.ResampleWeekly(src)
		periodEnd = func(c candles.Candle) time.Time { return c.Start.AddDate(0, 0, 7) }
	case target == candles.Month:
		out = candles.ResampleMonthly(src)
		periodEnd = func(c candles.Candle) time.Time { return c.Start.AddDate(0, 1, 0) }
	case target.IsIntraday():
		out = candles.ResampleIntraday(src, target.Minutes(), func(d calendar.Date) (time.Time, time.Time, bool) { return s.ses(token, d) })
		periodEnd = func(c candles.Candle) time.Time {
			_, end, _ := s.ses(token, calendar.DateOf(c.Start))
			e := c.Start.Add(target.Duration())
			if e.After(end) {
				e = end
			}
			return e
		}
	default:
		return nil, fmt.Errorf("candles: cannot resample to %q", target)
	}
	if n := len(out); n > 0 && s.clk != nil && periodEnd(out[n-1]).After(s.clk.Now()) {
		out[n-1].Complete = false
	}
	return out, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/candles/sqlite/`
Expected: `ok  	tradeloom/internal/candles/sqlite`; no `DATA RACE`.

- [ ] **Step 5: Commit**

```bash
git add internal/candles/sqlite/migrations/0001_init.sql \
  internal/candles/sqlite/open.go \
  internal/candles/sqlite/store.go \
  internal/candles/sqlite/admin.go \
  internal/candles/sqlite/resampled.go \
  internal/candles/sqlite/testutil_test.go \
  internal/candles/sqlite/store_test.go \
  internal/candles/sqlite/admin_test.go
git commit -m "$(cat <<'EOF'
feat(candles/sqlite): candle store with coverage, checksums, resampled reads and guarded purge

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
