//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestMIDPRICEOracle(t *testing.T) {
	periods := []int{2, 3, 4, 5, 14, 30, 31, 64, 200}
	for _, p := range periods {
		lb := MIDPRICELookback(p)
		if c := mustLookback(t, "MIDPRICE", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := MIDPRICE(s.H, s.L, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "MIDPRICE", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("MIDPRICE ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
	// Maximum period: lookback, a short series inside it, and one long enough
	// to produce output.
	if lb, c := MIDPRICELookback(100000), mustLookback(t, "MIDPRICE", 100000); lb != c {
		t.Fatalf("lookback p=100000: go %d c %d", lb, c)
	}
	for _, n := range []int{64, 100000, 100001, 100005} {
		s := randomWalk("walk", 1, n, 1000, 1)
		got, _ := MIDPRICE(s.H, s.L, 100000)
		r := mustCall(t, "MIDPRICE", s.ohlcv(), 100000)
		assertReal(t, lbl("MIDPRICE p=100000 n=", n), got, r.AlignedReal(0, n), relTol)
	}
}

// NaN inputs exercise the incumbent-keeps-on-NaN comparison semantics.
func TestMIDPRICEOracleNaN(t *testing.T) {
	for _, p := range []int{2, 3, 7} {
		s := fixtures(64)[0]
		h := append([]float64(nil), s.H...)
		l := append([]float64(nil), s.L...)
		for _, i := range []int{0, 5, 6, 20, 33, 63} {
			h[i] = math.NaN()
		}
		for _, i := range []int{1, 6, 14, 40, 62} {
			l[i] = math.NaN()
		}
		got, _ := MIDPRICE(h, l, p)
		in := s.ohlcv()
		in.High, in.Low = h, l
		r := mustCall(t, "MIDPRICE", in, float64(p))
		assertReal(t, lbl("MIDPRICE NaN p=", p), got, r.AlignedReal(0, 64), relTol)
	}
}

func TestMIDPRICEOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{2, 1, 0, -5, 100000, 100001} {
		r := mustCall(t, "MIDPRICE", s.ohlcv(), float64(p))
		_, err := MIDPRICE(s.H, s.L, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if lb, c := MIDPRICELookback(p), mustLookback(t, "MIDPRICE", float64(p)); lb != c {
			t.Errorf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
}

func TestMIDPRICEOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {5, 30}, {100, 119}, {11, 12}} {
		for _, p := range []int{2, 3, 10} {
			out := make([]float64, 120)
			beg, nb, err := midpriceRange(rg[0], rg[1], s.H, s.L, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "MIDPRICE", s.ohlcv(), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("MIDPRICE range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
