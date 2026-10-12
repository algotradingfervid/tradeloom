> **P0-3 Session & Service Layer Implementation Plan — Task 31.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 31: Calendar façade — info, CSV import, upsert and remove of overrides (FR-DATA-001 UI backend)

**Files:**
- Create: `internal/service/calendar.go`
- Test: `internal/service/calendar_test.go`

**Interfaces:**
- Consumes: P0-2 `calendar.ParseFile`, `*calendar.RowError`, `calendar.Entry`, `calendar.PhaseSpec`, `calendar.ToRecord`, `calendar.Exchanges`, `calendar.ParseDate`, `calendar.ParseHM`, `calendar.DayTrading`/`DayHoliday`/`DaySpecial`, `(*calendar.Service).MergeOverrides`, `DeleteOverride`, `Overrides`, `Version`, `CoversUntil`, `Stale`, `IsTradingDay`; P0-1 `(*DB).UpsertCalendarOverrides`, `(*DB).DeleteCalendarOverride`, `(*DB).CalendarOverrides`, `store.CalendarOverrideRow`; Task 30 `Deps.DB`, Task 16 `Deps.Calendar`.
- Produces:
  ```go
  type PhaseDTO struct{ Kind, Start, End string }                                                 // TS service.PhaseDTO (HH:MM)
  type CalendarEntryDTO struct{ Exchange, Date, Type string; Phases []PhaseDTO; Source string }  // TS service.CalendarEntryDTO
  type CalendarEntryInput struct{ Exchange, Date, Type string; Phases []PhaseDTO }               // TS service.CalendarEntryInput
  type CalendarInfo struct{ Version, CoversUntil string; Stale bool; Entries []CalendarEntryDTO } // TS service.CalendarInfo
  func (s *Service) CalendarInfo(ctx context.Context) (CalendarInfo, error)
  func (s *Service) ImportCalendar(ctx context.Context, path string) (CalendarInfo, error)                 // not_found, invalid_file (reason "Row N: msg")
  func (s *Service) UpsertCalendarEntry(ctx context.Context, in CalendarEntryInput) (CalendarInfo, error) // invalid_entry
  func (s *Service) RemoveCalendarEntry(ctx context.Context, exchange, date string) (CalendarInfo, error) // invalid_entry, not_found
  ```

The shipped calendar (P0-2) is read-only; user and imported entries are *overrides* stored in `calendar_overrides` and merged into the running `calendar.Service`. `Entries` lists the overrides only (the UI shows them in Settings → Data → Trading calendar); the shipped days are what `market.status` and chart ranges already use. An import is all-or-nothing: P0-2's `ParseFile` reports the first bad row as `Row N: msg`, which becomes the `invalid_file` reason verbatim (FR-DATA-001 AC2). Writes go to the database first and then into the in-memory service, so a failed write never leaves the two apart.

- [ ] **Step 1: Write the failing tests**

