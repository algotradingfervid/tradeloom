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
