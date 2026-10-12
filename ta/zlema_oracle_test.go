//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func zlemaOracleCompare(t *testing.T, tag string, periods []int) {
	t.Helper()
	for _, p := range periods {
		lb := ZLEMALookback(p)
		if c := mustLookback(t, "ZLEMA", float64(p)); lb != c {
			t.Fatalf("%s lookback p=%d: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := ZLEMA(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "ZLEMA", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("ZLEMA ", tag, s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestZLEMAOracle(t *testing.T) {
	zlemaOracleCompare(t, "", []int{1, 2, 3, 4, 5, 9, 14, 30, 200, 100000})
}

func TestZLEMAOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	// math.MinInt32 is TA_INTEGER_DEFAULT, which C replaces with 30; Go has no
	// sentinel, so it is not included here.
	for _, p := range []int{0, -1, -5, 100001, 1 << 30} {
		r := mustCall(t, "ZLEMA", realIn(in), float64(p))
		_, err := ZLEMA(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "ZLEMA", float64(p)); c != ZLEMALookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, ZLEMALookback(p), c)
		}
	}
	for _, p := range []int{1, 100000} {
		r := mustCall(t, "ZLEMA", realIn(in), float64(p))
		if _, err := ZLEMA(in, p); err != nil || r.RetCode != oracle.Success {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
	}
}

func zlemaOracleRanges(t *testing.T, tag string, s series, ranges [][2]int, periods []int) {
	t.Helper()
	in := s.Real()
	for _, rg := range ranges {
		for _, p := range periods {
			out := make([]float64, len(in))
			beg, nb, err := zlemaRange(rg[0], rg[1], in, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "ZLEMA", realIn(in), rg[0], rg[1], float64(p))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s%s range %v p=%d: go (%d,%d) c (%d,%d)", tag, s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("ZLEMA range ", tag, s.Name, rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}

func TestZLEMAOracleRanges(t *testing.T) {
	ranges := [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {8, 9}, {13, 13}, {30, 119}, {100, 105}}
	for _, s := range []series{fixtures(120)[1], fixtures(120)[4], fixtures(120)[13]} {
		zlemaOracleRanges(t, "", s, ranges, []int{1, 2, 3, 6, 10, 30})
	}
}

func TestZLEMAOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 37} {
		SetUnstablePeriod(UnstEMA, u)
		oracle.SetUnstablePeriod(int(UnstEMA), u)
		zlemaOracleCompare(t, lbl("unst=", u, " "), []int{1, 2, 3, 5, 14, 30})
		zlemaOracleRanges(t, lbl("unst=", u, " "), fixtures(150)[0],
			[][2]int{{0, 149}, {40, 40}, {0, 10}, {60, 149}}, []int{1, 3, 4, 14})
	}
	// Another function's unstable period must not leak into ZLEMA.
	SetAllUnstablePeriods(0)
	oracle.ResetUnstablePeriods()
	SetUnstablePeriod(UnstRSI, 9)
	oracle.SetUnstablePeriod(int(UnstRSI), 9)
	zlemaOracleCompare(t, "rsi-unst ", []int{3, 14})
}
