//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

var t3OracleVFactors = []float64{0, 0.01, 0.5, 0.7, 1}

func t3OracleCompare(t *testing.T, prefix string, periods []int) {
	t.Helper()
	for _, p := range periods {
		for _, v := range t3OracleVFactors {
			lb := T3Lookback(p, v)
			if c := mustLookback(t, "T3", float64(p), v); lb != c {
				t.Fatalf("%slookback p=%d v=%g: go %d c %d", prefix, p, v, lb, c)
			}
			for _, n := range edgeLengths(lb) {
				for _, s := range fixtures(n) {
					got, err := T3(s.Real(), p, v)
					if err != nil {
						t.Fatal(err)
					}
					if n == 0 {
						continue // C rejects endIdx -1; Go returns empty output
					}
					r := mustCall(t, "T3", realIn(s.Real()), float64(p), v)
					if r.RetCode != oracle.Success {
						t.Fatalf("C retcode %d", r.RetCode)
					}
					assertReal(t, lbl(prefix, "T3 ", s.Name, " n=", n, " p=", p, " v=", v), got, r.AlignedReal(0, n), relTol)
				}
			}
		}
	}
}

func TestT3Oracle(t *testing.T) {
	t3OracleCompare(t, "", []int{1, 2, 3, 5, 14, 30})
	// Large periods: lookback only (and one long series for 100).
	for _, p := range []int{100, 1000, 100000} {
		if lb, c := T3Lookback(p, 0.7), mustLookback(t, "T3", float64(p), 0.7); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
	s := fixtures(2000)[0]
	got, _ := T3(s.Real(), 100, 0.7)
	r := mustCall(t, "T3", realIn(s.Real()), 100, 0.7)
	assertReal(t, "T3 p=100 n=2000", got, r.AlignedReal(0, 2000), relTol)
}

func TestT3OracleBadParams(t *testing.T) {
	in := fixtures(40)[0].Real()
	cases := []struct {
		p int
		v float64
	}{
		{0, 0.7}, {-5, 0.7}, {100001, 0.7},
		{5, -0.01}, {5, 1.01}, {5, 2}, {5, -1e-12},
		{1, 0}, {1, 1}, {100000, 0.7}, {5, 0}, {5, 1},
	}
	for _, c := range cases {
		r := mustCall(t, "T3", realIn(in), float64(c.p), c.v)
		_, err := T3(in, c.p, c.v)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d v=%g: C retcode %d, Go err %v", c.p, c.v, r.RetCode, err)
		}
		lb := T3Lookback(c.p, c.v)
		clb := mustLookback(t, "T3", float64(c.p), c.v)
		if lb != clb {
			t.Errorf("lookback p=%d v=%g: go %d c %d", c.p, c.v, lb, clb)
		}
	}
}

func t3OracleRanges(t *testing.T, prefix string) {
	t.Helper()
	s := fixtures(150)[1]
	in := s.Real()
	for _, rg := range [][2]int{{0, 0}, {0, 149}, {3, 3}, {10, 50}, {149, 149}, {0, 23}, {24, 24}, {25, 25}, {60, 149}, {0, 80}} {
		for _, p := range []int{1, 2, 5, 10} {
			for _, v := range []float64{0, 0.7, 1} {
				out := make([]float64, len(in))
				beg, nb, err := t3Range(rg[0], rg[1], in, p, v, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "T3", realIn(in), rg[0], rg[1], float64(p), v)
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%srange %v p=%d v=%g: go (%d,%d) c (%d,%d)", prefix, rg, p, v, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl(prefix, "T3 range ", rg, " p=", p, " v=", v), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

func TestT3OracleRanges(t *testing.T) {
	t3OracleRanges(t, "")
}

func TestT3OracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 37} {
		SetUnstablePeriod(UnstT3, u)
		oracle.SetUnstablePeriod(int(UnstT3), u)
		t3OracleCompare(t, lbl("unst=", u, " "), []int{1, 2, 5, 14})
		t3OracleRanges(t, lbl("unst=", u, " "))
	}
	// Another function's unstable period must not affect T3.
	SetAllUnstablePeriods(0)
	oracle.ResetUnstablePeriods()
	SetUnstablePeriod(UnstEMA, 9)
	oracle.SetUnstablePeriod(int(UnstEMA), 9)
	t3OracleCompare(t, "ema-unst ", []int{3, 14})
}
