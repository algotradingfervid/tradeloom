package sqlite

import (
	"context"
	"errors"
	"testing"

	"tradeloom/internal/candles"
)

func TestPurgeRefusesReferencedRunsUntilConfirmed(t *testing.T) { // FR-DATA-002 AC3
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	old := ist(2024, 3, 4, 9, 15)
	cs := []candles.Candle{minute(old, 0, 100, true), minute(ist(2026, 10, 9, 9, 15), 0, 300, true)}
	if err := s.UpsertCovered(ctx, 5, candles.Minute, candles.Range{From: old, To: ist(2026, 10, 10, 0, 0)}, cs, candles.SourceHistorical); err != nil {
		t.Fatal(err)
	}
	if err := s.AddRunRef(ctx, "R", 5, candles.Minute, candles.Range{From: old, To: ist(2024, 3, 5, 0, 0)}); err != nil {
		t.Fatal(err)
	}
	rule := candles.PurgeRule{Interval: candles.Minute, Before: ist(2024, 10, 12, 0, 0)}
	err := s.Purge(ctx, rule, nil)
	var re *candles.RunsReferencedError
	if !errors.As(err, &re) || len(re.Runs) != 1 || re.Runs[0] != "R" {
		t.Fatalf("err = %v, want RunsReferencedError{R}", err)
	}
	if got, _ := s.Read(ctx, 5, candles.Minute, old, old.AddDate(0, 0, 1)); len(got) != 1 {
		t.Fatal("refused purge deleted data")
	}
	if err := s.Purge(ctx, rule, []string{"R"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Read(ctx, 5, candles.Minute, old, old.AddDate(0, 0, 1)); len(got) != 0 {
		t.Fatal("confirmed purge kept old data")
	}
	if got, _ := s.Read(ctx, 5, candles.Minute, ist(2026, 10, 9, 0, 0), ist(2026, 10, 10, 0, 0)); len(got) != 1 {
		t.Fatal("purge removed recent data")
	}
	if sum, _ := s.Checksum(ctx, 5, candles.Minute, 202403); sum != "" {
		t.Fatal("purged month kept its checksum")
	}
	miss, _ := s.Missing(ctx, 5, candles.Minute, ist(2024, 1, 1, 0, 0), ist(2024, 10, 1, 0, 0))
	if len(miss) != 1 || !miss[0].To.Equal(ist(2024, 10, 1, 0, 0)) {
		t.Fatalf("purged range still covered: %+v", miss)
	}
}

func TestClearAndStatsAndBudget(t *testing.T) {
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	var cs []candles.Candle
	for i := 0; i < 500; i++ {
		cs = append(cs, minute(ist(2026, 10, 9, 9, 15), i, 100, true))
	}
	if err := s.Upsert(ctx, 9, candles.Minute, cs, candles.SourceHistorical); err != nil {
		t.Fatal(err)
	}
	if err := s.SetBudget(ctx, 5<<30); err != nil {
		t.Fatal(err)
	}
	st, err := s.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Intervals[0].Interval != candles.Minute || st.Intervals[0].Rows != 500 || st.Intervals[0].Bytes == 0 || st.BudgetBytes != 5<<30 {
		t.Fatalf("stats %+v", st)
	}
	_ = s.AddRunRef(ctx, "R2", 9, candles.Day, candles.Range{From: cs[0].Start, To: cs[1].Start})
	if err := s.Clear(ctx, nil); err == nil {
		t.Fatal("Clear ignored a referenced run")
	}
	if err := s.Clear(ctx, []string{"R2"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Read(ctx, 9, candles.Minute, cs[0].Start, cs[499].Start.Add(1)); len(got) != 0 {
		t.Fatal("Clear left candles")
	}
}

func TestReadResampledWeekAndIncompleteTail(t *testing.T) {
	ctx := context.Background()
	s, fc := openTest(t, ist(2026, 10, 14, 12, 0)) // Wednesday of the current week
	var days []candles.Candle
	for d := 5; d <= 14; d++ {
		if dt := ist(2026, 10, d, 0, 0); dt.Weekday() >= 1 && dt.Weekday() <= 5 {
			days = append(days, candles.Candle{Start: dt, Open: float64(d), High: float64(d) + 1, Low: float64(d) - 1, Close: float64(d), Volume: 1, Complete: true})
		}
	}
	if err := s.Upsert(ctx, 3, candles.Day, days, candles.SourceHistorical); err != nil {
		t.Fatal(err)
	}
	w, err := s.ReadResampled(ctx, 3, candles.Week, ist(2026, 10, 1, 0, 0), ist(2026, 10, 20, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(w) != 2 || !w[0].Complete || w[1].Complete || w[0].Open != 5 || w[0].Close != 9 || w[0].Volume != 5 {
		t.Fatalf("weeks %+v", w)
	}
	fc.Set(ist(2026, 10, 19, 0, 0))
	w, _ = s.ReadResampled(ctx, 3, candles.Week, ist(2026, 10, 1, 0, 0), ist(2026, 10, 20, 0, 0))
	if !w[1].Complete {
		t.Fatal("week ending before now must be complete")
	}
}
