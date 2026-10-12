//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestROCR100Oracle(t *testing.T) {
	periods := []int{1, 2, 3, 5, 10, 14, 30, 200, 999, 1000, 100000}
	for _, p := range periods {
		lb := ROCR100Lookback(p)
		if c := mustLookback(t, "ROCR100", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := ROCR100(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "ROCR100", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("ROCR100 ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

// Zero previous prices take C's tempReal == 0 branch.
func TestROCR100OracleZeros(t *testing.T) {
	in := []float64{0, 1, 0, -2, 3, 0, 0, 5, -0.0, 4, 1e-300, 7}
	for _, p := range []int{1, 2, 3} {
		got, err := ROCR100(in, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "ROCR100", realIn(in), float64(p))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		assertReal(t, lbl("ROCR100 zeros p=", p), got, r.AlignedReal(0, len(in)), relTol)
	}
}

func TestROCR100OracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{0, -5, 100001, 1, 100000} {
		r := mustCall(t, "ROCR100", realIn(in), float64(p))
		_, err := ROCR100(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "ROCR100", float64(p)); c != ROCR100Lookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, ROCR100Lookback(p), c)
		}
	}
}

func TestROCR100OracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.Real()
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 9}, {0, 10}, {10, 10}, {9, 11}} {
		for _, p := range []int{1, 10} {
			out := make([]float64, len(in))
			beg, nb, err := rocr100Range(rg[0], rg[1], in, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "ROCR100", realIn(in), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("ROCR100 range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
