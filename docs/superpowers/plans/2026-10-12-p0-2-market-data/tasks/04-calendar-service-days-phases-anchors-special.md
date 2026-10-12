> **P0-2 Time & Market Data Implementation Plan — Task 4.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 4: Calendar service: days, phases, anchors, special sessions, staleness, labels

**Files:**
- Create: `internal/calendar/service.go`
- Create: `internal/calendar/record.go`
- Test: `internal/calendar/service_test.go`

**Interfaces:**
- Consumes: Task 1: `clock.IST`, `clock.NextBoundary`, `clock.BucketStart` (tests). Task 3: `Date`, `HM`, `ParseFile`, `Entry`, `PhaseSpec`, `Header`, the embedded `calendardata.Shipped`.
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
type Service struct{ /* unexported fields */ }
func New(shipped File) *Service
func NewDefault() (*Service, error)
func (s *Service) SetOverrides(es []Entry)
func (s *Service) MergeOverrides(es []Entry) []Entry
func (s *Service) DeleteOverride(ex Exchange, d Date)
func (s *Service) Overrides() []Entry
func (s *Service) Version() string
func (s *Service) Day(ex Exchange, d Date) Day
func (s *Service) IsTradingDay(ex Exchange, d Date) bool
func (s *Service) PhaseAt(ex Exchange, t time.Time) (Phase, Day)
func (s *Service) Session(ex Exchange, d Date) (start, end time.Time, ok bool)
func (s *Service) Anchor(ex Exchange, d Date) (time.Time, bool)
func (s *Service) NextTradingDay(ex Exchange, d Date) (Date, bool)
func (s *Service) PrevTradingDay(ex Exchange, d Date) (Date, bool)
func (s *Service) CoversUntil() Date
func (s *Service) Stale(now time.Time) bool
func (s *Service) InstrumentsReadyAt(d Date) time.Time
type Label struct {
	Text    string `json:"text"`    // Pre-open, Open, Closing, Post-closing, Closed, Holiday, Special session
	Tooltip string `json:"tooltip"` // "Non-trading day (inferred)" or ""
}
func (s *Service) LabelAt(ex Exchange, t time.Time) Label
type Record struct {
	Exchange string
	Date     int    // yyyymmdd
	Type     string // holiday | special | trading | inferred-non-trading
	Phases   string // JSON array of PhaseSpec; "[]" when none
	Source   string
}
func ToRecord(e Entry) Record
func FromRecord(r Record) (Entry, error)
```

FR-DATA-001 AC1 (holiday), AC2 (special session 18:15–19:15 anchors 5-minute candles and the square-off candle), AC3 (post-closing is cash-only), AC6 (stale when the next 30 days are not covered); `Version()` feeds FR-BT-044 run records; `LabelAt` gives FR-SHELL-005 its phase text.

- [ ] **Step 1: Write the failing tests**

`internal/calendar/service_test.go`:

```go
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
	if s.Version() != "2026.1" {
		t.Fatalf("version = %q", s.Version())
	}
	s.MergeOverrides([]Entry{{Date: Date{2026, 12, 24}, Exchange: NSE, Type: DayHoliday, Source: "user"}})
	v := s.Version()
	if !strings.HasPrefix(v, "2026.1+u") || len(v) != len("2026.1+u")+8 {
		t.Fatalf("version = %q", v)
	}
	s.DeleteOverride(NSE, Date{2026, 12, 24})
	if s.Version() != "2026.1" {
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/calendar/`
Expected: the build fails — output contains `undefined: NewDefault` and `[build failed]`.

- [ ] **Step 3: Write the implementation**

`internal/calendar/service.go`:

```go
package calendar

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	calendardata "tradeloom/assets/calendar"
)

type key struct {
	ex Exchange
	d  Date
}

// Service answers every calendar question in the app. It is safe for concurrent use.
type Service struct {
	mu        sync.RWMutex
	shipped   map[key]Entry
	overrides map[key]Entry
	version   string
	coversTo  Date
}

// New builds a Service from a parsed shipped file.
func New(shipped File) *Service {
	s := &Service{shipped: map[key]Entry{}, overrides: map[key]Entry{}, version: shipped.Version, coversTo: shipped.CoversTo}
	for _, e := range shipped.Entries {
		s.shipped[key{e.Exchange, e.Date}] = e
		if e.Date.After(s.coversTo) {
			s.coversTo = Date{e.Date.Year, time.December, 31}
		}
	}
	return s
}

// NewDefault builds a Service from the embedded shipped CSV.
func NewDefault() (*Service, error) {
	f, err := ParseFile(bytes.NewReader(calendardata.Shipped), "shipped")
	if err != nil {
		return nil, err
	}
	return New(f), nil
}

