//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestDPOOracle(t *testing.T) {
	periods := []int{2, 3, 4, 5, 10, 14, 20, 21, 60, 200}
	for _, p := range periods {
		lb := DPOLookback(p)
		if c := mustLookback(t, "DPO", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := DPO(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "DPO", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("DPO ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
	// Maximum period: lookback only (no input long enough to emit).
	for _, p := range []int{100000, 99999} {
		if lb, c := DPOLookback(p), mustLookback(t, "DPO", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
	in := fixtures(64)[0].Real()
	r := mustCall(t, "DPO", realIn(in), 100000)
	got, err := DPO(in, 100000)
	if err != nil || r.RetCode != oracle.Success {
		t.Fatalf("p=100000: go err %v, C retcode %d", err, r.RetCode)
	}
	assertReal(t, "DPO p=100000", got, r.AlignedReal(0, len(in)), relTol)
}

func TestDPOOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{1, 0, -5, 100001} {
		if c := mustLookback(t, "DPO", float64(p)); c != -1 || DPOLookback(p) != -1 {
			t.Errorf("lookback p=%d: go %d c %d", p, DPOLookback(p), c)
		}
		r := mustCall(t, "DPO", realIn(in), float64(p))
		_, err := DPO(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
	}
	for _, p := range []int{2, 100000} {
		r := mustCall(t, "DPO", realIn(in), float64(p))
		_, err := DPO(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
	}
}

func TestDPOOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.Real()
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {1, 1}, {2, 2}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {5, 12}} {
		for _, p := range []int{2, 3, 4, 10, 11} {
			out := make([]float64, len(in))
			beg, nb, err := dpoRange(rg[0], rg[1], in, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "DPO", realIn(in), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("DPO range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
