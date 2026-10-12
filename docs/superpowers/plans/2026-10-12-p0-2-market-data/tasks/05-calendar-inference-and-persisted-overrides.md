> **P0-2 Time & Market Data Implementation Plan — Task 5.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 5: Calendar inference and persisted overrides

**Files:**
- Create: `internal/calendar/infer.go`
- Create: `internal/store/calendar_overrides.go`
- Test: `internal/calendar/infer_test.go`
- Test: `internal/store/calendar_overrides_test.go`

**Interfaces:**
- Consumes: Task 4: `Service`, `MergeOverrides`, `lookup`, `LabelAt`. P0-1: `store.Open(ctx, path, store.Options{BackupDir, Now})`, `DB.W`, `DB.R`, `(*store.DB).Close`, the v1 `calendar_overrides` table.
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
type DailyCandleCheck func(ctx context.Context, d Date) (bool, error)
func (s *Service) Infer(ctx context.Context, now time.Time, d Date, has DailyCandleCheck) (Entry, bool, error)
type CalendarOverrideRow struct {
	Exchange string
	Date     int // yyyymmdd
	Type     string
	Phases   string // JSON
	Source   string
}
func (db *DB) CalendarOverrides(ctx context.Context) ([]CalendarOverrideRow, error)
func (db *DB) UpsertCalendarOverrides(ctx context.Context, rs []CalendarOverrideRow) error
func (db *DB) DeleteCalendarOverride(ctx context.Context, exchange string, date int) error
```

FR-DATA-001 AC5: an unlisted weekday with no NIFTY 50 daily candle after 16:00 becomes an inferred non-trading day (tooltip "Non-trading day (inferred)"). `calendar` stays pure: the candle check is a function (`DailyCandleCheck`) that `backfill.DailyCheck` (Task 14) supplies, and persistence goes through `store` rows that P0-3 copies to and from `calendar.Record`.

- [ ] **Step 1: Write the failing tests**

`internal/calendar/infer_test.go`:

```go
package calendar

import (
	"context"
	"testing"
)

func TestInferNonTradingDayWithoutNifty50Candle(t *testing.T) { // FR-DATA-001 AC5
	s := mustDefault(t)
	d := Date{2026, 10, 14} // Wednesday, absent from the list
	none := func(context.Context, Date) (bool, error) { return false, nil }

	if _, ok, _ := s.Infer(context.Background(), ist(2026, 10, 14, 15, 59), d, none); ok {
		t.Fatal("must not infer before 16:00")
	}
	e, ok, err := s.Infer(context.Background(), ist(2026, 10, 14, 16, 1), d, none)
	if err != nil || !ok || e.Type != DayInferredClosed || e.Source != "inferred" {
		t.Fatalf("infer = %+v %v %v", e, ok, err)
	}
	if s.IsTradingDay(NSE, d) || s.IsTradingDay(NFO, d) {
		t.Fatal("inferred day still trading")
	}
	l := s.LabelAt(NSE, ist(2026, 10, 14, 17, 0))
	if l.Tooltip != "Non-trading day (inferred)" {
		t.Fatalf("tooltip = %q", l.Tooltip)
	}
}

