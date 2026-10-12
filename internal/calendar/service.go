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
	extra     Date // furthest `# covers:` end of imported files
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
		e.Phases = append([]PhaseSpec(nil), e.Phases...)
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
		e.Phases = append([]PhaseSpec(nil), e.Phases...)
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

// ExtendCoverage records the `# covers:` end date of an imported calendar file. Only shipped
// files and imported files with an explicit covers range extend coverage; single user rows or
// inferred entries never do (FR-DATA-001 AC6).
func (s *Service) ExtendCoverage(to Date) {
	s.mu.Lock()
	if to.After(s.extra) {
		s.extra = to
	}
	s.mu.Unlock()
}

// CoversUntil is the last date the list is known to describe.
func (s *Service) CoversUntil() Date {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.extra.After(s.coversTo) {
		return s.extra
	}
	return s.coversTo
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
