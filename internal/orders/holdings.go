// internal/orders/holdings.go
package orders

import (
	"sort"

	"tradeloom/internal/kite"
)

type HoldingRow struct {
	Exchange       string `json:"exchange"`
	Symbol         string `json:"symbol"`
	Token          uint32 `json:"token"`
	Qty            int64  `json:"qty"`
	AvgPaise       int64  `json:"avgPaise"`
	LTPPaise       int64  `json:"ltpPaise"`
	PrevClosePaise int64  `json:"prevClosePaise"`
	InvestedPaise  int64  `json:"investedPaise"`
	CurrentPaise   int64  `json:"currentPaise"`
	PnLPaise       int64  `json:"pnlPaise"`
	NetChangeBp    int64  `json:"netChangeBp"`
	DayChangePaise int64  `json:"dayChangePaise"`
	DayChangeBp    int64  `json:"dayChangeBp"`
	OutOfScope     bool   `json:"outOfScope"`
}

type HoldingsSummary struct {
	Empty             bool  `json:"empty"`
	CurrentValuePaise int64 `json:"currentValuePaise"`
	InvestmentPaise   int64 `json:"investmentPaise"`
	PnLPaise          int64 `json:"pnlPaise"`
	PnLBp             int64 `json:"pnlBp"`
	DayChangePaise    int64 `json:"dayChangePaise"`
	OutOfScopeCount   int   `json:"outOfScopeCount"`
}

type HoldingsSnapshot struct {
	Rows    []HoldingRow    `json:"rows"`
	Summary HoldingsSummary `json:"summary"`
	AsOfMs  int64           `json:"asOfMs"`
	Stale   bool            `json:"stale"`
}

func holdingRow(h kite.Holding, ltp map[uint32]float64) HoldingRow {
	q := int64(h.Quantity + h.T1Quantity)
	last := h.LastPrice
	if v, ok := ltp[h.InstrumentToken]; ok && v > 0 {
		last = v
	}
	avg, l, pc := Paise(h.AveragePrice), Paise(last), Paise(h.ClosePrice)
	inv, cur := q*avg, q*l
	var dayChange int64
	if pc != 0 {
		dayChange = q * (l - pc)
	}
	return HoldingRow{Exchange: h.Exchange, Symbol: h.Tradingsymbol, Token: h.InstrumentToken, Qty: q, AvgPaise: avg, LTPPaise: l,
		PrevClosePaise: pc, InvestedPaise: inv, CurrentPaise: cur, PnLPaise: cur - inv, NetChangeBp: RatioBp(l-avg, avg),
		DayChangePaise: dayChange, DayChangeBp: RatioBp(l-pc, pc),
		OutOfScope: outOfScope(h.Exchange, h.Tradingsymbol)}
}

// Summarize implements the FR-HOME-002 card definitions.
func Summarize(rows []HoldingRow) HoldingsSummary {
	if len(rows) == 0 {
		return HoldingsSummary{Empty: true}
	}
	var s HoldingsSummary
	for _, r := range rows {
		s.CurrentValuePaise += r.CurrentPaise
		s.InvestmentPaise += r.InvestedPaise
		s.DayChangePaise += r.DayChangePaise
		if r.OutOfScope {
			s.OutOfScopeCount++
		}
	}
	s.PnLPaise = s.CurrentValuePaise - s.InvestmentPaise
	s.PnLBp = RatioBp(s.PnLPaise, s.InvestmentPaise)
	return s
}

func BuildHoldings(hs kite.Holdings, ltp map[uint32]float64, asOfMs int64, stale bool) HoldingsSnapshot {
	rows := make([]HoldingRow, 0, len(hs))
	for _, h := range hs {
		if h.Quantity+h.T1Quantity == 0 {
			continue
		}
		rows = append(rows, holdingRow(h, ltp))
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Symbol < rows[j].Symbol })
	return HoldingsSnapshot{Rows: rows, Summary: Summarize(rows), AsOfMs: asOfMs, Stale: stale}
}
