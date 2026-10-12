//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestLINEARREGOracle(t *testing.T) {
	periods := []int{2, 3, 4, 5, 14, 30, 200}
	for _, p := range periods {
		lb := LINEARREGLookback(p)
		if c := mustLookback(t, "LINEARREG", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := LINEARREG(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "LINEARREG", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("LINEARREG ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestLINEARREGOracleMaxPeriod(t *testing.T) {
	p := 100000
	if c := mustLookback(t, "LINEARREG", float64(p)); c != LINEARREGLookback(p) {
		t.Fatalf("lookback: go %d c %d", LINEARREGLookback(p), c)
	}
	for _, n := range []int{p - 1, p, p + 1, p + 5} {
		s := fixtures(n)[0]
		got, err := LINEARREG(s.Real(), p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "LINEARREG", realIn(s.Real()), float64(p))
		assertReal(t, lbl("LINEARREG max n=", n), got, r.AlignedReal(0, n), relTol)
	}
}

// A spike exercises the re-anchor trigger on the bar it leaves the window.
func TestLINEARREGOracleSpike(t *testing.T) {
	in := fixtures(400)[1].Real()
	in[50] = 1e12
	in[200] = -1e13
	for _, p := range []int{2, 5, 14} {
		got, _ := LINEARREG(in, p)
		r := mustCall(t, "LINEARREG", realIn(in), float64(p))
		assertReal(t, lbl("LINEARREG spike p=", p), got, r.AlignedReal(0, len(in)), relTol)
	}
}

func TestLINEARREGOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{1, 0, -5, 100001} {
		r := mustCall(t, "LINEARREG", realIn(in), float64(p))
		_, err := LINEARREG(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "LINEARREG", float64(p)); c != LINEARREGLookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, LINEARREGLookback(p), c)
		}
	}
}

func TestLINEARREGOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.Real()
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {1, 1}, {5, 119}} {
		for _, p := range []int{2, 3, 10} {
			out := make([]float64, len(in))
			beg, nb, err := linearregRange(rg[0], rg[1], in, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "LINEARREG", realIn(in), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("LINEARREG range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
