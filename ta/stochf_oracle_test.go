//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length, every MAType and a spread of periods.

func stochfOracleSeries(t *testing.T, sets []stochfParams) {
	t.Helper()
	for _, p := range sets {
		opt := []float64{float64(p.k), float64(p.d), float64(p.mt)}
		lb := STOCHFLookback(p.k, p.d, p.mt)
		if c := mustLookback(t, "STOCHF", opt...); lb != c {
			t.Fatalf("lookback %v: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				k, d, err := STOCHF(s.H, s.L, s.C, p.k, p.d, p.mt)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "STOCHF", s.ohlcv(), opt...)
				if r.RetCode != oracle.Success {
					t.Fatalf("%v C retcode %d", p, r.RetCode)
				}
				l := lbl("STOCHF ", s.Name, " n=", n, " ", p)
				assertReal(t, l+" K", k, r.AlignedReal(0, n), relTol)
				assertReal(t, l+" D", d, r.AlignedReal(1, n), relTol)
			}
		}
	}
}

func TestSTOCHFOracle(t *testing.T) {
	var sets []stochfParams
	for _, mt := range maAllTypes {
		sets = append(sets, stochfParams{5, 3, mt}, stochfParams{14, 1, mt}, stochfParams{2, 7, mt})
	}
	sets = append(sets,
		stochfParams{1, 1, MATypeSMA}, stochfParams{1, 2, MATypeEMA}, stochfParams{3, 3, MATypeSMA},
		stochfParams{30, 10, MATypeEMA}, stochfParams{200, 3, MATypeSMA}, stochfParams{5, 200, MATypeSMA},
		stochfParams{100000, 1, MATypeSMA}, stochfParams{1, 100000, MATypeWMA},
	)
	stochfOracleSeries(t, sets)
}

func TestSTOCHFOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	cases := []stochfParams{
		{0, 3, MATypeSMA}, {-5, 3, MATypeSMA}, {100001, 3, MATypeSMA},
		{5, 0, MATypeSMA}, {5, -5, MATypeSMA}, {5, 100001, MATypeSMA},
		{5, 3, -1}, {5, 3, 14}, {5, 3, 99},
		{1, 1, MATypeSMA}, {100000, 100000, MATypeSMA}, {5, 3, MATypeDefault}, {5, 3, MATypeRMA},
	}
	for _, p := range cases {
		r := mustCall(t, "STOCHF", s.ohlcv(), float64(p.k), float64(p.d), float64(p.mt))
		_, _, err := STOCHF(s.H, s.L, s.C, p.k, p.d, p.mt)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("%v: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if err == nil && r.RetCode != oracle.Success {
			t.Errorf("%v: C retcode %d, Go ok", p, r.RetCode)
		}
		if c := mustLookback(t, "STOCHF", float64(p.k), float64(p.d), float64(p.mt)); c != STOCHFLookback(p.k, p.d, p.mt) {
			t.Errorf("%v: lookback go %d c %d", p, STOCHFLookback(p.k, p.d, p.mt), c)
		}
	}
}

func stochfOracleRanges(t *testing.T, s series, sets []stochfParams, ranges [][2]int) {
	t.Helper()
	for _, p := range sets {
		for _, rg := range ranges {
			k := make([]float64, rg[1]-rg[0]+1)
			d := make([]float64, rg[1]-rg[0]+1)
			beg, nb, err := stochfRange(rg[0], rg[1], s.H, s.L, s.C, p.k, p.d, p.mt, k, d)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "STOCHF", s.ohlcv(), rg[0], rg[1], float64(p.k), float64(p.d), float64(p.mt))
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v %v: C retcode %d", rg, p, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v %v: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			l := lbl("STOCHF range ", rg, " ", p)
			assertReal(t, l+" K", k[:nb], r.Outputs[0].Real, relTol)
			assertReal(t, l+" D", d[:nb], r.Outputs[1].Real, relTol)
		}
	}
}

func TestSTOCHFOracleRanges(t *testing.T) {
	var sets []stochfParams
	for _, mt := range maAllTypes {
		sets = append(sets, stochfParams{5, 3, mt}, stochfParams{1, 4, mt})
	}
	sets = append(sets, stochfParams{10, 10, MATypeEMA}, stochfParams{1, 1, MATypeSMA})
	ranges := [][2]int{{0, 0}, {0, 119}, {3, 3}, {6, 6}, {7, 7}, {10, 50}, {119, 119}, {0, 8}, {9, 9},
		{0, 32}, {33, 33}, {40, 119}, {100, 119}}
	for _, i := range []int{1, 9, 10} { // walk2, zigzag, doji
		stochfOracleRanges(t, fixtures(120)[i], sets, ranges)
	}
}

func TestSTOCHFOracleUnstable(t *testing.T) {
	defer func() {
		SetAllUnstablePeriods(0)
		oracle.ResetUnstablePeriods()
	}()
	cases := []struct {
		id FuncUnstID
		mt MAType
	}{
		{UnstEMA, MATypeEMA}, {UnstKAMA, MATypeKAMA}, {UnstT3, MATypeT3}, {UnstMAMA, MATypeMAMA}, {UnstRMA, MATypeRMA},
	}
	for _, c := range cases {
		for _, u := range []int{1, 7, 40} {
			if err := SetUnstablePeriod(c.id, u); err != nil {
				t.Fatal(err)
			}
			oracle.SetUnstablePeriod(int(c.id), u)
			sets := []stochfParams{{5, 3, c.mt}, {14, 9, c.mt}}
			stochfOracleSeries(t, sets)
			stochfOracleRanges(t, fixtures(150)[0], sets, [][2]int{{0, 149}, {20, 60}, {149, 149}})
			SetAllUnstablePeriods(0)
			oracle.ResetUnstablePeriods()
		}
	}
}