// SetOverrides replaces all user/import/inferred entries (loaded from app.db at start).
func (s *Service) SetOverrides(es []Entry) {
	m := make(map[key]Entry, len(es))
	for _, e := range es {
		m[key{e.Exchange, e.Date}] = e
	}
	s.mu.Lock()
	s.overrides = m
	s.mu.Unlock()
}

// MergeOverrides adds or replaces entries (import, add, edit, inference) and returns them for
// the caller to persist.
func (s *Service) MergeOverrides(es []Entry) []Entry {
	s.mu.Lock()
	for _, e := range es {
		s.overrides[key{e.Exchange, e.Date}] = e
	}
	s.mu.Unlock()
	return es
}

// DeleteOverride removes a user entry; the shipped entry (if any) applies again.
func (s *Service) DeleteOverride(ex Exchange, d Date) {
	s.mu.Lock()
	delete(s.overrides, key{ex, d})
	s.mu.Unlock()
}

// Overrides returns the current override entries sorted by date, exchange.
func (s *Service) Overrides() []Entry {
	s.mu.RLock()
	out := make([]Entry, 0, len(s.overrides))
	for _, e := range s.overrides {
		out = append(out, e)
	}
	s.mu.RUnlock()
	sortEntries(out)
	return out
}

// Version identifies the calendar in effect for run records (FR-BT-044): the shipped version,
// plus "+u" and 8 hex digits of the overrides' hash when overrides exist.
func (s *Service) Version() string {
	ov := s.Overrides()
	if len(ov) == 0 {
		return s.version
	}
	b, _ := json.Marshal(ov)
	h := sha256.Sum256(b)
	return s.version + "+u" + hex.EncodeToString(h[:4])
}

// lookup finds the entry that governs (ex, d): override, then shipped, then the parent cash
// exchange's entry for derivatives.
func (s *Service) lookup(ex Exchange, d Date) (Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if e, ok := s.overrides[key{ex, d}]; ok {
		return e, true
	}
	if e, ok := s.shipped[key{ex, d}]; ok {
		return e, true
	}
	if p, ok := parent(ex); ok {
		if e, ok := s.overrides[key{p, d}]; ok {
			return derive(ex, e), true
		}
		if e, ok := s.shipped[key{p, d}]; ok {
			return derive(ex, e), true
		}
	}
	if e, ok := s.overrides[key{NSE, d}]; ok && e.Type == DayInferredClosed {
		e.Exchange = ex
		return e, true
	}
	return Entry{}, false
}

// derive adapts a cash-exchange entry to its derivatives exchange: same day type, special
// sessions keep only their continuous phase.
func derive(ex Exchange, e Entry) Entry {
	e.Exchange = ex
	if e.Type == DaySpecial || (e.Type == DayTrading && len(e.Phases) > 0) {
		var ps []PhaseSpec
		for _, p := range e.Phases {
			if p.Kind == PhaseContinuous {
				ps = append(ps, p)
			}
		}
		e.Phases = ps
	}
	return e
}

// Day resolves (ex, d).
func (s *Service) Day(ex Exchange, d Date) Day {
	e, ok := s.lookup(ex, d)
	switch {
	case ok && (e.Type == DayHoliday || e.Type == DayInferredClosed):
		return Day{Exchange: ex, Date: d, Type: e.Type}
	case ok && (e.Type == DaySpecial || e.Type == DayTrading):
		specs := e.Phases
		if len(specs) == 0 {
			specs = regularFor(ex)
		}
		return Day{Exchange: ex, Date: d, Type: e.Type, Phases: concrete(d, specs)}
	case d.IsWeekend():
		return Day{Exchange: ex, Date: d, Type: DayWeekend}
	default:
		return Day{Exchange: ex, Date: d, Type: DayTrading, Phases: concrete(d, regularFor(ex))}
	}
}

func concrete(d Date, specs []PhaseSpec) []Phase {
	out := make([]Phase, len(specs))
	for i, p := range specs {
		out[i] = Phase{Kind: p.Kind, Start: d.At(p.Start), End: d.At(p.End)}
	}
	return out
}

// IsTradingDay reports whether ex trades on d.
func (s *Service) IsTradingDay(ex Exchange, d Date) bool { return s.Day(ex, d).IsTrading() }

// PhaseAt returns the phase in force at t (Kind closed, zero bounds, when none) and the day.
func (s *Service) PhaseAt(ex Exchange, t time.Time) (Phase, Day) {
	day := s.Day(ex, DateOf(t))
	for _, p := range day.Phases {
		if !t.Before(p.Start) && t.Before(p.End) {
			return p, day
		}
	}
	return Phase{Kind: PhaseClosed}, day
}

