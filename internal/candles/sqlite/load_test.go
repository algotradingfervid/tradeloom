package sqlite

import (
	"context"
	"testing"
	"time"

	"tradeloom/internal/candles"
)

// loadMinutes ingests instruments × tradingDays × 375 one-minute candles the way backfill does:
// one UpsertCovered per (token, 60-day window). It returns rows written and the elapsed time.
func loadMinutes(tb testing.TB, s *Store, instruments, tradingDays int) (int, time.Duration) {
	tb.Helper()
	ctx := context.Background()
	start := time.Now()
	rows := 0
	first := ist(2024, 10, 14, 0, 0)
	for tok := 1; tok <= instruments; tok++ {
		d := first
		done := 0
		for done < tradingDays {
			winStart := d
			var batch []candles.Candle
			for len(batch) < 60*375 && done < tradingDays {
				if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
					open := d.Add(9*time.Hour + 15*time.Minute)
					for m := 0; m < 375; m++ {
						p := 1000 + float64((tok*7+m)%300)/20
						batch = append(batch, candles.Candle{Start: open.Add(time.Duration(m) * time.Minute), Open: p, High: p + 0.5, Low: p - 0.5, Close: p + 0.05, Volume: int64(100 + m), Complete: true})
					}
					done++
				}
				d = d.AddDate(0, 0, 1)
			}
			if err := s.UpsertCovered(ctx, uint32(tok), candles.Minute, candles.Range{From: winStart, To: d}, batch, candles.SourceHistorical); err != nil {
				tb.Fatal(err)
			}
			rows += len(batch)
		}
	}
	return rows, time.Since(start)
}

// measureQueries returns the slowest instrument-day read and the slowest missing-ranges query
// over n probes spread across instruments and days.
func measureQueries(tb testing.TB, s *Store, instruments, n int) (time.Duration, time.Duration) {
	tb.Helper()
	ctx := context.Background()
	var worstRead, worstMissing time.Duration
	for i := 0; i < n; i++ {
		tok := uint32(1 + i*7%instruments)
		day := ist(2025, 3, 3, 0, 0).AddDate(0, 0, i%5+7*(i%20))
		t0 := time.Now()
		cs, err := s.Read(ctx, tok, candles.Minute, day, day.AddDate(0, 0, 1))
		if err != nil || len(cs) != 375 {
			tb.Fatalf("read %d rows err %v (day %v tok %d)", len(cs), err, day, tok)
		}
		if d := time.Since(t0); d > worstRead {
			worstRead = d
		}
		t0 = time.Now()
		if _, err := s.Missing(ctx, tok, candles.Minute, ist(2015, 1, 1, 0, 0), ist(2026, 10, 12, 0, 0)); err != nil {
			tb.Fatal(err)
		}
		if d := time.Since(t0); d > worstMissing {
			worstMissing = d
		}
	}
	return worstRead, worstMissing
}

// TestPerf_StoreQueriesAtCIScale re-asserts FR-DATA-002 AC4 at 10× tolerance on a CI-sized load
// (10 instruments × 250 days). The full-scale numbers come from the S0-2 harness.
func TestPerf_StoreQueriesAtCIScale(t *testing.T) {
	if testing.Short() {
		t.Skip("perf test")
	}
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	rows, el := loadMinutes(t, s, 10, 250)
	t.Logf("ingested %d rows in %v (%.0f rows/s)", rows, el, float64(rows)/el.Seconds())
	read, missing := measureQueries(t, s, 10, 200)
	t.Logf("worst instrument-day read %v, worst missing-ranges %v", read, missing)
	if read > 500*time.Millisecond {
		t.Fatalf("instrument-day read %v > 500 ms (10× the 50 ms budget)", read)
	}
	if missing > 50*time.Millisecond {
		t.Fatalf("missing-ranges %v > 50 ms (10× the 5 ms budget)", missing)
	}
}
