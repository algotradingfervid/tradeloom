> **P0-3 Session & Service Layer Implementation Plan — Task 12.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 12: Order book, positions and holdings builders (pure)

**Files:**
- Create: `internal/kite/portfolio_types.go`, `internal/orders/book.go`, `internal/orders/positions.go`, `internal/orders/holdings.go`, `internal/orders/scope.go`
- Test: `internal/orders/builders_test.go`

**Interfaces:**
- Consumes: P0-2 `kite.Order`, `kite.Orders`, `kite.Positions`, `kite.Holdings` aliases (`types.go`); `kite.Position` and `kite.Holding` are missing there and are added by this task (`portfolio_types.go`, same alias style, so `internal/orders` never imports gokiteconnect — rule 3). The fields used are verified in gokiteconnect v4.4.3:
  - `orders.go` `Order`: `OrderID, Status, StatusMessage, OrderTimestamp, ExchangeUpdateTimestamp, Variety, Exchange, TradingSymbol, InstrumentToken, OrderType, TransactionType, Product, Quantity, Price, TriggerPrice, AveragePrice, FilledQuantity, Tag`.
  - `portfolio.go` `Position`: `Tradingsymbol, Exchange, InstrumentToken, Product, Quantity, Multiplier, AveragePrice, LastPrice, PnL`.
  - `portfolio.go` `Holding`: `Tradingsymbol, Exchange, InstrumentToken, Quantity, T1Quantity, AveragePrice, LastPrice, ClosePrice`.
- Produces:
  ```go
  // package kite (portfolio_types.go)
  type ( Position = kiteconnect.Position; Holding = kiteconnect.Holding )
  // package orders
  func Paise(rupees float64) int64
  func RatioBp(num, den int64) int64
  const SectionOpen, SectionExecuted, SectionRejected, SectionCancelled = "open", "executed", "rejected", "cancelled"
  func SectionOf(status string) string
  type OrderRow struct{ OrderID string; TimeMs int64; Side, Exchange, Symbol string; Token uint32; Product, OrderType, Variety string
      Qty, FilledQty, PricePaise, TriggerPaise, AvgPricePaise int64; Status, StatusMessage, Section, Tag, Source string }
  type SectionCounts struct{ Open, Executed, Rejected, Cancelled int }
  type BookSnapshot struct{ Orders []OrderRow; Counts SectionCounts; AsOfMs int64; Stale bool }
  func BuildBook(os []kite.Order, asOfMs int64, stale bool) BookSnapshot
  type PositionRow struct{ Key, Exchange, Symbol string; Token uint32; Product string; Qty, Multiplier, AvgPaise, LTPPaise, PnLPaise, NetChangeBp int64; Closed bool } // Multiplier: Kite multiplier, 0 → 1 (P0-4 re-prices between positions.changed events)
  type PositionsSnapshot struct{ Net, Day []PositionRow; NetTotalPnLPaise, DayTotalPnLPaise, AsOfMs int64; Stale bool }
  func BuildPositions(p kite.Positions, ltp map[uint32]float64, asOfMs int64, stale bool) PositionsSnapshot
  type HoldingRow struct{ Exchange, Symbol string; Token uint32; Qty, AvgPaise, LTPPaise, PrevClosePaise, InvestedPaise, CurrentPaise, PnLPaise, NetChangeBp, DayChangePaise, DayChangeBp int64 }
  type HoldingsSummary struct{ Empty bool; CurrentValuePaise, InvestmentPaise, PnLPaise, PnLBp, DayChangePaise int64 }
  type HoldingsSnapshot struct{ Rows []HoldingRow; Summary HoldingsSummary; AsOfMs int64; Stale bool }
  func BuildHoldings(h kite.Holdings, ltp map[uint32]float64, asOfMs int64, stale bool) HoldingsSnapshot
  type Scope uint8; const ScopeOrders, ScopePositions, ScopeHoldings, ScopeAll Scope
  func ParseScope(s string) (Scope, error) // "orders" | "positions" | "holdings" | "all" | ""
  ```