func TestInferLeavesListedAndCandleDaysAlone(t *testing.T) {
	s := mustDefault(t)
	called := false
	has := func(context.Context, Date) (bool, error) { called = true; return true, nil }
	if _, ok, _ := s.Infer(context.Background(), ist(2026, 10, 20, 17, 0), Date{2026, 10, 20}, has); ok || called {
		t.Fatal("listed holiday must not be checked")
	}
	if _, ok, _ := s.Infer(context.Background(), ist(2026, 10, 13, 17, 0), Date{2026, 10, 13}, has); ok {
		t.Fatal("a day with a NIFTY 50 candle is trading")
	}
	if !s.IsTradingDay(NSE, Date{2026, 10, 13}) {
		t.Fatal("13 Oct must stay trading")
	}
}
```

`internal/store/calendar_overrides_test.go`:

```go
package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestCalendarOverridesRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(ctx, filepath.Join(dir, "app.db"), Options{BackupDir: filepath.Join(dir, "backups"), Now: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows := []CalendarOverrideRow{
		{"NSE", 20261108, "special", `[{"kind":"continuous","start":"18:15","end":"19:15"}]`, "user"},
		{"NSE", 20261014, "inferred-non-trading", `[]`, "inferred"},
	}
	if err := db.UpsertCalendarOverrides(ctx, rows); err != nil {
		t.Fatal(err)
	}
	rows[0].Phases = `[{"kind":"continuous","start":"18:00","end":"19:00"}]`
	if err := db.UpsertCalendarOverrides(ctx, rows[:1]); err != nil {
		t.Fatal(err)
	}
	got, err := db.CalendarOverrides(ctx)
	if err != nil || len(got) != 2 || got[0].Date != 20261014 || got[1].Phases != rows[0].Phases {
		t.Fatalf("got %+v %v", got, err)
	}
	if err := db.DeleteCalendarOverride(ctx, "NSE", 20261014); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.CalendarOverrides(ctx); len(got) != 1 {
		t.Fatalf("after delete: %+v", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/calendar/ ./internal/store/`
Expected: the build fails — output contains `s.Infer undefined (type *Service has no field or method Infer)` and `[build failed]`.

- [ ] **Step 3: Write the implementation**

`internal/calendar/infer.go`:

```go
package calendar

import (
	"context"
	"time"
)

// DailyCandleCheck reports whether a NIFTY 50 daily candle exists for d (fetching it first if
// needed). candles/backfill provides it; calendar only sees the function (rule 1).
type DailyCandleCheck func(ctx context.Context, d Date) (bool, error)

// Infer applies the FR-DATA-001 inference rule to d at time now: a weekday absent from the list
// with no NIFTY 50 daily candle after 16:00 is an inferred non-trading day. It returns the entry
// to persist when the day was marked, and ok=false when nothing changed.
func (s *Service) Infer(ctx context.Context, now time.Time, d Date, has DailyCandleCheck) (Entry, bool, error) {
	if d.IsWeekend() {
		return Entry{}, false, nil
	}
	if _, listed := s.lookup(NSE, d); listed {
		return Entry{}, false, nil
	}
	if now.Before(d.At(inferenceAfter)) {
		return Entry{}, false, nil
	}
	ok, err := has(ctx, d)
	if err != nil || ok {
		return Entry{}, false, err
	}
	e := Entry{Date: d, Exchange: NSE, Type: DayInferredClosed, Source: "inferred"}
	s.MergeOverrides([]Entry{e})
	return e, true, nil
}
```

`internal/store/calendar_overrides.go`:

```go
package store

import (
	"context"
	"database/sql"
)

// CalendarOverrideRow is one row of calendar_overrides (§4.2). Fields mirror calendar.Record.
type CalendarOverrideRow struct {
	Exchange string
	Date     int // yyyymmdd
	Type     string
	Phases   string // JSON
	Source   string
}

// CalendarOverrides returns every override ordered by date, exchange.
func (db *DB) CalendarOverrides(ctx context.Context) ([]CalendarOverrideRow, error) {
	rows, err := db.R.QueryContext(ctx,
		`SELECT exchange, date, type, phases, source FROM calendar_overrides ORDER BY date, exchange`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CalendarOverrideRow
	for rows.Next() {
		var r CalendarOverrideRow
		if err := rows.Scan(&r.Exchange, &r.Date, &r.Type, &r.Phases, &r.Source); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpsertCalendarOverrides writes rows in one transaction (an import is all-or-nothing).
func (db *DB) UpsertCalendarOverrides(ctx context.Context, rs []CalendarOverrideRow) error {
	return db.writeTx(ctx, func(tx *sql.Tx) error {
		st, err := tx.PrepareContext(ctx, `INSERT INTO calendar_overrides(exchange, date, type, phases, source)
			VALUES(?,?,?,?,?) ON CONFLICT(exchange, date) DO UPDATE SET type=excluded.type, phases=excluded.phases, source=excluded.source`)
		if err != nil {
			return err
		}
		defer st.Close()
		for _, r := range rs {
			if _, err := st.ExecContext(ctx, r.Exchange, r.Date, r.Type, r.Phases, r.Source); err != nil {
				return err
			}
		}
		return nil
	})
}

// writeTx runs fn in one transaction on the writer connection; rollback on error.
func (db *DB) writeTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := db.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// DeleteCalendarOverride removes one override.
func (db *DB) DeleteCalendarOverride(ctx context.Context, exchange string, date int) error {
	_, err := db.W.ExecContext(ctx, `DELETE FROM calendar_overrides WHERE exchange=? AND date=?`, exchange, date)
	return err
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/calendar/ ./internal/store/`
Expected: `ok  	tradeloom/internal/calendar`, `ok  	tradeloom/internal/store`; no `DATA RACE`.

- [ ] **Step 5: Commit**

```bash
git add internal/calendar/infer.go \
  internal/store/calendar_overrides.go \
  internal/calendar/infer_test.go \
  internal/store/calendar_overrides_test.go
git commit -m "$(cat <<'EOF'
feat(calendar): inferred non-trading days and calendar_overrides repository

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
