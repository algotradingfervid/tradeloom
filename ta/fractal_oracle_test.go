//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func fractalHL(h, l []float64) oracle.Inputs {
	return oracle.Inputs{High: h, Low: l}
}

func TestFRACTALOracle(t *testing.T) {
	params := [][2]int{{1, 1}, {2, 2}, {1, 5}, {5, 1}, {3, 7}, {10, 10}, {50, 3}}
	fired := [2]int{}
	for _, p := range params {
		lb := FRACTALLookback(p[0], p[1])
		if c := mustLookback(t, "FRACTAL", float64(p[0]), float64(p[1])); lb != c {
			t.Fatalf("lookback %v: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				hi, lo, err := FRACTAL(s.H, s.L, p[0], p[1])
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "FRACTAL", fractalHL(s.H, s.L), float64(p[0]), float64(p[1]))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				name := lbl("FRACTAL ", s.Name, " n=", n, " p=", p)
				assertInt(t, name+" hi", hi, r.AlignedInt(0, n))
				assertInt(t, name+" lo", lo, r.AlignedInt(1, n))
				for k := 0; k < r.NbElement; k++ {
					fired[0] += r.Outputs[0].Int[k] / 100
					fired[1] += r.Outputs[1].Int[k] / 100
				}
			}
		}
	}
	// A comparison that never fires proves nothing.
	if fired[0] == 0 || fired[1] == 0 {
		t.Errorf("C fired %d swing highs and %d swing lows; want some of each", fired[0], fired[1])
	}
	t.Logf("C fired %d swing highs, %d swing lows", fired[0], fired[1])
}

// TestFRACTALOracleMax covers the parameter maxima around their (large)
// lookbacks on a few fixtures; the full fixture set would only repeat it.
func TestFRACTALOracleMax(t *testing.T) {
	for _, p := range [][2]int{{100000, 1}, {1, 100000}, {100000, 100000}} {
		lb := FRACTALLookback(p[0], p[1])
		if c := mustLookback(t, "FRACTAL", float64(p[0]), float64(p[1])); lb != c {
			t.Fatalf("lookback %v: go %d c %d", p, lb, c)
		}
		for _, n := range []int{1, 1000, lb, lb + 1, lb + 3} {
			for _, idx := range []int{0, 5, 9} {
				s := fixtures(n)[idx]
				hi, lo, err := FRACTAL(s.H, s.L, p[0], p[1])
				if err != nil {
					t.Fatal(err)
				}
				r := mustCall(t, "FRACTAL", fractalHL(s.H, s.L), float64(p[0]), float64(p[1]))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				name := lbl("FRACTAL max ", s.Name, " n=", n, " p=", p)
				assertInt(t, name+" hi", hi, r.AlignedInt(0, n))
				assertInt(t, name+" lo", lo, r.AlignedInt(1, n))
			}
		}
	}
}

// TestFRACTALOracleSpecial feeds ties, NaN and infinities: the comparisons
// must follow C's IEEE semantics exactly.
func TestFRACTALOracleSpecial(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	cases := []struct {
		name string
		h, l []float64
	}{
		{"ties", []float64{1, 3, 3, 1, 2, 2, 2, 5, 1, 5}, []float64{3, 1, 1, 3, 2, 2, 2, 0, 4, 0}},
		{"nan", []float64{1, 2, nan, 2, 1, 5, 1, nan, 0, 9, 1}, []float64{5, 4, nan, 4, 5, 0, 6, 3, nan, 1, 2}},
		{"nan-first", []float64{nan, 5, 1, 1, 7, 1, 1}, []float64{nan, 0, 3, 3, -1, 3, 3}},
		{"inf", []float64{1, inf, 1, -inf, 2, 1, inf, inf, 0}, []float64{-inf, 0, 1, -inf, 2, -1, 3, 4, 5}},
		{"signed-zero", []float64{0, math.Copysign(0, -1), 0, 1, 0}, []float64{math.Copysign(0, -1), 0, 0, -1, 0}},
	}
	for _, c := range cases {
		for _, p := range [][2]int{{1, 1}, {2, 2}, {1, 3}, {3, 1}} {
			n := len(c.h)
			hi, lo, err := FRACTAL(c.h, c.l, p[0], p[1])
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "FRACTAL", fractalHL(c.h, c.l), float64(p[0]), float64(p[1]))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("FRACTAL ", c.name, " p=", p, " hi"), hi, r.AlignedInt(0, n))
			assertInt(t, lbl("FRACTAL ", c.name, " p=", p, " lo"), lo, r.AlignedInt(1, n))
		}
	}
}

func TestFRACTALOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range [][2]int{{0, 2}, {2, 0}, {-5, 2}, {2, -5}, {100001, 2}, {2, 100001}, {1, 1}, {100000, 100000}} {
		r := mustCall(t, "FRACTAL", fractalHL(s.H, s.L), float64(p[0]), float64(p[1]))
		_, _, err := FRACTAL(s.H, s.L, p[0], p[1])
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%v: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if (r.RetCode == oracle.BadParam) != (FRACTALLookback(p[0], p[1]) == -1) {
			t.Errorf("p=%v: C retcode %d, Go lookback %d", p, r.RetCode, FRACTALLookback(p[0], p[1]))
		}
	}
}

func TestFRACTALOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {2, 4}, {4, 4}, {5, 119}} {
		for _, p := range [][2]int{{1, 1}, {2, 2}, {5, 4}} {
			hi := make([]int, 120)
			lo := make([]int, 120)
			beg, nb, err := fractalRange(rg[0], rg[1], s.H, s.L, p[0], p[1], hi, lo)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "FRACTAL", fractalHL(s.H, s.L), rg[0], rg[1], float64(p[0]), float64(p[1]))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%v: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("FRACTAL range ", rg, " p=", p, " hi"), hi[:nb], r.Outputs[0].Int)
			assertInt(t, lbl("FRACTAL range ", rg, " p=", p, " lo"), lo[:nb], r.Outputs[1].Int)
		}
	}
}
