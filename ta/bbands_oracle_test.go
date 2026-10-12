//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

type bbandsParams struct {
	p      int
	up, dn float64
	mt     MAType
}

func (b bbandsParams) opt() []float64 {
	return []float64{float64(b.p), b.up, b.dn, float64(b.mt)}
}

func bbandsOracleSeries(t *testing.T, sets []bbandsParams) {
	t.Helper()
	for _, b := range sets {
		lb := BBANDSLookback(b.p, b.up, b.dn, b.mt)
		if c := mustLookback(t, "BBANDS", b.opt()...); lb != c {
			t.Fatalf("lookback %v: go %d c %d", b, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				u, m, l, err := BBANDS(s.Real(), b.p, b.up, b.dn, b.mt)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "BBANDS", realIn(s.Real()), b.opt()...)
				if r.RetCode != oracle.Success {
					t.Fatalf("%v: C retcode %d", b, r.RetCode)
				}
				tag := lbl(" ", s.Name, " n=", n, " ", b)
				assertReal(t, "BBANDS upper"+tag, u, r.AlignedReal(0, n), relTol)
				assertReal(t, "BBANDS middle"+tag, m, r.AlignedReal(1, n), relTol)
				assertReal(t, "BBANDS lower"+tag, l, r.AlignedReal(2, n), relTol)
			}
		}
	}
}

func bbandsAllTypeSets(periods []int) []bbandsParams {
	var sets []bbandsParams
	for _, mt := range bbandsAllTypes {
		for _, p := range periods {
			sets = append(sets,
				bbandsParams{p, 2, 2, mt},     // equal multipliers
				bbandsParams{p, 2.5, 1.5, mt}, // distinct: fma upper band
			)
		}
	}
	return sets
}

func TestBBANDSOracle(t *testing.T) {
	sets := bbandsAllTypeSets([]int{2, 3, 5, 20, 33, 34, 50})
	sets = append(sets,
		bbandsParams{200, 2, 2, MATypeSMA},
		bbandsParams{200, 1, 3, MATypeEMA},
		bbandsParams{14, 0, 0, MATypeSMA},
		bbandsParams{14, -2, -2, MATypeSMA},
		bbandsParams{14, -1.3, 0.7, MATypeWMA},
		bbandsParams{10, taRealMax, taRealMin, MATypeSMA},
		bbandsParams{10, taRealMin, taRealMax, MATypeEMA},
		bbandsParams{10, 1e-300, 3, MATypeSMA},
	)
	bbandsOracleSeries(t, sets)
}

func TestBBANDSOracleMaxPeriod(t *testing.T) {
	p := 100000
	for _, mt := range []MAType{MATypeSMA, MATypeEMA, MATypeMAMA, MATypeDisabled} {
		b := bbandsParams{p, 2, 1, mt}
		lb := BBANDSLookback(p, 2, 1, mt)
		if c := mustLookback(t, "BBANDS", b.opt()...); lb != c {
			t.Fatalf("lookback %v: go %d c %d", b, lb, c)
		}
		for _, n := range []int{lb, lb + 1, lb + 50} {
			s := fixtures(n)[0]
			u, m, l, err := BBANDS(s.Real(), p, 2, 1, mt)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "BBANDS", realIn(s.Real()), b.opt()...)
			assertReal(t, lbl("max upper n=", n, " mt=", mt), u, r.AlignedReal(0, n), relTol)
			assertReal(t, lbl("max middle n=", n, " mt=", mt), m, r.AlignedReal(1, n), relTol)
			assertReal(t, lbl("max lower n=", n, " mt=", mt), l, r.AlignedReal(2, n), relTol)
		}
	}
}

