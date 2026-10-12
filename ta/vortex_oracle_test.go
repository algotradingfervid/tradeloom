//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestVORTEXOracle(t *testing.T) {
	for _, p := range []int{1, 2, 3, 5, 14, 21, 30, 200} {
		lb := VORTEXLookback(p)
		if c := mustLookback(t, "VORTEX", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				plus, minus, err := VORTEX(s.H, s.L, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "VORTEX", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("VORTEX+ ", s.Name, " n=", n, " p=", p), plus, r.AlignedReal(0, n), relTol)
				assertReal(t, lbl("VORTEX- ", s.Name, " n=", n, " p=", p), minus, r.AlignedReal(1, n), relTol)
			}
		}
	}
	// Maximum period: lookback check, and series inside and just past it.
	if lb, c := VORTEXLookback(100000), mustLookback(t, "VORTEX", 100000); lb != c {
		t.Fatalf("lookback p=100000: go %d c %d", lb, c)
	}
	for _, n := range []int{64, 100001} {
		s := fixtures(n)[0]
		plus, minus, _ := VORTEX(s.H, s.L, s.C, 100000)
		r := mustCall(t, "VORTEX", s.ohlcv(), 100000)
		assertReal(t, lbl("VORTEX+ p=100000 n=", n), plus, r.AlignedReal(0, n), relTol)
		assertReal(t, lbl("VORTEX- p=100000 n=", n), minus, r.AlignedReal(1, n), relTol)
	}
}

// Flat runs exercise the nullRun/sTR purge and the zero-denominator gate.
func TestVORTEXOracleFlatRuns(t *testing.T) {
	s := fixtures(300)[0]
	h := append([]float64(nil), s.H...)
	l := append([]float64(nil), s.L...)
	c := append([]float64(nil), s.C...)
	for _, run := range [][2]int{{40, 60}, {100, 103}, {150, 200}, {250, 251}} {
		v := c[run[0]-1]
		for i := run[0]; i < run[1]; i++ {
			h[i], l[i], c[i] = v, v, v
		}
	}
	in := oracle.Inputs{Open: c, High: h, Low: l, Close: c, Volume: s.V}
	for _, p := range []int{1, 2, 3, 5, 14, 20, 30} {
		plus, minus, _ := VORTEX(h, l, c, p)
		r := mustCall(t, "VORTEX", in, float64(p))
		assertReal(t, lbl("VORTEX+ flat p=", p), plus, r.AlignedReal(0, 300), relTol)
		assertReal(t, lbl("VORTEX- flat p=", p), minus, r.AlignedReal(1, 300), relTol)
	}
}

func TestVORTEXOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{0, -5, 1, 100000, 100001} {
		r := mustCall(t, "VORTEX", s.ohlcv(), float64(p))
		_, _, err := VORTEX(s.H, s.L, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if lb, c := VORTEXLookback(p), mustLookback(t, "VORTEX", float64(p)); lb != c {
			t.Errorf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
}

func TestVORTEXOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[11]} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 9}, {10, 10}, {5, 30}, {100, 119}} {
			for _, p := range []int{1, 2, 10} {
				plus := make([]float64, 120)
				minus := make([]float64, 120)
				beg, nb, err := vortexRange(rg[0], rg[1], s.H, s.L, s.C, p, plus, minus)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "VORTEX", s.ohlcv(), rg[0], rg[1], float64(p))
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%d: go (%d,%d) c (%d,%d)", s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("VORTEX+ range ", s.Name, rg, " p=", p), plus[:nb], r.Outputs[0].Real, relTol)
				assertReal(t, lbl("VORTEX- range ", s.Name, rg, " p=", p), minus[:nb], r.Outputs[1].Real, relTol)
			}
		}
	}
}
