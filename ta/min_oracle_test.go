//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestMINOracle(t *testing.T) {
	periods := []int{2, 3, 4, 5, 14, 30, 31, 200, 1000}
	for _, p := range periods {
		lb := MINLookback(p)
		if c := mustLookback(t, "MIN", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := MIN(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "MIN", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("MIN ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
	for _, p := range []int{2, 30, 100000} {
		if c := mustLookback(t, "MIN", float64(p)); MINLookback(p) != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, MINLookback(p), c)
		}
	}
}

// NaN and signed-zero inputs exercise the exact comparison order of the
// block scan (NaN never wins a '<' test, ties keep the incumbent).
func TestMINOracleSpecialValues(t *testing.T) {
	in := fixtures(120)[0].Real()
	in = append([]float64(nil), in...)
	for _, i := range []int{0, 7, 8, 33, 60, 61, 62, 119} {
		in[i] = math.NaN()
	}
	in[40], in[41], in[42] = 0, math.Copysign(0, -1), 0
	for _, p := range []int{2, 3, 5, 8, 30} {
		got, err := MIN(in, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "MIN", realIn(in), float64(p))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		want := r.AlignedReal(0, len(in))
		assertReal(t, lbl("MIN special p=", p), got, want, relTol)
		for i := range got {
			if !math.IsNaN(got[i]) && math.Signbit(got[i]) != math.Signbit(want[i]) {
				t.Errorf("p=%d [%d]: sign of zero differs", p, i)
			}
		}
	}
}

func TestMINOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{1, 0, -5, 100001, 2, 100000} {
		r := mustCall(t, "MIN", realIn(in), float64(p))
		_, err := MIN(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
	}
}

func TestMINOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.Real()
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {9, 10}, {50, 119}, {100, 118}} {
		for _, p := range []int{2, 3, 10, 31} {
			out := make([]float64, len(in))
			beg, nb, err := minRange(rg[0], rg[1], in, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "MIN", realIn(in), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("MIN range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
