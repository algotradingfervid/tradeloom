//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length, every MAType and a spread of fast/slow periods.

func apoOracleSeries(t *testing.T, periods [][2]int, types []MAType) {
	t.Helper()
	for _, mt := range types {
		for _, pp := range periods {
			fast, slow := pp[0], pp[1]
			lb := APOLookback(fast, slow, mt)
			if c := mustLookback(t, "APO", float64(fast), float64(slow), float64(mt)); lb != c {
				t.Fatalf("lookback %v mt=%d: go %d c %d", pp, mt, lb, c)
			}
			for _, n := range edgeLengths(lb) {
				for _, s := range fixtures(n) {
					got, err := APO(s.Real(), fast, slow, mt)
					if err != nil {
						t.Fatal(err)
					}
					if n == 0 {
						continue // C rejects endIdx -1; Go returns empty output
					}
					r := mustCall(t, "APO", realIn(s.Real()), float64(fast), float64(slow), float64(mt))
					if r.RetCode != oracle.Success {
						t.Fatalf("%v mt=%d C retcode %d", pp, mt, r.RetCode)
					}
					assertReal(t, lbl("APO ", s.Name, " n=", n, " ", pp, " mt=", mt), got, r.AlignedReal(0, n), relTol)
				}
			}
		}
	}
}

func TestAPOOracle(t *testing.T) {
	apoOracleSeries(t, [][2]int{{2, 2}, {2, 3}, {12, 26}, {26, 12}, {5, 30}, {3, 100}}, maAllTypes)
}

func TestAPOOracleLargePeriods(t *testing.T) {
	apoOracleSeries(t, [][2]int{{2, 300}, {300, 2}}, []MAType{MATypeSMA, MATypeEMA, MATypeWMA})
	// Maximum periods: lookback only (inputs would need 100000 bars).
	for _, mt := range maAllTypes {
		for _, pp := range [][2]int{{100000, 100000}, {2, 100000}, {100000, 2}} {
			if g, c := APOLookback(pp[0], pp[1], mt), mustLookback(t, "APO", float64(pp[0]), float64(pp[1]), float64(mt)); g != c {
				t.Errorf("lookback %v mt=%d: go %d c %d", pp, mt, g, c)
			}
		}
	}
}

func TestAPOOracleBadParams(t *testing.T) {
	in := fixtures(40)[0].Real()
	cases := []struct {
		fast, slow int
		mt         MAType
	}{
		{1, 26, MATypeSMA}, {0, 26, MATypeEMA}, {-5, 26, MATypeEMA}, {100001, 26, MATypeSMA},
		{12, 1, MATypeSMA}, {12, 0, MATypeEMA}, {12, 100001, MATypeSMA},
		{12, 26, -1}, {12, 26, 14}, {12, 26, 99},
		{2, 2, MATypeSMA}, {100000, 26, MATypeSMA}, {12, 100000, MATypeSMA},
		{12, 26, MATypeDefault}, {12, 26, MATypeRMA}, {12, 26, MATypeDisabled},
	}
	for _, c := range cases {
		r := mustCall(t, "APO", realIn(in), float64(c.fast), float64(c.slow), float64(c.mt))
		_, err := APO(in, c.fast, c.slow, c.mt)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("%v: C retcode %d, Go err %v", c, r.RetCode, err)
		}
		lb := APOLookback(c.fast, c.slow, c.mt)
		if cl := mustLookback(t, "APO", float64(c.fast), float64(c.slow), float64(c.mt)); lb != cl {
			t.Errorf("%v: lookback go %d c %d", c, lb, cl)
		}
	}
}

func TestAPOOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.Real()
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {0, 25}, {25, 25}, {30, 30}} {
		for _, mt := range maAllTypes {
			for _, pp := range [][2]int{{2, 3}, {12, 26}, {10, 4}} {
				out := make([]float64, len(in))
				beg, nb, err := apoRange(rg[0], rg[1], in, pp[0], pp[1], mt, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "APO", realIn(in), rg[0], rg[1], float64(pp[0]), float64(pp[1]), float64(mt))
				if r.RetCode != oracle.Success {
					t.Fatalf("range %v %v mt=%d: C retcode %d", rg, pp, mt, r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("range %v %v mt=%d: go (%d,%d) c (%d,%d)", rg, pp, mt, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("APO range ", rg, " ", pp, " mt=", mt), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

func TestAPOOracleUnstable(t *testing.T) {
	defer func() {
		SetAllUnstablePeriods(0)
		oracle.ResetUnstablePeriods()
	}()
	cases := []struct {
		id FuncUnstID
		mt MAType
	}{
		{UnstEMA, MATypeEMA}, {UnstEMA, MATypeDefault}, {UnstEMA, MATypeDEMA}, {UnstKAMA, MATypeKAMA},
		{UnstMAMA, MATypeMAMA}, {UnstT3, MATypeT3}, {UnstRMA, MATypeRMA},
	}
	for _, c := range cases {
		for _, u := range []int{0, 7, 40} {
			if err := SetUnstablePeriod(c.id, u); err != nil {
				t.Fatal(err)
			}
			oracle.SetUnstablePeriod(int(c.id), u)
			apoOracleSeries(t, [][2]int{{3, 10}, {12, 26}}, []MAType{c.mt})
		}
		SetAllUnstablePeriods(0)
		oracle.ResetUnstablePeriods()
	}
}
