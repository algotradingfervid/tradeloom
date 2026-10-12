//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestAROONOracle(t *testing.T) {
	periods := []int{2, 3, 5, 14, 25, 200}
	for _, p := range periods {
		lb := AROONLookback(p)
		if c := mustLookback(t, "AROON", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				down, up, err := AROON(s.H, s.L, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "AROON", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("AROON down ", s.Name, " n=", n, " p=", p), down, r.AlignedReal(0, n), relTol)
				assertReal(t, lbl("AROON up ", s.Name, " n=", n, " p=", p), up, r.AlignedReal(1, n), relTol)
			}
		}
	}
	// Maximum period: lookback check, and a short series entirely inside it.
	if lb, c := AROONLookback(100000), mustLookback(t, "AROON", 100000); lb != c {
		t.Fatalf("lookback p=100000: go %d c %d", lb, c)
	}
	s := fixtures(64)[0]
	down, up, _ := AROON(s.H, s.L, 100000)
	r := mustCall(t, "AROON", s.ohlcv(), 100000)
	assertReal(t, "AROON down p=100000", down, r.AlignedReal(0, 64), relTol)
	assertReal(t, "AROON up p=100000", up, r.AlignedReal(1, 64), relTol)
}

func TestAROONOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{1, 0, -5, 100001} {
		r := mustCall(t, "AROON", s.ohlcv(), float64(p))
		_, _, err := AROON(s.H, s.L, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if lb, c := AROONLookback(p), mustLookback(t, "AROON", float64(p)); lb != c {
			t.Errorf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
}

func TestAROONOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 9}, {10, 10}, {5, 30}, {100, 119}} {
		for _, p := range []int{2, 10} {
			down := make([]float64, 120)
			up := make([]float64, 120)
			beg, nb, err := aroonRange(rg[0], rg[1], s.H, s.L, p, down, up)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "AROON", s.ohlcv(), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("AROON down range ", rg, " p=", p), down[:nb], r.Outputs[0].Real, relTol)
			assertReal(t, lbl("AROON up range ", rg, " p=", p), up[:nb], r.Outputs[1].Real, relTol)
		}
	}
}
