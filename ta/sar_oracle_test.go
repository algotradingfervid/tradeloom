//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

// sarOracleParams covers defaults, zero, acceleration > maximum (clamped),
// equal values, large steps and the range limits.
var sarOracleParams = [][2]float64{
	{0.02, 0.2}, {0, 0}, {0, 0.2}, {0.02, 0}, {0.01, 0.4}, {0.2, 0.2},
	{0.5, 0.1}, {1, 1}, {0.0001, 0.0005}, {2, 5}, {taRealMax, taRealMax}, {0.02, taRealMax},
}

func sarOracleCompare(t *testing.T, tag string) {
	t.Helper()
	for _, p := range sarOracleParams {
		lb := SARLookback(p[0], p[1])
		if c := mustLookback(t, "SAR", p[0], p[1]); lb != c {
			t.Fatalf("%slookback p=%v: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := SAR(s.H, s.L, p[0], p[1])
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "SAR", s.ohlcv(), p[0], p[1])
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl(tag, "SAR ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestSAROracle(t *testing.T) {
	sarOracleCompare(t, "")
}

// SAR calls MINUS_DM with period 1, whose lookback ignores the MINUS_DM
// unstable period; verify that a non-zero setting changes nothing in either.
func TestSAROracleUnstable(t *testing.T) {
	defer func() {
		SetAllUnstablePeriods(0)
		oracle.ResetUnstablePeriods()
	}()
	for _, u := range []int{1, 5} {
		if err := SetUnstablePeriod(UnstMinusDM, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstMinusDM), u)
		sarOracleCompare(t, lbl("unstable=", u, " "))
	}
}

func TestSAROracleBadParams(t *testing.T) {
	s := fixtures(40)[0]
	// TA_REAL_DEFAULT (-4e37) is deliberately absent: C substitutes the
	// default for it, while Go has no sentinel and rejects it.
	cases := [][2]float64{
		{-1e-12, 0.2}, {-0.02, 0.2}, {math.NaN(), 0.2}, {math.Inf(1), 0.2}, {math.Inf(-1), 0.2}, {3.0000001e37, 0.2},
		{0.02, -1e-12}, {0.02, -0.2}, {0.02, math.NaN()}, {0.02, math.Inf(1)}, {0.02, 3.0000001e37},
		{0, 0}, {0.02, 0.2}, {taRealMax, taRealMax}, {taRealMax, 0},
	}
	for _, c := range cases {
		r := mustCall(t, "SAR", s.ohlcv(), c[0], c[1])
		_, err := SAR(s.H, s.L, c[0], c[1])
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%v: C retcode %d, Go err %v", c, r.RetCode, err)
		}
		if lb, clb := SARLookback(c[0], c[1]), mustLookback(t, "SAR", c[0], c[1]); lb != clb {
			t.Errorf("lookback p=%v: go %d c %d", c, lb, clb)
		}
	}
}

func TestSAROracleRanges(t *testing.T) {
	for _, fi := range []int{1, 9, 11} {
		s := fixtures(120)[fi]
		for _, rg := range [][2]int{{0, 0}, {0, 1}, {1, 1}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {118, 119}} {
			for _, p := range [][2]float64{{0.02, 0.2}, {0.5, 0.1}, {0, 0}} {
				out := make([]float64, 120)
				beg, nb, err := sarRange(rg[0], rg[1], s.H, s.L, p[0], p[1], out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "SAR", s.ohlcv(), rg[0], rg[1], p[0], p[1])
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%v: go (%d,%d) c (%d,%d)", s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("SAR range ", s.Name, " ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}
