//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func eriOracleCompare(t *testing.T, tag string, periods []int) {
	t.Helper()
	for _, p := range periods {
		lb := ERILookback(p)
		if c := mustLookback(t, "ERI", float64(p)); lb != c {
			t.Fatalf("%s lookback p=%d: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				bu, be, err := ERI(s.H, s.L, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "ERI", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				l := lbl(tag, s.Name, " n=", n, " p=", p)
				assertReal(t, "ERI bull "+l, bu, r.AlignedReal(0, n), relTol)
				assertReal(t, "ERI bear "+l, be, r.AlignedReal(1, n), relTol)
			}
		}
	}
}

func TestERIOracle(t *testing.T) {
	eriOracleCompare(t, "", []int{1, 2, 3, 5, 13, 14, 30, 200})
}

func TestERIOracleMaxPeriod(t *testing.T) {
	p := 100000
	if lb, c := ERILookback(p), mustLookback(t, "ERI", float64(p)); lb != c {
		t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
	}
	for _, n := range []int{p - 1, p, p + 1, p + 50} {
		s := fixtures(n)[0]
		bu, be, err := ERI(s.H, s.L, s.C, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "ERI", s.ohlcv(), float64(p))
		assertReal(t, lbl("ERI max bull n=", n), bu, r.AlignedReal(0, n), relTol)
		assertReal(t, lbl("ERI max bear n=", n), be, r.AlignedReal(1, n), relTol)
	}
}

func TestERIOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	// math.MinInt32 is TA_INTEGER_DEFAULT, which C replaces with 13; Go has no
	// sentinel, so it is not included here.
	for _, p := range []int{0, -1, -5, 100001, 1 << 30} {
		r := mustCall(t, "ERI", s.ohlcv(), float64(p))
		_, _, err := ERI(s.H, s.L, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "ERI", float64(p)); c != ERILookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, ERILookback(p), c)
		}
	}
	for _, p := range []int{1, 100000} {
		r := mustCall(t, "ERI", s.ohlcv(), float64(p))
		if _, _, err := ERI(s.H, s.L, s.C, p); err != nil || r.RetCode != oracle.Success {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
	}
}

func eriOracleRanges(t *testing.T, tag string, ss []series, ranges [][2]int, periods []int) {
	t.Helper()
	for _, s := range ss {
		for _, rg := range ranges {
			for _, p := range periods {
				n := len(s.C)
				bu := make([]float64, n)
				be := make([]float64, n)
				beg, nb, err := eriRange(rg[0], rg[1], s.H, s.L, s.C, p, bu, be)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "ERI", s.ohlcv(), rg[0], rg[1], float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s%s range %v p=%d: go (%d,%d) c (%d,%d)", tag, s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("ERI range bull ", tag, s.Name, rg, " p=", p), bu[:nb], r.Outputs[0].Real, relTol)
				assertReal(t, lbl("ERI range bear ", tag, s.Name, rg, " p=", p), be[:nb], r.Outputs[1].Real, relTol)
			}
		}
	}
}

func TestERIOracleRanges(t *testing.T) {
	f := fixtures(120)
	eriOracleRanges(t, "", []series{f[1], f[4], f[13]},
		[][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {8, 9}, {12, 12}, {30, 119}, {100, 105}},
		[]int{1, 2, 10, 13, 30})
}

func TestERIOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 37} {
		SetUnstablePeriod(UnstEMA, u)
		oracle.SetUnstablePeriod(int(UnstEMA), u)
		eriOracleCompare(t, lbl("unst=", u, " "), []int{1, 2, 5, 13, 30})
		eriOracleRanges(t, lbl("unst=", u, " "), []series{fixtures(150)[0]},
			[][2]int{{0, 149}, {40, 40}, {0, 10}, {60, 149}}, []int{1, 3, 14})
	}
	// Another function's unstable period must not leak into ERI.
	SetAllUnstablePeriods(0)
	oracle.ResetUnstablePeriods()
	SetUnstablePeriod(UnstRSI, 9)
	oracle.SetUnstablePeriod(int(UnstRSI), 9)
	eriOracleCompare(t, "rsi-unst ", []int{3, 13})
}
