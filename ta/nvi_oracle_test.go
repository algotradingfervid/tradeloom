//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length. NVI has no optional inputs.

func TestNVIOracle(t *testing.T) {
	lb := NVILookback()
	if c := mustLookback(t, "NVI"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := NVI(s.C, s.V)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "NVI", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("NVI ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

// Degenerate inputs exercising the zero-close and non-finite guards.
func TestNVIOracleDegenerate(t *testing.T) {
	nan := math.NaN()
	cases := []struct{ c, v []float64 }{
		{[]float64{0, 5, 10, 0, 3}, []float64{10, 5, 1, 0, -1}},
		{[]float64{1e-300, 1e300, 2e300, 1e308, 1}, []float64{5, 4, 3, 2, 1}},
		{[]float64{10, nan, 10, 12, 11}, []float64{5, 4, 3, 2, 1}},
		{[]float64{10, 11, 12, 13}, []float64{nan, 1, nan, 0}},
		{[]float64{-5, -6, 4, -2}, []float64{4, 3, 2, 1}},
	}
	for i, tc := range cases {
		got, err := NVI(tc.c, tc.v)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "NVI", oracle.Inputs{Close: tc.c, Volume: tc.v})
		if r.RetCode != oracle.Success {
			t.Fatalf("case %d: C retcode %d", i, r.RetCode)
		}
		assertReal(t, lbl("NVI degenerate ", i), got, r.AlignedReal(0, len(tc.c)), relTol)
	}
}

// NVI has no optional inputs, so C can only reject the index range; Go must
// reject the same ranges.
func TestNVIOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	out := make([]float64, 20)
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}, {0, 19}} {
		r := mustCallRange(t, "NVI", s.ohlcv(), rg[0], rg[1])
		_, _, err := nviRange(rg[0], rg[1], s.C, s.V, out)
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestNVIOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[4], fixtures(120)[11], fixtures(120)[13]} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {1, 1}, {50, 119}} {
			out := make([]float64, 120)
			beg, nb, err := nviRange(rg[0], rg[1], s.C, s.V, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "NVI", s.ohlcv(), rg[0], rg[1])
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("NVI range ", s.Name, " ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
