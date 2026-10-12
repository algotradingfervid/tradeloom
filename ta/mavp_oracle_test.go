//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length, every MAType, several period series and a spread of windows.

var mavpOracleWindows = [][2]int{{1, 1}, {2, 30}, {2, 5}, {5, 14}, {1, 200}, {30, 30}}

func mavpOracleSeries(t *testing.T, windows [][2]int, types []MAType) {
	t.Helper()
	for _, mt := range types {
		for _, w := range windows {
			lb := MAVPLookback(w[0], w[1], mt)
			if c := mustLookback(t, "MAVP", float64(w[0]), float64(w[1]), float64(mt)); lb != c {
				t.Fatalf("lookback %v mt=%d: go %d c %d", w, mt, lb, c)
			}
			for _, n := range edgeLengths(lb) {
				for _, ps := range mavpPeriodSets(n) {
					for _, s := range fixtures(n) {
						got, err := MAVP(s.Real(), ps.P, w[0], w[1], mt)
						if err != nil {
							t.Fatal(err)
						}
						if n == 0 {
							continue // C rejects endIdx -1; Go returns empty output
						}
						r := mustCall(t, "MAVP", realIn(s.Real(), ps.P), float64(w[0]), float64(w[1]), float64(mt))
						if r.RetCode != oracle.Success {
							t.Fatalf("%v mt=%d C retcode %d", w, mt, r.RetCode)
						}
						assertReal(t, lbl("MAVP ", s.Name, "/", ps.Name, " n=", n, " ", w, " mt=", mt), got, r.AlignedReal(0, n), relTol)
					}
				}
			}
		}
	}
}

func TestMAVPOracle(t *testing.T) {
	mavpOracleSeries(t, mavpOracleWindows, maAllTypes)
}

func TestMAVPOracleLookbackExtremes(t *testing.T) {
	for _, mt := range maAllTypes {
		for _, w := range [][2]int{{1, 100000}, {100000, 100000}, {99999, 100000}} {
			lb := MAVPLookback(w[0], w[1], mt)
			if c := mustLookback(t, "MAVP", float64(w[0]), float64(w[1]), float64(mt)); lb != c {
				t.Fatalf("lookback %v mt=%d: go %d c %d", w, mt, lb, c)
			}
		}
	}
}

func TestMAVPOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	per := mavpPeriodSets(20)[2].P
	cases := []struct {
		mn, mx int
		mt     MAType
	}{
		{0, 30, MATypeSMA}, {-5, 30, MATypeEMA}, {100001, 100000, MATypeSMA},
		{2, 0, MATypeSMA}, {2, 100001, MATypeSMA},
		{2, 30, -1}, {2, 30, 14}, {2, 30, 99},
		{31, 30, MATypeSMA}, {100000, 1, MATypeSMA}, {6, 5, MATypeT3},
		{1, 1, MATypeSMA}, {1, 100000, MATypeSMA}, {100000, 100000, MATypeEMA},
		{5, 5, MATypeDefault}, {2, 30, MATypeRMA}, {2, 30, MATypeDisabled},
	}
	for _, c := range cases {
		r := mustCall(t, "MAVP", realIn(in, per), float64(c.mn), float64(c.mx), float64(c.mt))
		_, err := MAVP(in, per, c.mn, c.mx, c.mt)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("%v: C retcode %d, Go err %v", c, r.RetCode, err)
		}
		if err == nil && r.RetCode != oracle.Success {
			t.Errorf("%v: C retcode %d, Go ok", c, r.RetCode)
		}
		if lb := MAVPLookback(c.mn, c.mx, c.mt); lb != mustLookback(t, "MAVP", float64(c.mn), float64(c.mx), float64(c.mt)) {
			t.Errorf("%v: lookback go %d differs from C", c, lb)
		}
	}
}

func TestMAVPOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.Real()
	for _, ps := range mavpPeriodSets(120) {
		for _, mt := range maAllTypes {
			for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {0, 32}, {33, 33}, {40, 119}} {
				for _, w := range [][2]int{{1, 1}, {2, 4}, {3, 10}} {
					out := make([]float64, len(in))
					beg, nb, err := mavpRange(rg[0], rg[1], in, ps.P, w[0], w[1], mt, out)
					if err != nil {
						t.Fatal(err)
					}
					r := mustCallRange(t, "MAVP", realIn(in, ps.P), rg[0], rg[1], float64(w[0]), float64(w[1]), float64(mt))
					if r.RetCode != oracle.Success {
						t.Fatalf("range %v %v mt=%d: C retcode %d", rg, w, mt, r.RetCode)
					}
					if beg != r.BegIdx || nb != r.NbElement {
						t.Fatalf("range %v %v mt=%d %s: go (%d,%d) c (%d,%d)", rg, w, mt, ps.Name, beg, nb, r.BegIdx, r.NbElement)
					}
					assertReal(t, lbl("MAVP range ", rg, " ", w, " mt=", mt, " ", ps.Name), out[:nb], r.Outputs[0].Real, relTol)
				}
			}
		}
	}
}

func TestMAVPOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, id := range []FuncUnstID{UnstEMA, UnstKAMA, UnstMAMA, UnstT3, UnstRMA} {
		SetUnstablePeriod(id, 7)
		oracle.SetUnstablePeriod(int(id), 7)
	}
	mavpOracleSeries(t, [][2]int{{2, 5}, {2, 30}}, maAllTypes)
}
