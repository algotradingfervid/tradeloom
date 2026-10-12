//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestRVOLOracle(t *testing.T) {
	periods := []int{1, 2, 3, 5, 14, 20, 30, 200}
	for _, p := range periods {
		lb := RVOLLookback(p)
		if c := mustLookback(t, "RVOL", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := RVOL(s.V, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "RVOL", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("RVOL ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
	// Maximum period on a long-enough input.
	for _, s := range fixtures(100002)[:2] {
		got, err := RVOL(s.V, 100000)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "RVOL", s.ohlcv(), 100000)
		assertReal(t, lbl("RVOL max ", s.Name), got, r.AlignedReal(0, len(s.V)), relTol)
	}
	if c := mustLookback(t, "RVOL", 100000); c != RVOLLookback(100000) {
		t.Fatalf("lookback max: go %d c %d", RVOLLookback(100000), c)
	}
}

func TestRVOLOracleDeadWindow(t *testing.T) {
	v := []float64{0, 0, 0, 5, 0, 0, 0, 0, 3, 1, 0}
	got, _ := RVOL(v, 3)
	r := mustCall(t, "RVOL", oracle.Inputs{Volume: v}, 3)
	assertReal(t, "RVOL dead window", got, r.AlignedReal(0, len(v)), relTol)
}

func TestRVOLOracleBadParams(t *testing.T) {
	in := fixtures(20)[0]
	for _, p := range []int{0, -5, 100001} {
		r := mustCall(t, "RVOL", in.ohlcv(), float64(p))
		_, err := RVOL(in.V, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "RVOL", float64(p)); c != RVOLLookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, RVOLLookback(p), c)
		}
	}
}

func TestRVOLOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.V
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 9}, {0, 10}, {10, 10}, {9, 9}, {5, 119}} {
		for _, p := range []int{1, 10, 20} {
			out := make([]float64, len(in))
			beg, nb, err := rvolRange(rg[0], rg[1], in, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "RVOL", s.ohlcv(), rg[0], rg[1], float64(p))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("RVOL range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
