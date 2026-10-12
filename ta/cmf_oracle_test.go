//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestCMFOracle(t *testing.T) {
	// 50 and 51 straddle C's static circular-buffer size (heap above 50).
	periods := []int{2, 3, 5, 14, 20, 50, 51, 200, 100000}
	for _, p := range periods {
		lb := CMFLookback(p)
		if c := mustLookback(t, "CMF", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := CMF(s.H, s.L, s.C, s.V, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "CMF", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("CMF ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestCMFOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{1, 0, -5, 100001, 2, 100000} {
		if lb, c := CMFLookback(p), mustLookback(t, "CMF", float64(p)); lb != c {
			t.Errorf("lookback p=%d: go %d c %d", p, lb, c)
		}
		r := mustCall(t, "CMF", s.ohlcv(), float64(p))
		_, err := CMF(s.H, s.L, s.C, s.V, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
	}
	out := make([]float64, 20)
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}, {0, 19}} {
		r := mustCallRange(t, "CMF", s.ohlcv(), rg[0], rg[1], 3)
		_, _, err := cmfRange(rg[0], rg[1], s.H, s.L, s.C, s.V, 3, out)
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCMFOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[4], fixtures(120)[10], fixtures(120)[12]} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {50, 119}} {
			for _, p := range []int{2, 10, 60} {
				out := make([]float64, 120)
				beg, nb, err := cmfRange(rg[0], rg[1], s.H, s.L, s.C, s.V, p, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "CMF", s.ohlcv(), rg[0], rg[1], float64(p))
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%d: go (%d,%d) c (%d,%d)", s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("CMF range ", s.Name, " ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

// Degenerate bars (zero range, high < low, close outside the bar, zero and NaN
// volume) must take the same branches in Go and C.
func TestCMFOracleDegenerate(t *testing.T) {
	nan := math.NaN()
	h := []float64{10, 10, 10, 7, 10, 5, 10, 10, 10, 10, 10, 10}
	l := []float64{6, 6, 6, 7, 6, 9, 6, 6, 6, 6, 6, 6}
	c := []float64{8, 10, 6, 7, 9, 7, 12, 3, 8, 9, 7, 8}
	v := []float64{100, 50, 20, 30, 8, 2, 0, 0, 0, 5, nan, 4}
	for _, p := range []int{2, 3, 5} {
		got, err := CMF(h, l, c, v, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CMF", oracle.Inputs{High: h, Low: l, Close: c, Volume: v}, float64(p))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		assertReal(t, lbl("CMF degenerate p=", p), got, r.AlignedReal(0, len(h)), relTol)
	}
}
