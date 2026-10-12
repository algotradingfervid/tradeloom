//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters. Outputs are integers, so the
// comparison is exact.

func TestMININDEXOracle(t *testing.T) {
	for _, p := range []int{2, 3, 5, 14, 30, 200} {
		lb := MININDEXLookback(p)
		if c := mustLookback(t, "MININDEX", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := MININDEX(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "MININDEX", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertInt(t, lbl("MININDEX ", s.Name, " n=", n, " p=", p), got, r.AlignedInt(0, n))
			}
		}
	}
	// Large period (max of the range) on a series long enough to emit output.
	in := fixtures(100005)[0].Real()
	got, _ := MININDEX(in, 100000)
	r := mustCall(t, "MININDEX", realIn(in), 100000)
	assertInt(t, "MININDEX p=100000", got, r.AlignedInt(0, len(in)))
	if c := mustLookback(t, "MININDEX", 100000); c != MININDEXLookback(100000) {
		t.Fatalf("lookback p=100000: go %d c %d", MININDEXLookback(100000), c)
	}
}

// Ties, NaN and infinities exercise the exact comparison semantics (< in the
// rescan, <= on the incremental path).
func TestMININDEXOracleSpecialValues(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	cases := [][]float64{
		{3, 3, 3, 1, 3, 3, 2, 2, 2, 2, 5, 5, 1, 1, 1, 1},
		{1, nan, 2, 3, nan, nan, 1, 0, 4, nan, 2, 1, 1, 1},
		{nan, 1, 2, 3, 4, 5, 6, 7},
		{inf, 1, -inf, 2, inf, inf, 0, -1, -2, -3},
		{0, math.Copysign(0, -1), 0, math.Copysign(0, -1), -1, -1, -1},
	}
	for ci, in := range cases {
		for _, p := range []int{2, 3, 4} {
			got, err := MININDEX(in, p)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "MININDEX", realIn(in), float64(p))
			assertInt(t, lbl("special ", ci, " p=", p), got, r.AlignedInt(0, len(in)))
		}
	}
}

func TestMININDEXOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{1, 0, -5, 100001} {
		r := mustCall(t, "MININDEX", realIn(in), float64(p))
		_, err := MININDEX(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "MININDEX", float64(p)); c != MININDEXLookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, MININDEXLookback(p), c)
		}
	}
}

func TestMININDEXOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[9], fixtures(120)[5]} {
		in := s.Real()
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {5, 12}} {
			for _, p := range []int{2, 10} {
				out := make([]int, len(in))
				beg, nb, err := minindexRange(rg[0], rg[1], in, p, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "MININDEX", realIn(in), rg[0], rg[1], float64(p))
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%d: go (%d,%d) c (%d,%d)", s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertInt(t, lbl("MININDEX range ", s.Name, rg, " p=", p), out[:nb], r.Outputs[0].Int)
			}
		}
	}
}
