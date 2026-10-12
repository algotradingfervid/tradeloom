//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters, including every MAType for both
// smoothing stages.

func kdjOpt(p kdjParamSet) []float64 {
	return []float64{float64(p.fastK), float64(p.slowK), float64(p.slowKT), float64(p.slowD), float64(p.slowDT)}
}

func kdjOracleSeries(t *testing.T, sets []kdjParamSet) {
	t.Helper()
	for _, p := range sets {
		lb := kdjLB(p)
		if c := mustLookback(t, "KDJ", kdjOpt(p)...); lb != c {
			t.Fatalf("lookback %+v: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				k, d, j, err := kdjCall(s, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "KDJ", s.ohlcv(), kdjOpt(p)...)
				if r.RetCode != oracle.Success {
					t.Fatalf("%+v C retcode %d", p, r.RetCode)
				}
				assertReal(t, lbl("KDJ K ", s.Name, " n=", n, " ", p), k, r.AlignedReal(0, n), relTol)
				assertReal(t, lbl("KDJ D ", s.Name, " n=", n, " ", p), d, r.AlignedReal(1, n), relTol)
				assertReal(t, lbl("KDJ J ", s.Name, " n=", n, " ", p), j, r.AlignedReal(2, n), relTol)
			}
		}
	}
}

func TestKDJOracle(t *testing.T) {
	kdjOracleSeries(t, []kdjParamSet{
		{9, 3, MATypeRMA, 3, MATypeRMA},         // defaults
		{9, 3, MATypeDefault, 3, MATypeDefault}, // C substitutes RMA
		{1, 1, MATypeSMA, 1, MATypeSMA},         // minimums
		{1, 1, MATypeRMA, 1, MATypeRMA},
		{2, 1, MATypeEMA, 2, MATypeSMA},
		{5, 3, MATypeSMA, 3, MATypeSMA}, // classic slow stochastic
		{14, 5, MATypeRMA, 4, MATypeRMA},
		{30, 10, MATypeWMA, 7, MATypeDEMA},
		{200, 3, MATypeRMA, 3, MATypeRMA},
		{9, 3, MATypeDefault, 3, MATypeSMA},
	})
}

func TestKDJOracleAllMATypes(t *testing.T) {
	var sets []kdjParamSet
	for _, mt := range maAllTypes {
		sets = append(sets,
			kdjParamSet{9, 3, mt, 3, MATypeRMA},
			kdjParamSet{7, 2, MATypeEMA, 4, mt},
			kdjParamSet{3, 4, mt, 5, mt},
		)
	}
	kdjOracleSeries(t, sets)
}

func TestKDJOracleBadParams(t *testing.T) {
	s := fixtures(40)[0]
	cases := []kdjParamSet{
		{0, 3, MATypeRMA, 3, MATypeRMA},
		{100001, 3, MATypeRMA, 3, MATypeRMA},
		{9, 0, MATypeRMA, 3, MATypeRMA},
		{9, 100001, MATypeRMA, 3, MATypeRMA},
		{9, 3, -1, 3, MATypeRMA},
		{9, 3, MATypeRMA + 1, 3, MATypeRMA},
		{9, 3, MATypeRMA, 0, MATypeRMA},
		{9, 3, MATypeRMA, 100001, MATypeRMA},
		{9, 3, MATypeRMA, 3, -1},
		{9, 3, MATypeRMA, 3, MATypeRMA + 1},
		// valid boundaries
		{1, 1, MATypeSMA, 1, MATypeSMA},
		{100000, 3, MATypeRMA, 3, MATypeRMA},
		{9, 100000, MATypeRMA, 3, MATypeRMA},
		{9, 3, MATypeRMA, 100000, MATypeRMA},
		{9, 3, MATypeDefault, 3, MATypeDefault},
	}
	for _, p := range cases {
		r := mustCall(t, "KDJ", s.ohlcv(), kdjOpt(p)...)
		_, _, _, err := kdjCall(s, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("%+v: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if err == nil {
			if lb, c := kdjLB(p), mustLookback(t, "KDJ", kdjOpt(p)...); lb != c {
				t.Errorf("%+v: lookback go %d c %d", p, lb, c)
			}
		}
	}
}

func kdjOracleRanges(t *testing.T, s series, sets []kdjParamSet, ranges [][2]int) {
	t.Helper()
	for _, rg := range ranges {
		for _, p := range sets {
			k := make([]float64, len(s.C))
			d := make([]float64, len(s.C))
			j := make([]float64, len(s.C))
			beg, nb, err := kdjRange(rg[0], rg[1], s.H, s.L, s.C, p.fastK, p.slowK, p.slowKT, p.slowD, p.slowDT, k, d, j)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "KDJ", s.ohlcv(), rg[0], rg[1], kdjOpt(p)...)
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v %+v: C retcode %d", rg, p, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v %+v: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("KDJ range K ", s.Name, " ", rg, " ", p), k[:nb], r.Outputs[0].Real[:nb], relTol)
			assertReal(t, lbl("KDJ range D ", s.Name, " ", rg, " ", p), d[:nb], r.Outputs[1].Real[:nb], relTol)
			assertReal(t, lbl("KDJ range J ", s.Name, " ", rg, " ", p), j[:nb], r.Outputs[2].Real[:nb], relTol)
		}
	}
}

func TestKDJOracleRanges(t *testing.T) {
	sets := []kdjParamSet{
		{9, 3, MATypeRMA, 3, MATypeRMA}, // lookback 12
		{5, 3, MATypeSMA, 3, MATypeSMA}, // lookback 8
		{1, 1, MATypeSMA, 1, MATypeSMA}, // lookback 0
		{7, 2, MATypeEMA, 4, MATypeKAMA},
	}
	ranges := [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {8, 8}, {11, 11}, {12, 12}, {0, 12}, {12, 60}, {100, 119}}
	for _, s := range []series{fixtures(120)[1], fixtures(120)[5], fixtures(120)[11]} {
		kdjOracleRanges(t, s, sets, ranges)
	}
}

func TestKDJOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, id := range []FuncUnstID{UnstRMA, UnstEMA} {
		for _, u := range []int{1, 7, 40} {
			SetAllUnstablePeriods(0)
			oracle.ResetUnstablePeriods()
			SetUnstablePeriod(id, u)
			oracle.SetUnstablePeriod(int(id), u)
			kdjOracleSeries(t, []kdjParamSet{
				{9, 3, MATypeRMA, 3, MATypeRMA},
				{9, 3, MATypeDefault, 3, MATypeDefault},
				{5, 3, MATypeEMA, 3, MATypeRMA},
			})
			kdjOracleRanges(t, fixtures(150)[0], []kdjParamSet{{9, 3, MATypeRMA, 3, MATypeRMA}}, [][2]int{{0, 149}, {5, 60}, {60, 60}, {149, 149}})
		}
	}
}
