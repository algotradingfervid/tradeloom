//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

var acOraclePeriods = [][3]int{
	{2, 2, 2}, {2, 3, 2}, {3, 2, 2}, {5, 34, 5}, {34, 5, 5}, {4, 4, 3},
	{10, 30, 32}, {10, 30, 33}, {2, 200, 7}, {200, 50, 2}, {5, 34, 100},
}

func TestACOracle(t *testing.T) {
	for _, pp := range acOraclePeriods {
		lb := ACLookback(pp[0], pp[1], pp[2])
		if c := mustLookback(t, "AC", float64(pp[0]), float64(pp[1]), float64(pp[2])); lb != c {
			t.Fatalf("lookback p=%v: go %d c %d", pp, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := AC(s.H, s.L, pp[0], pp[1], pp[2])
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "AC", s.ohlcv(), float64(pp[0]), float64(pp[1]), float64(pp[2]))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("AC ", s.Name, " n=", n, " p=", pp), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestACOracleMaxPeriod(t *testing.T) {
	for _, pp := range [][3]int{{5, 100000, 5}, {100000, 2, 2}, {5, 34, 100000}, {100000, 100000, 100000}} {
		lb := ACLookback(pp[0], pp[1], pp[2])
		if c := mustLookback(t, "AC", float64(pp[0]), float64(pp[1]), float64(pp[2])); lb != c {
			t.Fatalf("lookback p=%v: go %d c %d", pp, lb, c)
		}
		for _, n := range []int{lb, lb + 1, lb + 50} {
			s := fixtures(n)[0]
			got, err := AC(s.H, s.L, pp[0], pp[1], pp[2])
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "AC", s.ohlcv(), float64(pp[0]), float64(pp[1]), float64(pp[2]))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("AC max n=", n, " p=", pp), got, r.AlignedReal(0, n), relTol)
		}
	}
}

func TestACOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, pp := range [][3]int{
		{1, 34, 5}, {0, 34, 5}, {-5, 34, 5}, {100001, 34, 5},
		{5, 1, 5}, {5, 0, 5}, {5, 100001, 5},
		{5, 34, 1}, {5, 34, 0}, {5, 34, -3}, {5, 34, 100001},
		{2, 2, 2}, {100000, 100000, 100000},
	} {
		r := mustCall(t, "AC", s.ohlcv(), float64(pp[0]), float64(pp[1]), float64(pp[2]))
		_, err := AC(s.H, s.L, pp[0], pp[1], pp[2])
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%v: C retcode %d, Go err %v", pp, r.RetCode, err)
		}
		lb := ACLookback(pp[0], pp[1], pp[2])
		if (r.RetCode == oracle.BadParam) != (lb == -1) {
			t.Errorf("p=%v: C retcode %d, Go lookback %d", pp, r.RetCode, lb)
		}
		if c := mustLookback(t, "AC", float64(pp[0]), float64(pp[1]), float64(pp[2])); c != lb {
			t.Errorf("p=%v: lookback go %d c %d", pp, lb, c)
		}
	}
}

func TestACOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[4], fixtures(120)[11]} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {11, 11}, {37, 37}, {37, 119}, {5, 40}, {36, 38}} {
			for _, pp := range [][3]int{{2, 3, 2}, {5, 10, 3}, {10, 5, 3}, {5, 34, 5}} {
				out := make([]float64, len(s.H))
				beg, nb, err := acRange(rg[0], rg[1], s.H, s.L, pp[0], pp[1], pp[2], out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "AC", s.ohlcv(), rg[0], rg[1], float64(pp[0]), float64(pp[1]), float64(pp[2]))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%v: go (%d,%d) c (%d,%d)", s.Name, rg, pp, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("AC range ", s.Name, " ", rg, " p=", pp), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}
