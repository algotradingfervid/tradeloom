> **P0-2 Time & Market Data Implementation Plan — Task 3.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 3: Calendar model, CSV parser and shipped 2026 calendar

**Files:**
- Create: `internal/calendar/date.go`
- Create: `internal/calendar/model.go`
- Create: `internal/calendar/regular.go`
- Create: `internal/calendar/csv.go`
- Create: `assets/calendar/calendar.go`
- Create: `assets/calendar/nse-bse-2026.csv`
- Test: `internal/calendar/csv_test.go`

**Interfaces:**
- Consumes: Task 1: `clock.IST`.
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
type Date struct {
	Year  int
	Month time.Month
	Day   int
}
func DateOf(t time.Time) Date
func ParseDate(s string) (Date, error)
func DateFromInt(n int) Date
func (d Date) Int() int
func (d Date) String() string
func (d Date) Display() string
func (d Date) Midnight() time.Time
func (d Date) AddDays(n int) Date
func (d Date) Weekday() time.Weekday
func (d Date) Before(o Date) bool
func (d Date) After(o Date) bool
func (d Date) IsWeekend() bool
func (d Date) At(hm HM) time.Time
type HM int
func ParseHM(s string) (HM, error)
func (h HM) String() string
func (h HM) MarshalText() ([]byte, error)
func (h *HM) UnmarshalText(b []byte) error
type Exchange string
const (
	NSE Exchange = "NSE"
	BSE Exchange = "BSE"
	NFO Exchange = "NFO"
	BFO Exchange = "BFO"
)
var Exchanges = []Exchange{NSE, BSE, NFO, BFO}
func IsCash(e Exchange) bool
type PhaseKind string
const (
	PhasePreOpen     PhaseKind = "pre-open"
	PhaseContinuous  PhaseKind = "continuous"
	PhaseClosing     PhaseKind = "closing"
	PhasePostClosing PhaseKind = "post-closing"
	PhaseClosed      PhaseKind = "closed"
)
type DayType string
const (
	DayTrading        DayType = "trading"
	DayHoliday        DayType = "holiday"
	DaySpecial        DayType = "special"
	DayWeekend        DayType = "weekend"
	DayInferredClosed DayType = "inferred-non-trading"
)
type PhaseSpec struct {
	Kind  PhaseKind `json:"kind"`
	Start HM        `json:"start"`
	End   HM        `json:"end"`
}
type Entry struct {
	Date     Date
	Exchange Exchange
	Type     DayType
	Phases   []PhaseSpec
	Source   string // "shipped", "import", "user", "inferred"
}
type Phase struct {
	Kind  PhaseKind `json:"kind"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}
type Day struct {
	Exchange Exchange `json:"exchange"`
	Date     Date     `json:"-"`
	Type     DayType  `json:"type"`
	Phases   []Phase  `json:"phases"`
}
func (d Day) IsTrading() bool
func (d Day) Phase(k PhaseKind) (Phase, bool)
const Header = "date,exchange,type,phase,start,end"
type File struct {
	Version    string // from a "# version: X" comment; "" if absent
	CoversFrom Date   // from "# covers: FROM..TO"; zero if absent
	CoversTo   Date
	Entries    []Entry // one per (exchange, date), sorted by date then exchange
}
type RowError struct {
	Row int
	Msg string
}
func (e *RowError) Error() string
func ParseFile(r io.Reader, source string) (File, error)
var Shipped []byte
const ShippedName = "nse-bse-2026.csv"
```

FR-DATA-001 data file and import format (`date, exchange, type ∈ {holiday, special, trading}, phase, start, end`). Parsing is all-or-nothing so a bad import leaves the calendar unchanged (AC4). The shipped file carries `# version:` and `# covers:` comments.

- [ ] **Step 1: Write the failing tests**

`internal/calendar/csv_test.go`:

```go
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
	if len(byEx[NSE]) < 15 || strings.Join(byEx[NSE], ",") != strings.Join(byEx[BSE], ",") {
		t.Fatalf("NSE %d entries, BSE %d entries; the lists must match", len(byEx[NSE]), len(byEx[BSE]))
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
		"2026-13-01,NSE,holiday,,,":                     "Row 1: invalid date",
		"2026-11-20,MCX,holiday,,,":                     "Row 1: invalid exchange",
		"2026-11-20,NSE,closed,,,":                      "Row 1: invalid type",
		"2026-11-20,NSE,holiday,continuous,09:15,15:30": "Row 1: holiday rows take no phase or times",
		"2026-11-08,NSE,special,lunch,18:15,19:15":      "Row 1: invalid phase",
		"2026-11-08,NSE,special,continuous,19:15,18:15": "Row 1: start must be before end",
		"2026-11-08,NSE,special,pre-open,18:00,18:15":   "Row 1: special session needs a continuous phase",
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/calendar/`
Expected: the build fails — output contains `package tradeloom/assets/calendar is not in std` and `[setup failed]`.

- [ ] **Step 3: Write the implementation**

`internal/calendar/date.go`:

```go
// Package calendar is the market calendar and session model (FR-DATA-001): trading days,
// holidays, special sessions and phases per exchange, from a shipped CSV plus user overrides.
// It is pure: no I/O packages (ARCHITECTURE §2.4 rule 1); persistence is done by callers.
package calendar

import (
	"fmt"
	"time"

	"tradeloom/internal/clock"
)

// Date is an IST calendar date.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// DateOf returns the IST date containing t.
func DateOf(t time.Time) Date {
	y, m, d := t.In(clock.IST).Date()
	return Date{y, m, d}
}

// ParseDate parses YYYY-MM-DD.
func ParseDate(s string) (Date, error) {
	t, err := time.ParseInLocation("2006-01-02", s, clock.IST)
	if err != nil {
		return Date{}, err
	}
	return DateOf(t), nil
}

// DateFromInt converts yyyymmdd.
func DateFromInt(n int) Date { return Date{n / 10000, time.Month(n / 100 % 100), n % 100} }

func (d Date) Int() int              { return d.Year*10000 + int(d.Month)*100 + d.Day }
func (d Date) String() string        { return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day) }
func (d Date) Display() string       { return fmt.Sprintf("%02d-%02d-%04d", d.Day, int(d.Month), d.Year) }
func (d Date) Midnight() time.Time   { return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, clock.IST) }
func (d Date) AddDays(n int) Date    { return DateOf(d.Midnight().AddDate(0, 0, n)) }
func (d Date) Weekday() time.Weekday { return d.Midnight().Weekday() }
func (d Date) Before(o Date) bool    { return d.Int() < o.Int() }
func (d Date) After(o Date) bool     { return d.Int() > o.Int() }
func (d Date) IsWeekend() bool {
	w := d.Weekday()
	return w == time.Saturday || w == time.Sunday
}

// At returns the instant hm on date d in IST.
func (d Date) At(hm HM) time.Time {
	return time.Date(d.Year, d.Month, d.Day, int(hm)/60, int(hm)%60, 0, 0, clock.IST)
}

// HM is a time of day in minutes after midnight.
type HM int

// ParseHM parses HH:MM (00:00–23:59).
func ParseHM(s string) (HM, error) {
	var h, m int
	if len(s) != 5 || s[2] != ':' {
		return 0, fmt.Errorf("invalid time %q", s)
	}
	if _, err := fmt.Sscanf(s, "%02d:%02d", &h, &m); err != nil || h > 23 || m > 59 || h < 0 || m < 0 {
		return 0, fmt.Errorf("invalid time %q", s)
	}
	return HM(h*60 + m), nil
}

func (h HM) String() string { return fmt.Sprintf("%02d:%02d", int(h)/60, int(h)%60) }

func (h HM) MarshalText() ([]byte, error) { return []byte(h.String()), nil }

func (h *HM) UnmarshalText(b []byte) error {
	v, err := ParseHM(string(b))
	if err != nil {
		return err
	}
	*h = v
	return nil
}
```

`internal/calendar/model.go`:

```go
package calendar

import "time"

// Exchange is a Kite exchange code. INDICES rows use their Kite exchange (NSE or BSE).
type Exchange string

const (
	NSE Exchange = "NSE"
	BSE Exchange = "BSE"
	NFO Exchange = "NFO"
	BFO Exchange = "BFO"
)

// Exchanges lists the exchanges the calendar models.
var Exchanges = []Exchange{NSE, BSE, NFO, BFO}

func validExchange(e Exchange) bool {
	for _, x := range Exchanges {
		if x == e {
			return true
		}
	}
	return false
}

// parent maps a derivatives exchange to the cash exchange whose list it follows.
func parent(e Exchange) (Exchange, bool) {
	switch e {
	case NFO:
		return NSE, true
	case BFO:
		return BSE, true
	}
	return "", false
}

// IsCash reports whether e is a cash-equity exchange (pre-open and post-closing apply).
func IsCash(e Exchange) bool { return e == NSE || e == BSE }

// PhaseKind names a session phase.
type PhaseKind string

const (
	PhasePreOpen     PhaseKind = "pre-open"
	PhaseContinuous  PhaseKind = "continuous"
	PhaseClosing     PhaseKind = "closing"
	PhasePostClosing PhaseKind = "post-closing"
	PhaseClosed      PhaseKind = "closed"
)

func validPhase(k PhaseKind) bool {
	switch k {
	case PhasePreOpen, PhaseContinuous, PhaseClosing, PhasePostClosing:
		return true
	}
	return false
}

// DayType classifies a date for one exchange.
type DayType string

const (
	DayTrading        DayType = "trading"
	DayHoliday        DayType = "holiday"
	DaySpecial        DayType = "special"
	DayWeekend        DayType = "weekend"
	DayInferredClosed DayType = "inferred-non-trading"
)

// PhaseSpec is a phase as written in the calendar file: kind and HH:MM bounds [Start, End).
type PhaseSpec struct {
	Kind  PhaseKind `json:"kind"`
	Start HM        `json:"start"`
	End   HM        `json:"end"`
}

// Entry is one calendar list entry for (Exchange, Date): a holiday, a special session with its
// phases, a trading day (no phases = regular phases) or an inferred non-trading day.
type Entry struct {
	Date     Date
	Exchange Exchange
	Type     DayType
	Phases   []PhaseSpec
	Source   string // "shipped", "import", "user", "inferred"
}

// Phase is a concrete phase on a date: [Start, End) in IST.
type Phase struct {
	Kind  PhaseKind `json:"kind"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Day is the resolved calendar for (Exchange, Date).
type Day struct {
	Exchange Exchange `json:"exchange"`
	Date     Date     `json:"-"`
	Type     DayType  `json:"type"`
	Phases   []Phase  `json:"phases"`
}

// IsTrading reports whether the exchange trades on this day.
func (d Day) IsTrading() bool { return d.Type == DayTrading || d.Type == DaySpecial }

// Phase returns the phase of kind k, if the day has one.
func (d Day) Phase(k PhaseKind) (Phase, bool) {
	for _, p := range d.Phases {
		if p.Kind == k {
			return p, true
		}
	}
	return Phase{}, false
}
```

`internal/calendar/regular.go`:

```go
package calendar

// Regular-day phases (FR-DATA-001). These are the only session-time constants in the code base;
// every other package asks the Service (ARCHITECTURE §10.5 "literal session times" check).
var (
	cashRegular = []PhaseSpec{
		{PhasePreOpen, 9*60 + 0, 9*60 + 15},
		{PhaseContinuous, 9*60 + 15, 15*60 + 30},
		{PhaseClosing, 15*60 + 30, 15*60 + 40},
		{PhasePostClosing, 15*60 + 40, 16 * 60},
	}
	derivRegular = []PhaseSpec{
		{PhaseContinuous, 9*60 + 15, 15*60 + 30},
	}
	// instrumentsReady is when Kite publishes the day's instrument dump (A07 §A5: "around 08:30").
	instrumentsReady HM = 8*60 + 30
	// inferenceAfter is the instant after which a missing NIFTY 50 daily candle marks an unlisted
	// weekday as non-trading: end of the regular post-closing phase.
	inferenceAfter HM = 16 * 60
)

func regularFor(e Exchange) []PhaseSpec {
	if IsCash(e) {
		return cashRegular
	}
	return derivRegular
}
```

`internal/calendar/csv.go`:

```go
package calendar

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Header is the required first record of a calendar CSV (FR-DATA-001).
const Header = "date,exchange,type,phase,start,end"

// File is a parsed calendar CSV.
type File struct {
	Version    string // from a "# version: X" comment; "" if absent
	CoversFrom Date   // from "# covers: FROM..TO"; zero if absent
	CoversTo   Date
	Entries    []Entry // one per (exchange, date), sorted by date then exchange
}

// RowError reports the 1-based data row (header and comment lines not counted).
type RowError struct {
	Row int
	Msg string
}

func (e *RowError) Error() string { return fmt.Sprintf("Row %d: %s", e.Row, e.Msg) }

// ParseFile parses a calendar CSV. It either returns every entry or an error; it never returns
// a partial list (FR-DATA-001 AC4). source is stored on every entry.
func ParseFile(r io.Reader, source string) (File, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return File{}, err
	}
	var f File
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if v, ok := strings.CutPrefix(line, "# version:"); ok {
			f.Version = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "# covers:"); ok {
			from, to, found := strings.Cut(strings.TrimSpace(v), "..")
			if !found {
				return File{}, fmt.Errorf("invalid covers comment %q", line)
			}
			if f.CoversFrom, err = ParseDate(from); err != nil {
				return File{}, fmt.Errorf("invalid covers comment %q", line)
			}
			if f.CoversTo, err = ParseDate(to); err != nil {
				return File{}, fmt.Errorf("invalid covers comment %q", line)
			}
		}
	}
	cr := csv.NewReader(bytes.NewReader(raw))
	cr.Comment = '#'
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	head, err := cr.Read()
	if err != nil {
		return File{}, fmt.Errorf("missing header %q", Header)
	}
	if strings.Join(head, ",") != Header {
		return File{}, fmt.Errorf("header must be %q", Header)
	}
	type k struct {
		ex Exchange
		d  Date
	}
	byKey := map[k]*Entry{}
	firstRow := map[k]int{}
	row := 0
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		row++
		if err != nil {
			return File{}, &RowError{row, "unreadable row"}
		}
		if len(rec) != 6 {
			return File{}, &RowError{row, "expected 6 fields"}
		}
		d, err := ParseDate(rec[0])
		if err != nil {
			return File{}, &RowError{row, "invalid date"}
		}
		ex := Exchange(rec[1])
		if !validExchange(ex) {
			return File{}, &RowError{row, "invalid exchange"}
		}
		typ := DayType(rec[2])
		key := k{ex, d}
		e := byKey[key]
		switch typ {
		case DayHoliday:
			if rec[3] != "" || rec[4] != "" || rec[5] != "" {
				return File{}, &RowError{row, "holiday rows take no phase or times"}
			}
			if e != nil {
				return File{}, &RowError{row, fmt.Sprintf("conflicts with row %d", firstRow[key])}
			}
			byKey[key] = &Entry{Date: d, Exchange: ex, Type: DayHoliday, Source: source}
			firstRow[key] = row
		case DaySpecial, DayTrading:
			if e != nil && e.Type != typ {
				return File{}, &RowError{row, fmt.Sprintf("conflicts with row %d", firstRow[key])}
			}
			if e == nil {
				e = &Entry{Date: d, Exchange: ex, Type: typ, Source: source}
				byKey[key] = e
				firstRow[key] = row
			}
			if typ == DayTrading && rec[3] == "" && rec[4] == "" && rec[5] == "" {
				continue // trading day with the regular phases
			}
			ph := PhaseKind(rec[3])
			if !validPhase(ph) {
				return File{}, &RowError{row, "invalid phase"}
			}
			start, err1 := ParseHM(rec[4])
			end, err2 := ParseHM(rec[5])
			if err1 != nil || err2 != nil {
				return File{}, &RowError{row, "invalid time"}
			}
			if start >= end {
				return File{}, &RowError{row, "start must be before end"}
			}
			for _, p := range e.Phases {
				if start < p.End && p.Start < end {
					return File{}, &RowError{row, "overlaps another phase"}
				}
			}
			e.Phases = append(e.Phases, PhaseSpec{ph, start, end})
		default:
			return File{}, &RowError{row, "invalid type"}
		}
	}
	for _, e := range byKey {
		sort.Slice(e.Phases, func(i, j int) bool { return e.Phases[i].Start < e.Phases[j].Start })
		if e.Type == DaySpecial {
			if _, ok := specContinuous(e.Phases); !ok {
				return File{}, &RowError{firstRow[k{e.Exchange, e.Date}], "special session needs a continuous phase"}
			}
		}
		f.Entries = append(f.Entries, *e)
	}
	sortEntries(f.Entries)
	return f, nil
}

