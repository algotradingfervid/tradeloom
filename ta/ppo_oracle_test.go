//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length, every MAType and a spread of fast/slow periods.

func TestPPOOracle(t *testing.T) {
	periods := [][2]int{{2, 2}, {2, 3}, {3, 2}, {12, 26}, {26, 12}, {5, 14}, {2, 200}}
	for _, mt := range maAllTypes {
		for _, pp := range periods {
			lb := PPOLookback(pp[0], pp[1], mt)
			if c := mustLookback(t, "PPO", float64(pp[0]), float64(pp[1]), float64(mt)); lb != c {
				t.Fatalf("lookback %v mt=%d: go %d c %d", pp, mt, lb, c)
			}
			for _, n := range edgeLengths(lb) {
				for _, s := range fixtures(n) {
					got, err := PPO(s.Real(), pp[0], pp[1], mt)
					if err != nil {
						t.Fatal(err)
					}
					if n == 0 {
						continue // C rejects endIdx -1; Go returns empty output
					}
					r := mustCall(t, "PPO", realIn(s.Real()), float64(pp[0]), float64(pp[1]), float64(mt))
					if r.RetCode != oracle.Success {
						t.Fatalf("%v mt=%d C retcode %d", pp, mt, r.RetCode)
					}
					assertReal(t, lbl("PPO ", s.Name, " n=", n, " p=", pp, " mt=", mt), got, r.AlignedReal(0, n), relTol)
				}
			}
		}
	}
	// Maximum periods: lookback only (and a short series fully inside it).
	for _, mt := range []MAType{MATypeSMA, MATypeEMA, MATypeDefault} {
		lb := PPOLookback(100000, 100000, mt)
		if c := mustLookback(t, "PPO", 100000, 100000, float64(mt)); lb != c {
			t.Fatalf("max lookback mt=%d: go %d c %d", mt, lb, c)
		}
		in := fixtures(50)[0].Real()
		got, _ := PPO(in, 2, 100000, mt)
		r := mustCall(t, "PPO", realIn(in), 2, 100000, float64(mt))
		assertReal(t, lbl("PPO max mt=", mt), got, r.AlignedReal(0, len(in)), relTol)
	}
}

func TestPPOOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	cases := []struct {
		f, s int
		mt   MAType
	}{
		{1, 26, MATypeEMA}, {0, 26, MATypeEMA}, {100001, 26, MATypeEMA},
		{12, 1, MATypeEMA}, {12, -5, MATypeEMA}, {12, 100001, MATypeEMA},
		{12, 26, -1}, {12, 26, 14}, {12, 26, 99},
		{2, 2, MATypeSMA}, {100000, 100000, MATypeSMA}, {12, 26, MATypeDefault}, {12, 26, MATypeRMA},
	}
	for _, c := range cases {
		r := mustCall(t, "PPO", realIn(in), float64(c.f), float64(c.s), float64(c.mt))
		_, err := PPO(in, c.f, c.s, c.mt)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("%+v: C retcode %d, Go err %v", c, r.RetCode, err)
		}
		if err == nil && r.RetCode != oracle.Success {
			t.Errorf("%+v: C retcode %d, Go ok", c, r.RetCode)
		}
		if lb, clb := PPOLookback(c.f, c.s, c.mt), mustLookback(t, "PPO", float64(c.f), float64(c.s), float64(c.mt)); lb != clb {
			t.Errorf("%+v: lookback go %d c %d", c, lb, clb)
		}
	}
}

func TestPPOOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.Real()
	for _, mt := range maAllTypes {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {0, 32}, {33, 33}, {40, 119}} {
			for _, pp := range [][2]int{{2, 4}, {10, 4}, {3, 10}} {
				out := make([]float64, len(in))
				beg, nb, err := ppoRange(rg[0], rg[1], in, pp[0], pp[1], mt, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "PPO", realIn(in), rg[0], rg[1], float64(pp[0]), float64(pp[1]), float64(mt))
				if r.RetCode != oracle.Success {
					t.Fatalf("range %v %v mt=%d: C retcode %d", rg, pp, mt, r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("range %v %v mt=%d: go (%d,%d) c (%d,%d)", rg, pp, mt, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("PPO range ", rg, " p=", pp, " mt=", mt), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

func TestPPOOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	SetUnstablePeriod(UnstEMA, 15)
	oracle.SetUnstablePeriod(int(UnstEMA), 15)
	for _, n := range []int{10, 40, 41, 42, 100, 257} {
		for _, s := range fixtures(n) {
			for _, mt := range []MAType{MATypeEMA, MATypeDEMA, MATypeDefault} {
				got, err := PPO(s.Real(), 5, 12, mt)
				if err != nil {
					t.Fatal(err)
				}
				if lb, c := PPOLookback(5, 12, mt), mustLookback(t, "PPO", 5, 12, float64(mt)); lb != c {
					t.Fatalf("unstable lookback mt=%d: go %d c %d", mt, lb, c)
				}
				r := mustCall(t, "PPO", realIn(s.Real()), 5, 12, float64(mt))
				assertReal(t, lbl("PPO unstable ", s.Name, " n=", n, " mt=", mt), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}
