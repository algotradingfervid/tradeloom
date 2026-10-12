//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func demaOracleCompare(t *testing.T, tag string, periods []int) {
	t.Helper()
	for _, p := range periods {
		lb := DEMALookback(p)
		if c := mustLookback(t, "DEMA", float64(p)); lb != c {
			t.Fatalf("%s lookback p=%d: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := DEMA(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "DEMA", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("DEMA ", tag, s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestDEMAOracle(t *testing.T) {
	demaOracleCompare(t, "", []int{1, 2, 3, 5, 9, 14, 30, 200, 100000})
}

func TestDEMAOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	// math.MinInt32 is TA_INTEGER_DEFAULT, which C replaces with 30; Go has no
	// sentinel, so it is not included here.
	for _, p := range []int{0, -1, -5, 100001, 1 << 30} {
		r := mustCall(t, "DEMA", realIn(in), float64(p))
		_, err := DEMA(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "DEMA", float64(p)); c != DEMALookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, DEMALookback(p), c)
		}
	}
	for _, p := range []int{1, 100000} {
		r := mustCall(t, "DEMA", realIn(in), float64(p))
		if _, err := DEMA(in, p); err != nil || r.RetCode != oracle.Success {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
	}
}

func demaOracleRanges(t *testing.T, tag string, ss []series, ranges [][2]int, periods []int) {
	t.Helper()
	for _, s := range ss {
		in := s.Real()
		for _, rg := range ranges {
			for _, p := range periods {
				out := make([]float64, len(in))
				beg, nb, err := demaRange(rg[0], rg[1], in, p, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "DEMA", realIn(in), rg[0], rg[1], float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s%s range %v p=%d: go (%d,%d) c (%d,%d)", tag, s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("DEMA range ", tag, s.Name, rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

func TestDEMAOracleRanges(t *testing.T) {
	f := fixtures(120)
	demaOracleRanges(t, "", []series{f[1], f[4], f[13]},
		[][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {8, 9}, {17, 18}, {18, 18}, {30, 119}, {58, 119}, {100, 105}},
		[]int{1, 2, 10, 30})
}

func TestDEMAOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 37} {
		SetUnstablePeriod(UnstEMA, u)
		oracle.SetUnstablePeriod(int(UnstEMA), u)
		demaOracleCompare(t, lbl("unst=", u, " "), []int{1, 2, 5, 14, 30})
		demaOracleRanges(t, lbl("unst=", u, " "), []series{fixtures(200)[0]},
			[][2]int{{0, 199}, {40, 40}, {0, 10}, {60, 199}, {100, 150}}, []int{1, 3, 14})
	}
	// Another function's unstable period must not leak into DEMA.
	SetAllUnstablePeriods(0)
	oracle.ResetUnstablePeriods()
	SetUnstablePeriod(UnstRSI, 9)
	oracle.SetUnstablePeriod(int(UnstRSI), 9)
	demaOracleCompare(t, "rsi-unst ", []int{3, 14})
}
