// internal/orders/book.go
package orders

import (
	"math"
	"sort"
	"strings"

	"tradeloom/internal/kite"
)

// Kite order statuses (gokiteconnect connect.go OrderStatusComplete/Rejected/Cancelled; everything else is "open").
const (
	statusComplete  = "COMPLETE"
	statusRejected  = "REJECTED"
	statusCancelled = "CANCELLED"

	SectionOpen      = "open"
	SectionExecuted  = "executed"
	SectionRejected  = "rejected"
	SectionCancelled = "cancelled"
)

// Paise converts Kite's float rupees to integer paise (AD-04).
func Paise(rupees float64) int64 { return int64(math.Round(rupees * 100)) }

// RatioBp returns num/den in basis points (2.50 % = 250), rounded half away from zero.
func RatioBp(num, den int64) int64 {
	if den == 0 {
		return 0
	}
	return int64(math.Round(float64(num) * 10000 / float64(den)))
}

func SectionOf(status string) string {
	switch strings.ToUpper(status) {
	case statusComplete:
		return SectionExecuted
	case statusRejected:
		return SectionRejected
	case statusCancelled:
		return SectionCancelled
	}
	return SectionOpen
}

func terminal(status string) bool { return SectionOf(status) != SectionOpen }

type OrderRow struct {
	OrderID       string `json:"orderId"`
	TimeMs        int64  `json:"timeMs"`
	Side          string `json:"side"`
	Exchange      string `json:"exchange"`
	Symbol        string `json:"symbol"`
	Token         uint32 `json:"token"`
	Product       string `json:"product"`
	OrderType     string `json:"orderType"`
	Variety       string `json:"variety"`
	Qty           int64  `json:"qty"`
	FilledQty     int64  `json:"filledQty"`
	PricePaise    int64  `json:"pricePaise"`
	TriggerPaise  int64  `json:"triggerPaise"`
	AvgPricePaise int64  `json:"avgPricePaise"`
	Status        string `json:"status"`
	StatusMessage string `json:"statusMessage"`
	Section       string `json:"section"`
	Tag           string `json:"tag"`
	Source        string `json:"source"`
}

type SectionCounts struct {
	Open      int `json:"open"`
	Executed  int `json:"executed"`
	Rejected  int `json:"rejected"`
	Cancelled int `json:"cancelled"`
}

type BookSnapshot struct {
	Orders []OrderRow    `json:"orders"`
	Counts SectionCounts `json:"counts"`
	AsOfMs int64         `json:"asOfMs"`
	Stale  bool          `json:"stale"`
}

func sourceOf(tag string) string {
	if strings.HasPrefix(tag, "TL") {
		return "tradeloom"
	}
	return "external"
}

func orderRowFrom(o kite.Order) OrderRow {
	side := "Buy"
	if strings.EqualFold(o.TransactionType, "SELL") {
		side = "Sell"
	}
	ts := o.OrderTimestamp.Time
	if ts.IsZero() {
		ts = o.ExchangeUpdateTimestamp.Time
	}
	var ms int64
	if !ts.IsZero() {
		ms = ts.UnixMilli()
	}
	return OrderRow{
		OrderID: o.OrderID, TimeMs: ms, Side: side, Exchange: o.Exchange, Symbol: o.TradingSymbol, Token: o.InstrumentToken,
		Product: o.Product, OrderType: o.OrderType, Variety: o.Variety,
		Qty: int64(math.Round(o.Quantity)), FilledQty: int64(math.Round(o.FilledQuantity)),
		PricePaise: Paise(o.Price), TriggerPaise: Paise(o.TriggerPrice), AvgPricePaise: Paise(o.AveragePrice),
		Status: o.Status, StatusMessage: o.StatusMessage, Section: SectionOf(o.Status), Tag: o.Tag, Source: sourceOf(o.Tag),
	}
}

func BuildBook(os []kite.Order, asOfMs int64, stale bool) BookSnapshot {
	b := BookSnapshot{Orders: make([]OrderRow, 0, len(os)), AsOfMs: asOfMs, Stale: stale}
	for _, o := range os {
		if !inEquityScope(o.Exchange, o.TradingSymbol) {
			continue
		}
		r := orderRowFrom(o)
		switch r.Section {
		case SectionOpen:
			b.Counts.Open++
		case SectionExecuted:
			b.Counts.Executed++
		case SectionRejected:
			b.Counts.Rejected++
		case SectionCancelled:
			b.Counts.Cancelled++
		}
		b.Orders = append(b.Orders, r)
	}
	sort.SliceStable(b.Orders, func(i, j int) bool {
		if b.Orders[i].TimeMs != b.Orders[j].TimeMs {
			return b.Orders[i].TimeMs > b.Orders[j].TimeMs
		}
		return b.Orders[i].OrderID > b.Orders[j].OrderID
	})
	return b
}
