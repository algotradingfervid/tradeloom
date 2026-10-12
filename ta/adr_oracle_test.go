//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestADROracle(t *testing.T) {
	for _, p := range []int{1, 2, 3, 5, 14, 30, 200} {
		lb := ADRLookback(p)
		if c := mustLookback(t, "ADR", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := ADR(s.H, s.L, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "ADR", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("ADR ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestADROracleMaxPeriod(t *testing.T) {
	p := 100000
	lb := ADRLookback(p)
	if c := mustLookback(t, "ADR", float64(p)); lb != c {
		t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
	}
	for _, n := range []int{lb, lb + 1, lb + 50} {
		s := fixtures(n)[0]
		got, err := ADR(s.H, s.L, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "ADR", s.ohlcv(), float64(p))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		assertReal(t, lbl("ADR max n=", n), got, r.AlignedReal(0, n), relTol)
	}
}

func TestADROracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{0, -5, 1, 100000, 100001} {
		r := mustCall(t, "ADR", s.ohlcv(), float64(p))
		_, err := ADR(s.H, s.L, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "ADR", float64(p)); c != ADRLookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, ADRLookback(p), c)
		}
	}
}

func TestADROracleRanges(t *testing.T) {
	for _, s := range fixtures(120) {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {12, 13}} {
			for _, p := range []int{1, 10, 14} {
				out := make([]float64, 120)
				beg, nb, err := adrRange(rg[0], rg[1], s.H, s.L, p, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "ADR", s.ohlcv(), rg[0], rg[1], float64(p))
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%d: go (%d,%d) c (%d,%d)", s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("ADR range ", s.Name, rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}
