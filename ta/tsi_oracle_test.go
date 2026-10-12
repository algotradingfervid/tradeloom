//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

var tsiOracleParams = [][2]int{{2, 2}, {2, 3}, {3, 2}, {5, 3}, {25, 13}, {13, 25}, {40, 20}, {100, 2}}

func tsiOracleCompare(t *testing.T, tag string, p [2]int) {
	t.Helper()
	lb := TSILookback(p[0], p[1])
	if c := mustLookback(t, "TSI", float64(p[0]), float64(p[1])); lb != c {
		t.Fatalf("%slookback p=%v: go %d c %d", tag, p, lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := TSI(s.Real(), p[0], p[1])
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "TSI", realIn(s.Real()), float64(p[0]), float64(p[1]))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl(tag, "TSI ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
		}
	}
}

func TestTSIOracle(t *testing.T) {
	for _, p := range tsiOracleParams {
		tsiOracleCompare(t, "", p)
	}
	// Maximum periods: lookback 199999, so only check the lookback and a short
	// series that sits entirely inside it.
	for _, p := range [][2]int{{100000, 100000}, {100000, 2}, {2, 100000}} {
		lb := TSILookback(p[0], p[1])
		if c := mustLookback(t, "TSI", float64(p[0]), float64(p[1])); lb != c {
			t.Fatalf("lookback p=%v: go %d c %d", p, lb, c)
		}
		s := fixtures(300)[0]
		got, _ := TSI(s.Real(), p[0], p[1])
		r := mustCall(t, "TSI", realIn(s.Real()), float64(p[0]), float64(p[1]))
		assertReal(t, lbl("TSI max p=", p), got, r.AlignedReal(0, 300), relTol)
	}
}

func TestTSIOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range [][2]int{{1, 13}, {0, 13}, {-5, 13}, {100001, 13}, {25, 1}, {25, 0}, {25, 100001}, {2, 2}, {100000, 100000}} {
		r := mustCall(t, "TSI", realIn(in), float64(p[0]), float64(p[1]))
		_, err := TSI(in, p[0], p[1])
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%v: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if lb, c := TSILookback(p[0], p[1]), mustLookback(t, "TSI", float64(p[0]), float64(p[1])); lb != c {
			t.Errorf("p=%v lookback: go %d c %d", p, lb, c)
		}
	}
}

func TestTSIOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[5], fixtures(120)[9]} {
		in := s.Real()
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {2, 4}, {30, 30}, {40, 119}} {
			for _, p := range [][2]int{{2, 2}, {5, 3}, {10, 10}} {
				out := make([]float64, len(in))
				beg, nb, err := tsiRange(rg[0], rg[1], in, p[0], p[1], out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "TSI", realIn(in), rg[0], rg[1], float64(p[0]), float64(p[1]))
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%v: go (%d,%d) c (%d,%d)", s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("TSI range ", s.Name, rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

func TestTSIOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 30} {
		SetUnstablePeriod(UnstEMA, u)
		oracle.SetUnstablePeriod(int(UnstEMA), u)
		for _, p := range [][2]int{{2, 2}, {5, 3}, {25, 13}} {
			tsiOracleCompare(t, lbl("unst=", u, " "), p)
		}
	}
}
