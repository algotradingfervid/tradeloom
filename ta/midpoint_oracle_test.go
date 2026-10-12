//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestMIDPOINTOracle(t *testing.T) {
	// 30/31 straddle the C stack-buffer size; 1000 exceeds the longest fixture.
	periods := []int{2, 3, 4, 5, 14, 30, 31, 200, 1000}
	for _, p := range periods {
		lb := MIDPOINTLookback(p)
		if c := mustLookback(t, "MIDPOINT", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := MIDPOINT(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "MIDPOINT", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("MIDPOINT ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
	for _, p := range []int{2, 100000} {
		if lb, c := MIDPOINTLookback(p), mustLookback(t, "MIDPOINT", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
}

func TestMIDPOINTOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{1, 0, -5, 100001, 2, 100000} {
		r := mustCall(t, "MIDPOINT", realIn(in), float64(p))
		_, err := MIDPOINT(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if lb, c := MIDPOINTLookback(p), mustLookback(t, "MIDPOINT", float64(p)); lb != c {
			t.Errorf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
}

func TestMIDPOINTOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.Real()
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {9, 10}, {11, 30}, {40, 119}} {
		for _, p := range []int{2, 3, 10, 40} {
			out := make([]float64, len(in))
			beg, nb, err := midpointRange(rg[0], rg[1], in, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "MIDPOINT", realIn(in), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("MIDPOINT range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
