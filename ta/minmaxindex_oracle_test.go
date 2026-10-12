//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters. Outputs are integer indexes, so the
// comparison is exact.

func TestMINMAXINDEXOracle(t *testing.T) {
	for _, p := range []int{2, 3, 4, 5, 14, 30, 200, 1000} {
		lb := MINMAXINDEXLookback(p)
		if c := mustLookback(t, "MINMAXINDEX", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				mn, mx, err := MINMAXINDEX(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "MINMAXINDEX", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				name := lbl(s.Name, " n=", n, " p=", p)
				assertInt(t, "MINMAXINDEX min "+name, mn, r.AlignedInt(0, n))
				assertInt(t, "MINMAXINDEX max "+name, mx, r.AlignedInt(1, n))
			}
		}
	}
}

// Ties and NaN exercise the comparison semantics (strict on rescan, >=/<= on
// incremental update; NaN never compares).
func TestMINMAXINDEXOracleTiesAndNaN(t *testing.T) {
	nan := math.NaN()
	inputs := [][]float64{
		{1, 2, 2, 2, 1, 1, 3, 3, 0, 0, 0, 3, 3, 1},
		{5, 5, 5, 5, 5, 5, 5, 5},
		{nan, 1, 2, nan, 3, 0, nan, nan, 4, -1, 2, nan},
		{1, 2, 3, nan, nan, nan, nan, 1, 0},
		{math.Inf(1), 0, math.Inf(-1), 0, 0, math.Inf(1), 1, 2},
		{0, math.Copysign(0, -1), 0, math.Copysign(0, -1), 0, 0},
	}
	for k, in := range inputs {
		for _, p := range []int{2, 3, 4, 5} {
			mn, mx, err := MINMAXINDEX(in, p)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "MINMAXINDEX", realIn(in), float64(p))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			n := len(in)
			assertInt(t, lbl("ties min #", k, " p=", p), mn, r.AlignedInt(0, n))
			assertInt(t, lbl("ties max #", k, " p=", p), mx, r.AlignedInt(1, n))
		}
	}
}

func TestMINMAXINDEXOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{1, 0, -5, 100001, 2, 100000} {
		r := mustCall(t, "MINMAXINDEX", realIn(in), float64(p))
		_, _, err := MINMAXINDEX(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "MINMAXINDEX", float64(p)); c != MINMAXINDEXLookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, MINMAXINDEXLookback(p), c)
		}
	}
}

func TestMINMAXINDEXOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[9], fixtures(120)[5]} {
		in := s.Real()
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {5, 15}, {100, 119}} {
			for _, p := range []int{2, 10, 30} {
				mn := make([]int, len(in))
				mx := make([]int, len(in))
				beg, nb, err := minmaxindexRange(rg[0], rg[1], in, p, mn, mx)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "MINMAXINDEX", realIn(in), rg[0], rg[1], float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%d: go (%d,%d) c (%d,%d)", s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertInt(t, lbl(s.Name, " range min ", rg, " p=", p), mn[:nb], r.Outputs[0].Int[:nb])
				assertInt(t, lbl(s.Name, " range max ", rg, " p=", p), mx[:nb], r.Outputs[1].Int[:nb])
			}
		}
	}
}
