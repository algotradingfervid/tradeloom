//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length. TRANGE has no optional inputs.

func TestTRANGEOracle(t *testing.T) {
	lb := TRANGELookback()
	if c := mustLookback(t, "TRANGE"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := TRANGE(s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "TRANGE", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("TRANGE ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

// TestTRANGEOracleNaN feeds NaN inputs to both sides; the comparison order
// must reproduce C's NaN behaviour exactly.
func TestTRANGEOracleNaN(t *testing.T) {
	s := fixtures(40)[0]
	nan := math.NaN()
	h := append([]float64(nil), s.H...)
	l := append([]float64(nil), s.L...)
	c := append([]float64(nil), s.C...)
	h[5], l[10], c[15], c[20], h[21], l[30], h[30] = nan, nan, nan, nan, nan, nan, nan
	got, err := TRANGE(h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	ns := s
	ns.H, ns.L, ns.C = h, l, c
	r := mustCall(t, "TRANGE", ns.ohlcv())
	if r.RetCode != oracle.Success {
		t.Fatalf("C retcode %d", r.RetCode)
	}
	assertReal(t, "TRANGE NaN", got, r.AlignedReal(0, 40), relTol)
}

func TestTRANGEOracleBadParams(t *testing.T) {
	// TRANGE has no optional inputs: the only failures are index errors,
	// which C reports as out-of-range codes, not BadParam.
	s := fixtures(20)[0]
	r := mustCall(t, "TRANGE", s.ohlcv())
	_, err := TRANGE(s.H, s.L, s.C)
	if (r.RetCode == oracle.BadParam) != (err != nil) {
		t.Errorf("C retcode %d, Go err %v", r.RetCode, err)
	}
}

func TestTRANGEOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	ranges := [][2]int{{0, 0}, {0, 1}, {1, 1}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 9}, {2, 2}, {50, 119}}
	for _, rg := range ranges {
		out := make([]float64, 120)
		beg, nb, err := trangeRange(rg[0], rg[1], s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "TRANGE", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertReal(t, lbl("TRANGE range ", rg), out[:nb], r.Outputs[0].Real, relTol)
	}
}
