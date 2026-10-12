package instruments

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/kite"
)

// keep implements the NFR-15 scope guard: indices, NSE/BSE cash equities and NFO/BFO futures
// survive; options, MCX, currency and everything else are dropped, and so are expired futures.
// It fails closed: only these exact (segment, exchange, instrument type) shapes are kept, so a
// new or inconsistent Kite segment/type never enters the master (and the tag, which the frontend
// also derives from the exchange, can never disagree with the exchange).
func keep(r kite.Instrument, today calendar.Date) bool {
	if !inScope(r.Exchange, r.Segment, r.InstrumentType) {
		return false
	}
	if r.InstrumentType == "FUT" {
		return !r.Expiry.Time.IsZero() && !calendar.DateOf(r.Expiry.Time).Before(today)
	}
	return true
}

// inScope is the (segment, exchange, instrument type) shape check of keep, without the expiry
// test; fromRow reuses it so rows loaded from the database obey the same scope guard.
func inScope(exchange, segment, instrumentType string) bool {
	switch {
	case segment == "INDICES":
		return (exchange == "NSE" || exchange == "BSE") && instrumentType == "EQ"
	case (segment == "NSE" || segment == "BSE") && exchange == segment:
		return instrumentType == "EQ"
	case (segment == "NFO-FUT" && exchange == "NFO") || (segment == "BFO-FUT" && exchange == "BFO"):
		return instrumentType == "FUT"
	}
	return false
}

// Filter keeps in-scope rows of a Kite dump and derives display names, tags, underlyings and the
// weekly marker. today is the IST date of the import (app clock).
func Filter(rows kite.Instruments, today calendar.Date) []Instrument {
	var out []Instrument
	for _, r := range rows {
		if !keep(r, today) {
			continue
		}
		tag, _ := tagOf(r.Exchange, r.Segment) // keep() guarantees a valid tag
		in := Instrument{
			Token: uint32(r.InstrumentToken), ExchangeToken: uint32(r.ExchangeToken), Exchange: r.Exchange,
			Segment: r.Segment, Tag: tag, Tradingsymbol: r.Tradingsymbol,
			Name: r.Name, Type: r.InstrumentType, TickSizePaise: int64(math.Round(r.TickSize * 100)),
			LotSize: int64(math.Round(r.LotSize)), IsIndex: r.Segment == "INDICES",
		}
		if in.IsFuture() {
			in.Expiry = midnight(r.Expiry.Time)
			in.Underlying = r.Name
		}
		out = append(out, in)
	}
	deriveFutures(out)
	for i := range out {
		if !out[i].IsFuture() {
			out[i].DisplayName = out[i].Tradingsymbol
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Token < out[j].Token })
	return out
}

// deriveFutures sets IsWeekly (expiry is not the last expiry of its month for that underlying
// on that exchange) and the FR-WATCH-013 display name: "UNDERLYING MON FUT", or
// "UNDERLYING DD MON YYYY FUT" when two contracts of the same underlying share a month.
func deriveFutures(all []Instrument) {
	type key struct {
		seg, und string
		y        int
		m        time.Month
	}
	groups := map[key][]int{}
	for i, in := range all {
		if in.IsFuture() {
			y, m, _ := in.Expiry.Date()
			k := key{in.Segment, in.Underlying, y, m}
			groups[k] = append(groups[k], i)
		}
	}
	for _, idx := range groups {
		var last time.Time
		for _, i := range idx {
			if all[i].Expiry.After(last) {
				last = all[i].Expiry
			}
		}
		for _, i := range idx {
			in := &all[i]
			in.IsWeekly = in.Expiry.Before(last)
			mon := strings.ToUpper(in.Expiry.Format("Jan"))
			if len(idx) > 1 {
				in.DisplayName = fmt.Sprintf("%s %02d %s %d FUT", in.Underlying, in.Expiry.Day(), mon, in.Expiry.Year())
			} else {
				in.DisplayName = fmt.Sprintf("%s %s FUT", in.Underlying, mon)
			}
		}
	}
}
