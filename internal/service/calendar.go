// internal/service/calendar.go
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/calendar"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
)

const (
	calendarStage = "calendar"

	// An imported calendar is untrusted input. A full year for four exchanges is a few hundred rows.
	maxCalendarFileBytes = 1 << 20 // 1 MiB
	maxCalendarRows      = 5000    // data rows (header and # comments not counted)
	calendarMinYear      = 2000    // earliest year an override may describe
	calendarYearsAhead   = 2       // latest date: 31 Dec of (this year + 2), app clock
	maxCalendarPhases    = 4       // pre-open, continuous, closing, post-closing
	maxCalendarReason    = 120     // longest parse message echoed back to the UI (Ruling 77)
)

// calendarCoversUntil persists the furthest "# covers:" end of an imported file (Ruling 28). It is
// deliberately not registered, so the generic settings façade can neither list nor change it.
var calendarCoversUntil = settings.Key[string]{Name: "calendar.covers_until", Default: "", Validate: settings.ValidateDateOrEmpty}

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
	for _, e := range cal.Overrides() { // sorted by date, then exchange
		info.Entries = append(info.Entries, entryDTO(e))
	}
	return info
}

func (s *Service) calendarUnavailable() (CalendarInfo, error) {
	return s.calendarInfo(), s.refuse(calendarStage, refused(calendarStage, "unavailable", "The trading calendar is not loaded."))
}

func (s *Service) calendarRefuse(code, reason string) (CalendarInfo, error) {
	return s.calendarInfo(), s.refuse(calendarStage, refused(calendarStage, code, reason))
}

// calendarChanged makes an override change visible at once: market.status is re-evaluated and published
// (chart ranges, backfill and the builder read the same *calendar.Service, so they see it on their next call).
func (s *Service) calendarChanged() {
	bus.Publish(s.d.Bus, TopicMarketStatus, s.MarketStatus())
}

// calendarWindow is the inclusive range of dates an override may describe.
func (s *Service) calendarWindow() (lo, hi calendar.Date) {
	return calendar.Date{Year: calendarMinYear, Month: time.January, Day: 1},
		calendar.Date{Year: calendar.DateOf(s.d.Clock.Now()).Year + calendarYearsAhead, Month: time.December, Day: 31}
}

func (s *Service) CalendarInfo(_ context.Context) (CalendarInfo, error) {
	if s.d.Calendar == nil {
		return s.calendarUnavailable()
	}
	return s.calendarInfo(), nil
}

// LoadCalendar applies the persisted overrides and the imported coverage (Ruling 28) to the running
// calendar. The wiring calls it once at start-up, after app.db and settings are open.
func (s *Service) LoadCalendar(ctx context.Context) error {
	s.calMu.Lock()
	defer s.calMu.Unlock()
	if s.d.Calendar == nil {
		_, err := s.calendarUnavailable()
		return err
	}
	if s.d.DB == nil {
		_, err := s.calendarRefuse("unavailable", "The database is not open.")
		return err
	}
	rows, err := s.d.DB.CalendarOverrides(ctx)
	if err != nil {
		return s.refuse(calendarStage, err)
	}
	es := make([]calendar.Entry, 0, len(rows))
	for _, r := range rows {
		e, err := calendar.FromRecord(calendar.Record{Exchange: r.Exchange, Date: r.Date, Type: r.Type, Phases: r.Phases, Source: r.Source})
		if err != nil {
			s.d.Log.Warn("calendar override skipped", "exchange", r.Exchange, "date", r.Date, "err", err)
			continue
		}
		es = append(es, e)
	}
	s.d.Calendar.SetOverrides(es)
	if d, ok := s.storedCoverage(); ok {
		s.d.Calendar.ExtendCoverage(d)
	}
	s.calendarChanged()
	return nil
}

