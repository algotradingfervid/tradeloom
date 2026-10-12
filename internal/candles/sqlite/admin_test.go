package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

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
	if err := s.AddRunRef(ctx, "R2", 9, candles.Day, candles.Range{From: cs[0].Start, To: cs[1].Start}); err != nil {
		t.Fatal(err)
	}
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

// weekdays returns complete day candles for each weekday in [from, to] with Open = Close = day of
// month and Volume 1.
func weekdays(from, to time.Time) []candles.Candle {
	var out []candles.Candle
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if d.Weekday() >= time.Monday && d.Weekday() <= time.Friday {
			v := float64(d.Day())
			out = append(out, candles.Candle{Start: d, Open: v, High: v + 1, Low: v - 1, Close: v, Volume: 1, Complete: true})
		}
	}
	return out
}

// Fix round 1, item 2: a range that starts or ends inside a period returns the whole period's
// bar, never a partial bar presented as complete.
func TestReadResampledReturnsWholePeriods(t *testing.T) {
	ctx := context.Background()
	s, fc := openTest(t, ist(2026, 10, 14, 12, 0))
	if err := s.Upsert(ctx, 3, candles.Day, weekdays(ist(2026, 9, 1, 0, 0), ist(2026, 10, 30, 0, 0)), candles.SourceHistorical); err != nil {
		t.Fatal(err)
	}
	week := func(from, to time.Time) []candles.Candle {
		t.Helper()
		w, err := s.ReadResampled(ctx, 3, candles.Week, from, to)
		if err != nil {
			t.Fatal(err)
		}
		return w
	}
	// from = Wednesday 7 Oct: the week of Monday 5 Oct, all five days.
	w := week(ist(2026, 10, 7, 0, 0), ist(2026, 10, 10, 0, 0))
	if len(w) != 1 || !w[0].Start.Equal(ist(2026, 10, 5, 0, 0)) || w[0].Open != 5 || w[0].Close != 9 || w[0].Volume != 5 || !w[0].Complete {
		t.Fatalf("from mid-week: %+v", w)
	}
	// to = Wednesday 7 Oct: still the whole week (Thursday and Friday included).
	w = week(ist(2026, 10, 5, 0, 0), ist(2026, 10, 7, 0, 0))
	if len(w) != 1 || w[0].Close != 9 || w[0].Volume != 5 {
		t.Fatalf("to mid-week: %+v", w)
	}
	// to exactly on a period start excludes that period.
	if w = week(ist(2026, 9, 28, 0, 0), ist(2026, 10, 5, 0, 0)); len(w) != 1 || !w[0].Start.Equal(ist(2026, 9, 28, 0, 0)) {
		t.Fatalf("to on a Monday: %+v", w)
	}

	fc.Set(ist(2026, 11, 2, 0, 0))
	m, err := s.ReadResampled(ctx, 3, candles.Month, ist(2026, 9, 15, 0, 0), ist(2026, 10, 10, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 || !m[0].Start.Equal(ist(2026, 9, 1, 0, 0)) || m[0].Open != 1 || m[0].Volume != 22 || m[1].Close != 30 || m[1].Volume != 22 || !m[0].Complete || !m[1].Complete {
		t.Fatalf("months %+v", m)
	}
}

// Fix round 1, item 2: a 2-hour bar asked for from inside its bucket is built from the whole
// session bucket (09:15–11:15 from 60-minute base candles).
func TestReadResampledIntradayWholeBuckets(t *testing.T) {
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	var hours []candles.Candle
	for i := 0; i < 7; i++ { // 09:15 … 15:15
		p := float64(100 + i)
		hours = append(hours, candles.Candle{Start: ist(2026, 10, 12, 9, 15).Add(time.Duration(i) * time.Hour), Open: p, High: p + 1, Low: p - 1, Close: p + 0.5, Volume: 10, Complete: true})
	}
	if err := s.Upsert(ctx, 4, candles.Minute60, hours, candles.SourceHistorical); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadResampled(ctx, 4, candles.Interval("120minute"), ist(2026, 10, 12, 10, 0), ist(2026, 10, 12, 12, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].Start.Equal(ist(2026, 10, 12, 9, 15)) || got[0].Open != 100 || got[0].Close != 101.5 || got[0].Volume != 20 ||
		!got[1].Start.Equal(ist(2026, 10, 12, 11, 15)) || got[1].Close != 103.5 || got[1].Volume != 20 || !got[0].Complete || !got[1].Complete {
		t.Fatalf("2h bars %+v", got)
	}
}

// Fix round 1, item 3: a run on a resampled interval blocks purging its stored base interval.
func TestRunOnResampledIntervalBlocksBasePurge(t *testing.T) {
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	old := candles.Range{From: ist(2024, 3, 4, 9, 15), To: ist(2024, 3, 5, 0, 0)}
	for _, c := range []struct {
		run  string
		used candles.Interval
		base candles.Interval
	}{
		{"W", candles.Week, candles.Day},
		{"M", candles.Month, candles.Day},
		{"H2", candles.Interval("120minute"), candles.Minute60},
		{"M2", candles.Interval("2minute"), candles.Minute},
	} {
		if err := s.AddRunRef(ctx, c.run, 5, c.used, old); err != nil {
			t.Fatal(err)
		}
		err := s.Purge(ctx, candles.PurgeRule{Interval: c.base, Before: ist(2025, 1, 1, 0, 0)}, nil)
		var re *candles.RunsReferencedError
		if !errors.As(err, &re) || !contains(re.Runs, c.run) {
			t.Fatalf("%s run did not block a %s purge: %v", c.used, c.base, err)
		}
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// Fix round 1, item 5: purge deletes per token in bounded chunks and ends with consistent
// coverage and checksums.
func TestPurgeInChunksAcrossTokens(t *testing.T) {
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	defer func(n int) { purgeChunk = n }(purgeChunk)
	purgeChunk = 7
	old := ist(2024, 3, 4, 9, 15)
	recent := ist(2026, 10, 9, 9, 15)
	for tok := uint32(1); tok <= 3; tok++ {
		var cs []candles.Candle
		for i := 0; i < 50; i++ {
			cs = append(cs, minute(old, i, 100, true), minute(recent, i, 300, true))
		}
		if err := s.UpsertCovered(ctx, tok, candles.Minute, candles.Range{From: old, To: ist(2026, 10, 10, 0, 0)}, cs, candles.SourceHistorical); err != nil {
			t.Fatal(err)
		}
	}
	cut := ist(2024, 10, 1, 0, 0)
	if err := s.Purge(ctx, candles.PurgeRule{Interval: candles.Minute, Before: ist(2024, 10, 12, 0, 0)}, nil); err != nil {
		t.Fatal(err)
	}
	for tok := uint32(1); tok <= 3; tok++ {
		if got, _ := s.Read(ctx, tok, candles.Minute, ist(2000, 1, 1, 0, 0), cut); len(got) != 0 {
			t.Fatalf("token %d kept %d old rows", tok, len(got))
		}
		if got, _ := s.Read(ctx, tok, candles.Minute, cut, ist(2027, 1, 1, 0, 0)); len(got) != 50 {
			t.Fatalf("token %d has %d recent rows, want 50", tok, len(got))
		}
		if sum, _ := s.Checksum(ctx, tok, candles.Minute, 202403); sum != "" {
			t.Fatalf("token %d kept the purged month's checksum", tok)
		}
		if sum, _ := s.Checksum(ctx, tok, candles.Minute, 202610); sum == "" {
			t.Fatalf("token %d lost the recent month's checksum", tok)
		}
		cov, _ := s.Coverage(ctx, tok, candles.Minute)
		if len(cov.Ranges) != 1 || !cov.Ranges[0].From.Equal(cut) {
			t.Fatalf("token %d coverage %+v", tok, cov.Ranges)
		}
	}
}
