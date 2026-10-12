// internal/service/calendar_test.go
package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/calendar"
	"tradeloom/internal/settings"
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
		"2026-10-19,NSE,holiday,,,\n" +
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
	if h := info.Entries[0]; h.Exchange != "NSE" || h.Date != "2026-10-19" || h.Type != "holiday" || h.Source != "import" || len(h.Phases) != 0 {
		t.Fatalf("%+v", h)
	}
	if sp := info.Entries[1]; sp.Date != "2026-11-01" || sp.Type != "special" || len(sp.Phases) != 2 || sp.Phases[1].Kind != "continuous" || sp.Phases[1].Start != "18:00" || sp.Phases[1].End != "19:00" {
		t.Fatalf("%+v", sp)
	}
	d, _ := calendar.ParseDate("2026-10-19")
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
	info, err := e.svc.UpsertCalendarEntry(ctx, CalendarEntryInput{Exchange: "NSE", Date: "2026-10-19", Type: "holiday"})
	if err != nil || len(info.Entries) != 1 || info.Entries[0].Source != "user" {
		t.Fatalf("%+v %v", info, err)
	}
	info, err = e.svc.UpsertCalendarEntry(ctx, CalendarEntryInput{Exchange: "NSE", Date: "2026-10-19", Type: "special", Phases: []PhaseDTO{{Kind: "continuous", Start: "09:15", End: "12:00"}}})
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
	info, err = e.svc.RemoveCalendarEntry(ctx, "NSE", "2026-10-19")
	if err != nil || len(info.Entries) != 0 {
		t.Fatalf("%+v %v", info, err)
	}
	if rows, _ := db.CalendarOverrides(ctx); len(rows) != 0 {
		t.Fatalf("persisted %+v", rows)
	}
	d, _ := calendar.ParseDate("2026-10-19")
	if !e.svc.d.Calendar.IsTradingDay(calendar.NSE, d) {
		t.Fatal("removed override still applied")
	}
}

// --- Hardening beyond the plan's tests (untrusted CSV, Ruling 28, market.status on change) ---

func writeCal(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func assertNoOverrides(t *testing.T, e *svcEnv, db *store.DB) {
	t.Helper()
	if info, _ := e.svc.CalendarInfo(context.Background()); len(info.Entries) != 0 {
		t.Fatalf("memory changed: %+v", info.Entries)
	}
	if rows, err := db.CalendarOverrides(context.Background()); err != nil || len(rows) != 0 {
		t.Fatalf("db changed: %+v %v", rows, err)
	}
}

func TestImportCalendar_RejectsUntrustedInput(t *testing.T) {
	e := newSvcEnv(t)
	db := withCalendarData(t, e)
	ctx := context.Background()
	big := calendar.Header + "\n" + strings.Repeat("# padding padding padding padding padding padding padding\n", (maxCalendarFileBytes/56)+10)
	var many strings.Builder
	many.WriteString(calendar.Header + "\n")
	d, _ := calendar.ParseDate("2026-01-01")
	for i := 0; i <= maxCalendarRows; i++ {
		many.WriteString(d.AddDays(i%300).String() + ",NSE,trading,,,\n")
	}
	cases := map[string]struct{ path, want string }{
		"directory":       {t.TempDir(), "not a regular file"},
		"too large":       {writeCal(t, "big.csv", big), "larger than"},
		"too many rows":   {writeCal(t, "many.csv", many.String()), "more than"},
		"bad header":      {writeCal(t, "h.csv", "day,exchange\n2026-10-20,NSE\n"), "header"},
		"bad phase":       {writeCal(t, "p.csv", calendar.Header+"\n2026-11-01,NSE,special,lunch,12:00,13:00\n"), "Row 1: invalid phase"},
		"bad type":        {writeCal(t, "t.csv", calendar.Header+"\n2026-11-01,NSE,closed,,,\n"), "Row 1: invalid type"},
		"bad exchange":    {writeCal(t, "x.csv", calendar.Header+"\n2026-11-01,MCX,holiday,,,\n"), "Row 1: invalid exchange"},
		"too far ahead":   {writeCal(t, "f.csv", calendar.Header+"\n2026-10-20,NSE,holiday,,,\n2031-01-02,NSE,holiday,,,\n"), "2031-01-02"},
		"too far back":    {writeCal(t, "b.csv", calendar.Header+"\n1999-12-31,NSE,holiday,,,\n"), "1999-12-31"},
		"outside covers":  {writeCal(t, "c.csv", "# covers: 2026-01-01..2026-12-31\n"+calendar.Header+"\n2027-01-26,NSE,holiday,,,\n"), "outside the file's covers range"},
		"covers too far":  {writeCal(t, "cf.csv", "# covers: 2026-01-01..2099-12-31\n"+calendar.Header+"\n2026-10-20,NSE,holiday,,,\n"), "2099-12-31"},
		"covers reversed": {writeCal(t, "cr.csv", "# covers: 2026-12-31..2026-01-01\n"+calendar.Header+"\n"), "covers range"},
	}
	before := e.svc.d.Calendar.CoversUntil()
	for name, c := range cases {
		_, err := e.svc.ImportCalendar(ctx, c.path)
		if refusedCode(t, err) != "invalid_file" || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v (want invalid_file containing %q)", name, err, c.want)
		}
	}
	assertNoOverrides(t, e, db)
	if e.svc.d.Calendar.CoversUntil() != before || settings.Get(e.svc.d.Settings, calendarCoversUntil) != "" {
		t.Fatal("a refused import must not extend coverage")
	}
}

