//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length, every MAType and a spread of periods.

func pvoVolIn(v []float64) oracle.Inputs {
	return oracle.Inputs{Volume: v}
}

func pvoOracleSeries(t *testing.T, fast, slow int, mt MAType) {
	t.Helper()
	lb := PVOLookback(fast, slow, mt)
	if c := mustLookback(t, "PVO", float64(fast), float64(slow), float64(mt)); lb != c {
		t.Fatalf("lookback fast=%d slow=%d mt=%d: go %d c %d", fast, slow, mt, lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			// Drive PVO with the real volume and, for wider coverage
			// (negative, zero-crossing and tiny values), with the close.
			for k, v := range [][]float64{s.V, s.C} {
				got, err := PVO(v, fast, slow, mt)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "PVO", pvoVolIn(v), float64(fast), float64(slow), float64(mt))
				if r.RetCode != oracle.Success {
					t.Fatalf("fast=%d slow=%d mt=%d C retcode %d", fast, slow, mt, r.RetCode)
				}
				assertReal(t, lbl("PVO ", s.Name, " k=", k, " n=", n, " fast=", fast, " slow=", slow, " mt=", mt), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestPVOOracle(t *testing.T) {
	for _, mt := range maAllTypes {
		for _, p := range [][2]int{{2, 2}, {2, 3}, {3, 2}, {12, 26}, {26, 12}, {5, 14}, {10, 50}} {
			pvoOracleSeries(t, p[0], p[1], mt)
		}
	}
	// Large periods (lookback beyond most fixture lengths).
	for _, mt := range []MAType{MATypeSMA, MATypeEMA, MATypeDefault} {
		pvoOracleSeries(t, 2, 400, mt)
		pvoOracleSeries(t, 1000, 3, mt)
	}
}

func TestPVOOracleLookbackExtremes(t *testing.T) {
	for _, mt := range maAllTypes {
		for _, p := range [][2]int{{2, 100000}, {100000, 2}, {100000, 100000}} {
			lb := PVOLookback(p[0], p[1], mt)
			if c := mustLookback(t, "PVO", float64(p[0]), float64(p[1]), float64(mt)); lb != c {
				t.Errorf("lookback %v mt=%d: go %d c %d", p, mt, lb, c)
			}
		}
	}
}

func TestPVOOracleBadParams(t *testing.T) {
	in := fixtures(30)[0].V
	cases := []struct {
		fast, slow int
		mt         MAType
	}{
		{1, 26, MATypeEMA}, {0, 26, MATypeEMA}, {-5, 26, MATypeEMA}, {100001, 26, MATypeEMA},
		{12, 1, MATypeEMA}, {12, 0, MATypeEMA}, {12, 100001, MATypeEMA},
		{12, 26, -1}, {12, 26, 14}, {12, 26, 99},
		{2, 2, MATypeSMA}, {100000, 100000, MATypeSMA}, {12, 26, MATypeDefault}, {12, 26, MATypeRMA}, {12, 26, MATypeDisabled},
	}
	for _, c := range cases {
		r := mustCall(t, "PVO", pvoVolIn(in), float64(c.fast), float64(c.slow), float64(c.mt))
		_, err := PVO(in, c.fast, c.slow, c.mt)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("%v: C retcode %d, Go err %v", c, r.RetCode, err)
		}
		if err == nil && r.RetCode != oracle.Success {
			t.Errorf("%v: C retcode %d, Go ok", c, r.RetCode)
		}
		lb := PVOLookback(c.fast, c.slow, c.mt)
		if cl := mustLookback(t, "PVO", float64(c.fast), float64(c.slow), float64(c.mt)); lb != cl {
			t.Errorf("%v: lookback go %d c %d", c, lb, cl)
		}
	}
}

func TestPVOOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.V
	ranges := [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {0, 25}, {25, 25}, {26, 26}, {20, 30}, {40, 119}}
	for _, mt := range maAllTypes {
		for _, rg := range ranges {
			for _, p := range [][2]int{{2, 3}, {12, 26}, {10, 4}} {
				out := make([]float64, len(in))
				beg, nb, err := pvoRange(rg[0], rg[1], in, p[0], p[1], mt, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "PVO", pvoVolIn(in), rg[0], rg[1], float64(p[0]), float64(p[1]), float64(mt))
				if r.RetCode != oracle.Success {
					t.Fatalf("range %v %v mt=%d: C retcode %d", rg, p, mt, r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("range %v %v mt=%d: go (%d,%d) c (%d,%d)", rg, p, mt, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("PVO range ", rg, " ", p, " mt=", mt), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

func TestPVOOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 30} {
		for _, id := range []FuncUnstID{UnstEMA, UnstKAMA, UnstMAMA, UnstT3, UnstRMA} {
			SetUnstablePeriod(id, u)
			oracle.SetUnstablePeriod(int(id), u)
		}
		for _, mt := range []MAType{MATypeEMA, MATypeDEMA, MATypeTEMA, MATypeKAMA, MATypeMAMA, MATypeT3, MATypeRMA, MATypeZLEMA, MATypeDefault} {
			pvoOracleSeries(t, 5, 12, mt)
		}
		for _, rg := range [][2]int{{0, 119}, {30, 60}, {50, 50}} {
			in := fixtures(120)[0].V
			out := make([]float64, len(in))
			beg, nb, err := pvoRange(rg[0], rg[1], in, 12, 26, MATypeEMA, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "PVO", pvoVolIn(in), rg[0], rg[1], 12, 26, float64(MATypeEMA))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("unstable %d range %v: go (%d,%d) c (%d,%d)", u, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("PVO unstable ", u, " range ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
