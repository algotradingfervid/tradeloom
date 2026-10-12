//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

var adoscOracleParams = [][2]int{{2, 2}, {2, 3}, {3, 10}, {10, 3}, {5, 5}, {12, 26}, {2, 200}, {200, 2}}

func TestADOSCOracle(t *testing.T) {
	for _, pp := range adoscOracleParams {
		lb := ADOSCLookback(pp[0], pp[1])
		if c := mustLookback(t, "ADOSC", float64(pp[0]), float64(pp[1])); lb != c {
			t.Fatalf("lookback p=%v: go %d c %d", pp, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := ADOSC(s.H, s.L, s.C, s.V, pp[0], pp[1])
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "ADOSC", s.ohlcv(), float64(pp[0]), float64(pp[1]))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("ADOSC ", s.Name, " n=", n, " p=", pp), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestADOSCOracleLookbackExtremes(t *testing.T) {
	for _, pp := range [][2]int{{100000, 100000}, {2, 100000}, {100000, 2}, {1, 10}, {3, 1}, {100001, 3}, {3, 100001}, {0, 0}} {
		if g, c := ADOSCLookback(pp[0], pp[1]), mustLookback(t, "ADOSC", float64(pp[0]), float64(pp[1])); g != c {
			t.Errorf("lookback %v: go %d c %d", pp, g, c)
		}
	}
}

func TestADOSCOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, pp := range [][2]int{{1, 10}, {0, 10}, {-5, 10}, {100001, 10}, {3, 1}, {3, 0}, {3, 100001}, {2, 2}, {100000, 100000}} {
		r := mustCall(t, "ADOSC", s.ohlcv(), float64(pp[0]), float64(pp[1]))
		_, err := ADOSC(s.H, s.L, s.C, s.V, pp[0], pp[1])
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%v: C retcode %d, Go err %v", pp, r.RetCode, err)
		}
	}
}

func TestADOSCOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[11]} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {25, 25}, {30, 119}} {
			for _, pp := range [][2]int{{3, 10}, {10, 3}, {2, 26}} {
				out := make([]float64, 120)
				beg, nb, err := adoscRange(rg[0], rg[1], s.H, s.L, s.C, s.V, pp[0], pp[1], out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "ADOSC", s.ohlcv(), rg[0], rg[1], float64(pp[0]), float64(pp[1]))
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%v: go (%d,%d) c (%d,%d)", s.Name, rg, pp, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("ADOSC range ", s.Name, rg, " p=", pp), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

func TestADOSCOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 30} {
		SetUnstablePeriod(UnstEMA, u)
		oracle.SetUnstablePeriod(int(UnstEMA), u)
		for _, pp := range [][2]int{{3, 10}, {10, 3}} {
			lb := ADOSCLookback(pp[0], pp[1])
			if c := mustLookback(t, "ADOSC", float64(pp[0]), float64(pp[1])); lb != c {
				t.Fatalf("unstable %d lookback p=%v: go %d c %d", u, pp, lb, c)
			}
			for _, n := range edgeLengths(lb) {
				if n == 0 {
					continue
				}
				for _, s := range fixtures(n) {
					got, err := ADOSC(s.H, s.L, s.C, s.V, pp[0], pp[1])
					if err != nil {
						t.Fatal(err)
					}
					r := mustCall(t, "ADOSC", s.ohlcv(), float64(pp[0]), float64(pp[1]))
					assertReal(t, lbl("ADOSC unstable ", u, " ", s.Name, " n=", n, " p=", pp), got, r.AlignedReal(0, n), relTol)
				}
			}
		}
	}
}
