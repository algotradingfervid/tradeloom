//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestMFIOracle(t *testing.T) {
	// 50 and 51 straddle C's static circular-buffer size (heap above 50).
	periods := []int{2, 3, 5, 14, 20, 50, 51, 200, 100000}
	for _, p := range periods {
		lb := MFILookback(p)
		if c := mustLookback(t, "MFI", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := MFI(s.H, s.L, s.C, s.V, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "MFI", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("MFI ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestMFIOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{1, 0, -5, 100001, 2, 100000} {
		if lb, c := MFILookback(p), mustLookback(t, "MFI", float64(p)); lb != c {
			t.Errorf("lookback p=%d: go %d c %d", p, lb, c)
		}
		r := mustCall(t, "MFI", s.ohlcv(), float64(p))
		_, err := MFI(s.H, s.L, s.C, s.V, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
	}
	out := make([]float64, 20)
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}, {0, 19}} {
		r := mustCallRange(t, "MFI", s.ohlcv(), rg[0], rg[1], 3)
		_, _, err := mfiRange(rg[0], rg[1], s.H, s.L, s.C, s.V, 3, out)
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestMFIOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[4], fixtures(120)[10], fixtures(120)[12]} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {10, 10}, {50, 119}} {
			for _, p := range []int{2, 10, 60} {
				out := make([]float64, 120)
				beg, nb, err := mfiRange(rg[0], rg[1], s.H, s.L, s.C, s.V, p, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "MFI", s.ohlcv(), rg[0], rg[1], float64(p))
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%d: go (%d,%d) c (%d,%d)", s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("MFI range ", s.Name, " ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

// Degenerate bars (unchanged typical price, tiny moves near the scaled dead
// zone, zero and NaN volume, empty windows followed by flow) must take the
// same branches in Go and C.
func TestMFIOracleDegenerate(t *testing.T) {
	nan := math.NaN()
	h := []float64{10, 10, 10, 10, 10, 11, 9, 10, 10 + 1e-14, 10, 10, 12, 8, 10, 10, 10, 1e-20, 2e-20, 1e-20, 3e-20}
	l := []float64{8, 8, 8, 8, 8, 9, 7, 8, 8, 8, 8, 10, 6, 8, 8, 8, 1e-20, 2e-20, 1e-20, 3e-20}
	c := []float64{9, 9, 9, 9, 9, 10, 8, 9, 9, 9, 9, 11, 7, 9, 9, 9, 1e-20, 2e-20, 1e-20, 3e-20}
	v := []float64{1, 5, 5, 5, 5, 5, 5, 0, 5, 5, 5, nan, 5, 0, 0, 0, 5, 5, 5, 5}
	for _, p := range []int{2, 3, 5} {
		got, err := MFI(h, l, c, v, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "MFI", oracle.Inputs{High: h, Low: l, Close: c, Volume: v}, float64(p))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		assertReal(t, lbl("MFI degenerate p=", p), got, r.AlignedReal(0, len(h)), relTol)
	}
}
