//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters (odd and even periods take different
// code paths in C).

func TestTRIMAOracle(t *testing.T) {
	periods := []int{1, 2, 3, 4, 5, 6, 14, 30, 31, 200, 201, 99999, 100000}
	for _, p := range periods {
		lb := TRIMALookback(p)
		if c := mustLookback(t, "TRIMA", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := TRIMA(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "TRIMA", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("TRIMA ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestTRIMAOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	// math.MinInt32 is TA_INTEGER_DEFAULT, which C replaces with 30; Go has no
	// sentinel, so it is not included here.
	for _, p := range []int{0, -1, -5, 100001, 1 << 30, 1, 2, 100000} {
		r := mustCall(t, "TRIMA", realIn(in), float64(p))
		_, err := TRIMA(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "TRIMA", float64(p)); c != TRIMALookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, TRIMALookback(p), c)
		}
	}
}

func TestTRIMAOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.Real()
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {0, 9}, {50, 119}} {
		for _, p := range []int{1, 2, 9, 10} {
			out := make([]float64, len(in))
			beg, nb, err := trimaRange(rg[0], rg[1], in, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "TRIMA", realIn(in), rg[0], rg[1], float64(p))
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v p=%d: C retcode %d", rg, p, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("TRIMA range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
