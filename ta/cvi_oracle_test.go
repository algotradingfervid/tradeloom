//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func cviOracleFull(t *testing.T, params [][2]int, tag string) {
	t.Helper()
	for _, pp := range params {
		lb := CVILookback(pp[0], pp[1])
		if c := mustLookback(t, "CVI", float64(pp[0]), float64(pp[1])); lb != c {
			t.Fatalf("%s lookback p=%v: go %d c %d", tag, pp, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := CVI(s.H, s.L, pp[0], pp[1])
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "CVI", s.ohlcv(), float64(pp[0]), float64(pp[1]))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl(tag, " CVI ", s.Name, " n=", n, " p=", pp), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func cviOracleRanges(t *testing.T, s series, params [][2]int, ranges [][2]int, tag string) {
	t.Helper()
	for _, rg := range ranges {
		for _, pp := range params {
			out := make([]float64, len(s.H))
			beg, nb, err := cviRange(rg[0], rg[1], s.H, s.L, pp[0], pp[1], out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CVI", s.ohlcv(), rg[0], rg[1], float64(pp[0]), float64(pp[1]))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s %s range %v p=%v: go (%d,%d) c (%d,%d)", tag, s.Name, rg, pp, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl(tag, " CVI range ", s.Name, " ", rg, " p=", pp), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}

func TestCVIOracle(t *testing.T) {
	// Includes min (2,1), defaults (10,10), ring sizes around the C stack
	// buffer of 32 (31, 32, 33), and large periods.
	cviOracleFull(t, [][2]int{
		{2, 1}, {2, 2}, {3, 1}, {5, 3}, {10, 10}, {14, 7}, {7, 31}, {4, 32}, {4, 33},
		{30, 100}, {200, 1}, {2, 500},
	}, "")
}

func TestCVIOracleMaxParams(t *testing.T) {
	// Max periods: lookback 199999. Only lengths around the lookback are
	// practical.
	for _, pp := range [][2]int{{100000, 100000}, {100000, 1}, {2, 100000}} {
		lb := CVILookback(pp[0], pp[1])
		if c := mustLookback(t, "CVI", float64(pp[0]), float64(pp[1])); lb != c {
			t.Fatalf("lookback p=%v: go %d c %d", pp, lb, c)
		}
		for _, n := range []int{lb, lb + 1, lb + 3} {
			s := fixtures(n)[0]
			got, err := CVI(s.H, s.L, pp[0], pp[1])
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "CVI", s.ohlcv(), float64(pp[0]), float64(pp[1]))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("CVI max n=", n, " p=", pp), got, r.AlignedReal(0, n), relTol)
		}
	}
}

func TestCVIOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, pp := range [][2]int{
		{1, 10}, {0, 10}, {-5, 10}, {100001, 10},
		{10, 0}, {10, -5}, {10, 100001},
		{2, 1}, {100000, 100000},
	} {
		r := mustCall(t, "CVI", s.ohlcv(), float64(pp[0]), float64(pp[1]))
		_, err := CVI(s.H, s.L, pp[0], pp[1])
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%v: C retcode %d, Go err %v", pp, r.RetCode, err)
		}
		lbC := mustLookback(t, "CVI", float64(pp[0]), float64(pp[1]))
		if lb := CVILookback(pp[0], pp[1]); lb != lbC {
			t.Errorf("lookback p=%v: go %d c %d", pp, lb, lbC)
		}
	}
}

func TestCVIOracleRanges(t *testing.T) {
	// Lookback for (5,3) is 7; for (3,1) is 3.
	ranges := [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 6}, {7, 7}, {0, 7}, {5, 30}, {60, 119}}
	for _, i := range []int{0, 2, 5, 6, 11} {
		cviOracleRanges(t, fixtures(120)[i], [][2]int{{3, 1}, {5, 3}, {10, 40}}, ranges, "")
	}
}

func TestCVIOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 40} {
		if err := SetUnstablePeriod(UnstEMA, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstEMA), u)
		cviOracleFull(t, [][2]int{{2, 1}, {10, 10}}, lbl("unst=", u))
		lb := 9 + u + 10
		cviOracleRanges(t, fixtures(150)[4], [][2]int{{10, 10}},
			[][2]int{{0, 149}, {20, 20}, {30, 100}, {149, 149}, {0, lb - 1}, {lb, lb}}, lbl("unst=", u))
	}
}
