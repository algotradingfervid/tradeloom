//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func smiOracleOpt(p [4]int) []float64 {
	return []float64{float64(p[0]), float64(p[1]), float64(p[2]), float64(p[3])}
}

func smiOracleCompare(t *testing.T, tag string, sets [][4]int) {
	t.Helper()
	for _, p := range sets {
		lb := SMILookback(p[0], p[1], p[2], p[3])
		if c := mustLookback(t, "SMI", smiOracleOpt(p)...); lb != c {
			t.Fatalf("%s lookback p=%v: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				a, b, err := SMI(s.H, s.L, s.C, p[0], p[1], p[2], p[3])
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "SMI", s.ohlcv(), smiOracleOpt(p)...)
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("SMI ", tag, s.Name, " n=", n, " p=", p), a, r.AlignedReal(0, n), relTol)
				assertReal(t, lbl("SMI signal ", tag, s.Name, " n=", n, " p=", p), b, r.AlignedReal(1, n), relTol)
			}
		}
	}
}

func TestSMIOracle(t *testing.T) {
	smiOracleCompare(t, "", [][4]int{
		{13, 2, 25, 9}, // defaults
		{2, 2, 2, 2},   // all minimum
		{5, 3, 7, 4},
		{14, 3, 3, 3},
		{3, 10, 2, 6},
		{2, 30, 2, 2},
		{40, 2, 2, 20},
		{10, 5, 50, 2},
	})
	// Maximum periods: lookback check, and a short series entirely inside it.
	for _, p := range [][4]int{{100000, 100000, 100000, 100000}, {100000, 2, 2, 2}, {2, 2, 100000, 2}} {
		if lb, c := SMILookback(p[0], p[1], p[2], p[3]), mustLookback(t, "SMI", smiOracleOpt(p)...); lb != c {
			t.Fatalf("lookback p=%v: go %d c %d", p, lb, c)
		}
		s := fixtures(64)[0]
		a, b, _ := SMI(s.H, s.L, s.C, p[0], p[1], p[2], p[3])
		r := mustCall(t, "SMI", s.ohlcv(), smiOracleOpt(p)...)
		assertReal(t, lbl("SMI max ", p), a, r.AlignedReal(0, 64), relTol)
		assertReal(t, lbl("SMI signal max ", p), b, r.AlignedReal(1, 64), relTol)
	}
}

func TestSMIOracleBadParams(t *testing.T) {
	s := fixtures(40)[0]
	valid := [4]int{5, 3, 7, 4}
	// math.MinInt32 is TA_INTEGER_DEFAULT, which C replaces with the default;
	// Go has no sentinel, so it is not included here.
	for k := 0; k < 4; k++ {
		for _, v := range []int{1, 0, -5, 2, 100000, 100001} {
			p := valid
			p[k] = v
			r := mustCall(t, "SMI", s.ohlcv(), smiOracleOpt(p)...)
			_, _, err := SMI(s.H, s.L, s.C, p[0], p[1], p[2], p[3])
			if (r.RetCode == oracle.BadParam) != (err != nil) {
				t.Errorf("p=%v: C retcode %d, Go err %v", p, r.RetCode, err)
			}
			if lb, c := SMILookback(p[0], p[1], p[2], p[3]), mustLookback(t, "SMI", smiOracleOpt(p)...); lb != c {
				t.Errorf("lookback p=%v: go %d c %d", p, lb, c)
			}
		}
	}
}

func smiOracleRanges(t *testing.T, tag string, sets [][4]int) {
	t.Helper()
	for _, s := range []series{fixtures(120)[1], fixtures(120)[4], fixtures(120)[11], fixtures(120)[5]} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {4, 4}, {5, 9}, {10, 50}, {44, 44}, {45, 45}, {46, 80}, {119, 119}, {0, 8}, {100, 119}} {
			for _, p := range sets {
				out := make([]float64, len(s.C))
				out2 := make([]float64, len(s.C))
				beg, nb, err := smiRange(rg[0], rg[1], s.H, s.L, s.C, p[0], p[1], p[2], p[3], out, out2)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "SMI", s.ohlcv(), rg[0], rg[1], smiOracleOpt(p)...)
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s %s range %v p=%v: go (%d,%d) c (%d,%d)", tag, s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("SMI range ", tag, s.Name, rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
				assertReal(t, lbl("SMI signal range ", tag, s.Name, rg, " p=", p), out2[:nb], r.Outputs[1].Real, relTol)
			}
		}
	}
}

func TestSMIOracleRanges(t *testing.T) {
	smiOracleRanges(t, "", [][4]int{{2, 2, 2, 2}, {13, 2, 25, 9}, {5, 3, 7, 4}})
}

// SMI inherits TA_FUNC_UNST_EMA through every EMA stage lookback.
func TestSMIOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 23} {
		SetUnstablePeriod(UnstEMA, u)
		oracle.SetUnstablePeriod(int(UnstEMA), u)
		sets := [][4]int{{2, 2, 2, 2}, {13, 2, 25, 9}, {5, 3, 7, 4}}
		smiOracleCompare(t, lbl("unst=", u, " "), sets)
		smiOracleRanges(t, lbl("unst=", u), sets)
	}
	// Another function's unstable period must not leak into SMI.
	SetAllUnstablePeriods(0)
	oracle.ResetUnstablePeriods()
	SetUnstablePeriod(UnstRSI, 9)
	oracle.SetUnstablePeriod(int(UnstRSI), 9)
	smiOracleCompare(t, "rsi-unst ", [][4]int{{5, 3, 7, 4}})
}
