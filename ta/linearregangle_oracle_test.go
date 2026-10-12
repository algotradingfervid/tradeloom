//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestLINEARREGANGLEOracle(t *testing.T) {
	periods := []int{2, 3, 5, 14, 30, 200}
	for _, p := range periods {
		lb := LINEARREGANGLELookback(p)
		if c := mustLookback(t, "LINEARREG_ANGLE", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := LINEARREGANGLE(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "LINEARREG_ANGLE", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("LINEARREGANGLE ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
	// The maximum period on a series just long enough to produce output.
	for _, s := range fixtures(100001) {
		if s.Name != "walk" && s.Name != "huge" && s.Name != "zigzag" {
			continue
		}
		got, _ := LINEARREGANGLE(s.Real(), 100000)
		r := mustCall(t, "LINEARREG_ANGLE", realIn(s.Real()), 100000)
		assertReal(t, lbl("LINEARREGANGLE max ", s.Name), got, r.AlignedReal(0, len(got)), relTol)
	}
}

// Series that exercise the reseed trigger (a departing value outweighing the
// window) and the periodic 32*period rebuild.
func TestLINEARREGANGLEOracleReseed(t *testing.T) {
	base := fixtures(2000)[1].Real()
	spikes := append([]float64(nil), base...)
	for i := 100; i < len(spikes); i += 137 {
		spikes[i] = 1e12
	}
	alt := make([]float64, 2000)
	for i := range alt {
		alt[i] = 1
		if i%2 == 1 {
			alt[i] = -1
		}
	}
	for name, in := range map[string][]float64{"spikes": spikes, "alternating": alt} {
		for _, p := range []int{2, 5, 14, 30} {
			got, _ := LINEARREGANGLE(in, p)
			r := mustCall(t, "LINEARREG_ANGLE", realIn(in), float64(p))
			assertReal(t, lbl("LINEARREGANGLE ", name, " p=", p), got, r.AlignedReal(0, len(in)), relTol)
		}
	}
}

func TestLINEARREGANGLEOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{1, 0, -5, 2, 100000, 100001} {
		r := mustCall(t, "LINEARREG_ANGLE", realIn(in), float64(p))
		_, err := LINEARREGANGLE(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := LINEARREGANGLELookback(p); (r.RetCode == oracle.BadParam) != (c == -1) {
			t.Errorf("p=%d: C retcode %d, Go lookback %d", p, r.RetCode, c)
		}
	}
}

func TestLINEARREGANGLEOracleRanges(t *testing.T) {
	s := fixtures(1200)[1]
	in := s.Real()
	for _, rg := range [][2]int{{0, 0}, {0, 1199}, {3, 3}, {10, 50}, {1199, 1199}, {0, 8}, {9, 9}, {500, 1199}, {12, 13}} {
		for _, p := range []int{2, 10, 14} {
			out := make([]float64, len(in))
			beg, nb, err := linearregangleRange(rg[0], rg[1], in, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "LINEARREG_ANGLE", realIn(in), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("LINEARREGANGLE range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
