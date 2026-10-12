// internal/orders/positions.go
package orders

import "tradeloom/internal/kite"

type PositionRow struct {
	Key         string `json:"key"`
	Exchange    string `json:"exchange"`
	Symbol      string `json:"symbol"`
	Token       uint32 `json:"token"`
	Product     string `json:"product"`
	Qty         int64  `json:"qty"`
	Multiplier  int64  `json:"multiplier"` // Kite multiplier (lot multiplier for derivatives); 0 reported as 1
	AvgPaise    int64  `json:"avgPaise"`
	LTPPaise    int64  `json:"ltpPaise"`
	PnLPaise    int64  `json:"pnlPaise"`
	NetChangeBp int64  `json:"netChangeBp"`
	Closed      bool   `json:"closed"`
	OutOfScope  bool   `json:"outOfScope"`
}

type PositionsSnapshot struct {
	Net              []PositionRow `json:"net"`
	Day              []PositionRow `json:"day"`
	NetTotalPnLPaise int64         `json:"netTotalPnlPaise"`
	DayTotalPnLPaise int64         `json:"dayTotalPnlPaise"`
	// OutOfScopeCount counts flagged rows across Net and Day.
	OutOfScopeCount int   `json:"outOfScopeCount"`
	AsOfMs          int64 `json:"asOfMs"`
	Stale           bool  `json:"stale"`
}

func positionKey(kind string, p kite.Position) string {
	return kind + "|" + p.Exchange + "|" + p.Tradingsymbol + "|" + p.Product
}

func positionRow(kind string, p kite.Position, ltp map[uint32]float64) PositionRow {
	last := p.LastPrice
	if v, ok := ltp[p.InstrumentToken]; ok && v > 0 {
		last = v
	}
	mult := p.Multiplier
	if mult == 0 {
		mult = 1
	}
	pnl := p.PnL + float64(p.Quantity)*(last-p.LastPrice)*mult
	avg, l := Paise(p.AveragePrice), Paise(last)
	return PositionRow{Key: positionKey(kind, p), Exchange: p.Exchange, Symbol: p.Tradingsymbol, Token: p.InstrumentToken,
		Product: p.Product, Qty: int64(p.Quantity), Multiplier: int64(mult), AvgPaise: avg, LTPPaise: l, PnLPaise: Paise(pnl),
		NetChangeBp: RatioBp(l-avg, avg), Closed: p.Quantity == 0,
		OutOfScope: outOfScope(p.Exchange, p.Tradingsymbol)}
}

func BuildPositions(p kite.Positions, ltp map[uint32]float64, asOfMs int64, stale bool) PositionsSnapshot {
	s := PositionsSnapshot{Net: []PositionRow{}, Day: []PositionRow{}, AsOfMs: asOfMs, Stale: stale}
	for _, x := range p.Net {
		r := positionRow("net", x, ltp)
		s.Net = append(s.Net, r)
		s.NetTotalPnLPaise += r.PnLPaise
		if r.OutOfScope {
			s.OutOfScopeCount++
		}
	}
	for _, x := range p.Day {
		r := positionRow("day", x, ltp)
		s.Day = append(s.Day, r)
		s.DayTotalPnLPaise += r.PnLPaise
		if r.OutOfScope {
			s.OutOfScopeCount++
		}
	}
	return s
}
