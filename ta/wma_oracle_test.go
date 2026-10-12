//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestWMAOracle(t *testing.T) {
	periods := []int{1, 2, 3, 5, 14, 30, 200, 100000}
	for _, p := range periods {
		lb := WMALookback(p)
		if c := mustLookback(t, "WMA", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		lengths := edgeLengths(lb)
		if p == 100000 {
			lengths = []int{1, 2, lb - 1, lb, lb + 1, lb + 2, lb + 10}
		}
		for _, n := range lengths {
			for _, s := range fixtures(n) {
				got, err := WMA(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "WMA", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("WMA ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

// Long series so the 8*period re-anchor fires many times.
func TestWMAOracleLong(t *testing.T) {
	for _, p := range []int{2, 7, 50} {
		for _, s := range fixtures(5000) {
			got, _ := WMA(s.Real(), p)
			r := mustCall(t, "WMA", realIn(s.Real()), float64(p))
			assertReal(t, lbl("WMA long ", s.Name, " p=", p), got, r.AlignedReal(0, 5000), relTol)
		}
	}
}

func TestWMAOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{0, -5, 1, 100000, 100001} {
		r := mustCall(t, "WMA", realIn(in), float64(p))
		_, err := WMA(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "WMA", float64(p)); c != WMALookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, WMALookback(p), c)
		}
	}
}

func TestWMAOracleRanges(t *testing.T) {
	s := fixtures(400)[1]
	in := s.Real()
	for _, rg := range [][2]int{{0, 0}, {0, 399}, {3, 3}, {10, 50}, {399, 399}, {0, 8}, {9, 9}, {5, 300}, {100, 399}} {
		for _, p := range []int{1, 2, 10} {
			out := make([]float64, len(in))
			beg, nb, err := wmaRange(rg[0], rg[1], in, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "WMA", realIn(in), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("WMA range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
