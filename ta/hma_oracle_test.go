//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestHMAOracle(t *testing.T) {
	// 1 (copy), 2/3 (degenerate regime), 4 (smallest general), 20 (default),
	// 2703/2704 (C's stack ring vs heap ring boundary), 100000 (max).
	periods := []int{1, 2, 3, 4, 5, 8, 9, 14, 16, 20, 30, 200, 2703, 2704, 100000}
	for _, p := range periods {
		lb := HMALookback(p)
		if c := mustLookback(t, "HMA", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		lengths := edgeLengths(lb)
		if p >= 2703 {
			lengths = []int{1, 2, lb - 1, lb, lb + 1, lb + 2, lb + 10}
		}
		for _, n := range lengths {
			for _, s := range fixtures(n) {
				got, err := HMA(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "HMA", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("HMA ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

// Long series so all three 8*period re-anchors fire many times.
func TestHMAOracleLong(t *testing.T) {
	for _, p := range []int{2, 3, 4, 7, 16, 50} {
		for _, s := range fixtures(5000) {
			got, _ := HMA(s.Real(), p)
			r := mustCall(t, "HMA", realIn(s.Real()), float64(p))
			assertReal(t, lbl("HMA long ", s.Name, " p=", p), got, r.AlignedReal(0, 5000), relTol)
		}
	}
}

func TestHMAOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{0, -5, 1, 2, 100000, 100001} {
		r := mustCall(t, "HMA", realIn(in), float64(p))
		_, err := HMA(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if lb, c := HMALookback(p), mustLookback(t, "HMA", float64(p)); lb != c {
			t.Errorf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
}

func TestHMAOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.Real()
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {12, 12}, {13, 60}} {
		for _, p := range []int{1, 2, 3, 4, 10} {
			out := make([]float64, len(in))
			beg, nb, err := hmaRange(rg[0], rg[1], in, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "HMA", realIn(in), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("HMA range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