// TestBBANDSOracleReseed drives the SMA fast path through its variance
// reseeds: a jump in level, a collapse to a flat run, and a long series
// (past 32*period bars).
func TestBBANDSOracleReseed(t *testing.T) {
	var in []float64
	for i := 0; i < 60; i++ {
		in = append(in, 1e6+float64(i%3))
	}
	for i := 0; i < 40; i++ {
		in = append(in, 5)
	}
	for i := 0; i < 40; i++ {
		in = append(in, 1e-3*float64(i%5))
	}
	in = append(in, fixtures(500)[3].Real()...)
	in = append(in, math.NaN(), 1, 2, 3, 4, 5, 6, 7, 8)
	for _, mt := range []MAType{MATypeSMA, MATypeEMA} {
		for _, p := range []int{2, 5, 9} {
			for _, d := range [][2]float64{{2, 2}, {2, 1}} {
				u, m, l, err := BBANDS(in, p, d[0], d[1], mt)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCall(t, "BBANDS", realIn(in), float64(p), d[0], d[1], float64(mt))
				n := len(in)
				tag := lbl(" p=", p, " d=", d, " mt=", mt)
				assertReal(t, "reseed upper"+tag, u, r.AlignedReal(0, n), relTol)
				assertReal(t, "reseed middle"+tag, m, r.AlignedReal(1, n), relTol)
				assertReal(t, "reseed lower"+tag, l, r.AlignedReal(2, n), relTol)
			}
		}
	}
}

func TestBBANDSOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	cases := []bbandsParams{
		{1, 2, 2, MATypeSMA}, {0, 2, 2, MATypeSMA}, {-5, 2, 2, MATypeEMA}, {100001, 2, 2, MATypeSMA},
		{5, -3.0000001e37, 2, MATypeSMA}, {5, 3.0000001e37, 2, MATypeSMA},
		{5, 2, -3.0000001e37, MATypeSMA}, {5, 2, 3.0000001e37, MATypeSMA},
		{5, math.Inf(1), 2, MATypeSMA}, {5, 2, math.Inf(-1), MATypeSMA},
		{5, 2, 2, -1}, {5, 2, 2, 14}, {5, 2, 2, 99},
		{2, 2, 2, MATypeSMA}, {100000, 2, 2, MATypeSMA}, {5, 2, 2, MATypeDefault}, {5, 2, 2, MATypeRMA},
		{5, taRealMin, taRealMax, MATypeSMA}, {5, taRealMax, taRealMin, MATypeSMA},
	}
	for _, c := range cases {
		r := mustCall(t, "BBANDS", realIn(in), c.opt()...)
		_, _, _, err := BBANDS(in, c.p, c.up, c.dn, c.mt)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("%v: C retcode %d, Go err %v", c, r.RetCode, err)
		}
		if err == nil && r.RetCode != oracle.Success {
			t.Errorf("%v: C retcode %d, Go ok", c, r.RetCode)
		}
		lbErr := BBANDSLookback(c.p, c.up, c.dn, c.mt) == -1
		if cl := mustLookback(t, "BBANDS", c.opt()...); (cl == -1) != lbErr {
			t.Errorf("%v: C lookback %d, Go lookback err %v", c, cl, lbErr)
		}
	}
}

func TestBBANDSOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.Real()
	n := len(in)
	ranges := [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {0, 32}, {33, 33}, {34, 34}, {40, 119}, {100, 105}}
	for _, mt := range bbandsAllTypes {
		for _, rg := range ranges {
			for _, b := range []bbandsParams{{2, 2, 2, mt}, {4, 2, 1, mt}, {10, 1.5, 1.5, mt}, {35, 2, 3, mt}} {
				u, m, l := make([]float64, n), make([]float64, n), make([]float64, n)
				beg, nb, err := bbandsRange(rg[0], rg[1], in, b.p, b.up, b.dn, b.mt, u, m, l)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "BBANDS", realIn(in), rg[0], rg[1], b.opt()...)
				if r.RetCode != oracle.Success {
					t.Fatalf("range %v %v: C retcode %d", rg, b, r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("range %v %v: go (%d,%d) c (%d,%d)", rg, b, beg, nb, r.BegIdx, r.NbElement)
				}
				tag := lbl(" range ", rg, " ", b)
				assertReal(t, "upper"+tag, u[:nb], r.Outputs[0].Real, relTol)
				assertReal(t, "middle"+tag, m[:nb], r.Outputs[1].Real, relTol)
				assertReal(t, "lower"+tag, l[:nb], r.Outputs[2].Real, relTol)
			}
		}
	}
}

func TestBBANDSOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, id := range []FuncUnstID{UnstEMA, UnstKAMA, UnstMAMA, UnstT3, UnstRMA} {
		SetUnstablePeriod(id, 7)
		oracle.SetUnstablePeriod(int(id), 7)
	}
	bbandsOracleSeries(t, bbandsAllTypeSets([]int{2, 5, 14}))
}
