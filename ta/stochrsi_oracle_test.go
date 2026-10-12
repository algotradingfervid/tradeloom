//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length, every MAType and a spread of periods.

func (p stochrsiParams) opt() []float64 {
	return []float64{float64(p.t), float64(p.k), float64(p.d), float64(p.mt)}
}

func stochrsiOracleSeries(t *testing.T, sets []stochrsiParams) {
	t.Helper()
	for _, p := range sets {
		lb := STOCHRSILookback(p.t, p.k, p.d, p.mt)
		if c := mustLookback(t, "STOCHRSI", p.opt()...); lb != c {
			t.Fatalf("lookback %v: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				k, d, err := STOCHRSI(s.Real(), p.t, p.k, p.d, p.mt)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "STOCHRSI", realIn(s.Real()), p.opt()...)
				if r.RetCode != oracle.Success {
					t.Fatalf("%v C retcode %d", p, r.RetCode)
				}
				l := lbl("STOCHRSI ", s.Name, " n=", n, " ", p)
				assertReal(t, l+" K", k, r.AlignedReal(0, n), relTol)
				assertReal(t, l+" D", d, r.AlignedReal(1, n), relTol)
			}
		}
	}
}

func TestSTOCHRSIOracle(t *testing.T) {
	var sets []stochrsiParams
	for _, mt := range maAllTypes {
		sets = append(sets, stochrsiParams{14, 5, 3, mt}, stochrsiParams{2, 14, 1, mt}, stochrsiParams{5, 2, 7, mt})
	}
	sets = append(sets,
		stochrsiParams{2, 1, 1, MATypeSMA}, stochrsiParams{14, 14, 1, MATypeSMA}, stochrsiParams{3, 1, 2, MATypeEMA},
		stochrsiParams{30, 10, 10, MATypeEMA}, stochrsiParams{200, 3, 3, MATypeSMA}, stochrsiParams{5, 200, 3, MATypeSMA},
		stochrsiParams{5, 3, 200, MATypeSMA}, stochrsiParams{100000, 1, 1, MATypeSMA},
		stochrsiParams{2, 100000, 1, MATypeSMA}, stochrsiParams{2, 1, 100000, MATypeWMA},
	)
	stochrsiOracleSeries(t, sets)
}

func TestSTOCHRSIOracleBadParams(t *testing.T) {
	in := fixtures(30)[0].Real()
	cases := []stochrsiParams{
		{1, 5, 3, MATypeSMA}, {0, 5, 3, MATypeSMA}, {-5, 5, 3, MATypeSMA}, {100001, 5, 3, MATypeSMA},
		{14, 0, 3, MATypeSMA}, {14, -5, 3, MATypeSMA}, {14, 100001, 3, MATypeSMA},
		{14, 5, 0, MATypeSMA}, {14, 5, -5, MATypeSMA}, {14, 5, 100001, MATypeSMA},
		{14, 5, 3, -1}, {14, 5, 3, 14}, {14, 5, 3, 99},
		{2, 1, 1, MATypeSMA}, {100000, 100000, 100000, MATypeSMA}, {14, 5, 3, MATypeDefault}, {14, 5, 3, MATypeRMA},
	}
	for _, p := range cases {
		r := mustCall(t, "STOCHRSI", realIn(in), p.opt()...)
		_, _, err := STOCHRSI(in, p.t, p.k, p.d, p.mt)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("%v: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if err == nil && r.RetCode != oracle.Success {
			t.Errorf("%v: C retcode %d, Go ok", p, r.RetCode)
		}
		if c := mustLookback(t, "STOCHRSI", p.opt()...); c != STOCHRSILookback(p.t, p.k, p.d, p.mt) {
			t.Errorf("%v: lookback go %d c %d", p, STOCHRSILookback(p.t, p.k, p.d, p.mt), c)
		}
	}
}

func stochrsiOracleRanges(t *testing.T, s series, sets []stochrsiParams, ranges [][2]int) {
	t.Helper()
	in := s.Real()
	for _, p := range sets {
		for _, rg := range ranges {
			k := make([]float64, rg[1]-rg[0]+1)
			d := make([]float64, rg[1]-rg[0]+1)
			beg, nb, err := stochrsiRange(rg[0], rg[1], in, p.t, p.k, p.d, p.mt, k, d)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "STOCHRSI", realIn(in), rg[0], rg[1], p.opt()...)
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v %v: C retcode %d", rg, p, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v %v: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			l := lbl("STOCHRSI range ", rg, " ", p)
			assertReal(t, l+" K", k[:nb], r.Outputs[0].Real, relTol)
			assertReal(t, l+" D", d[:nb], r.Outputs[1].Real, relTol)
		}
	}
}

func TestSTOCHRSIOracleRanges(t *testing.T) {
	var sets []stochrsiParams
	for _, mt := range maAllTypes {
		sets = append(sets, stochrsiParams{14, 5, 3, mt}, stochrsiParams{3, 1, 4, mt})
	}
	sets = append(sets, stochrsiParams{10, 10, 10, MATypeEMA}, stochrsiParams{2, 1, 1, MATypeSMA})
	ranges := [][2]int{{0, 0}, {0, 119}, {3, 3}, {6, 6}, {19, 19}, {20, 20}, {21, 21}, {10, 50},
		{119, 119}, {0, 8}, {9, 9}, {0, 32}, {33, 33}, {40, 119}, {100, 119}}
	for _, i := range []int{1, 9, 10} { // walk2, zigzag, doji
		stochrsiOracleRanges(t, fixtures(120)[i], sets, ranges)
	}
}

func TestSTOCHRSIOracleUnstable(t *testing.T) {
	defer func() {
		SetAllUnstablePeriods(0)
		oracle.ResetUnstablePeriods()
	}()
	cases := []struct {
		id FuncUnstID
		mt MAType
	}{
		{UnstRSI, MATypeSMA}, {UnstRSI, MATypeEMA},
		{UnstEMA, MATypeEMA}, {UnstKAMA, MATypeKAMA}, {UnstT3, MATypeT3}, {UnstMAMA, MATypeMAMA}, {UnstRMA, MATypeRMA},
	}
	for _, c := range cases {
		for _, u := range []int{1, 7, 40} {
			if err := SetUnstablePeriod(c.id, u); err != nil {
				t.Fatal(err)
			}
			oracle.SetUnstablePeriod(int(c.id), u)
			sets := []stochrsiParams{{14, 5, 3, c.mt}, {5, 14, 9, c.mt}}
			stochrsiOracleSeries(t, sets)
			stochrsiOracleRanges(t, fixtures(150)[0], sets, [][2]int{{0, 149}, {20, 60}, {149, 149}})
			SetAllUnstablePeriods(0)
			oracle.ResetUnstablePeriods()
		}
	}
	// Unstable periods for both the RSI and the Fast-D MA at once.
	SetUnstablePeriod(UnstRSI, 10)
	oracle.SetUnstablePeriod(int(UnstRSI), 10)
	SetUnstablePeriod(UnstEMA, 5)
	oracle.SetUnstablePeriod(int(UnstEMA), 5)
	sets := []stochrsiParams{{14, 5, 3, MATypeEMA}}
	stochrsiOracleSeries(t, sets)
	stochrsiOracleRanges(t, fixtures(150)[1], sets, [][2]int{{0, 149}, {20, 60}, {149, 149}})
}
