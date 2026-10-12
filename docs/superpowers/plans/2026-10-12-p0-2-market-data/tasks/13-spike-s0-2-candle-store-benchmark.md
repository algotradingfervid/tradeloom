> **P0-2 Time & Market Data Implementation Plan — Task 13.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 13: Spike S0-2: candle store benchmark and CI-scale AC4 test

**Files:**
- Test: `internal/candles/sqlite/load_test.go`
- Test: `internal/candles/sqlite/s02_test.go`
- Create: `docs/architecture/spikes/S0-2-candle-store.md` (generated in this task)

**Interfaces:**
- Consumes: Task 11: `Store.UpsertCovered`, `Read`, `Missing`, `Stats`, `Open`, `openTest`.
- Produces: test harnesses and the spike record only.

ARCHITECTURE §12.1 S0-2 decides OQ-41 against FR-DATA-002 AC4 (< 50 ms instrument-day read and < 5 ms missing-ranges with 500 instruments × 2 years × 1-minute ≈ 94 M rows, ingest ≥ 100 k rows/s). CI re-asserts the read budgets at 10× tolerance on 10 instruments × 250 days; the full load runs on the reference machine with build tag `s02`.

- [ ] **Step 1: Write the load helpers and the CI-scale test**

`internal/candles/sqlite/load_test.go`:

```go
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
```

- [ ] **Step 2: Run the CI-scale test**

Run: `GOWORK=off go test -tags webkit2_41,noorders -run TestPerf -v ./internal/candles/sqlite/`
Expected: `--- PASS: TestPerf_StoreQueriesAtCIScale`, logging about 937 500 rows ingested and worst read / missing-range times well under 500 ms / 50 ms (verification run on an Apple M-series laptop: 452 798 rows/s, read 0.48 ms, missing-ranges 0.05 ms).

- [ ] **Step 3: Write the full-scale S0-2 harness (build tag `s02`)**

`internal/candles/sqlite/s02_test.go`:

```go
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
```

- [ ] **Step 4: Run S0-2 on the reference machine**

The reference machine is the one in REQUIREMENTS §5 (x86_64, 4 cores/8 threads ≥ 2.5 GHz, 16 GB RAM, NVMe, Ubuntu 24.04). The run needs about 6 GB free in `S02_DIR`.

Run:
```bash
mkdir -p "$HOME/s02" && \
S02_INSTRUMENTS=500 S02_DAYS=500 S02_DIR="$HOME/s02" GOWORK=off go test -tags s02,webkit2_41,noorders \
  -run TestS02 -timeout 6h -v ./internal/candles/sqlite/ 2>&1 | tee /tmp/s02.log
```
Expected: four lines starting `S0-2 ingest:`, `S0-2 instrument-day read:`, `S0-2 missing-ranges:`, `S0-2 size:` and a final `ok` (gates met) or `FAIL` with "S0-2 gate missed".

- [ ] **Step 5: Write the spike record from the run**

Run:
```bash
mkdir -p docs/architecture/spikes
{
  printf '# S0-2 Candle store benchmark\n\n'
  printf 'Decides OQ-41 (ARCHITECTURE §4.3, §12.1) against FR-DATA-002 AC4: one instrument-day of 1-minute candles in < 50 ms and\n'
  printf '"missing ranges for token T between A and B" in < 5 ms with 500 instruments x 2 years loaded; ingest >= 100 000 rows/s.\n\n'
  printf '## Machine\n\n```\n'; uname -srm; (lscpu 2>/dev/null | grep -E "Model name|^CPU\(s\)") ; (free -g 2>/dev/null | head -2); printf '```\n\n'
  printf '## Procedure\n\n`S02_INSTRUMENTS=500 S02_DAYS=500 go test -tags s02,webkit2_41,noorders -run TestS02 ./internal/candles/sqlite/` —\n'
  printf 'loads 500 x 500 trading days x 375 one-minute candles through `UpsertCovered` in 60-day windows (the backfill path),\n'
  printf 'then 5 x 200 random instrument-day reads and missing-range queries; worst of each 200, median and max of the 5 runs.\n\n'
  printf '## Measurements\n\n'
  grep -E 'S0-2 (ingest|instrument-day|missing-ranges|size):' /tmp/s02.log | sed -E 's/^[[:space:]]*s02_test\.go:[0-9]+: /- /'
  printf '\n## Decision\n\n'
  if grep -qE '^ok[[:space:]]' /tmp/s02.log; then
    printf 'All gates met: the SQLite layout of ARCHITECTURE §4.3 is confirmed for OQ-41; `candles/sqlite` is the `candles.Store` implementation.\n'
  else
    printf 'A gate was missed: OQ-41 switches to the `candles/colfile` fallback of ARCHITECTURE §4.3; a follow-up plan replaces the implementation behind `candles.Store`.\n'
  fi
} > docs/architecture/spikes/S0-2-candle-store.md
cat docs/architecture/spikes/S0-2-candle-store.md
```
Expected: the file lists the machine, the four measurement lines and one decision sentence.

- [ ] **Step 6: Commit**

```bash
git add internal/candles/sqlite/load_test.go \
  internal/candles/sqlite/s02_test.go \
  docs/architecture/spikes/S0-2-candle-store.md
git commit -m "$(cat <<'EOF'
test(candles/sqlite): S0-2 benchmark harness, CI-scale AC4 check and spike record

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
