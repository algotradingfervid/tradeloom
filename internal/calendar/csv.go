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
	if err := sc.Err(); err != nil {
		return File{}, fmt.Errorf("reading calendar: %w", err)
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
		if _, ok := specContinuous(e.Phases); !ok {
			switch {
			case e.Type == DaySpecial:
				return File{}, &RowError{firstRow[k{e.Exchange, e.Date}], "special session needs a continuous phase"}
			case e.Type == DayTrading && len(e.Phases) > 0: // phases replace the regular ones, so they need a session (Ruling 77)
				return File{}, &RowError{firstRow[k{e.Exchange, e.Date}], "trading day with phases needs a continuous phase"}
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
