//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func adxrOracleFull(t *testing.T, periods []int, tag string) {
	t.Helper()
	for _, p := range periods {
		lb := ADXRLookback(p)
		if c := mustLookback(t, "ADXR", float64(p)); lb != c {
			t.Fatalf("%s lookback p=%d: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := ADXR(s.H, s.L, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "ADXR", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("ADXR ", tag, " ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestADXROracle(t *testing.T) {
	adxrOracleFull(t, []int{2, 3, 5, 14, 30, 200}, "")
}

func TestADXROracleLargePeriod(t *testing.T) {
	// The maximum period: only the lookback and an all-NaN output are practical.
	for _, p := range []int{100000, 50000} {
		lb := ADXRLookback(p)
		if c := mustLookback(t, "ADXR", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
	s := fixtures(1000)[0]
	got, err := ADXR(s.H, s.L, s.C, 100000)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "ADXR", s.ohlcv(), 100000)
	assertReal(t, "ADXR p=max", got, r.AlignedReal(0, 1000), relTol)
}

func TestADXROracleBadParams(t *testing.T) {
	s := fixtures(40)[0]
	for _, p := range []int{1, 0, -5, 100001} {
		r := mustCall(t, "ADXR", s.ohlcv(), float64(p))
		_, err := ADXR(s.H, s.L, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if lb := ADXRLookback(p); lb != -1 {
			t.Errorf("p=%d: Go lookback %d", p, lb)
		}
	}
	for _, p := range []int{2, 100000} {
		r := mustCall(t, "ADXR", s.ohlcv(), float64(p))
		_, err := ADXR(s.H, s.L, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
	}
}

func adxrOracleRanges(t *testing.T, s series, periods []int, ranges [][2]int, tag string) {
	t.Helper()
	for _, rg := range ranges {
		for _, p := range periods {
			out := make([]float64, len(s.C))
			beg, nb, err := adxrRange(rg[0], rg[1], s.H, s.L, s.C, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "ADXR", s.ohlcv(), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v p=%d: go (%d,%d) c (%d,%d)", tag, rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("ADXR ", tag, " range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}

func TestADXROracleRanges(t *testing.T) {
	// Lookbacks: p=2 -> 4, p=5 -> 13, p=14 -> 40.
	ranges := [][2]int{{0, 0}, {0, 119}, {3, 3}, {4, 4}, {10, 50}, {119, 119},
		{0, 39}, {40, 40}, {0, 40}, {5, 45}, {60, 119}, {13, 13}, {12, 12}}
	for _, i := range []int{1, 4, 11} {
		adxrOracleRanges(t, fixtures(120)[i], []int{2, 5, 14}, ranges, fixtures(1)[i].Name)
	}
}

func TestADXROracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 40} {
		if err := SetUnstablePeriod(UnstADX, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstADX), u)
		adxrOracleFull(t, []int{2, 14}, lbl("unst=", u))
		adxrOracleRanges(t, fixtures(150)[4], []int{14},
			[][2]int{{0, 149}, {20, 20}, {30, 100}, {149, 149}, {0, 40 + u - 1}, {40 + u, 40 + u}}, lbl("unst=", u))
	}
}
