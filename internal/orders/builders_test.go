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

// Ruling 38: no silent scope filter. Out-of-scope rows are kept, flagged and counted in every total.
func TestBuilders_OutOfScopeFlaggedNotDropped(t *testing.T) {
	p := mustJSON[kite.Positions](t, `{"net":[
	 {"tradingsymbol":"NIFTY26OCTFUT","exchange":"NFO","product":"NRML","quantity":25,"multiplier":25,"average_price":100,"last_price":100,"pnl":10},
	 {"tradingsymbol":"NIFTY2610822500CE","exchange":"NFO","product":"NRML","quantity":25,"multiplier":25,"average_price":100,"last_price":100,"pnl":500},
	 {"tradingsymbol":"GOLDM26OCTFUT","exchange":"MCX","product":"NRML","quantity":1,"multiplier":1,"average_price":1,"last_price":1,"pnl":900},
	 {"tradingsymbol":"NIFTY26OCTXYZ","exchange":"NFO","product":"NRML","quantity":1,"multiplier":1,"average_price":1,"last_price":1,"pnl":1},
	 {"tradingsymbol":"USDINR26OCTFUT","exchange":"CDS","product":"NRML","quantity":1,"multiplier":1,"average_price":1,"last_price":1,"pnl":900}],"day":[]}`)
	s := BuildPositions(p, nil, 1, false)
	if len(s.Net) != 5 || s.NetTotalPnLPaise != 231100 || s.OutOfScopeCount != 3 {
		t.Fatalf("%+v", s)
	}
	want := []bool{false, true, true, false, true}
	for i, w := range want {
		if s.Net[i].OutOfScope != w {
			t.Fatalf("row %d (%s) outOfScope=%v want %v", i, s.Net[i].Symbol, s.Net[i].OutOfScope, w)
		}
	}
	b := BuildBook(mustJSON[kite.Orders](t, `[{"order_id":"1","status":"OPEN","exchange":"MCX","tradingsymbol":"GOLDM26OCTFUT"},{"order_id":"2","status":"OPEN","exchange":"NFO","tradingsymbol":"NIFTY2610822500PE"},{"order_id":"3","status":"OPEN","exchange":"NSE","tradingsymbol":"INFY"},{"order_id":"4","status":"COMPLETE","exchange":"NFO","tradingsymbol":"NIFTY26OCTFUT"},{"order_id":"5","status":"OPEN","tradingsymbol":"X"},{"order_id":"6","status":"OPEN","exchange":"BCD","tradingsymbol":"Y"}]`), 1, false)
	if len(b.Orders) != 6 || b.Counts.Open != 5 || b.Counts.Executed != 1 || b.OutOfScopeCount != 3 {
		t.Fatalf("%+v", b)
	}
	flag := map[string]bool{}
	for _, r := range b.Orders {
		flag[r.OrderID] = r.OutOfScope
	}
	if !flag["1"] || !flag["2"] || flag["3"] || flag["4"] || flag["5"] || !flag["6"] {
		t.Fatalf("flags %v", flag)
	}
	h := BuildHoldings(mustJSON[kite.Holdings](t, `[{"tradingsymbol":"GOLD","exchange":"MCX","quantity":1,"average_price":1,"last_price":2},{"tradingsymbol":"INFY","exchange":"NSE","quantity":1,"average_price":10,"last_price":11}]`), nil, 1, false)
	if h.Summary.Empty || len(h.Rows) != 2 || h.Summary.OutOfScopeCount != 1 || h.Summary.InvestmentPaise != 1100 || h.Summary.CurrentValuePaise != 1300 {
		t.Fatalf("%+v", h)
	}
}

func TestOutOfScope_FailsOpen(t *testing.T) {
	for _, c := range []struct {
		ex, sym string
		want    bool
	}{
		{"", "", false}, {"", "XCE", false}, {"NSE", "ABCCE", false}, {"BSE", "ABCPE", false},
		{"MCX", "GOLD", true}, {"mcx", "GOLD", true}, {"CDS", "USDINR", true}, {"BCD", "USDINR", true},
		{"NFO", "NIFTY26OCTFUT", false}, {"NFO", "NIFTY26OCTXYZ", false}, {"NFO", "NIFTY2610822500CE", true},
		{"BFO", "SENSEX2610822500PE", true}, {"BFO", "SENSEX26OCTFUT", false}, {"NFO", "", false},
	} {
		if got := outOfScope(c.ex, c.sym); got != c.want {
			t.Errorf("%q %q: got %v want %v", c.ex, c.sym, got, c.want)
		}
	}
}

// Minor fixes: a zero live LTP is ignored; zero close price gives zero day change.
func TestBuilders_ZeroLTPAndZeroClose(t *testing.T) {
	h := BuildHoldings(mustJSON[kite.Holdings](t, `[{"tradingsymbol":"A","exchange":"NSE","instrument_token":1,"quantity":10,"average_price":100,"last_price":110,"close_price":0}]`), map[uint32]float64{1: 0}, 1, false)
	if h.Rows[0].LTPPaise != 11000 || h.Rows[0].DayChangePaise != 0 || h.Summary.DayChangePaise != 0 {
		t.Fatalf("%+v", h.Rows[0])
	}
	p := mustJSON[kite.Positions](t, `{"net":[{"tradingsymbol":"P","exchange":"NSE","instrument_token":11,"product":"MIS","quantity":10,"multiplier":1,"average_price":50,"last_price":50,"pnl":120}],"day":[]}`)
	s := BuildPositions(p, map[uint32]float64{11: 0}, 1, false)
	if s.Net[0].LTPPaise != 5000 || s.Net[0].PnLPaise != 12000 {
		t.Fatalf("%+v", s.Net[0])
	}
}

// Hand-computed: holdings qty 3, avg 333.33, LTP 340.01, prev close 338.5.
// avg 33333p, ltp 34001p, close 33850p: invested 99999, current 102003, pnl 2004,
// netBp = 668/33333*1e4 = 200.4 -> 200; day = 3*(34001-33850)=453; dayBp = 151/33850*1e4 = 44.6 -> 45.
func TestBuildHoldings_PaiseRounding(t *testing.T) {
	r := BuildHoldings(mustJSON[kite.Holdings](t, `[{"tradingsymbol":"R","exchange":"NSE","quantity":3,"average_price":333.33,"last_price":340.01,"close_price":338.5}]`), nil, 1, true).Rows[0]
	if r.InvestedPaise != 99999 || r.CurrentPaise != 102003 || r.PnLPaise != 2004 || r.NetChangeBp != 200 || r.DayChangePaise != 453 || r.DayChangeBp != 45 {
		t.Fatalf("%+v", r)
	}
}

// Short position: qty -10, kite pnl 0 at last 100; LTP 99 -> pnl = -10*(99-100)*1 = +10 rupees.
func TestBuildPositions_ShortLive(t *testing.T) {
	p := mustJSON[kite.Positions](t, `{"net":[{"tradingsymbol":"S","exchange":"NSE","instrument_token":5,"product":"MIS","quantity":-10,"multiplier":1,"average_price":100,"last_price":100,"pnl":0}],"day":[]}`)
	s := BuildPositions(p, map[uint32]float64{5: 99}, 1, false)
	if s.Net[0].PnLPaise != 1000 || s.Net[0].NetChangeBp != -100 {
		t.Fatalf("%+v", s.Net[0])
	}
}