Definitions:
- Holdings quantity is `quantity + t1_quantity` (FR-HOME-002). Rows with zero quantity are omitted.
- Holdings "Net change" is `(LTP − avg)/avg`. "Day change" is `qty × (LTP − close_price)`, where `close_price` is the previous close per Kite's docs.
- Kite documents no position P&L formula, so live position P&L adjusts Kite's own `pnl` linearly: `pnl + quantity × (LTP_live − last_price) × multiplier`, with `multiplier` 0 treated as 1.
- Source is `tradeloom` for tags starting with `TL` (AD-24). Every other tag is `external` until P4 resolves journal sources.

- [ ] **Step 1: Write the failing tests**

```go
// internal/orders/builders_test.go
package orders

import (
	"encoding/json"
	"testing"

	"tradeloom/internal/kite"
)

func mustJSON[T any](t *testing.T, s string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

const homeHoldings = `[
 {"tradingsymbol":"A","exchange":"NSE","instrument_token":1,"quantity":10,"t1_quantity":0,"average_price":100,"last_price":110,"close_price":105},
 {"tradingsymbol":"B","exchange":"NSE","instrument_token":2,"quantity":5,"t1_quantity":0,"average_price":200,"last_price":190,"close_price":200}]`

// FR-HOME-002 AC1.
func TestBuildHoldings_HomeCardAC1(t *testing.T) {
	s := BuildHoldings(mustJSON[kite.Holdings](t, homeHoldings), nil, 1, false).Summary
	want := HoldingsSummary{CurrentValuePaise: 205000, InvestmentPaise: 200000, PnLPaise: 5000, PnLBp: 250, DayChangePaise: 0}
	if s != want {
		t.Fatalf("got %+v want %+v", s, want)
	}
}

// FR-HOME-002 AC2: a tick moves A to 120.
func TestBuildHoldings_LiveLTP(t *testing.T) {
	s := BuildHoldings(mustJSON[kite.Holdings](t, homeHoldings), map[uint32]float64{1: 120}, 1, false).Summary
	if s.CurrentValuePaise != 215000 {
		t.Fatalf("current %d", s.CurrentValuePaise)
	}
}

// FR-HOME-002 AC3 and FR-ORD-003 AC1 (data): no holdings → Empty, no zero values to show.
func TestBuildHoldings_Empty(t *testing.T) {
	h := BuildHoldings(nil, nil, 1, false)
	if !h.Summary.Empty || len(h.Rows) != 0 {
		t.Fatalf("%+v", h)
	}
	zero := BuildHoldings(mustJSON[kite.Holdings](t, `[{"tradingsymbol":"Z","quantity":0,"t1_quantity":0,"average_price":1,"last_price":1}]`), nil, 1, false)
	if !zero.Summary.Empty {
		t.Fatal("zero-quantity rows must not count as holdings")
	}
}

// FR-ORD-003 AC2, plus T1 quantity.
func TestBuildHoldings_RowValues(t *testing.T) {
	r := BuildHoldings(mustJSON[kite.Holdings](t, `[{"tradingsymbol":"X","exchange":"NSE","instrument_token":7,"quantity":6,"t1_quantity":4,"average_price":100,"last_price":110,"close_price":108}]`), nil, 1, false).Rows[0]
	if r.Qty != 10 || r.InvestedPaise != 100000 || r.CurrentPaise != 110000 || r.PnLPaise != 10000 || r.NetChangeBp != 1000 || r.DayChangePaise != 2000 {
		t.Fatalf("%+v", r)
	}
}

// FR-ORD-002 AC1, plus a live LTP adjustment.
func TestBuildPositions_TotalAndLive(t *testing.T) {
	p := mustJSON[kite.Positions](t, `{"net":[
	 {"tradingsymbol":"P","exchange":"NSE","instrument_token":11,"product":"MIS","quantity":10,"multiplier":1,"average_price":50,"last_price":50,"pnl":120},
	 {"tradingsymbol":"Q","exchange":"NSE","instrument_token":12,"product":"MIS","quantity":0,"multiplier":1,"average_price":0,"last_price":80,"pnl":-45.5}],"day":[]}`)
	s := BuildPositions(p, nil, 1, false)
	if s.NetTotalPnLPaise != 7450 || !s.Net[1].Closed {
		t.Fatalf("%+v", s)
	}
	live := BuildPositions(p, map[uint32]float64{11: 51}, 1, false)
	if live.Net[0].PnLPaise != 13000 || live.NetTotalPnLPaise != 8450 {
		t.Fatalf("live %+v", live.Net[0])
	}
	if s.Net[0].Multiplier != 1 || s.Net[1].Multiplier != 1 {
		t.Fatalf("multiplier %+v", s.Net)
	}
	// A futures position carries Kite's lot multiplier; a missing multiplier (0) is reported as 1.
	f := mustJSON[kite.Positions](t, `{"net":[
	 {"tradingsymbol":"NIFTY26OCTFUT","exchange":"NFO","instrument_token":13,"product":"NRML","quantity":25,"multiplier":25,"average_price":22500,"last_price":22510,"pnl":6250},
	 {"tradingsymbol":"R","exchange":"NSE","instrument_token":14,"product":"MIS","quantity":1,"multiplier":0,"average_price":10,"last_price":10,"pnl":0}],"day":[]}`)
	fs := BuildPositions(f, map[uint32]float64{13: 22511}, 1, false)
	if fs.Net[0].Multiplier != 25 || fs.Net[0].PnLPaise != 687500 || fs.Net[1].Multiplier != 1 {
		t.Fatalf("futures %+v", fs.Net)
	}
}

func TestBuildBook_SectionsSourceAndOrder(t *testing.T) {
	os := mustJSON[kite.Orders](t, `[
	 {"order_id":"1","status":"COMPLETE","tradingsymbol":"INFY","exchange":"NSE","transaction_type":"BUY","quantity":5,"filled_quantity":5,"average_price":1500.5,"order_timestamp":"2026-10-12 09:20:00","tag":"TLABCDEF"},
	 {"order_id":"2","status":"OPEN","tradingsymbol":"TCS","exchange":"NSE","transaction_type":"SELL","quantity":1,"price":4000,"order_timestamp":"2026-10-12 09:30:00"},
	 {"order_id":"3","status":"TRIGGER PENDING","order_timestamp":"2026-10-12 09:25:00"},
	 {"order_id":"4","status":"REJECTED","order_timestamp":"2026-10-12 09:10:00"},
	 {"order_id":"5","status":"CANCELLED","order_timestamp":"2026-10-12 09:05:00"}]`)
	b := BuildBook(os, 99, false)
	if b.Counts != (SectionCounts{Open: 2, Executed: 1, Rejected: 1, Cancelled: 1}) {
		t.Fatalf("counts %+v", b.Counts)
	}
	if b.Orders[0].OrderID != "2" || b.Orders[0].Side != "Sell" || b.Orders[0].PricePaise != 400000 {
		t.Fatalf("first %+v", b.Orders[0])
	}
	var infy OrderRow
	for _, r := range b.Orders {
		if r.OrderID == "1" {
			infy = r
		}
	}
	if infy.Source != "tradeloom" || infy.AvgPricePaise != 150050 || infy.Section != SectionExecuted || b.Orders[1].Source != "external" {
		t.Fatalf("infy %+v", infy)
	}
}
func TestParseScope(t *testing.T) {
	for in, want := range map[string]Scope{"orders": ScopeOrders, "positions": ScopePositions, "holdings": ScopeHoldings, "all": ScopeAll, "": ScopeAll} {
		if got, err := ParseScope(in); err != nil || got != want {
			t.Fatalf("%q -> %v %v", in, got, err)
		}
	}
	if _, err := ParseScope("gtt"); err == nil {
		t.Fatal("unknown scope accepted")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/orders/ -run 'TestBuild|TestParseScope' -v`
