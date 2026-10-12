//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length. CUMSUM has no optional inputs.

func TestCUMSUMOracle(t *testing.T) {
	lb := CUMSUMLookback()
	if c := mustLookback(t, "CUMSUM"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CUMSUM(s.Real())
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CUMSUM", realIn(s.Real()))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("CUMSUM ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

// Same left-to-right accumulation as C, so special values and cancellation
// must match bit for bit.
func TestCUMSUMOracleSpecial(t *testing.T) {
	nan := math.NaN()
	for k, in := range [][]float64{
		{1e16, 1, 1, -1e16, 0.1, 0.2, 0.3},
		{math.MaxFloat64, math.MaxFloat64, -math.MaxFloat64},
		{1, math.Inf(1), 2, math.Inf(-1), 3},
		{1, nan, 2},
		{5e-324, 5e-324, -1e-300, math.Copysign(0, -1)},
	} {
		got, err := CUMSUM(in)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CUMSUM", realIn(in))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		want := r.AlignedReal(0, len(in))
		for i := range in {
			if math.Float64bits(got[i]) != math.Float64bits(want[i]) && !(math.IsNaN(got[i]) && math.IsNaN(want[i])) {
				t.Errorf("special %d [%d]: go %g c %g", k, i, got[i], want[i])
			}
		}
	}
}

// CUMSUM has no optional inputs, so C can only reject the index range; Go
// must reject the same ranges.
func TestCUMSUMOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}, {3, 2}} {
		r := mustCallRange(t, "CUMSUM", realIn(in), rg[0], rg[1])
		_, _, err := cumsumRange(rg[0], rg[1], in, make([]float64, len(in)))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCUMSUMOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[13], fixtures(120)[4]} {
		in := s.Real()
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {118, 119}} {
			out := make([]float64, len(in))
			beg, nb, err := cumsumRange(rg[0], rg[1], in, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CUMSUM", realIn(in), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("CUMSUM range ", s.Name, " ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
