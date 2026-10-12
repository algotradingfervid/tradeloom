//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

type supertrendParams struct {
	p int
	m float64
}

var supertrendOracleParams = []supertrendParams{
	{2, 0}, {2, 1}, {2, 3}, {3, 0.5}, {7, 2}, {10, 3}, {14, 1.5}, {30, 4},
	{200, 3}, {10, 1e6}, {10, taRealMax}, {2, taRealMax},
}

func supertrendCompare(t *testing.T, label string, s series, p int, m float64) {
	t.Helper()
	n := len(s.C)
	gotSt, gotTr, err := SUPERTREND(s.H, s.L, s.C, p, m)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		return // C rejects endIdx -1; Go returns empty output
	}
	r := mustCall(t, "SUPERTREND", s.ohlcv(), float64(p), m)
	if r.RetCode != oracle.Success {
		t.Fatalf("%s: C retcode %d", label, r.RetCode)
	}
	assertReal(t, label, gotSt, r.AlignedReal(0, n), relTol)
	assertInt(t, label+" trend", gotTr, r.AlignedInt(1, n))
}

func TestSUPERTRENDOracle(t *testing.T) {
	for _, ps := range supertrendOracleParams {
		lb := SUPERTRENDLookback(ps.p, ps.m)
		if c := mustLookback(t, "SUPERTREND", float64(ps.p), ps.m); lb != c {
			t.Fatalf("lookback p=%d m=%g: go %d c %d", ps.p, ps.m, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				supertrendCompare(t, lbl("SUPERTREND ", s.Name, " n=", n, " p=", ps.p, " m=", ps.m), s, ps.p, ps.m)
			}
		}
	}
	// Maximum period on a series long enough to produce output.
	lb := SUPERTRENDLookback(100000, 3)
	if c := mustLookback(t, "SUPERTREND", 100000, 3); lb != c {
		t.Fatalf("lookback p=100000: go %d c %d", lb, c)
	}
	for _, s := range fixtures(100010)[:2] {
		supertrendCompare(t, lbl("SUPERTREND ", s.Name, " p=100000"), s, 100000, 3)
	}
}

func TestSUPERTRENDOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, ps := range []supertrendParams{
		{1, 3}, {0, 3}, {-5, 3}, {100001, 3},
		{10, -1}, {10, -1e-300}, {10, taRealMax * 1.0000001},
		{2, 0}, {100000, taRealMax},
	} {
		r := mustCall(t, "SUPERTREND", s.ohlcv(), float64(ps.p), ps.m)
		_, _, err := SUPERTREND(s.H, s.L, s.C, ps.p, ps.m)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d m=%g: C retcode %d, Go err %v", ps.p, ps.m, r.RetCode, err)
		}
		lb := SUPERTRENDLookback(ps.p, ps.m)
		// TA_GetLookback does not reject the parameters itself; it returns
		// TA_SUPERTREND_Lookback's -1, so compare the values.
		if c, cerr := oracle.Lookback("SUPERTREND", float64(ps.p), ps.m); cerr != nil || c != lb {
			t.Errorf("lookback p=%d m=%g: C %d (err %v), Go %d", ps.p, ps.m, c, cerr, lb)
		}
	}
}

func supertrendCompareRanges(t *testing.T, tag string, s series, ranges [][2]int, params []supertrendParams) {
	t.Helper()
	n := len(s.C)
	for _, rg := range ranges {
		for _, ps := range params {
			st := make([]float64, n)
			tr := make([]int, n)
			beg, nb, err := supertrendRange(rg[0], rg[1], s.H, s.L, s.C, ps.p, ps.m, st, tr)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "SUPERTREND", s.ohlcv(), rg[0], rg[1], float64(ps.p), ps.m)
			if r.RetCode != oracle.Success {
				t.Fatalf("%s range %v: C retcode %d", tag, rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v p=%d m=%g: go (%d,%d) c (%d,%d)", tag, rg, ps.p, ps.m, beg, nb, r.BegIdx, r.NbElement)
			}
			label := lbl("SUPERTREND ", tag, " range ", rg, " p=", ps.p, " m=", ps.m)
			assertReal(t, label, st[:nb], r.Outputs[0].Real, relTol)
			assertInt(t, label+" trend", tr[:nb], r.Outputs[1].Int)
		}
	}
}

func TestSUPERTRENDOracleRanges(t *testing.T) {
	ranges := [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {10, 10}, {11, 11}, {2, 13}, {60, 119}}
	params := []supertrendParams{{2, 3}, {10, 3}, {10, 0}, {5, 1.5}}
	for _, i := range []int{1, 9, 11} {
		s := fixtures(120)[i]
		supertrendCompareRanges(t, s.Name, s, ranges, params)
	}
}

func TestSUPERTRENDOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 40} {
		if err := SetUnstablePeriod(UnstATR, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstATR), u)
		for _, ps := range []supertrendParams{{2, 3}, {10, 3}, {14, 0}} {
			lb := SUPERTRENDLookback(ps.p, ps.m)
			if c := mustLookback(t, "SUPERTREND", float64(ps.p), ps.m); lb != c {
				t.Fatalf("unstable %d lookback p=%d: go %d c %d", u, ps.p, lb, c)
			}
			for _, n := range edgeLengths(lb) {
				for _, s := range fixtures(n) {
					supertrendCompare(t, lbl("SUPERTREND unstable=", u, " ", s.Name, " n=", n, " p=", ps.p, " m=", ps.m), s, ps.p, ps.m)
				}
			}
		}
		s := fixtures(120)[0]
		supertrendCompareRanges(t, lbl("unstable=", u), s, [][2]int{{0, 119}, {50, 50}, {60, 100}}, []supertrendParams{{10, 3}})
	}
}