// A database failure refuses the import and leaves both the running calendar and the coverage untouched.
func TestImportCalendar_DBFailureAppliesNothing(t *testing.T) {
	e := newSvcEnv(t)
	db := withCalendarData(t, e)
	path := writeCal(t, "ok.csv", "# covers: 2026-01-01..2027-12-31\n"+calendar.Header+"\n2026-10-20,NSE,holiday,,,\n")
	before := e.svc.d.Calendar.CoversUntil()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := e.svc.ImportCalendar(context.Background(), path)
	if refusedCode(t, err) != "internal" {
		t.Fatal(err)
	}
	if len(e.svc.d.Calendar.Overrides()) != 0 || e.svc.d.Calendar.CoversUntil() != before {
		t.Fatal("memory changed after a failed write")
	}
	if settings.Get(e.svc.d.Settings, calendarCoversUntil) != "" {
		t.Fatal("coverage setting left behind after a failed write")
	}
}

// Ruling 28: an import with "# covers:" extends coverage, persists it, and LoadCalendar re-applies it at start-up.
func TestImportCalendar_CoversExtendsAndSurvivesRestart(t *testing.T) {
	e := newSvcEnv(t)
	withCalendarData(t, e)
	ctx := context.Background()
	e.clock.Set(ist(2026, 12, 15, 10, 0))
	if info, _ := e.svc.CalendarInfo(ctx); !info.Stale {
		t.Fatalf("shipped calendar should be stale in mid-December: %+v", info)
	}
	path := writeCal(t, "next.csv", "# version: 2027.1\n# covers: 2027-01-01..2027-12-31\n"+calendar.Header+"\n2027-01-26,NSE,holiday,,,\n")
	info, err := e.svc.ImportCalendar(ctx, path)
	if err != nil || info.CoversUntil != "2027-12-31" || info.Stale || len(info.Entries) != 1 {
		t.Fatalf("%+v %v", info, err)
	}
	if got := settings.Get(e.svc.d.Settings, calendarCoversUntil); got != "2027-12-31" {
		t.Fatalf("coverage not persisted: %q", got)
	}
	// Restart: a fresh calendar.Service, then LoadCalendar from app.db and settings.
	e.svc.d.Calendar = mustCalendar(t)
	if err := e.svc.LoadCalendar(ctx); err != nil {
		t.Fatal(err)
	}
	info, _ = e.svc.CalendarInfo(ctx)
	if info.CoversUntil != "2027-12-31" || info.Stale || len(info.Entries) != 1 || info.Entries[0].Date != "2027-01-26" {
		t.Fatalf("not restored: %+v", info)
	}
	// A user row never extends coverage (FR-DATA-001 AC6).
	if info, _ := e.svc.UpsertCalendarEntry(ctx, CalendarEntryInput{Exchange: "NSE", Date: "2028-01-26", Type: "holiday"}); info.CoversUntil != "2027-12-31" {
		t.Fatalf("user row extended coverage: %+v", info)
	}
}

