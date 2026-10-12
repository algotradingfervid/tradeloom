//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length.

func bopOracleIn(s series) oracle.Inputs {
	return oracle.Inputs{Open: s.O, High: s.H, Low: s.L, Close: s.C}
}

func TestBOPOracle(t *testing.T) {
	lb := BOPLookback()
	if c := mustLookback(t, "BOP"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := BOP(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "BOP", bopOracleIn(s))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("BOP ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

// TestBOPOracleMalformed feeds bars violating low <= high (and NaN), where
// the exact <= 0 divisor test matters.
func TestBOPOracleMalformed(t *testing.T) {
	nan := math.NaN()
	s := series{
		Name: "malformed",
		O:    []float64{1, 1, 1, 1e-300, 5, nan, 1, 1},
		H:    []float64{2, 1, 0, 2e-300, nan, 6, 3, 1e-320},
		L:    []float64{0, 1, 1, 0, 4, 4, nan, 0},
		C:    []float64{2, 1, 0, 2e-300, 6, 5, 2, 1},
	}
	got, err := BOP(s.O, s.H, s.L, s.C)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "BOP", bopOracleIn(s))
	if r.RetCode != oracle.Success {
		t.Fatalf("C retcode %d", r.RetCode)
	}
	assertReal(t, "BOP malformed", got, r.AlignedReal(0, len(s.C)), relTol)
}

func TestBOPOracleRanges(t *testing.T) {
	s := fixtures(120)[11]
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {118, 119}} {
		out := make([]float64, len(s.C))
		beg, nb, err := bopRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "BOP", bopOracleIn(s), rg[0], rg[1])
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertReal(t, lbl("BOP range ", rg), out[:nb], r.Outputs[0].Real, relTol)
	}
}

// BOP has no optional inputs; the only C TA_BAD_PARAM paths are null
// buffers, which Go expresses as a length mismatch (checked in bop_test.go).
// The index errors are compared here.
func TestBOPOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "BOP", bopOracleIn(s), rg[0], rg[1])
		_, _, err := bopRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]float64, 20))
		if (r.RetCode == oracle.Success) != (err == nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}
