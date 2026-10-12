//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestAROONOSCOracle(t *testing.T) {
	for _, p := range []int{2, 3, 5, 14, 25, 200} {
		lb := AROONOSCLookback(p)
		if c := mustLookback(t, "AROONOSC", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := AROONOSC(s.H, s.L, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "AROONOSC", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("AROONOSC ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
	// Maximum period: lookback only, plus one series just past it.
	if lb, c := AROONOSCLookback(100000), mustLookback(t, "AROONOSC", 100000); lb != c {
		t.Fatalf("lookback p=100000: go %d c %d", lb, c)
	}
	s := fixtures(100003)[0]
	got, err := AROONOSC(s.H, s.L, 100000)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "AROONOSC", s.ohlcv(), 100000)
	if r.RetCode != oracle.Success {
		t.Fatalf("C retcode %d", r.RetCode)
	}
	assertReal(t, "AROONOSC p=100000", got, r.AlignedReal(0, len(s.H)), relTol)
}

func TestAROONOSCOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{1, 0, -5, 100001, 2, 100000} {
		r := mustCall(t, "AROONOSC", s.ohlcv(), float64(p))
		_, err := AROONOSC(s.H, s.L, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if lb, c := AROONOSCLookback(p), mustLookback(t, "AROONOSC", float64(p)); lb != c {
			t.Errorf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
}

func TestAROONOSCOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[9], fixtures(120)[11]} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 9}, {10, 10}, {5, 30}, {100, 119}} {
			for _, p := range []int{2, 10, 14} {
				out := make([]float64, 120)
				beg, nb, err := aroonoscRange(rg[0], rg[1], s.H, s.L, p, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "AROONOSC", s.ohlcv(), rg[0], rg[1], float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%d: go (%d,%d) c (%d,%d)", s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("AROONOSC range ", s.Name, " ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}