func TestUpsertCalendarEntry_RejectsInvalidSessions(t *testing.T) {
	e := newSvcEnv(t)
	db := withCalendarData(t, e)
	ctx := context.Background()
	ph := func(k, s, en string) PhaseDTO { return PhaseDTO{Kind: k, Start: s, End: en} }
	for name, in := range map[string]CalendarEntryInput{
		"unknown kind":          {Exchange: "NSE", Date: "2026-11-01", Type: "special", Phases: []PhaseDTO{ph("lunch", "12:00", "13:00")}},
		"closed kind":           {Exchange: "NSE", Date: "2026-11-01", Type: "special", Phases: []PhaseDTO{ph("closed", "12:00", "13:00")}},
		"holiday with phases":   {Exchange: "NSE", Date: "2026-11-01", Type: "holiday", Phases: []PhaseDTO{ph("continuous", "09:15", "15:30")}},
		"special no continuous": {Exchange: "NSE", Date: "2026-11-01", Type: "special", Phases: []PhaseDTO{ph("pre-open", "09:00", "09:08")}},
		"special no phases":     {Exchange: "NSE", Date: "2026-11-01", Type: "special"},
		"overlap":               {Exchange: "NSE", Date: "2026-11-01", Type: "special", Phases: []PhaseDTO{ph("pre-open", "09:00", "09:20"), ph("continuous", "09:15", "15:30")}},
		"duplicate kind":        {Exchange: "NSE", Date: "2026-11-01", Type: "special", Phases: []PhaseDTO{ph("continuous", "09:15", "11:00"), ph("continuous", "12:00", "15:30")}},
		"end before start":      {Exchange: "NSE", Date: "2026-11-01", Type: "special", Phases: []PhaseDTO{ph("continuous", "15:30", "09:15")}},
		"inferred type":         {Exchange: "NSE", Date: "2026-11-01", Type: "inferred-non-trading"},
		"too far ahead":         {Exchange: "NSE", Date: "2031-01-02", Type: "holiday"},
		"too far back":          {Exchange: "NSE", Date: "1999-12-31", Type: "holiday"},
	} {
		if _, err := e.svc.UpsertCalendarEntry(ctx, in); refusedCode(t, err) != "invalid_entry" {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := e.svc.RemoveCalendarEntry(ctx, "LSE", "2026-10-20"); refusedCode(t, err) != "invalid_entry" {
		t.Error(err)
	}
	assertNoOverrides(t, e, db)
	// Unsorted phases are stored sorted.
	info, err := e.svc.UpsertCalendarEntry(ctx, CalendarEntryInput{Exchange: "NSE", Date: "2026-11-01", Type: "special",
		Phases: []PhaseDTO{ph("continuous", "18:00", "19:00"), ph("pre-open", "17:45", "17:53")}})
	if err != nil || info.Entries[0].Phases[0].Kind != "pre-open" {
		t.Fatalf("%+v %v", info, err)
	}
}

// A calendar change is visible at once: market.status is republished with today's new day type.
func TestCalendarChange_RepublishesMarketStatus(t *testing.T) {
	e := newSvcEnv(t)
	withCalendarData(t, e)
	ctx := context.Background()
	sub := bus.Subscribe(e.bus, TopicMarketStatus, bus.SubOpts[MarketStatus]{Name: "t31", Buffer: 8})
	defer bus.Unsubscribe(e.bus, sub)
	next := func() MarketStatus {
		t.Helper()
		select {
		case v, ok := <-sub.C():
			if !ok {
				t.Fatal("market.status subscription closed")
			}
			return v
		case <-time.After(3 * time.Second):
			t.Fatal("no market.status event")
		}
		return MarketStatus{}
	}
	// The fake clock is Monday 2026-10-12 10:00 IST: an open trading day.
	if _, err := e.svc.UpsertCalendarEntry(ctx, CalendarEntryInput{Exchange: "NSE", Date: "2026-10-12", Type: "holiday"}); err != nil {
		t.Fatal(err)
	}
	if st := next(); st.DayType != "holiday" || st.Label != "Holiday" {
		t.Fatalf("%+v", st)
	}
	if _, err := e.svc.RemoveCalendarEntry(ctx, "NSE", "2026-10-12"); err != nil {
		t.Fatal(err)
	}
	if st := next(); st.DayType != "trading" || st.Label != "Open" {
		t.Fatalf("%+v", st)
	}
	// A refused change publishes nothing.
	e.svc.UpsertCalendarEntry(ctx, CalendarEntryInput{Exchange: "NSE", Date: "bad", Type: "holiday"})
	select {
	case v, ok := <-sub.C():
		t.Fatalf("unexpected event %+v (open %v)", v, ok)
	case <-time.After(50 * time.Millisecond):
	}
}