```go
// internal/service/calendar_test.go
package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"tradeloom/internal/calendar"
	"tradeloom/internal/store"
	"tradeloom/internal/store/storetest"
)

func withCalendarData(t *testing.T, e *svcEnv) *store.DB {
	t.Helper()
	db := storetest.Open(t)
	e.svc.d.DB = db
	e.svc.d.Calendar = mustCalendar(t)
	return db
}

func TestCalendarInfo_ShippedCalendar(t *testing.T) {
	e := newSvcEnv(t)
	withCalendarData(t, e)
	info, err := e.svc.CalendarInfo(context.Background())
	if err != nil || info.Version == "" || len(info.CoversUntil) != 10 || info.Stale || len(info.Entries) != 0 {
		t.Fatalf("%+v %v", info, err)
	}
}

// FR-DATA-001 AC1/AC2: a valid CSV is merged and persisted; a bad row refuses the whole file with "Row N: …".
func TestImportCalendar_AllOrNothing(t *testing.T) {
	e := newSvcEnv(t)
	db := withCalendarData(t, e)
	ctx := context.Background()
	dir := t.TempDir()
	good := filepath.Join(dir, "cal.csv")
	csv := "date,exchange,type,phase,start,end\n" +
		"2026-10-20,NSE,holiday,,,\n" +
		"2026-11-01,NSE,special,pre-open,17:45,18:00\n" +
		"2026-11-01,NSE,special,continuous,18:00,19:00\n"
	if err := os.WriteFile(good, []byte(csv), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.ImportCalendar(ctx, filepath.Join(dir, "missing.csv")); refusedCode(t, err) != "not_found" {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad.csv")
	if err := os.WriteFile(bad, []byte("date,exchange,type,phase,start,end\n2026-10-21,NSE,holiday,,,\n2026-13-40,NSE,holiday,,,\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := e.svc.ImportCalendar(ctx, bad)
	if refusedCode(t, err) != "invalid_file" || len(err.Error()) < 6 || err.Error()[:6] != "Row 2:" {
		t.Fatalf("%v", err)
	}
	if info, _ := e.svc.CalendarInfo(ctx); len(info.Entries) != 0 {
		t.Fatalf("a bad file must change nothing: %+v", info.Entries)
	}
	info, err := e.svc.ImportCalendar(ctx, good)
	if err != nil || len(info.Entries) != 2 {
		t.Fatalf("%+v %v", info, err)
	}
	if h := info.Entries[0]; h.Exchange != "NSE" || h.Date != "2026-10-20" || h.Type != "holiday" || h.Source != "import" || len(h.Phases) != 0 {
		t.Fatalf("%+v", h)
	}
	if sp := info.Entries[1]; sp.Date != "2026-11-01" || sp.Type != "special" || len(sp.Phases) != 2 || sp.Phases[1].Kind != "continuous" || sp.Phases[1].Start != "18:00" || sp.Phases[1].End != "19:00" {
		t.Fatalf("%+v", sp)
	}
	d, _ := calendar.ParseDate("2026-10-20")
	if e.svc.d.Calendar.IsTradingDay(calendar.NSE, d) {
		t.Fatal("imported holiday not applied to the running calendar")
	}
	rows, err := db.CalendarOverrides(ctx)
	if err != nil || len(rows) != 2 || rows[0].Source != "import" {
		t.Fatalf("persisted %+v %v", rows, err)
	}
}

func TestUpsertAndRemoveCalendarEntry(t *testing.T) {
	e := newSvcEnv(t)
	db := withCalendarData(t, e)
	ctx := context.Background()
	for _, bad := range []CalendarEntryInput{
		{Exchange: "LSE", Date: "2026-10-20", Type: "holiday"},
		{Exchange: "NSE", Date: "20/10/2026", Type: "holiday"},
		{Exchange: "NSE", Date: "2026-10-20", Type: "closed"},
		{Exchange: "NSE", Date: "2026-10-20", Type: "special", Phases: []PhaseDTO{{Kind: "continuous", Start: "9am", End: "10:00"}}},
		{Exchange: "NSE", Date: "2026-10-20", Type: "special", Phases: []PhaseDTO{{Kind: "", Start: "09:00", End: "10:00"}}},
	} {
		if _, err := e.svc.UpsertCalendarEntry(ctx, bad); refusedCode(t, err) != "invalid_entry" {
			t.Fatalf("%+v: %v", bad, err)
		}
	}
	info, err := e.svc.UpsertCalendarEntry(ctx, CalendarEntryInput{Exchange: "NSE", Date: "2026-10-20", Type: "holiday"})
	if err != nil || len(info.Entries) != 1 || info.Entries[0].Source != "user" {
		t.Fatalf("%+v %v", info, err)
	}
	info, err = e.svc.UpsertCalendarEntry(ctx, CalendarEntryInput{Exchange: "NSE", Date: "2026-10-20", Type: "special", Phases: []PhaseDTO{{Kind: "continuous", Start: "09:15", End: "12:00"}}})
	if err != nil || len(info.Entries) != 1 || info.Entries[0].Type != "special" || len(info.Entries[0].Phases) != 1 {
		t.Fatalf("upsert must replace: %+v %v", info, err)
	}
	if rows, _ := db.CalendarOverrides(ctx); len(rows) != 1 || rows[0].Type != "special" {
		t.Fatalf("persisted %+v", rows)
	}
	if _, err := e.svc.RemoveCalendarEntry(ctx, "NSE", "2026-10-21"); refusedCode(t, err) != "not_found" {
		t.Fatal(err)
	}
	if _, err := e.svc.RemoveCalendarEntry(ctx, "NSE", "yesterday"); refusedCode(t, err) != "invalid_entry" {
		t.Fatal(err)
	}
	info, err = e.svc.RemoveCalendarEntry(ctx, "NSE", "2026-10-20")
	if err != nil || len(info.Entries) != 0 {
		t.Fatalf("%+v %v", info, err)
	}
	if rows, _ := db.CalendarOverrides(ctx); len(rows) != 0 {
		t.Fatalf("persisted %+v", rows)
	}
	d, _ := calendar.ParseDate("2026-10-20")
	if !e.svc.d.Calendar.IsTradingDay(calendar.NSE, d) {
		t.Fatal("removed override still applied")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/service/ -run 'TestCalendarInfo|TestImportCalendar|TestUpsertAndRemoveCalendarEntry' -v`
