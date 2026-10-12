//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestLINEARREGINTERCEPTOracle(t *testing.T) {
	for _, p := range []int{2, 3, 5, 14, 30, 200, 1000, 100000} {
		lb := LINEARREGINTERCEPTLookback(p)
		if c := mustLookback(t, "LINEARREG_INTERCEPT", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			if n > 2000 {
				continue // p=100000: lb-1..lb+10 lengths are covered in TestLINEARREGINTERCEPTOracleMaxPeriod
			}
			for _, s := range fixtures(n) {
				got, err := LINEARREGINTERCEPT(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "LINEARREG_INTERCEPT", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("LINEARREGINTERCEPT ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

// The maximum period around its lookback, on two fixtures.
func TestLINEARREGINTERCEPTOracleMaxPeriod(t *testing.T) {
	const p = 100000
	for _, n := range []int{p - 2, p - 1, p, p + 1, p + 10} {
		for _, s := range fixtures(n)[:2] {
			got, err := LINEARREGINTERCEPT(s.Real(), p)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "LINEARREG_INTERCEPT", realIn(s.Real()), float64(p))
			assertReal(t, lbl("LINEARREGINTERCEPT max ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

// A series with a huge spike followed by small values drives the magnitude
// re-anchor trigger (|weightedTrailing| > 100*sumAbs); zeros are mixed in
// mid-series.
func TestLINEARREGINTERCEPTOracleReseed(t *testing.T) {
	in := make([]float64, 400)
	for i := range in {
		in[i] = float64(i%7) + 1
		if i%50 == 10 {
			in[i] = 1e12
		}
		if i%37 == 0 {
			in[i] = 0
		}
	}
	for _, p := range []int{2, 3, 5, 9} {
		got, err := LINEARREGINTERCEPT(in, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "LINEARREG_INTERCEPT", realIn(in), float64(p))
		assertReal(t, lbl("LINEARREGINTERCEPT reseed p=", p), got, r.AlignedReal(0, len(in)), relTol)
	}
}

func TestLINEARREGINTERCEPTOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{1, 0, -5, 100001, 2, 100000} {
		r := mustCall(t, "LINEARREG_INTERCEPT", realIn(in), float64(p))
		_, err := LINEARREGINTERCEPT(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "LINEARREG_INTERCEPT", float64(p)); c != LINEARREGINTERCEPTLookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, LINEARREGINTERCEPTLookback(p), c)
		}
	}
}

func TestLINEARREGINTERCEPTOracleRanges(t *testing.T) {
	for _, s := range fixtures(120)[:4] {
		in := s.Real()
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {9, 30}, {100, 119}} {
			for _, p := range []int{2, 10} {
				out := make([]float64, len(in))
				beg, nb, err := linearreginterceptRange(rg[0], rg[1], in, p, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "LINEARREG_INTERCEPT", realIn(in), rg[0], rg[1], float64(p))
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%d: go (%d,%d) c (%d,%d)", s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("LINEARREGINTERCEPT range ", s.Name, rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

// Long series run through the periodic 32*period re-anchor many times; a late
// sub-range starts its own priming scan mid-series.
func TestLINEARREGINTERCEPTOracleLong(t *testing.T) {
	for _, s := range fixtures(5000) {
		in := s.Real()
		for _, p := range []int{2, 3, 7, 50} {
			got, err := LINEARREGINTERCEPT(in, p)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "LINEARREG_INTERCEPT", realIn(in), float64(p))
			assertReal(t, lbl("LINEARREGINTERCEPT long ", s.Name, " p=", p), got, r.AlignedReal(0, len(in)), relTol)

			out := make([]float64, len(in))
			beg, nb, err := linearreginterceptRange(3333, 4999, in, p, out)
			if err != nil {
				t.Fatal(err)
			}
			rr := mustCallRange(t, "LINEARREG_INTERCEPT", realIn(in), 3333, 4999, float64(p))
			if beg != rr.BegIdx || nb != rr.NbElement {
				t.Fatalf("%s late range p=%d: go (%d,%d) c (%d,%d)", s.Name, p, beg, nb, rr.BegIdx, rr.NbElement)
			}
			assertReal(t, lbl("LINEARREGINTERCEPT late range ", s.Name, " p=", p), out[:nb], rr.Outputs[0].Real, relTol)
		}
	}
}
