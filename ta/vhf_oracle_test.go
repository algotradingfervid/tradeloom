//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestVHFOracle(t *testing.T) {
	periods := []int{2, 3, 5, 14, 28, 56, 200}
	for _, p := range periods {
		lb := VHFLookback(p)
		if c := mustLookback(t, "VHF", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := VHF(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "VHF", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("VHF ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestVHFOracleMaxPeriod(t *testing.T) {
	p := 100000
	if c := mustLookback(t, "VHF", float64(p)); c != VHFLookback(p) {
		t.Fatalf("lookback max: go %d c %d", VHFLookback(p), c)
	}
	for _, n := range []int{p, p + 1, p + 3} {
		s := fixtures(n)[0]
		got, err := VHF(s.Real(), p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "VHF", realIn(s.Real()), float64(p))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		assertReal(t, lbl("VHF max n=", n), got, r.AlignedReal(0, n), relTol)
	}
}

func TestVHFOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{1, 0, -5, 100001, 2, 100000} {
		r := mustCall(t, "VHF", realIn(in), float64(p))
		_, err := VHF(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if lb := mustLookback(t, "VHF", float64(p)); lb != VHFLookback(p) {
			t.Errorf("p=%d: lookback go %d c %d", p, VHFLookback(p), lb)
		}
	}
}

func TestVHFOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.Real()
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 9}, {10, 10}, {9, 11}} {
		for _, p := range []int{2, 10} {
			out := make([]float64, len(in))
			beg, nb, err := vhfRange(rg[0], rg[1], in, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "VHF", realIn(in), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("VHF range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
