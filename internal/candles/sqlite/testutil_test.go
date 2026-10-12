package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/clock"
)

func ist(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, clock.IST)
}

func openTest(t *testing.T, now time.Time) (*Store, *clock.Fake) {
	t.Helper()
	cal, err := calendar.NewDefault()
	if err != nil {
		t.Fatal(err)
	}
	fc := clock.NewFake(now)
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "candles.db"), Options{Clock: fc, Calendar: cal})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, fc
}

func minute(start time.Time, i int, p float64, complete bool) candles.Candle {
	return candles.Candle{Start: start.Add(time.Duration(i) * time.Minute), Open: p, High: p + 1, Low: p - 1, Close: p + 0.05, Volume: int64(100 + i), Complete: complete}
}
