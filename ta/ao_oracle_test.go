//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

var aoOraclePeriods = [][2]int{
	{2, 2}, {2, 3}, {3, 2}, {5, 34}, {34, 5}, {4, 4}, {10, 30}, {2, 200}, {200, 50},
}

func TestAOOracle(t *testing.T) {
	for _, pp := range aoOraclePeriods {
		lb := AOLookback(pp[0], pp[1])
		if c := mustLookback(t, "AO", float64(pp[0]), float64(pp[1])); lb != c {
			t.Fatalf("lookback p=%v: go %d c %d", pp, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := AO(s.H, s.L, pp[0], pp[1])
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "AO", s.ohlcv(), float64(pp[0]), float64(pp[1]))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("AO ", s.Name, " n=", n, " p=", pp), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestAOOracleMaxPeriod(t *testing.T) {
	for _, pp := range [][2]int{{5, 100000}, {100000, 2}, {100000, 100000}} {
		lb := AOLookback(pp[0], pp[1])
		if c := mustLookback(t, "AO", float64(pp[0]), float64(pp[1])); lb != c {
			t.Fatalf("lookback p=%v: go %d c %d", pp, lb, c)
		}
		for _, n := range []int{lb, lb + 1, lb + 50} {
			s := fixtures(n)[0]
			got, err := AO(s.H, s.L, pp[0], pp[1])
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "AO", s.ohlcv(), float64(pp[0]), float64(pp[1]))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("AO max n=", n, " p=", pp), got, r.AlignedReal(0, n), relTol)
		}
	}
}

func TestAOOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, pp := range [][2]int{{1, 34}, {0, 34}, {-5, 34}, {100001, 34}, {5, 1}, {5, 0}, {5, 100001}, {2, 2}, {100000, 100000}} {
		r := mustCall(t, "AO", s.ohlcv(), float64(pp[0]), float64(pp[1]))
		_, err := AO(s.H, s.L, pp[0], pp[1])
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%v: C retcode %d, Go err %v", pp, r.RetCode, err)
		}
		if (r.RetCode == oracle.BadParam) != (AOLookback(pp[0], pp[1]) == -1) {
			t.Errorf("p=%v: C retcode %d, Go lookback %d", pp, r.RetCode, AOLookback(pp[0], pp[1]))
		}
	}
}

func TestAOOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[4], fixtures(120)[11]} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {33, 33}, {33, 119}, {5, 40}} {
			for _, pp := range [][2]int{{2, 3}, {5, 10}, {10, 5}, {5, 34}} {
				out := make([]float64, len(s.H))
				beg, nb, err := aoRange(rg[0], rg[1], s.H, s.L, pp[0], pp[1], out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "AO", s.ohlcv(), rg[0], rg[1], float64(pp[0]), float64(pp[1]))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%v: go (%d,%d) c (%d,%d)", s.Name, rg, pp, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("AO range ", s.Name, " ", rg, " p=", pp), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}
