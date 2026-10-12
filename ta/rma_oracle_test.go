//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func rmaOracleCompare(t *testing.T, prefix string, periods []int) {
	t.Helper()
	for _, p := range periods {
		lb := RMALookback(p)
		if c := mustLookback(t, "RMA", float64(p)); lb != c {
			t.Fatalf("%slookback p=%d: go %d c %d", prefix, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := RMA(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "RMA", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("%sC retcode %d", prefix, r.RetCode)
				}
				assertReal(t, lbl(prefix, "RMA ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestRMAOracle(t *testing.T) {
	rmaOracleCompare(t, "", []int{1, 2, 3, 5, 14, 30, 200, 999})
	// Max period: lookback only (the series would need 100000 bars).
	if lb, c := RMALookback(100000), mustLookback(t, "RMA", 100000); lb != c {
		t.Fatalf("lookback p=100000: go %d c %d", lb, c)
	}
}

func TestRMAOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{0, -5, 100001, 1, 100000} {
		r := mustCall(t, "RMA", realIn(in), float64(p))
		_, err := RMA(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "RMA", float64(p)); c != RMALookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, RMALookback(p), c)
		}
	}
}

func rmaOracleRanges(t *testing.T, prefix string, ranges [][2]int, periods []int) {
	t.Helper()
	s := fixtures(150)[1]
	in := s.Real()
	for _, rg := range ranges {
		for _, p := range periods {
			out := make([]float64, len(in))
			beg, nb, err := rmaRange(rg[0], rg[1], in, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "RMA", realIn(in), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%srange %v p=%d: go (%d,%d) c (%d,%d)", prefix, rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl(prefix, "RMA range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}

func TestRMAOracleRanges(t *testing.T) {
	rmaOracleRanges(t, "",
		[][2]int{{0, 0}, {0, 149}, {3, 3}, {10, 50}, {149, 149}, {0, 8}, {9, 9}, {12, 13}, {100, 149}},
		[]int{1, 2, 10, 14})
}

func TestRMAOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 37} {
		SetUnstablePeriod(UnstRMA, u)
		oracle.SetUnstablePeriod(int(UnstRMA), u)
		rmaOracleCompare(t, lbl("unst=", u, " "), []int{1, 2, 5, 14, 30})
		rmaOracleRanges(t, lbl("unst=", u, " "),
			[][2]int{{0, 149}, {40, 40}, {0, 10}, {60, 149}, {149, 149}},
			[]int{1, 3, 14})
	}
	// Another function's unstable period must not leak into RMA.
	SetAllUnstablePeriods(0)
	oracle.ResetUnstablePeriods()
	SetUnstablePeriod(UnstEMA, 9)
	oracle.SetUnstablePeriod(int(UnstEMA), 9)
	rmaOracleCompare(t, "ema-unst ", []int{3, 14})
}