Expected: FAIL to compile with `undefined: BuildHoldings`.

- [ ] **Step 3: Write the implementation**

```go
// internal/kite/portfolio_types.go
package kite

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// Position and Holding complete P0-2's alias list (types.go has Positions and Holdings only), so
// internal/orders can name a single row without importing gokiteconnect (rule 3).
type (
	Position = kiteconnect.Position
	Holding  = kiteconnect.Holding
)
```

```go
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
```

```go
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
}

type PositionsSnapshot struct {
	Net              []PositionRow `json:"net"`
	Day              []PositionRow `json:"day"`
	NetTotalPnLPaise int64         `json:"netTotalPnlPaise"`
	DayTotalPnLPaise int64         `json:"dayTotalPnlPaise"`
	AsOfMs           int64         `json:"asOfMs"`
	Stale            bool          `json:"stale"`
}

func positionKey(kind string, p kite.Position) string {
	return kind + "|" + p.Exchange + "|" + p.Tradingsymbol + "|" + p.Product
}

func positionRow(kind string, p kite.Position, ltp map[uint32]float64) PositionRow {
	last := p.LastPrice
	if v, ok := ltp[p.InstrumentToken]; ok {
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
		NetChangeBp: RatioBp(l-avg, avg), Closed: p.Quantity == 0}
}

func BuildPositions(p kite.Positions, ltp map[uint32]float64, asOfMs int64, stale bool) PositionsSnapshot {
	s := PositionsSnapshot{Net: []PositionRow{}, Day: []PositionRow{}, AsOfMs: asOfMs, Stale: stale}
	for _, x := range p.Net {
		r := positionRow("net", x, ltp)
		s.Net = append(s.Net, r)
		s.NetTotalPnLPaise += r.PnLPaise
	}
	for _, x := range p.Day {
		r := positionRow("day", x, ltp)
		s.Day = append(s.Day, r)
		s.DayTotalPnLPaise += r.PnLPaise
	}
	return s
}
```