// storedCoverage reads calendar.covers_until as stored. settings.Get falls back to the default for a value
// that fails validation, so the raw row is checked too: a bad value is reported, never silently dropped.
func (s *Service) storedCoverage() (calendar.Date, bool) {
	if s.d.Settings == nil {
		return calendar.Date{}, false
	}
	for _, c := range s.d.Settings.Snapshot() {
		if c.Key != calendarCoversUntil.Name {
			continue
		}
		var v string
		if err := json.Unmarshal(c.Value, &v); err != nil {
			s.d.Log.Warn("calendar coverage setting ignored", "setting", calendarCoversUntil.Name, "err", err)
			return calendar.Date{}, false
		}
		if v == "" {
			return calendar.Date{}, false
		}
		d, err := calendar.ParseDate(v)
		if err != nil {
			s.d.Log.Warn("calendar coverage setting ignored", "setting", calendarCoversUntil.Name, "err", err)
			return calendar.Date{}, false
		}
		return d, true
	}
	return calendar.Date{}, false
}

// capReason shortens an echoed parse message to maxCalendarReason characters.
func capReason(r string) string {
	rs := []rune(r)
	if len(rs) <= maxCalendarReason {
		return r
	}
	return string(rs[:maxCalendarReason-1]) + "…"
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
	return s.d.DB.UpsertCalendarOverrides(ctx, rows)
}

// readCalendarFile reads an import file with the size and row bounds applied.
func readCalendarFile(path string) ([]byte, error) {
	// O_NONBLOCK: opening a FIFO or device must not hang; the type is checked on the opened file, so the
	// path cannot be swapped between the check and the read (Ruling 77).
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, refused(calendarStage, "not_found", "The calendar file was not found.")
	}
	if err != nil {
		return nil, refused(calendarStage, "invalid_file", "The calendar file could not be read.")
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, refused(calendarStage, "invalid_file", "The calendar file could not be read.")
	}
	if !st.Mode().IsRegular() {
		return nil, refused(calendarStage, "invalid_file", "The calendar file is not a regular file.")
	}
	if st.Size() > maxCalendarFileBytes {
		return nil, refused(calendarStage, "invalid_file", "The calendar file is larger than 1 MiB.")
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxCalendarFileBytes+1))
	if err != nil {
		return nil, refused(calendarStage, "invalid_file", "The calendar file could not be read.")
	}
	if len(raw) > maxCalendarFileBytes {
		return nil, refused(calendarStage, "invalid_file", "The calendar file is larger than 1 MiB.")
	}
	rows := -1 // the header
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) > 0 && line[0] != '#' {
			rows++
		}
	}
	if rows > maxCalendarRows {
		return nil, refused(calendarStage, "invalid_file", fmt.Sprintf("The calendar file has more than %d rows.", maxCalendarRows))
	}
	return raw, nil
}

// checkImportScope refuses a parsed file whose dates fall outside what the app models or outside the
// file's own "# covers:" range.
func (s *Service) checkImportScope(f calendar.File) error {
	lo, hi := s.calendarWindow()
	if f.CoversTo != (calendar.Date{}) {
		if f.CoversTo.Before(f.CoversFrom) {
			return refused(calendarStage, "invalid_file", "The covers range ends before it starts.")
		}
		if f.CoversTo.After(hi) || f.CoversFrom.Before(lo) {
			return refused(calendarStage, "invalid_file", fmt.Sprintf("The covers range %s..%s is outside %s..%s.", f.CoversFrom, f.CoversTo, lo, hi))
		}
	}
	for _, e := range f.Entries {
		if e.Date.Before(lo) || e.Date.After(hi) {
			return refused(calendarStage, "invalid_file", fmt.Sprintf("%s %s is outside %s..%s.", e.Date, e.Exchange, lo, hi))
		}
		if f.CoversTo != (calendar.Date{}) && (e.Date.Before(f.CoversFrom) || e.Date.After(f.CoversTo)) {
			return refused(calendarStage, "invalid_file", fmt.Sprintf("%s %s is outside the file's covers range.", e.Date, e.Exchange))
		}
	}
	return nil
}

