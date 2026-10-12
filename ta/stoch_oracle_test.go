//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters, including every MAType for both
// smoothing stages.

func stochOpt(p stochParamSet) []float64 {
	return []float64{float64(p.fastK), float64(p.slowK), float64(p.slowKT), float64(p.slowD), float64(p.slowDT)}
}

func stochOracleSeries(t *testing.T, sets []stochParamSet) {
	t.Helper()
	for _, p := range sets {
		lb := STOCHLookback(p.fastK, p.slowK, p.slowKT, p.slowD, p.slowDT)
		if c := mustLookback(t, "STOCH", stochOpt(p)...); lb != c {
			t.Fatalf("lookback %+v: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				k, d, err := STOCH(s.H, s.L, s.C, p.fastK, p.slowK, p.slowKT, p.slowD, p.slowDT)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "STOCH", s.ohlcv(), stochOpt(p)...)
				if r.RetCode != oracle.Success {
					t.Fatalf("%+v C retcode %d", p, r.RetCode)
				}
				assertReal(t, lbl("STOCH K ", s.Name, " n=", n, " ", p), k, r.AlignedReal(0, n), relTol)
				assertReal(t, lbl("STOCH D ", s.Name, " n=", n, " ", p), d, r.AlignedReal(1, n), relTol)
			}
		}
	}
}

func TestSTOCHOracle(t *testing.T) {
	stochOracleSeries(t, []stochParamSet{
		{5, 3, MATypeSMA, 3, MATypeSMA}, // defaults
		{1, 1, MATypeSMA, 1, MATypeSMA}, // minimums
		{2, 1, MATypeEMA, 2, MATypeSMA},
		{14, 3, MATypeSMA, 3, MATypeSMA},
		{9, 5, MATypeEMA, 4, MATypeEMA},
		{30, 10, MATypeWMA, 7, MATypeDEMA},
		{200, 3, MATypeSMA, 3, MATypeSMA},
		{5, 3, MATypeDefault, 3, MATypeDefault},
	})
}

func TestSTOCHOracleAllMATypes(t *testing.T) {
	var sets []stochParamSet
	for _, mt := range maAllTypes {
		sets = append(sets,
			stochParamSet{5, 3, mt, 3, MATypeSMA},
			stochParamSet{7, 2, MATypeEMA, 4, mt},
			stochParamSet{3, 4, mt, 5, mt},
		)
	}
	stochOracleSeries(t, sets)
}

func TestSTOCHOracleBadParams(t *testing.T) {
	s := fixtures(40)[0]
	cases := []stochParamSet{
		{0, 3, MATypeSMA, 3, MATypeSMA},
		{100001, 3, MATypeSMA, 3, MATypeSMA},
		{5, 0, MATypeSMA, 3, MATypeSMA},
		{5, 100001, MATypeSMA, 3, MATypeSMA},
		{5, 3, -1, 3, MATypeSMA},
		{5, 3, 14, 3, MATypeSMA},
		{5, 3, MATypeSMA, 0, MATypeSMA},
		{5, 3, MATypeSMA, 100001, MATypeSMA},
		{5, 3, MATypeSMA, 3, -1},
		{5, 3, MATypeSMA, 3, 99},
		{1, 1, MATypeSMA, 1, MATypeSMA},
		{100000, 3, MATypeSMA, 3, MATypeSMA},
		{5, 100000, MATypeSMA, 3, MATypeSMA},
		{5, 3, MATypeSMA, 100000, MATypeSMA},
		{5, 3, MATypeDefault, 3, MATypeDefault},
		{5, 3, MATypeRMA, 3, MATypeRMA},
	}
	for _, p := range cases {
		r := mustCall(t, "STOCH", s.ohlcv(), stochOpt(p)...)
		_, _, err := STOCH(s.H, s.L, s.C, p.fastK, p.slowK, p.slowKT, p.slowD, p.slowDT)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("%+v: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if err == nil && r.RetCode != oracle.Success {
			t.Errorf("%+v: C retcode %d, Go ok", p, r.RetCode)
		}
		lbGo := STOCHLookback(p.fastK, p.slowK, p.slowKT, p.slowD, p.slowDT)
		if lbC := mustLookback(t, "STOCH", stochOpt(p)...); lbGo != lbC {
			t.Errorf("%+v: lookback go %d c %d", p, lbGo, lbC)
		}
	}
}

func TestSTOCHOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	sets := []stochParamSet{
		{5, 3, MATypeSMA, 3, MATypeSMA},
		{1, 1, MATypeSMA, 1, MATypeSMA},
		{10, 4, MATypeEMA, 6, MATypeWMA},
		{6, 3, MATypeKAMA, 3, MATypeT3},
	}
	for _, p := range sets {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {7, 7}, {8, 8}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {0, 40}, {41, 41}, {60, 119}} {
			k := make([]float64, len(s.C))
			d := make([]float64, len(s.C))
			beg, nb, err := stochRange(rg[0], rg[1], s.H, s.L, s.C, p.fastK, p.slowK, p.slowKT, p.slowD, p.slowDT, k, d)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "STOCH", s.ohlcv(), rg[0], rg[1], stochOpt(p)...)
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v %+v: C retcode %d", rg, p, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v %+v: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("STOCH K range ", rg, " ", p), k[:nb], r.Outputs[0].Real, relTol)
			assertReal(t, lbl("STOCH D range ", rg, " ", p), d[:nb], r.Outputs[1].Real, relTol)
		}
	}
}

func TestSTOCHOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, id := range []FuncUnstID{UnstEMA, UnstKAMA, UnstMAMA, UnstT3, UnstRMA} {
		SetUnstablePeriod(id, 7)
		oracle.SetUnstablePeriod(int(id), 7)
	}
	var sets []stochParamSet
	for _, mt := range []MAType{MATypeEMA, MATypeKAMA, MATypeMAMA, MATypeT3, MATypeRMA} {
		sets = append(sets, stochParamSet{5, 3, mt, 4, mt})
	}
	stochOracleSeries(t, sets)
}
