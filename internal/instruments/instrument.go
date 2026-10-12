// Package instruments is the instrument master (FR-WATCH-008): import and scope filter (NFR-15),
// display naming (FR-WATCH-013), exchange tags (FR-WATCH-014), local search and daily refresh.
package instruments

import (
	"strings"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/clock"
	"tradeloom/internal/store"
)

// Tag is the exchange tag shown everywhere (FR-WATCH-014).
type Tag string

const (
	TagIndices Tag = "INDICES"
	TagNSE     Tag = "NSE"
	TagBSE     Tag = "BSE"
	TagNFOFut  Tag = "NFO-FUT"
	TagBFOFut  Tag = "BFO-FUT"
)

// Instrument is one kept row of the Kite dump plus derived fields.
type Instrument struct {
	Token         uint32    `json:"token"`
	ExchangeToken uint32    `json:"exchangeToken"`
	Exchange      string    `json:"exchange"` // Kite exchange: NSE, BSE, NFO, BFO
	Segment       string    `json:"segment"`
	Tag           Tag       `json:"tag"`
	Tradingsymbol string    `json:"tradingsymbol"`
	Name          string    `json:"name"`
	Type          string    `json:"instrumentType"` // EQ or FUT
	Expiry        time.Time `json:"expiry"`         // zero unless a future; 00:00 IST of the expiry date
	TickSizePaise int64     `json:"tickSizePaise"`
	LotSize       int64     `json:"lotSize"`
	DisplayName   string    `json:"displayName"`
	Underlying    string    `json:"underlying"` // futures: the dump's name field
	IsIndex       bool      `json:"isIndex"`
	IsWeekly      bool      `json:"isWeekly"`
}

// ID is Kite's EXCHANGE:TRADINGSYMBOL form used internally and in deep links (FR-WATCH-013).
func (i Instrument) ID() string { return i.Exchange + ":" + i.Tradingsymbol }

// IsFuture reports whether i is a futures contract (shown with an "F" badge).
func (i Instrument) IsFuture() bool { return i.Type == "FUT" }

// CalendarExchange is the calendar the instrument trades on.
func (i Instrument) CalendarExchange() calendar.Exchange { return calendar.Exchange(i.Exchange) }

// UnitsForLots converts a quantity in lots to units at the instrument's current lot size
// (FR-WATCH-008: saved lot quantities follow lot-size changes).
func UnitsForLots(lots int64, in Instrument) int64 {
	if in.LotSize <= 0 {
		return lots
	}
	return lots * in.LotSize
}

func toRow(i Instrument, importedAt time.Time) store.InstrumentRow {
	var exp int64
	if !i.Expiry.IsZero() {
		exp = int64(calendar.DateOf(i.Expiry).Int())
	}
	return store.InstrumentRow{
		Token: int64(i.Token), ExchangeToken: int64(i.ExchangeToken), Tradingsymbol: i.Tradingsymbol, Name: i.Name,
		Exchange: i.Exchange, Segment: i.Segment, InstrumentType: i.Type, Expiry: exp, TickSizePaise: i.TickSizePaise,
		LotSize: i.LotSize, DisplayName: i.DisplayName, Underlying: i.Underlying, IsIndex: i.IsIndex, IsWeekly: i.IsWeekly,
		ImportedAt: importedAt.UnixMilli(),
	}
}

func fromRow(r store.InstrumentRow) Instrument {
	var exp time.Time
	if r.Expiry != 0 {
		exp = calendar.DateFromInt(int(r.Expiry)).Midnight()
	}
	return Instrument{
		Token: uint32(r.Token), ExchangeToken: uint32(r.ExchangeToken), Exchange: r.Exchange, Segment: r.Segment,
		Tag: tagOf(r.Exchange, r.Segment), Tradingsymbol: r.Tradingsymbol, Name: r.Name, Type: r.InstrumentType, Expiry: exp,
		TickSizePaise: r.TickSizePaise, LotSize: r.LotSize, DisplayName: r.DisplayName, Underlying: r.Underlying,
		IsIndex: r.IsIndex, IsWeekly: r.IsWeekly,
	}
}

// tagOf derives the tag from the instrument's own record only (FR-WATCH-014).
func tagOf(exchange, segment string) Tag {
	switch segment {
	case "INDICES":
		return TagIndices
	case "NFO-FUT":
		return TagNFOFut
	case "BFO-FUT":
		return TagBFOFut
	}
	return Tag(strings.ToUpper(exchange))
}

func midnight(t time.Time) time.Time { return clock.MidnightIST(t) }