```go
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
}

type HoldingsSummary struct {
	Empty             bool  `json:"empty"`
	CurrentValuePaise int64 `json:"currentValuePaise"`
	InvestmentPaise   int64 `json:"investmentPaise"`
	PnLPaise          int64 `json:"pnlPaise"`
	PnLBp             int64 `json:"pnlBp"`
	DayChangePaise    int64 `json:"dayChangePaise"`
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
	if v, ok := ltp[h.InstrumentToken]; ok {
		last = v
	}
	avg, l, pc := Paise(h.AveragePrice), Paise(last), Paise(h.ClosePrice)
	inv, cur := q*avg, q*l
	return HoldingRow{Exchange: h.Exchange, Symbol: h.Tradingsymbol, Token: h.InstrumentToken, Qty: q, AvgPaise: avg, LTPPaise: l,
		PrevClosePaise: pc, InvestedPaise: inv, CurrentPaise: cur, PnLPaise: cur - inv, NetChangeBp: RatioBp(l-avg, avg),
		DayChangePaise: q * (l - pc), DayChangeBp: RatioBp(l-pc, pc)}
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
```

```go
// internal/orders/scope.go
package orders

import "fmt"

// Scope selects what a refresh fetches.
type Scope uint8

const (
	ScopeOrders Scope = 1 << iota
	ScopePositions
	ScopeHoldings
	ScopeAll = ScopeOrders | ScopePositions | ScopeHoldings
)

func ParseScope(s string) (Scope, error) {
	switch s {
	case "", "all":
		return ScopeAll, nil
	case "orders":
		return ScopeOrders, nil
	case "positions":
		return ScopePositions, nil
	case "holdings":
		return ScopeHoldings, nil
	}
	return 0, fmt.Errorf("unknown refresh scope %q", s)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders ./internal/orders/ -run 'TestBuild|TestParseScope' -v`
Expected: PASS (7 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/kite/portfolio_types.go internal/orders/book.go internal/orders/positions.go internal/orders/holdings.go internal/orders/scope.go internal/orders/builders_test.go
git commit -m "feat(orders): read-only order book, positions and holdings builders

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
