//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestSUMOracle(t *testing.T) {
	for _, p := range []int{2, 3, 5, 14, 30, 200, 1000} {
		lb := SUMLookback(p)
		if c := mustLookback(t, "SUM", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := SUM(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "SUM", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("SUM ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
	// Maximum period: lookback only plus one long-enough series.
	if lb, c := SUMLookback(100000), mustLookback(t, "SUM", 100000); lb != c {
		t.Fatalf("lookback p=100000: go %d c %d", lb, c)
	}
}

func TestSUMOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{1, 0, -5, 100001} {
		r := mustCall(t, "SUM", realIn(in), float64(p))
		_, err := SUM(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "SUM", float64(p)); c != SUMLookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, SUMLookback(p), c)
		}
	}
	for _, p := range []int{2, 100000} {
		r := mustCall(t, "SUM", realIn(in), float64(p))
		_, err := SUM(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
	}
}

func TestSUMOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.Real()
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {5, 119}} {
		for _, p := range []int{2, 10} {
			out := make([]float64, len(in))
			beg, nb, err := sumRange(rg[0], rg[1], in, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "SUM", realIn(in), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("SUM range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
