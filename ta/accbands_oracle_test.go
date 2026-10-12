//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestACCBANDSOracle(t *testing.T) {
	for _, p := range []int{2, 3, 5, 14, 20, 30, 200} {
		lb := ACCBANDSLookback(p)
		if c := mustLookback(t, "ACCBANDS", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				u, m, l, err := ACCBANDS(s.H, s.L, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "ACCBANDS", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				tag := lbl(" ", s.Name, " n=", n, " p=", p)
				assertReal(t, "ACCBANDS upper"+tag, u, r.AlignedReal(0, n), relTol)
				assertReal(t, "ACCBANDS middle"+tag, m, r.AlignedReal(1, n), relTol)
				assertReal(t, "ACCBANDS lower"+tag, l, r.AlignedReal(2, n), relTol)
			}
		}
	}
}

func TestACCBANDSOracleMaxPeriod(t *testing.T) {
	p := 100000
	if lb, c := ACCBANDSLookback(p), mustLookback(t, "ACCBANDS", float64(p)); lb != c {
		t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
	}
	for _, n := range []int{p - 1, p, p + 1, p + 50} {
		s := fixtures(n)[0]
		u, m, l, err := ACCBANDS(s.H, s.L, s.C, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "ACCBANDS", s.ohlcv(), float64(p))
		assertReal(t, lbl("ACCBANDS max upper n=", n), u, r.AlignedReal(0, n), relTol)
		assertReal(t, lbl("ACCBANDS max middle n=", n), m, r.AlignedReal(1, n), relTol)
		assertReal(t, lbl("ACCBANDS max lower n=", n), l, r.AlignedReal(2, n), relTol)
	}
}

// TestACCBANDSOracleCancel exercises the TA_IS_ZERO_SCALED branch: bars whose
// high and low have opposite signs and (nearly) cancel.
func TestACCBANDSOracleCancel(t *testing.T) {
	h := []float64{1, 2, 1e-300, 5, 1, 3, 0.1, 1 + 1e-15, -2, 4}
	l := []float64{-1, -2, -1e-300, -5 + 1e-14, -1 + 1e-16, 1, -0.1, -1, -3, -4}
	c := []float64{0, 1, 0, 2, 0, 2, 0, 0, -2.5, 0}
	for _, p := range []int{2, 3, 5} {
		u, m, lo, err := ACCBANDS(h, l, c, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "ACCBANDS", oracle.Inputs{High: h, Low: l, Close: c}, float64(p))
		n := len(h)
		assertReal(t, lbl("cancel upper p=", p), u, r.AlignedReal(0, n), relTol)
		assertReal(t, lbl("cancel middle p=", p), m, r.AlignedReal(1, n), relTol)
		assertReal(t, lbl("cancel lower p=", p), lo, r.AlignedReal(2, n), relTol)
	}
}

func TestACCBANDSOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{0, -5, 1, 2, 100000, 100001} {
		r := mustCall(t, "ACCBANDS", s.ohlcv(), float64(p))
		_, _, _, err := ACCBANDS(s.H, s.L, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := ACCBANDSLookback(p); (err != nil) != (c == -1) {
			t.Errorf("p=%d: lookback %d, err %v", p, c, err)
		}
		if c, g := mustLookback(t, "ACCBANDS", float64(p)), ACCBANDSLookback(p); c != g {
			t.Errorf("p=%d: lookback go %d c %d", p, g, c)
		}
	}
}

func TestACCBANDSOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	ranges := [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {10, 10}, {0, 9}, {50, 119}, {1, 1}, {2, 2}}
	for _, rg := range ranges {
		for _, p := range []int{2, 3, 10} {
			u, m, l := make([]float64, 120), make([]float64, 120), make([]float64, 120)
			beg, nb, err := accbandsRange(rg[0], rg[1], s.H, s.L, s.C, p, u, m, l)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "ACCBANDS", s.ohlcv(), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("ACCBANDS range upper ", rg, " p=", p), u[:nb], r.Outputs[0].Real, relTol)
			assertReal(t, lbl("ACCBANDS range middle ", rg, " p=", p), m[:nb], r.Outputs[1].Real, relTol)
			assertReal(t, lbl("ACCBANDS range lower ", rg, " p=", p), l[:nb], r.Outputs[2].Real, relTol)
		}
	}
}
