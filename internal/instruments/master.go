package instruments

import (
	"sort"
	"strings"
	"sync"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/clock"
)

// entry is an Instrument with search keys precomputed.
type entry struct {
	in        Instrument
	sym       string   // lower tradingsymbol
	alias     string   // lower F&O name for indices ("banknifty"), else ""
	name      string   // lower name
	tokens    []string // lower display-name tokens
	alsoOnNSE bool
}

// Master is the in-memory instrument master. It is replaced wholesale on each import.
type Master struct {
	clk        clock.Clock
	mu         sync.RWMutex
	all        []entry
	byToken    map[uint32]int
	byID       map[string]int
	importedAt time.Time
}

// NewMaster returns an empty master.
func NewMaster(clk clock.Clock) *Master {
	return &Master{clk: clk, byToken: map[uint32]int{}, byID: map[string]int{}}
}

// Replace installs a new set of instruments.
func (m *Master) Replace(ins []Instrument, importedAt time.Time) {
	nse := map[string]bool{}
	for _, in := range ins {
		if in.Tag == TagNSE {
			nse[strings.ToLower(in.Tradingsymbol)] = true
			if in.Name != "" {
				nse["name:"+strings.ToLower(in.Name)] = true
			}
		}
	}
	all := make([]entry, len(ins))
	byToken := make(map[uint32]int, len(ins))
	byID := make(map[string]int, len(ins))
	for i, in := range ins {
		e := entry{in: in, sym: strings.ToLower(in.Tradingsymbol), name: strings.ToLower(in.Name),
			tokens: strings.Fields(strings.ToLower(in.DisplayName))}
		if in.IsIndex {
			e.alias = strings.ToLower(indexAliases[in.Tradingsymbol])
		}
		if in.Tag == TagBSE {
			e.alsoOnNSE = nse[e.sym] || (e.name != "" && nse["name:"+e.name])
		}
		all[i] = e
		byToken[in.Token] = i
		byID[in.ID()] = i
	}
	m.mu.Lock()
	m.all, m.byToken, m.byID, m.importedAt = all, byToken, byID, importedAt
	m.mu.Unlock()
}

// ImportedAt is when the current list was downloaded.
func (m *Master) ImportedAt() time.Time { m.mu.RLock(); defer m.mu.RUnlock(); return m.importedAt }

// Len is the number of instruments.
func (m *Master) Len() int { m.mu.RLock(); defer m.mu.RUnlock(); return len(m.all) }

// ByToken looks an instrument up by Kite token.
func (m *Master) ByToken(token uint32) (Instrument, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	i, ok := m.byToken[token]
	if !ok {
		return Instrument{}, false
	}
	return m.all[i].in, true
}

// ByID looks an instrument up by "EXCHANGE:TRADINGSYMBOL".
func (m *Master) ByID(id string) (Instrument, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	i, ok := m.byID[id]
	if !ok {
		return Instrument{}, false
	}
	return m.all[i].in, true
}

// ExchangeOf returns the calendar exchange of token (NSE when unknown).
func (m *Master) ExchangeOf(token uint32) calendar.Exchange {
	if in, ok := m.ByToken(token); ok {
		return in.CalendarExchange()
	}
	return calendar.NSE
}

// Futures returns the unexpired futures of an underlying on segment ("NFO-FUT"/"BFO-FUT"),
// nearest expiry first.
func (m *Master) Futures(segment, underlying string) []Instrument {
	today := calendar.DateOf(m.clk.Now())
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Instrument
	for _, e := range m.all {
		if e.in.IsFuture() && e.in.Segment == segment && e.in.Underlying == underlying && !calendar.DateOf(e.in.Expiry).Before(today) {
			out = append(out, e.in)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Expiry.Before(out[j].Expiry) })
	return out
}
