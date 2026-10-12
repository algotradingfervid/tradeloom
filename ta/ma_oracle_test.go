//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length, every MAType and a spread of periods.

func maOracleSeries(t *testing.T, periods []int, types []MAType) {
	t.Helper()
	for _, mt := range types {
		for _, p := range periods {
			lb := MALookback(p, mt)
			if c := mustLookback(t, "MA", float64(p), float64(mt)); lb != c {
				t.Fatalf("lookback p=%d mt=%d: go %d c %d", p, mt, lb, c)
			}
			for _, n := range edgeLengths(lb) {
				for _, s := range fixtures(n) {
					got, err := MA(s.Real(), p, mt)
					if err != nil {
						t.Fatal(err)
					}
					if n == 0 {
						continue // C rejects endIdx -1; Go returns empty output
					}
					r := mustCall(t, "MA", realIn(s.Real()), float64(p), float64(mt))
					if r.RetCode != oracle.Success {
						t.Fatalf("p=%d mt=%d C retcode %d", p, mt, r.RetCode)
					}
					assertReal(t, lbl("MA ", s.Name, " n=", n, " p=", p, " mt=", mt), got, r.AlignedReal(0, n), relTol)
				}
			}
		}
	}
}

func TestMAOracle(t *testing.T) {
	maOracleSeries(t, []int{1, 2, 3, 5, 14, 30, 200}, maAllTypes)
}

func TestMAOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	cases := []struct {
		p  int
		mt MAType
	}{
		{0, MATypeSMA}, {-5, MATypeEMA}, {100001, MATypeSMA},
		{5, -1}, {5, 14}, {5, 99},
		{1, MATypeSMA}, {100000, MATypeSMA}, {5, MATypeDefault}, {5, MATypeRMA},
	}
	for _, c := range cases {
		r := mustCall(t, "MA", realIn(in), float64(c.p), float64(c.mt))
		_, err := MA(in, c.p, c.mt)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d mt=%d: C retcode %d, Go err %v", c.p, c.mt, r.RetCode, err)
		}
		if err == nil && r.RetCode != oracle.Success {
			t.Errorf("p=%d mt=%d: C retcode %d, Go ok", c.p, c.mt, r.RetCode)
		}
	}
}

func TestMAOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.Real()
	for _, mt := range maAllTypes {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {0, 32}, {33, 33}, {40, 119}} {
			for _, p := range []int{1, 4, 10} {
				out := make([]float64, len(in))
				beg, nb, err := maRange(rg[0], rg[1], in, p, mt, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "MA", realIn(in), rg[0], rg[1], float64(p), float64(mt))
				if r.RetCode != oracle.Success {
					t.Fatalf("range %v p=%d mt=%d: C retcode %d", rg, p, mt, r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("range %v p=%d mt=%d: go (%d,%d) c (%d,%d)", rg, p, mt, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("MA range ", rg, " p=", p, " mt=", mt), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

func TestMAOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, id := range []FuncUnstID{UnstEMA, UnstKAMA, UnstMAMA, UnstT3, UnstRMA} {
		SetUnstablePeriod(id, 7)
		oracle.SetUnstablePeriod(int(id), 7)
	}
	maOracleSeries(t, []int{2, 5, 14}, maAllTypes)
}
