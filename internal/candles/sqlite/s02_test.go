//go:build s02

package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/clock"
)

func envInt(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil && v > 0 {
		return v
	}
	return def
}

// TestS02 is spike S0-2 (ARCHITECTURE §12.1): full FR-DATA-002 AC4 load on the reference machine.
//
//	S02_INSTRUMENTS=500 S02_DAYS=500 S02_DIR=/path/with/10GB go test -tags s02,webkit2_41,noorders \
//	  -run TestS02 -timeout 6h -v ./internal/candles/sqlite/
func TestS02(t *testing.T) {
	n, days := envInt("S02_INSTRUMENTS", 500), envInt("S02_DAYS", 500)
	if days < 250 {
		t.Fatal("S02_DAYS must be ≥ 250: the probes read days from March to August 2025")
	}
	dir := os.Getenv("S02_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	path := filepath.Join(dir, "s02-candles.db")
	_ = os.Remove(path)
	cal, _ := calendar.NewDefault()
	s, err := Open(context.Background(), path, Options{Clock: clock.NewFake(time.Now()), Calendar: cal})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, el := loadMinutes(t, s, n, days)
	t.Logf("S0-2 ingest: %d rows in %v = %.0f rows/s (gate ≥ 100000)", rows, el.Round(time.Second), float64(rows)/el.Seconds())

	var reads, misses []time.Duration
	for i := 0; i < 5; i++ { // 5 × 200 probes for stable percentiles
		r, m := measureQueries(t, s, n, 200)
		reads, misses = append(reads, r), append(misses, m)
	}
	sort.Slice(reads, func(i, j int) bool { return reads[i] < reads[j] })
	sort.Slice(misses, func(i, j int) bool { return misses[i] < misses[j] })
	t.Logf("S0-2 instrument-day read: worst-of-200 median %v, max %v (gate < 50ms)", reads[2], reads[4])
	t.Logf("S0-2 missing-ranges: worst-of-200 median %v, max %v (gate < 5ms)", misses[2], misses[4])

	st, err := s.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(path)
	t.Logf("S0-2 size: candles_1m %d bytes for %d rows = %.1f B/row; file %d bytes",
		st.Intervals[0].Bytes, st.Intervals[0].Rows, float64(st.Intervals[0].Bytes)/float64(st.Intervals[0].Rows), fi.Size())
	if reads[4] >= 50*time.Millisecond || misses[4] >= 5*time.Millisecond || float64(rows)/el.Seconds() < 100000 {
		t.Errorf("S0-2 gate missed: switch to the colfile fallback (§4.3)")
	}
}
