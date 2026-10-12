//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func massiOracleCompare(t *testing.T, prefix string, params [][2]int) {
	t.Helper()
	for _, p := range params {
		lb := MASSILookback(p[0], p[1])
		if c := mustLookback(t, "MASSI", float64(p[0]), float64(p[1])); lb != c {
			t.Fatalf("%slookback p=%v: go %d c %d", prefix, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := MASSI(s.H, s.L, p[0], p[1])
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "MASSI", s.ohlcv(), float64(p[0]), float64(p[1]))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl(prefix, "MASSI ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestMASSIOracle(t *testing.T) {
	massiOracleCompare(t, "", [][2]int{{2, 2}, {2, 3}, {3, 2}, {9, 25}, {25, 9}, {5, 40}, {14, 14}, {50, 50}, {2, 300}})
}

func TestMASSIOracleMaxPeriod(t *testing.T) {
	for _, p := range [][2]int{{100000, 2}, {2, 100000}} {
		lb := MASSILookback(p[0], p[1])
		if c := mustLookback(t, "MASSI", float64(p[0]), float64(p[1])); lb != c {
			t.Fatalf("lookback p=%v: go %d c %d", p, lb, c)
		}
		for _, n := range []int{lb, lb + 1, lb + 50} {
			s := fixtures(n)[0]
			got, err := MASSI(s.H, s.L, p[0], p[1])
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "MASSI", s.ohlcv(), float64(p[0]), float64(p[1]))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("MASSI max p=", p, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

func TestMASSIOracleBadParams(t *testing.T) {
	s := fixtures(40)[0]
	for _, p := range [][2]int{{0, 25}, {1, 25}, {-5, 25}, {100001, 25}, {9, 0}, {9, 1}, {9, -3}, {9, 100001}, {2, 2}, {100000, 100000}} {
		r := mustCall(t, "MASSI", s.ohlcv(), float64(p[0]), float64(p[1]))
		_, err := MASSI(s.H, s.L, p[0], p[1])
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%v: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "MASSI", float64(p[0]), float64(p[1])); c != MASSILookback(p[0], p[1]) {
			t.Errorf("lookback p=%v: go %d c %d", p, MASSILookback(p[0], p[1]), c)
		}
	}
}

func massiOracleRanges(t *testing.T, prefix string, n int, ranges [][2]int, params [][2]int) {
	t.Helper()
	for _, s := range fixtures(n) {
		for _, rg := range ranges {
			for _, p := range params {
				out := make([]float64, n)
				beg, nb, err := massiRange(rg[0], rg[1], s.H, s.L, p[0], p[1], out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "MASSI", s.ohlcv(), rg[0], rg[1], float64(p[0]), float64(p[1]))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s%s range %v p=%v: go (%d,%d) c (%d,%d)", prefix, s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl(prefix, "MASSI range ", s.Name, rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

func TestMASSIOracleRanges(t *testing.T) {
	massiOracleRanges(t, "", 120,
		[][2]int{{0, 0}, {0, 119}, {3, 3}, {4, 4}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {40, 41}, {50, 119}},
		[][2]int{{2, 3}, {3, 5}, {9, 25}})
}

func TestMASSIOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 37} {
		SetUnstablePeriod(UnstEMA, u)
		oracle.SetUnstablePeriod(int(UnstEMA), u)
		massiOracleCompare(t, lbl("unst=", u, " "), [][2]int{{2, 2}, {3, 5}, {9, 25}})
		massiOracleRanges(t, lbl("unst=", u, " "), 150,
			[][2]int{{0, 149}, {40, 40}, {0, 10}, {60, 149}, {100, 120}},
			[][2]int{{2, 3}, {9, 25}})
	}
	// An unrelated unstable period must not shift MASSI.
	SetAllUnstablePeriods(0)
	oracle.ResetUnstablePeriods()
	SetUnstablePeriod(UnstRSI, 9)
	oracle.SetUnstablePeriod(int(UnstRSI), 9)
	massiOracleCompare(t, "rsi-unst ", [][2]int{{3, 5}, {9, 25}})
}
