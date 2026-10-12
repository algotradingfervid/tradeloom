//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func adxOracleFull(t *testing.T, periods []int, tag string) {
	t.Helper()
	for _, p := range periods {
		lb := ADXLookback(p)
		if c := mustLookback(t, "ADX", float64(p)); lb != c {
			t.Fatalf("%s lookback p=%d: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := ADX(s.H, s.L, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "ADX", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("ADX ", tag, " ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestADXOracle(t *testing.T) {
	adxOracleFull(t, []int{2, 3, 5, 14, 30, 200}, "")
}

func TestADXOracleLargePeriod(t *testing.T) {
	// The maximum period: only the lookback and an all-NaN output are practical.
	for _, p := range []int{100000, 50000} {
		lb := ADXLookback(p)
		if c := mustLookback(t, "ADX", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
	s := fixtures(1000)[0]
	got, err := ADX(s.H, s.L, s.C, 100000)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "ADX", s.ohlcv(), 100000)
	assertReal(t, "ADX p=max", got, r.AlignedReal(0, 1000), relTol)
}

func TestADXOracleBadParams(t *testing.T) {
	s := fixtures(40)[0]
	for _, p := range []int{1, 0, -5, 100001} {
		r := mustCall(t, "ADX", s.ohlcv(), float64(p))
		_, err := ADX(s.H, s.L, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if lb := ADXLookback(p); lb != -1 {
			t.Errorf("p=%d: Go lookback %d", p, lb)
		}
	}
	for _, p := range []int{2, 100000} {
		r := mustCall(t, "ADX", s.ohlcv(), float64(p))
		_, err := ADX(s.H, s.L, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
	}
}

func adxOracleRanges(t *testing.T, s series, periods []int, ranges [][2]int, tag string) {
	t.Helper()
	for _, rg := range ranges {
		for _, p := range periods {
			out := make([]float64, len(s.C))
			beg, nb, err := adxRange(rg[0], rg[1], s.H, s.L, s.C, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "ADX", s.ohlcv(), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v p=%d: go (%d,%d) c (%d,%d)", tag, rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("ADX ", tag, " range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}

func TestADXOracleRanges(t *testing.T) {
	ranges := [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 26}, {27, 27}, {0, 27}, {5, 30}, {60, 119}}
	for _, i := range []int{1, 4, 11} {
		adxOracleRanges(t, fixtures(120)[i], []int{2, 5, 14}, ranges, fixtures(1)[i].Name)
	}
}

func TestADXOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 40} {
		if err := SetUnstablePeriod(UnstADX, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstADX), u)
		adxOracleFull(t, []int{2, 14}, lbl("unst=", u))
		adxOracleRanges(t, fixtures(150)[4], []int{14},
			[][2]int{{0, 149}, {20, 20}, {30, 100}, {149, 149}, {0, 27 + u - 1}, {27 + u, 27 + u}}, lbl("unst=", u))
	}
}
