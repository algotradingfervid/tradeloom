//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func efiOracleCompare(t *testing.T, prefix string, periods []int) {
	t.Helper()
	for _, p := range periods {
		lb := EFILookback(p)
		if c := mustLookback(t, "EFI", float64(p)); lb != c {
			t.Fatalf("%slookback p=%d: go %d c %d", prefix, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := EFI(s.C, s.V, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "EFI", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl(prefix, "EFI ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestEFIOracle(t *testing.T) {
	efiOracleCompare(t, "", []int{1, 2, 3, 5, 13, 22, 100, 200})
}

func TestEFIOracleLargePeriod(t *testing.T) {
	s := fixtures(1200)[0]
	for _, p := range []int{999, 1198, 1199, 1200, 100000} {
		lb := EFILookback(p)
		if c := mustLookback(t, "EFI", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		got, err := EFI(s.C, s.V, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "EFI", s.ohlcv(), float64(p))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		assertReal(t, lbl("EFI large p=", p), got, r.AlignedReal(0, len(s.C)), relTol)
	}
}

func TestEFIOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{0, -5, 100001, 1, 100000} {
		r := mustCall(t, "EFI", s.ohlcv(), float64(p))
		_, err := EFI(s.C, s.V, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if lb, c := EFILookback(p), mustLookback(t, "EFI", float64(p)); lb != c {
			t.Errorf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
}

func efiOracleRanges(t *testing.T, prefix string, ranges [][2]int, periods []int) {
	t.Helper()
	for _, s := range []series{fixtures(150)[1], fixtures(150)[4], fixtures(150)[13]} {
		for _, rg := range ranges {
			for _, p := range periods {
				out := make([]float64, len(s.C))
				beg, nb, err := efiRange(rg[0], rg[1], s.C, s.V, p, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "EFI", s.ohlcv(), rg[0], rg[1], float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s%s range %v p=%d: go (%d,%d) c (%d,%d)", prefix, s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl(prefix, "EFI range ", s.Name, rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

func TestEFIOracleRanges(t *testing.T) {
	ranges := [][2]int{{0, 0}, {0, 149}, {1, 1}, {3, 3}, {10, 50}, {13, 13}, {14, 14}, {149, 149}, {0, 9}, {12, 60}, {100, 149}}
	efiOracleRanges(t, "", ranges, []int{1, 2, 10, 13})
}

func TestEFIOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 37} {
		SetUnstablePeriod(UnstEMA, u)
		oracle.SetUnstablePeriod(int(UnstEMA), u)
		efiOracleCompare(t, lbl("unst=", u, " "), []int{1, 2, 5, 13, 30})
		efiOracleRanges(t, lbl("unst=", u, " "), [][2]int{{0, 149}, {40, 40}, {0, 10}, {60, 149}, {149, 149}}, []int{1, 3, 13})
	}
	// Another function's unstable period must not leak into EFI.
	SetAllUnstablePeriods(0)
	oracle.ResetUnstablePeriods()
	SetUnstablePeriod(UnstRSI, 9)
	oracle.SetUnstablePeriod(int(UnstRSI), 9)
	efiOracleCompare(t, "rsi-unst ", []int{3, 13})
}