// ImportCalendar loads a CSV (header date,exchange,type,phase,start,end) as "import" overrides. It is
// all-or-nothing: a refused file changes neither app.db, the coverage setting nor the running calendar.
func (s *Service) ImportCalendar(ctx context.Context, path string) (CalendarInfo, error) {
	if s.d.Calendar == nil {
		return s.calendarUnavailable()
	}
	raw, err := readCalendarFile(path)
	if err != nil {
		return s.calendarInfo(), s.refuse(calendarStage, err)
	}
	file, err := calendar.ParseFile(bytes.NewReader(raw), "import")
	if err != nil {
		var re *calendar.RowError
		if errors.As(err, &re) {
			return s.calendarRefuse("invalid_file", capReason(re.Error()))
		}
		return s.calendarRefuse("invalid_file", capReason(err.Error()))
	}
	if err := s.checkImportScope(file); err != nil {
		return s.calendarInfo(), s.refuse(calendarStage, err)
	}

	s.calMu.Lock()
	defer s.calMu.Unlock()
	// Coverage first (it can be rolled back), then the overrides in one transaction, then memory.
	extends := file.CoversTo != (calendar.Date{})
	var oldCov string
	if extends {
		if s.d.Settings == nil {
			return s.calendarRefuse("unavailable", "The settings store is not open.")
		}
		oldCov = settings.Get(s.d.Settings, calendarCoversUntil)
		if old, err := calendar.ParseDate(oldCov); err == nil && !file.CoversTo.After(old) {
			extends = false // already covered at least as far
		}
	}
	if extends {
		if err := settings.Set(ctx, s.d.Settings, calendarCoversUntil, file.CoversTo.String()); err != nil {
			return s.calendarInfo(), s.refuse(calendarStage, err)
		}
	}
	if err := s.persistOverrides(ctx, file.Entries); err != nil {
		if extends {
			if rerr := settings.Set(context.WithoutCancel(ctx), s.d.Settings, calendarCoversUntil, oldCov); rerr != nil {
				s.d.Log.Warn("calendar coverage rollback failed", "err", rerr)
			}
		}
		return s.calendarInfo(), s.refuse(calendarStage, err)
	}
	s.d.Calendar.MergeOverrides(file.Entries)
	if file.CoversTo != (calendar.Date{}) {
		s.d.Calendar.ExtendCoverage(file.CoversTo)
	}
	s.d.Log.Info("calendar imported", "entries", len(file.Entries), "coversTo", file.CoversTo.String(), "file", filepath.Base(path))
	s.calendarChanged()
	return s.calendarInfo(), nil
}

func parseExchange(v string) (calendar.Exchange, bool) {
	for _, x := range calendar.Exchanges {
		if string(x) == v {
			return x, true
		}
	}
	return "", false
}

