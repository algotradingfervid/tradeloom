//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

var coppockOracleParams = [][3]int{
	{1, 1, 1}, {1, 11, 14}, {2, 1, 2}, {2, 2, 1}, {3, 5, 5},
	{10, 11, 14}, {10, 14, 11}, {14, 1, 30}, {51, 2, 3}, {60, 20, 5},
	{100, 50, 120},
}

func TestCOPPOCKOracle(t *testing.T) {
	for _, ps := range coppockOracleParams {
		w, r1, r2 := ps[0], ps[1], ps[2]
		lb := COPPOCKLookback(w, r1, r2)
		if c := mustLookback(t, "COPPOCK", float64(w), float64(r1), float64(r2)); lb != c {
			t.Fatalf("lookback p=%v: go %d c %d", ps, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := COPPOCK(s.Real(), w, r1, r2)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "COPPOCK", realIn(s.Real()), float64(w), float64(r1), float64(r2))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("COPPOCK ", s.Name, " n=", n, " p=", ps), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestCOPPOCKOracleLookbackExtremes(t *testing.T) {
	for _, ps := range [][3]int{{100000, 100000, 100000}, {1, 100000, 1}, {100000, 1, 1}, {0, 11, 14}, {10, 100001, 14}, {10, 11, -1}} {
		lb := COPPOCKLookback(ps[0], ps[1], ps[2])
		if c := mustLookback(t, "COPPOCK", float64(ps[0]), float64(ps[1]), float64(ps[2])); lb != c {
			t.Errorf("lookback p=%v: go %d c %d", ps, lb, c)
		}
	}
}

func TestCOPPOCKOracleBadParams(t *testing.T) {
	in := fixtures(40)[0].Real()
	for _, ps := range [][3]int{
		{0, 11, 14}, {-5, 11, 14}, {100001, 11, 14},
		{10, 0, 14}, {10, -5, 14}, {10, 100001, 14},
		{10, 11, 0}, {10, 11, -5}, {10, 11, 100001},
		{1, 1, 1}, {100000, 100000, 100000},
	} {
		r := mustCall(t, "COPPOCK", realIn(in), float64(ps[0]), float64(ps[1]), float64(ps[2]))
		_, err := COPPOCK(in, ps[0], ps[1], ps[2])
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%v: C retcode %d, Go err %v", ps, r.RetCode, err)
		}
	}
}

func TestCOPPOCKOracleRanges(t *testing.T) {
	s := fixtures(300)[1]
	in := s.Real()
	for _, rg := range [][2]int{{0, 0}, {0, 299}, {3, 3}, {10, 50}, {299, 299}, {0, 22}, {23, 23}, {22, 24}, {30, 250}, {100, 299}} {
		for _, ps := range [][3]int{{1, 3, 2}, {10, 11, 14}, {4, 6, 2}} {
			out := make([]float64, len(in))
			beg, nb, err := coppockRange(rg[0], rg[1], in, ps[0], ps[1], ps[2], out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "COPPOCK", realIn(in), rg[0], rg[1], float64(ps[0]), float64(ps[1]), float64(ps[2]))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%v: go (%d,%d) c (%d,%d)", rg, ps, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("COPPOCK range ", rg, " p=", ps), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
