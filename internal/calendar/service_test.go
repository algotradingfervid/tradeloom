package calendar

import (
	"strings"
	"testing"
	"time"

	"tradeloom/internal/clock"
)

func ist(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, clock.IST)
}

func mustDefault(t *testing.T) *Service {
	t.Helper()
	s, err := NewDefault()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestHolidayLabelAndNoSession(t *testing.T) { // FR-DATA-001 AC1 (calendar side)
	s := mustDefault(t)
	at := ist(2026, 10, 20, 11, 0) // Dussehra in the shipped list
	if got := s.LabelAt(NSE, at).Text; got != "Holiday" {
		t.Fatalf("label = %q, want Holiday", got)
	}
	if s.IsTradingDay(NSE, DateOf(at)) || s.IsTradingDay(NFO, DateOf(at)) {
		t.Fatal("holiday must not be a trading day for NSE or NFO")
	}
	if _, ok := s.Anchor(NSE, DateOf(at)); ok {
		t.Fatal("holiday must have no anchor")
	}
}

func TestRegularDayPhases(t *testing.T) {
	s := mustDefault(t)
	cases := []struct {
		ex   Exchange
		h, m int
		want string
	}{
		{NSE, 8, 59, "Closed"}, {NSE, 9, 0, "Pre-open"}, {NSE, 9, 15, "Open"}, {NSE, 15, 29, "Open"},
		{NSE, 15, 30, "Closing"}, {NSE, 15, 40, "Post-closing"}, {NSE, 16, 0, "Closed"},
		{NFO, 9, 5, "Closed"}, {NFO, 9, 15, "Open"}, {NFO, 15, 35, "Closed"},
	}
	for _, c := range cases {
		if got := s.LabelAt(c.ex, ist(2026, 10, 12, c.h, c.m)).Text; got != c.want {
			t.Errorf("%s %02d:%02d = %q, want %q", c.ex, c.h, c.m, got, c.want)
		}
	}
	if got := s.LabelAt(NSE, ist(2026, 10, 11, 11, 0)).Text; got != "Closed" {
		t.Errorf("Sunday = %q", got)
	}
}

func TestPostClosingIsCashOnly(t *testing.T) { // FR-DATA-001 AC3 (calendar side)
	s := mustDefault(t)
	at := ist(2026, 10, 12, 15, 45)
	if p, _ := s.PhaseAt(NSE, at); p.Kind != PhasePostClosing {
		t.Fatalf("NSE 15:45 = %s, want post-closing", p.Kind)
	}
	if p, _ := s.PhaseAt(NFO, at); p.Kind != PhaseClosed {
		t.Fatalf("NFO 15:45 = %s, want closed (post-closing is equity cash only)", p.Kind)
	}
}

func TestSpecialSessionAnchorsCandlesAndSquareOff(t *testing.T) { // FR-DATA-001 AC2 (calendar side)
	s := mustDefault(t)
	f, err := ParseFile(strings.NewReader(Header+"\n2026-11-08,NSE,special,continuous,18:15,19:15\n"), "import")
	if err != nil {
		t.Fatal(err)
	}
	s.MergeOverrides(f.Entries)
	d := Date{2026, 11, 8}
	start, end, ok := s.Session(NSE, d)
	if !ok || start.Format("15:04") != "18:15" || end.Format("15:04") != "19:15" {
		t.Fatalf("session = %v–%v ok=%v", start, end, ok)
	}
	// 5-minute candles: first starts 18:15, last is 19:10–19:15.
	var starts []string
	for c := start; c.Before(end); c = clock.NextBoundary(c, start, 5*time.Minute) {
		starts = append(starts, c.Format("15:04"))
	}
	if starts[0] != "18:15" || starts[len(starts)-1] != "19:10" || len(starts) != 12 {
		t.Fatalf("candle starts = %v", starts)
	}
	// MIS square-off candle contains end − N; with N = 10 min that is 19:05 → candle 19:05–19:10.
	so := end.Add(-10 * time.Minute)
	if got := clock.BucketStart(so, start, 5*time.Minute).Format("15:04"); got != "19:05" {
		t.Fatalf("square-off candle = %s, want 19:05", got)
	}
	if got := s.LabelAt(NSE, ist(2026, 11, 8, 18, 30)).Text; got != "Special session" {
		t.Fatalf("label = %q", got)
	}
	// NFO follows NSE's special session (continuous phase only).
	if a, ok := s.Anchor(NFO, d); !ok || a.Format("15:04") != "18:15" {
		t.Fatalf("NFO anchor = %v %v", a, ok)
	}
}

func TestStaleWhenNext30DaysNotCovered(t *testing.T) { // FR-DATA-001 AC6 (calendar side)
	s := mustDefault(t)
	if s.Stale(ist(2026, 11, 30, 10, 0)) {
		t.Fatal("30 Nov + 30 d = 30 Dec is covered")
	}
	if !s.Stale(ist(2026, 12, 5, 10, 0)) {
		t.Fatal("05 Dec + 30 d = 04 Jan is not covered; want stale")
	}
	f, _ := ParseFile(strings.NewReader(Header+"\n2027-01-26,NSE,holiday,,,\n"), "import")
	s.MergeOverrides(f.Entries)
	if s.Stale(ist(2026, 12, 5, 10, 0)) {
		t.Fatal("an imported 2027 list extends coverage")
	}
}

func TestVersionChangesWithOverrides(t *testing.T) {
	s := mustDefault(t)
	if s.Version() != "2026.3" {
		t.Fatalf("version = %q", s.Version())
	}
	s.MergeOverrides([]Entry{{Date: Date{2026, 12, 24}, Exchange: NSE, Type: DayHoliday, Source: "user"}})
	v := s.Version()
	if !strings.HasPrefix(v, "2026.3+u") || len(v) != len("2026.3+u")+8 {
		t.Fatalf("version = %q", v)
	}
	s.DeleteOverride(NSE, Date{2026, 12, 24})
	if s.Version() != "2026.3" {
		t.Fatalf("version after delete = %q", s.Version())
	}
}

func TestNextAndPrevTradingDaySkipHolidaysAndWeekends(t *testing.T) {
	s := mustDefault(t)
	if n, _ := s.NextTradingDay(NSE, Date{2026, 10, 16}); n != (Date{2026, 10, 19}) {
		t.Fatalf("next after Fri 16 Oct = %v", n)
	}
	if n, _ := s.NextTradingDay(NSE, Date{2026, 10, 19}); n != (Date{2026, 10, 21}) {
		t.Fatalf("next after Mon 19 Oct (20th holiday) = %v", n)
	}
	if p, _ := s.PrevTradingDay(NSE, Date{2026, 10, 21}); p != (Date{2026, 10, 19}) {
		t.Fatalf("prev before 21 Oct = %v", p)
	}
}

func TestRecordRoundTrip(t *testing.T) {
	e := Entry{Date: Date{2026, 11, 8}, Exchange: NSE, Type: DaySpecial, Source: "user",
		Phases: []PhaseSpec{{PhaseContinuous, 18*60 + 15, 19*60 + 15}}}
	r := ToRecord(e)
	if r.Date != 20261108 || r.Phases != `[{"kind":"continuous","start":"18:15","end":"19:15"}]` {
		t.Fatalf("record = %+v", r)
	}
	back, err := FromRecord(r)
	if err != nil || back.Phases[0] != e.Phases[0] || back.Date != e.Date {
		t.Fatalf("round trip = %+v %v", back, err)
	}
}
