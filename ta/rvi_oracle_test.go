//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

var rviOracleParams = [][2]int{{1, 2}, {2, 2}, {1, 10}, {14, 10}, {3, 5}, {5, 20}, {30, 3}, {14, 50}, {200, 2}, {2, 200}}

func rviOracleCompare(t *testing.T, prefix string, params [][2]int) {
	t.Helper()
	for _, p := range params {
		lb := RVILookback(p[0], p[1])
		if c := mustLookback(t, "RVI", float64(p[0]), float64(p[1])); lb != c {
			t.Fatalf("%slookback p=%v: go %d c %d", prefix, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := RVI(s.Real(), p[0], p[1])
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "RVI", realIn(s.Real()), float64(p[0]), float64(p[1]))
				if r.RetCode != oracle.Success {
					t.Fatalf("%sC retcode %d", prefix, r.RetCode)
				}
				assertReal(t, lbl(prefix, "RVI ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestRVIOracle(t *testing.T) {
	rviOracleCompare(t, "", rviOracleParams)
	// Max periods: lookback only (the series would need 100000+ bars).
	for _, p := range [][2]int{{100000, 2}, {1, 100000}, {100000, 100000}} {
		if lb, c := RVILookback(p[0], p[1]), mustLookback(t, "RVI", float64(p[0]), float64(p[1])); lb != c {
			t.Fatalf("lookback p=%v: go %d c %d", p, lb, c)
		}
	}
}

func TestRVIOracleBadParams(t *testing.T) {
	in := fixtures(40)[0].Real()
	for _, p := range [][2]int{{0, 10}, {-5, 10}, {100001, 10}, {14, 1}, {14, 0}, {14, 100001},
		{1, 2}, {100000, 2}, {1, 100000}, {14, 10}} {
		r := mustCall(t, "RVI", realIn(in), float64(p[0]), float64(p[1]))
		_, err := RVI(in, p[0], p[1])
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%v: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "RVI", float64(p[0]), float64(p[1])); c != RVILookback(p[0], p[1]) {
			t.Errorf("lookback p=%v: go %d c %d", p, RVILookback(p[0], p[1]), c)
		}
	}
}

func rviOracleRanges(t *testing.T, prefix string, ranges [][2]int, params [][2]int) {
	t.Helper()
	for _, s := range []series{fixtures(150)[1], fixtures(150)[3], fixtures(150)[11]} {
		in := s.Real()
		for _, rg := range ranges {
			for _, p := range params {
				out := make([]float64, len(in))
				beg, nb, err := rviRange(rg[0], rg[1], in, p[0], p[1], out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "RVI", realIn(in), rg[0], rg[1], float64(p[0]), float64(p[1]))
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s%s range %v p=%v: go (%d,%d) c (%d,%d)", prefix, s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl(prefix, "RVI ", s.Name, " range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

func TestRVIOracleRanges(t *testing.T) {
	rviOracleRanges(t, "",
		[][2]int{{0, 0}, {0, 149}, {3, 3}, {10, 50}, {149, 149}, {0, 8}, {9, 9}, {22, 22}, {21, 23}, {100, 149}},
		[][2]int{{1, 2}, {2, 2}, {14, 10}, {5, 3}})
}

func TestRVIOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 37} {
		SetUnstablePeriod(UnstRVI, u)
		oracle.SetUnstablePeriod(int(UnstRVI), u)
		rviOracleCompare(t, lbl("unst=", u, " "), [][2]int{{1, 2}, {2, 2}, {14, 10}, {5, 20}})
		rviOracleRanges(t, lbl("unst=", u, " "),
			[][2]int{{0, 149}, {60, 60}, {0, 10}, {70, 149}, {149, 149}},
			[][2]int{{1, 2}, {3, 5}, {14, 10}})
	}
	// Another function's unstable period must not leak into RVI.
	SetAllUnstablePeriods(0)
	oracle.ResetUnstablePeriods()
	SetUnstablePeriod(UnstRSI, 9)
	oracle.SetUnstablePeriod(int(UnstRSI), 9)
	rviOracleCompare(t, "rsi-unst ", [][2]int{{3, 5}, {14, 10}})
}
