//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestWILLROracle(t *testing.T) {
	for _, p := range []int{2, 3, 4, 5, 14, 29, 30, 31, 200} {
		lb := WILLRLookback(p)
		if c := mustLookback(t, "WILLR", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := WILLR(s.H, s.L, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "WILLR", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("WILLR ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestWILLROracleMaxPeriod(t *testing.T) {
	p := 100000
	if lb, c := WILLRLookback(p), mustLookback(t, "WILLR", float64(p)); lb != c {
		t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
	}
	for _, n := range []int{p - 1, p, p + 1, p + 50} {
		s := fixtures(n)[0]
		got, err := WILLR(s.H, s.L, s.C, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "WILLR", s.ohlcv(), float64(p))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		assertReal(t, lbl("WILLR max n=", n), got, r.AlignedReal(0, n), relTol)
	}
}

// Closes outside their own bar exercise the clamp in C and Go alike.
func TestWILLROracleMalformed(t *testing.T) {
	s := fixtures(300)[1]
	c := append([]float64(nil), s.C...)
	for i := range c {
		switch i % 5 {
		case 1:
			c[i] = s.H[i] + 3
		case 3:
			c[i] = s.L[i] - 3
		}
	}
	in := oracle.Inputs{Open: s.O, High: s.H, Low: s.L, Close: c, Volume: s.V}
	for _, p := range []int{2, 3, 14} {
		got, err := WILLR(s.H, s.L, c, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "WILLR", in, float64(p))
		assertReal(t, lbl("WILLR malformed p=", p), got, r.AlignedReal(0, len(c)), relTol)
	}
}

func TestWILLROracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{1, 0, -5, 2, 14, 100000, 100001} {
		r := mustCall(t, "WILLR", s.ohlcv(), float64(p))
		_, err := WILLR(s.H, s.L, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		lb := WILLRLookback(p)
		if c := mustLookback(t, "WILLR", float64(p)); lb != c {
			t.Errorf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
}

func TestWILLROracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {8, 9}, {12, 31}, {100, 119}} {
		for _, p := range []int{2, 3, 10, 14} {
			out := make([]float64, len(s.C))
			beg, nb, err := willrRange(rg[0], rg[1], s.H, s.L, s.C, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "WILLR", s.ohlcv(), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("WILLR range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