func (s *Service) parseEntry(in CalendarEntryInput) (calendar.Entry, error) {
	bad := func(reason string) (calendar.Entry, error) {
		return calendar.Entry{}, refused(calendarStage, "invalid_entry", reason)
	}
	ex, ok := parseExchange(in.Exchange)
	if !ok {
		return bad("Exchange must be NSE, BSE, NFO or BFO.")
	}
	d, err := calendar.ParseDate(in.Date)
	if err != nil {
		return bad("Dates are YYYY-MM-DD.")
	}
	if lo, hi := s.calendarWindow(); d.Before(lo) || d.After(hi) {
		return bad(fmt.Sprintf("The date must be between %s and %s.", lo, hi))
	}
	ty := calendar.DayType(in.Type)
	if ty != calendar.DayHoliday && ty != calendar.DaySpecial && ty != calendar.DayTrading {
		return bad("Type must be holiday, special or trading.")
	}
	if ty == calendar.DayHoliday && len(in.Phases) > 0 {
		return bad("A holiday has no session phases.")
	}
	if len(in.Phases) > maxCalendarPhases {
		return bad("A day has at most four phases.")
	}
	e := calendar.Entry{Date: d, Exchange: ex, Type: ty, Source: "user"}
	seen := map[calendar.PhaseKind]bool{}
	for _, p := range in.Phases {
		k := calendar.PhaseKind(p.Kind)
		switch k {
		case calendar.PhasePreOpen, calendar.PhaseContinuous, calendar.PhaseClosing, calendar.PhasePostClosing:
		case "":
			return bad("Every phase needs a kind.")
		default:
			return bad("Phase kind must be pre-open, continuous, closing or post-closing.")
		}
		if seen[k] {
			return bad("Each phase kind appears at most once.")
		}
		seen[k] = true
		start, err1 := calendar.ParseHM(p.Start)
		end, err2 := calendar.ParseHM(p.End)
		if err1 != nil || err2 != nil || end <= start {
			return bad("Phase times are HH:MM and end after start.")
		}
		for _, q := range e.Phases {
			if start < q.End && q.Start < end {
				return bad("Phases must not overlap.")
			}
		}
		e.Phases = append(e.Phases, calendar.PhaseSpec{Kind: k, Start: start, End: end})
	}
	sort.Slice(e.Phases, func(i, j int) bool { return e.Phases[i].Start < e.Phases[j].Start })
	if ty == calendar.DaySpecial && !seen[calendar.PhaseContinuous] {
		return bad("A special session needs a continuous phase.")
	}
	if ty == calendar.DayTrading && len(e.Phases) > 0 && !seen[calendar.PhaseContinuous] {
		return bad("A trading day with phases needs a continuous phase.")
	}
	return e, nil
}

// UpsertCalendarEntry adds or replaces one user override.
func (s *Service) UpsertCalendarEntry(ctx context.Context, in CalendarEntryInput) (CalendarInfo, error) {
	if s.d.Calendar == nil {
		return s.calendarUnavailable()
	}
	e, err := s.parseEntry(in)
	if err != nil {
		return s.calendarInfo(), s.refuse(calendarStage, err)
	}
	s.calMu.Lock()
	defer s.calMu.Unlock()
	if err := s.persistOverrides(ctx, []calendar.Entry{e}); err != nil {
		return s.calendarInfo(), s.refuse(calendarStage, err)
	}
	s.d.Calendar.MergeOverrides([]calendar.Entry{e})
	s.calendarChanged()
	return s.calendarInfo(), nil
}

// RemoveCalendarEntry deletes one override (user, imported or inferred); the shipped day applies again.
func (s *Service) RemoveCalendarEntry(ctx context.Context, exchange, date string) (CalendarInfo, error) {
	if s.d.Calendar == nil {
		return s.calendarUnavailable()
	}
	ex, ok := parseExchange(exchange)
	if !ok {
		return s.calendarRefuse("invalid_entry", "Exchange must be NSE, BSE, NFO or BFO.")
	}
	d, err := calendar.ParseDate(date)
	if err != nil {
		return s.calendarRefuse("invalid_entry", "Dates are YYYY-MM-DD.")
	}
	s.calMu.Lock()
	defer s.calMu.Unlock()
	found := false
	for _, e := range s.d.Calendar.Overrides() {
		if e.Exchange == ex && e.Date == d {
			found = true
			break
		}
	}
	if !found {
		return s.calendarRefuse("not_found", "No override for "+string(ex)+" on "+d.String()+".")
	}
	if s.d.DB == nil {
		return s.calendarRefuse("unavailable", "The database is not open.")
	}
	if err := s.d.DB.DeleteCalendarOverride(ctx, string(ex), d.Int()); err != nil {
		return s.calendarInfo(), s.refuse(calendarStage, err)
	}
	s.d.Calendar.DeleteOverride(ex, d)
	s.calendarChanged()
	return s.calendarInfo(), nil
}
