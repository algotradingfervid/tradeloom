package calendar

import (
	"context"
	"testing"
)

func TestInferNonTradingDayWithoutNifty50Candle(t *testing.T) { // FR-DATA-001 AC5
	s := mustDefault(t)
	d := Date{2026, 10, 14} // Wednesday, absent from the list
	none := func(context.Context, Date) (bool, error) { return false, nil }

	if _, ok, _ := s.Infer(context.Background(), ist(2026, 10, 14, 15, 59), d, none); ok {
		t.Fatal("must not infer before 16:00")
	}
	e, ok, err := s.Infer(context.Background(), ist(2026, 10, 14, 16, 1), d, none)
	if err != nil || !ok || e.Type != DayInferredClosed || e.Source != "inferred" {
		t.Fatalf("infer = %+v %v %v", e, ok, err)
	}
	if s.IsTradingDay(NSE, d) || s.IsTradingDay(NFO, d) {
		t.Fatal("inferred day still trading")
	}
	l := s.LabelAt(NSE, ist(2026, 10, 14, 17, 0))
	if l.Tooltip != "Non-trading day (inferred)" {
		t.Fatalf("tooltip = %q", l.Tooltip)
	}
}

func TestInferLeavesListedAndCandleDaysAlone(t *testing.T) {
	s := mustDefault(t)
	called := false
	has := func(context.Context, Date) (bool, error) { called = true; return true, nil }
	if _, ok, _ := s.Infer(context.Background(), ist(2026, 10, 20, 17, 0), Date{2026, 10, 20}, has); ok || called {
		t.Fatal("listed holiday must not be checked")
	}
	if _, ok, _ := s.Infer(context.Background(), ist(2026, 10, 13, 17, 0), Date{2026, 10, 13}, has); ok {
		t.Fatal("a day with a NIFTY 50 candle is trading")
	}
	if !s.IsTradingDay(NSE, Date{2026, 10, 13}) {
		t.Fatal("13 Oct must stay trading")
	}
}