// Session returns the continuous phase of (ex, d): the candle anchor and the session end.
func (s *Service) Session(ex Exchange, d Date) (start, end time.Time, ok bool) {
	p, ok := s.Day(ex, d).Phase(PhaseContinuous)
	return p.Start, p.End, ok
}

// Anchor returns the candle-alignment anchor of (ex, d): the continuous-phase start.
func (s *Service) Anchor(ex Exchange, d Date) (time.Time, bool) {
	a, _, ok := s.Session(ex, d)
	return a, ok
}

// NextTradingDay returns the first trading day strictly after d (searches up to 30 days).
func (s *Service) NextTradingDay(ex Exchange, d Date) (Date, bool) {
	for i := 1; i <= 30; i++ {
		n := d.AddDays(i)
		if s.IsTradingDay(ex, n) {
			return n, true
		}
	}
	return Date{}, false
}

// PrevTradingDay returns the last trading day strictly before d (searches up to 30 days).
func (s *Service) PrevTradingDay(ex Exchange, d Date) (Date, bool) {
	for i := 1; i <= 30; i++ {
		p := d.AddDays(-i)
		if s.IsTradingDay(ex, p) {
			return p, true
		}
	}
	return Date{}, false
}

// CoversUntil is the last date the list is known to describe.
func (s *Service) CoversUntil() Date {
	s.mu.RLock()
	defer s.mu.RUnlock()
	to := s.coversTo
	for _, e := range s.overrides {
		if e.Source != "inferred" && e.Date.After(to) {
			to = Date{e.Date.Year, time.December, 31}
		}
	}
	return to
}

// Stale reports whether the list does not cover the next 30 calendar days from now.
func (s *Service) Stale(now time.Time) bool {
	return DateOf(now).AddDays(30).After(s.CoversUntil())
}

// InstrumentsReadyAt is when the day's Kite instrument dump is expected (08:30 IST).
func (s *Service) InstrumentsReadyAt(d Date) time.Time { return d.At(instrumentsReady) }

// Label is the status-indicator text for (ex, t) (FR-SHELL-005) and its tooltip addition.
type Label struct {
	Text    string `json:"text"`    // Pre-open, Open, Closing, Post-closing, Closed, Holiday, Special session
	Tooltip string `json:"tooltip"` // "Non-trading day (inferred)" or ""
}

// LabelAt computes the status label.
func (s *Service) LabelAt(ex Exchange, t time.Time) Label {
	p, day := s.PhaseAt(ex, t)
	switch day.Type {
	case DayHoliday:
		return Label{Text: "Holiday"}
	case DayInferredClosed:
		return Label{Text: "Closed", Tooltip: "Non-trading day (inferred)"}
	case DayWeekend:
		return Label{Text: "Closed"}
	}
	if day.Type == DaySpecial && p.Kind != PhaseClosed {
		return Label{Text: "Special session"}
	}
	switch p.Kind {
	case PhasePreOpen:
		return Label{Text: "Pre-open"}
	case PhaseContinuous:
		return Label{Text: "Open"}
	case PhaseClosing:
		return Label{Text: "Closing"}
	case PhasePostClosing:
		return Label{Text: "Post-closing"}
	}
	return Label{Text: "Closed"}
}
```

`internal/calendar/record.go`:

```go
package calendar

import "encoding/json"

// Record is the persisted form of an override (app.db table calendar_overrides, §4.2).
// internal/store.CalendarOverrideRow has the same fields; the service layer copies between them.
type Record struct {
	Exchange string
	Date     int    // yyyymmdd
	Type     string // holiday | special | trading | inferred-non-trading
	Phases   string // JSON array of PhaseSpec; "[]" when none
	Source   string
}

// ToRecord encodes e for storage.
func ToRecord(e Entry) Record {
	ps := e.Phases
	if ps == nil {
		ps = []PhaseSpec{}
	}
	b, _ := json.Marshal(ps)
	return Record{Exchange: string(e.Exchange), Date: e.Date.Int(), Type: string(e.Type), Phases: string(b), Source: e.Source}
}

// FromRecord decodes a stored override.
func FromRecord(r Record) (Entry, error) {
	var ps []PhaseSpec
	if err := json.Unmarshal([]byte(r.Phases), &ps); err != nil {
		return Entry{}, err
	}
	if len(ps) == 0 {
		ps = nil
	}
	return Entry{Exchange: Exchange(r.Exchange), Date: DateFromInt(r.Date), Type: DayType(r.Type), Phases: ps, Source: r.Source}, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/calendar/`
Expected: `ok  	tradeloom/internal/calendar`; no `DATA RACE`.

- [ ] **Step 5: Commit**

```bash
git add internal/calendar/service.go \
  internal/calendar/record.go \
  internal/calendar/service_test.go
git commit -m "$(cat <<'EOF'
feat(calendar): calendar service with phases, anchors, special sessions and staleness

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