Expected: FAIL to compile with `undefined: (*Service).CalendarInfo`, `undefined: CalendarEntryInput`.

- [ ] **Step 3: Write the implementation**

```go
// internal/service/calendar.go
package service

import (
	"context"
	"errors"
	"os"
	"sort"

	"tradeloom/internal/calendar"
	"tradeloom/internal/store"
)

const calendarStage = "calendar"

type PhaseDTO struct {
	Kind  string `json:"kind"`
	Start string `json:"start"`
	End   string `json:"end"`
}

type CalendarEntryDTO struct {
	Exchange string     `json:"exchange"`
	Date     string     `json:"date"`
	Type     string     `json:"type"`
	Phases   []PhaseDTO `json:"phases"`
	Source   string     `json:"source"`
}

type CalendarEntryInput struct {
	Exchange string     `json:"exchange"`
	Date     string     `json:"date"`
	Type     string     `json:"type"`
	Phases   []PhaseDTO `json:"phases"`
}

type CalendarInfo struct {
	Version     string             `json:"version"`
	CoversUntil string             `json:"coversUntil"`
	Stale       bool               `json:"stale"`
	Entries     []CalendarEntryDTO `json:"entries"`
}

func entryDTO(e calendar.Entry) CalendarEntryDTO {
	out := CalendarEntryDTO{Exchange: string(e.Exchange), Date: e.Date.String(), Type: string(e.Type), Phases: []PhaseDTO{}, Source: e.Source}
	for _, p := range e.Phases {
		out.Phases = append(out.Phases, PhaseDTO{Kind: string(p.Kind), Start: p.Start.String(), End: p.End.String()})
	}
	return out
}

func (s *Service) calendarInfo() CalendarInfo {
	cal := s.d.Calendar
	info := CalendarInfo{Entries: []CalendarEntryDTO{}}
	if cal == nil {
		return info
	}
	info.Version, info.CoversUntil, info.Stale = cal.Version(), cal.CoversUntil().String(), cal.Stale(s.d.Clock.Now())
	es := cal.Overrides()
	sort.Slice(es, func(i, j int) bool {
		if es[i].Date.Int() != es[j].Date.Int() {
			return es[i].Date.Int() < es[j].Date.Int()
		}
		return es[i].Exchange < es[j].Exchange
	})
	for _, e := range es {
		info.Entries = append(info.Entries, entryDTO(e))
	}
	return info
}

func (s *Service) CalendarInfo(_ context.Context) (CalendarInfo, error) {
	if s.d.Calendar == nil {
		return CalendarInfo{Entries: []CalendarEntryDTO{}}, refused(calendarStage, "unavailable", "The trading calendar is not loaded.")
	}
	return s.calendarInfo(), nil
}

func (s *Service) persistOverrides(ctx context.Context, es []calendar.Entry) error {
	if s.d.DB == nil {
		return refused(calendarStage, "unavailable", "The database is not open.")
	}
	rows := make([]store.CalendarOverrideRow, 0, len(es))
	for _, e := range es {
		r := calendar.ToRecord(e)
		rows = append(rows, store.CalendarOverrideRow{Exchange: r.Exchange, Date: r.Date, Type: r.Type, Phases: r.Phases, Source: r.Source})
	}
	return asRefused(calendarStage, s.d.DB.UpsertCalendarOverrides(ctx, rows))
}

// ImportCalendar loads a CSV (header date,exchange,type,phase,start,end) as "import" overrides.
func (s *Service) ImportCalendar(ctx context.Context, path string) (CalendarInfo, error) {
	if s.d.Calendar == nil {
		return s.calendarInfo(), refused(calendarStage, "unavailable", "The trading calendar is not loaded.")
	}
	f, err := os.Open(path)
	if err != nil {
		return s.calendarInfo(), refused(calendarStage, "not_found", "The calendar file was not found.")
	}
	defer f.Close()
	file, err := calendar.ParseFile(f, "import")
	if err != nil {
		var re *calendar.RowError
		if errors.As(err, &re) {
			return s.calendarInfo(), refused(calendarStage, "invalid_file", re.Error())
		}
		return s.calendarInfo(), refused(calendarStage, "invalid_file", err.Error())
	}
	if err := s.persistOverrides(ctx, file.Entries); err != nil {
		return s.calendarInfo(), err
	}
	s.d.Calendar.MergeOverrides(file.Entries)
	s.d.Log.Info("calendar imported", "entries", len(file.Entries), "path", path)
	return s.calendarInfo(), nil
}

func parseEntry(in CalendarEntryInput) (calendar.Entry, error) {
	var e calendar.Entry
	ex := calendar.Exchange(in.Exchange)
	known := false
	for _, x := range calendar.Exchanges {
		if x == ex {
			known = true
		}
	}
	if !known {
		return e, refused(calendarStage, "invalid_entry", "Unknown exchange "+in.Exchange+".")
	}
	d, err := calendar.ParseDate(in.Date)
	if err != nil {
		return e, refused(calendarStage, "invalid_entry", "Dates are YYYY-MM-DD.")
	}
	ty := calendar.DayType(in.Type)
	if ty != calendar.DayHoliday && ty != calendar.DaySpecial && ty != calendar.DayTrading {
		return e, refused(calendarStage, "invalid_entry", "Type must be holiday, special or trading.")
	}
	e = calendar.Entry{Date: d, Exchange: ex, Type: ty, Source: "user"}
	for _, p := range in.Phases {
		if p.Kind == "" {
			return e, refused(calendarStage, "invalid_entry", "Every phase needs a kind.")
		}
		start, err1 := calendar.ParseHM(p.Start)
		end, err2 := calendar.ParseHM(p.End)
		if err1 != nil || err2 != nil || end <= start {
			return e, refused(calendarStage, "invalid_entry", "Phase times are HH:MM and end after start.")
		}
		e.Phases = append(e.Phases, calendar.PhaseSpec{Kind: calendar.PhaseKind(p.Kind), Start: start, End: end})
	}
	return e, nil
}

// UpsertCalendarEntry adds or replaces one user override.
func (s *Service) UpsertCalendarEntry(ctx context.Context, in CalendarEntryInput) (CalendarInfo, error) {
	if s.d.Calendar == nil {
		return s.calendarInfo(), refused(calendarStage, "unavailable", "The trading calendar is not loaded.")
	}
	e, err := parseEntry(in)
	if err != nil {
		return s.calendarInfo(), err
	}
	if err := s.persistOverrides(ctx, []calendar.Entry{e}); err != nil {
		return s.calendarInfo(), err
	}
	s.d.Calendar.MergeOverrides([]calendar.Entry{e})
	return s.calendarInfo(), nil
}

// RemoveCalendarEntry deletes one override (user or imported).
func (s *Service) RemoveCalendarEntry(ctx context.Context, exchange, date string) (CalendarInfo, error) {
	if s.d.Calendar == nil {
		return s.calendarInfo(), refused(calendarStage, "unavailable", "The trading calendar is not loaded.")
	}
	ex := calendar.Exchange(exchange)
	d, err := calendar.ParseDate(date)
	if err != nil {
		return s.calendarInfo(), refused(calendarStage, "invalid_entry", "Dates are YYYY-MM-DD.")
	}
	found := false
	for _, e := range s.d.Calendar.Overrides() {
		if e.Exchange == ex && e.Date == d {
			found = true
		}
	}
	if !found {
		return s.calendarInfo(), refused(calendarStage, "not_found", "No override for "+exchange+" on "+date+".")
	}
	if s.d.DB == nil {
		return s.calendarInfo(), refused(calendarStage, "unavailable", "The database is not open.")
	}
	if err := s.d.DB.DeleteCalendarOverride(ctx, exchange, d.Int()); err != nil {
		return s.calendarInfo(), asRefused(calendarStage, err)
	}
	s.d.Calendar.DeleteOverride(ex, d)
	return s.calendarInfo(), nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/service/ -run 'TestCalendarInfo|TestImportCalendar|TestUpsertAndRemoveCalendarEntry' -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/service/calendar.go internal/service/calendar_test.go
git commit -m "feat(service): calendar info, CSV import and override upsert/remove

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