func specContinuous(ps []PhaseSpec) (PhaseSpec, bool) {
	for _, p := range ps {
		if p.Kind == PhaseContinuous {
			return p, true
		}
	}
	return PhaseSpec{}, false
}

func sortEntries(es []Entry) {
	sort.Slice(es, func(i, j int) bool {
		if es[i].Date != es[j].Date {
			return es[i].Date.Before(es[j].Date)
		}
		return es[i].Exchange < es[j].Exchange
	})
}
```

`assets/calendar/calendar.go`:

```go
// Package calendardata embeds the shipped market calendar (FR-DATA-001).
package calendardata

import _ "embed"

// Shipped is the current year's NSE/BSE trading-holiday list.
//
//go:embed nse-bse-2026.csv
var Shipped []byte

// ShippedName is the file name recorded with the calendar version.
const ShippedName = "nse-bse-2026.csv"
```

`assets/calendar/nse-bse-2026.csv`:

```csv
# Tradeloom shipped market calendar
# version: 2026.1
# covers: 2026-01-01..2026-12-31
# Weekday NSE/BSE equity trading holidays for 2026. Transcribed 2026-10-12 from published
# copies of the exchanges' 2026 holiday lists; checked against the NSE circular before release
# (P0-2 Task 3 Step 5). Derivatives (NFO/BFO) follow their cash exchange.
# Muhurat trading 2026 (Sunday 08-11-2026, Diwali Laxmi Pujan) is not listed: the exchanges had
# not announced its timings. Add it in Settings -> Market calendar or import an updated file.
date,exchange,type,phase,start,end
2026-01-15,NSE,holiday,,,
2026-01-15,BSE,holiday,,,
2026-01-26,NSE,holiday,,,
2026-01-26,BSE,holiday,,,
2026-03-03,NSE,holiday,,,
2026-03-03,BSE,holiday,,,
2026-03-26,NSE,holiday,,,
2026-03-26,BSE,holiday,,,
2026-03-31,NSE,holiday,,,
2026-03-31,BSE,holiday,,,
2026-04-03,NSE,holiday,,,
2026-04-03,BSE,holiday,,,
2026-04-14,NSE,holiday,,,
2026-04-14,BSE,holiday,,,
2026-05-01,NSE,holiday,,,
2026-05-01,BSE,holiday,,,
2026-05-28,NSE,holiday,,,
2026-05-28,BSE,holiday,,,
2026-06-26,NSE,holiday,,,
2026-06-26,BSE,holiday,,,
2026-09-14,NSE,holiday,,,
2026-09-14,BSE,holiday,,,
2026-10-02,NSE,holiday,,,
2026-10-02,BSE,holiday,,,
2026-10-20,NSE,holiday,,,
2026-10-20,BSE,holiday,,,
2026-11-10,NSE,holiday,,,
2026-11-10,BSE,holiday,,,
2026-11-24,NSE,holiday,,,
2026-11-24,BSE,holiday,,,
2026-12-25,NSE,holiday,,,
2026-12-25,BSE,holiday,,,
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/calendar/`
Expected: `ok  	tradeloom/internal/calendar`; no `DATA RACE`.

- [ ] **Step 5: Check the shipped dates against the exchange circular**

The 2026 dates were transcribed from published copies of the NSE/BSE holiday lists (the NSE site was unreachable on 2026-10-12). Before committing:

1. Open `https://www.nseindia.com/resources/exchange-communication-holidays`, tab "Trading Holidays", segment "Equities", year 2026 (and the BSE equivalent `https://www.bseindia.com/static/markets/marketinfo/listholi.aspx`).
2. Compare every weekday holiday with the rows of `assets/calendar/nse-bse-2026.csv`. For each difference, add/remove the `NSE` and `BSE` rows (`YYYY-MM-DD,NSE,holiday,,,`).
3. If the exchanges have announced the Diwali Muhurat session timings for Sunday 08-11-2026, add its phases for both exchanges, for example (use the announced times):
   `2026-11-08,NSE,special,pre-open,<start>,<end>` and `2026-11-08,NSE,special,continuous,<start>,<end>` (same for `BSE`).
4. If anything changed, set `# version: 2026.2`, then rerun:

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/calendar/`
Expected: `ok  	tradeloom/internal/calendar`

- [ ] **Step 6: Commit**

```bash
git add internal/calendar/date.go \
  internal/calendar/model.go \
  internal/calendar/regular.go \
  internal/calendar/csv.go \
  assets/calendar/calendar.go \
  assets/calendar/nse-bse-2026.csv \
  internal/calendar/csv_test.go
git commit -m "$(cat <<'EOF'
feat(calendar): calendar model, CSV import parser and shipped 2026 NSE/BSE list

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
