package calendar

import (
	"bytes"
	"strings"
	"testing"

	calendardata "tradeloom/assets/calendar"
)

func TestParseShippedCalendar(t *testing.T) {
	f, err := ParseFile(bytes.NewReader(calendardata.Shipped), "shipped")
	if err != nil {
		t.Fatal(err)
	}
	if f.Version == "" || f.CoversFrom != (Date{2026, 1, 1}) || f.CoversTo != (Date{2026, 12, 31}) {
		t.Fatalf("header: version %q covers %v..%v", f.Version, f.CoversFrom, f.CoversTo)
	}
	byEx := map[Exchange][]string{}
	for _, e := range f.Entries {
		switch e.Type {
		case DayHoliday:
			if e.Date.IsWeekend() {
				t.Errorf("holiday on a weekend: %v", e.Date)
			}
		case DaySpecial:
			if _, ok := specContinuous(e.Phases); !ok {
				t.Errorf("special session without continuous phase: %v", e.Date)
			}
		default:
			t.Errorf("unexpected shipped entry %+v", e)
		}
		byEx[e.Exchange] = append(byEx[e.Exchange], e.Date.String()+string(e.Type))
	}
	if strings.Join(byEx[NSE], ",") != strings.Join(byEx[BSE], ",") {
		t.Fatalf("NSE %v, BSE %v; the lists must match", byEx[NSE], byEx[BSE])
	}
	want := []string{"2026-01-15", "2026-01-26", "2026-03-03", "2026-03-26", "2026-03-31", "2026-04-03",
		"2026-04-14", "2026-05-01", "2026-05-28", "2026-06-26", "2026-09-14", "2026-10-02",
		"2026-10-20", "2026-11-10", "2026-11-24", "2026-12-25"}
	var gotHol []string
	for _, e := range f.Entries {
		if e.Exchange == NSE && e.Type == DayHoliday {
			gotHol = append(gotHol, e.Date.String())
		}
	}
	if strings.Join(gotHol, ",") != strings.Join(want, ",") {
		t.Fatalf("NSE holidays = %v, want %v", gotHol, want)
	}
	for _, ex := range []Exchange{NSE, BSE} {
		var found bool
		for _, e := range f.Entries {
			if e.Exchange == ex && e.Date == (Date{2026, 2, 1}) && e.Type == DaySpecial {
				c, ok := specContinuous(e.Phases)
				found = ok && c.Start == 9*60+15 && c.End == 15*60+30 && len(e.Phases) == 2
			}
		}
		if !found {
			t.Errorf("%s: 2026-02-01 Budget-day special session missing or wrong", ex)
		}
	}
}

func TestImportRefusesInvalidTimeAndNamesRow(t *testing.T) { // FR-DATA-001 AC4
	csv := Header + "\n" +
		"2026-11-20,NSE,holiday,,,\n" +
		"2026-11-21,NSE,holiday,,,\n" +
		"# a comment line is not a row\n" +
		"2026-11-23,NSE,holiday,,,\n" +
		"2026-11-25,NSE,holiday,,,\n" +
		"2026-11-26,NSE,holiday,,,\n" +
		"2026-11-27,NSE,holiday,,,\n" +
		"2026-11-08,NSE,special,continuous,18:15,25:00\n"
	_, err := ParseFile(strings.NewReader(csv), "import")
	if err == nil || err.Error() != "Row 7: invalid time" {
		t.Fatalf("err = %v, want Row 7: invalid time", err)
	}
}

func TestParseRejections(t *testing.T) {
	cases := map[string]string{
		"2026-13-01,NSE,holiday,,,":                                                                 "Row 1: invalid date",
		"2026-11-20,MCX,holiday,,,":                                                                 "Row 1: invalid exchange",
		"2026-11-20,NSE,closed,,,":                                                                  "Row 1: invalid type",
		"2026-11-20,NSE,holiday,continuous,09:15,15:30":                                             "Row 1: holiday rows take no phase or times",
		"2026-11-08,NSE,special,lunch,18:15,19:15":                                                  "Row 1: invalid phase",
		"2026-11-08,NSE,special,continuous,19:15,18:15":                                             "Row 1: start must be before end",
		"2026-11-08,NSE,special,pre-open,18:00,18:15":                                               "Row 1: special session needs a continuous phase",
		"2026-11-09,NSE,trading,pre-open,09:00,09:08":                                               "Row 1: trading day with phases needs a continuous phase", // Ruling 77 (5)
		"2026-11-08,NSE,special,continuous,18:15,19:15\n2026-11-08,NSE,special,closing,19:00,19:30": "Row 2: overlaps another phase",
		"2026-11-08,NSE,special,continuous,18:15,19:15\n2026-11-08,NSE,holiday,,,":                  "Row 2: conflicts with row 1",
	}
	for body, want := range cases {
		_, err := ParseFile(strings.NewReader(Header+"\n"+body+"\n"), "import")
		if err == nil || err.Error() != want {
			t.Errorf("%q: err = %v, want %q", body, err, want)
		}
	}
	if _, err := ParseFile(strings.NewReader("date,exch,type,phase,start,end\n"), "import"); err == nil {
		t.Error("wrong header accepted")
	}
}

func TestParseSpecialSessionGroupsPhases(t *testing.T) {
	csv := Header + "\n" +
		"2026-11-08,NSE,special,continuous,18:15,19:15\n" +
		"2026-11-08,NSE,special,pre-open,18:00,18:08\n"
	f, err := ParseFile(strings.NewReader(csv), "import")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Entries) != 1 || len(f.Entries[0].Phases) != 2 || f.Entries[0].Phases[0].Kind != PhasePreOpen {
		t.Fatalf("entries = %+v", f.Entries)
	}
}
