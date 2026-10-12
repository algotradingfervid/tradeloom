//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

var mamaOracleParams = [][2]float64{
	{0.5, 0.05}, // defaults
	{0.01, 0.01},
	{0.99, 0.99},
	{0.99, 0.01},
	{0.01, 0.99},
	{0.21, 0.6},
	{0.8, 0.1},
}

func mamaOracleCompare(t *testing.T, tag string) {
	t.Helper()
	for _, p := range mamaOracleParams {
		lb := MAMALookback(p[0], p[1])
		if c := mustLookback(t, "MAMA", p[0], p[1]); lb != c {
			t.Fatalf("%slookback p=%v: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				m, f, err := MAMA(s.Real(), p[0], p[1])
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "MAMA", realIn(s.Real()), p[0], p[1])
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				l := lbl(tag, s.Name, " n=", n, " p=", p)
				assertReal(t, "MAMA "+l, m, r.AlignedReal(0, n), relTol)
				assertReal(t, "FAMA "+l, f, r.AlignedReal(1, n), relTol)
			}
		}
	}
}

func TestMAMAOracle(t *testing.T) {
	mamaOracleCompare(t, "")
}

func TestMAMAOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 10, 50} {
		if err := SetUnstablePeriod(UnstMAMA, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstMAMA), u)
		mamaOracleCompare(t, lbl("unstable=", u, " "))
	}
}

func TestMAMAOracleBadParams(t *testing.T) {
	in := fixtures(60)[0].Real()
	cases := [][2]float64{
		{0.0099, 0.05}, {0.991, 0.05}, {0, 0.05}, {-0.5, 0.05}, {1, 0.05},
		{0.5, 0.0099}, {0.5, 0.991}, {0.5, 0}, {0.5, 2},
		{0.01, 0.01}, {0.99, 0.99}, // valid boundaries
	}
	for _, p := range cases {
		r := mustCall(t, "MAMA", realIn(in), p[0], p[1])
		_, _, err := MAMA(in, p[0], p[1])
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%v: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if err == nil && r.RetCode != oracle.Success {
			t.Errorf("p=%v: C retcode %d for valid params", p, r.RetCode)
		}
	}
}

func TestMAMAOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(300)[1], fixtures(300)[11]} {
		in := s.Real()
		last := len(in) - 1
		for _, rg := range [][2]int{{0, 0}, {0, last}, {3, 3}, {0, 31}, {0, 32}, {32, 32}, {33, 33},
			{10, 50}, {40, 41}, {41, 120}, {100, 299}, {last, last}, {31, 40}} {
			for _, p := range [][2]float64{{0.5, 0.05}, {0.9, 0.02}} {
				om, of := make([]float64, len(in)), make([]float64, len(in))
				beg, nb, err := mamaRange(rg[0], rg[1], in, p[0], p[1], om, of)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "MAMA", realIn(in), rg[0], rg[1], p[0], p[1])
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%v: go (%d,%d) c (%d,%d)", s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				l := lbl(s.Name, " range ", rg, " p=", p)
				assertReal(t, "MAMA "+l, om[:nb], r.Outputs[0].Real, relTol)
				assertReal(t, "FAMA "+l, of[:nb], r.Outputs[1].Real, relTol)
			}
		}
	}
}
