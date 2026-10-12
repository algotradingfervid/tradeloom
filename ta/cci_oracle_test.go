//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestCCIOracle(t *testing.T) {
	// 30 and 31 straddle C's switch from the stack circular buffer to malloc.
	for _, p := range []int{2, 3, 5, 14, 20, 30, 31, 200} {
		lb := CCILookback(p)
		if c := mustLookback(t, "CCI", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := CCI(s.H, s.L, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "CCI", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("CCI ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestCCIOracleMaxPeriod(t *testing.T) {
	p := 100000
	if lb, c := CCILookback(p), mustLookback(t, "CCI", float64(p)); lb != c {
		t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
	}
	for _, n := range []int{p - 1, p, p + 2} {
		s := fixtures(n)[0]
		got, err := CCI(s.H, s.L, s.C, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CCI", s.ohlcv(), float64(p))
		assertReal(t, lbl("CCI max n=", n), got, r.AlignedReal(0, n), relTol)
	}
}

func TestCCIOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{-5, 0, 1, 2, 100000, 100001} {
		if lb := CCILookback(p); lb != mustLookback(t, "CCI", float64(p)) {
			t.Errorf("p=%d: lookback go %d c %d", p, lb, mustLookback(t, "CCI", float64(p)))
		}
		r := mustCall(t, "CCI", s.ohlcv(), float64(p))
		_, err := CCI(s.H, s.L, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
	}
}

func TestCCIOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[3], fixtures(120)[9]} {
		ranges := [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {0, 9}, {10, 10}, {50, 119}}
		for _, rg := range ranges {
			for _, p := range []int{2, 10, 35} {
				out := make([]float64, 120)
				beg, nb, err := cciRange(rg[0], rg[1], s.H, s.L, s.C, p, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "CCI", s.ohlcv(), rg[0], rg[1], float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%d: go (%d,%d) c (%d,%d)", s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("CCI ", s.Name, " range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}
