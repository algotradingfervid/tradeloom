//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length, with and without an unstable period. Every output bar is
// compared, including flat and zigzag series, where the DFT phase is decided
// by the last ulp of libm's sin/cos (reproduced by htSinCos).

func htsineOracleCompare(t *testing.T, tag string) {
	t.Helper()
	lb := HTSINELookback()
	if c := mustLookback(t, "HT_SINE"); lb != c {
		t.Fatalf("%slookback: go %d c %d", tag, lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			sine, lead, err := HTSINE(s.Real())
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "HT_SINE", realIn(s.Real()))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			l := lbl(tag, s.Name, " n=", n)
			// The lookback region must be NaN in both.
			cs, cl := r.AlignedReal(0, n), r.AlignedReal(1, n)
			k := cMinInt(lb, n)
			assertReal(t, "HT_SINE sine lookback "+l, sine[:k], cs[:k], 0)
			assertReal(t, "HT_SINE lead lookback "+l, lead[:k], cl[:k], 0)
			if n <= lb {
				continue
			}
			assertReal(t, "HT_SINE sine "+l, sine[lb:], cs[lb:], relTol)
			assertReal(t, "HT_SINE lead "+l, lead[lb:], cl[lb:], relTol)
		}
	}
}

func TestHTSINEOracle(t *testing.T) {
	htsineOracleCompare(t, "")
}

func TestHTSINEOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 10, 50} {
		if err := SetUnstablePeriod(UnstHTSine, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstHTSine), u)
		htsineOracleCompare(t, lbl("unstable=", u, " "))
	}
}

// HT_SINE has no optional inputs, so there is no bad parameter to pass; check
// instead that C accepts every input Go accepts, including the shortest.
func TestHTSINEOracleBadParams(t *testing.T) {
	for _, n := range []int{1, 63, 64} {
		in := fixtures(n)[0].Real()
		r := mustCall(t, "HT_SINE", realIn(in))
		_, _, err := HTSINE(in)
		if (r.RetCode == oracle.BadParam) != (err != nil) || r.RetCode != oracle.Success {
			t.Errorf("n=%d: C retcode %d, Go err %v", n, r.RetCode, err)
		}
	}
}

func TestHTSINEOracleRanges(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{0, 5} {
		if err := SetUnstablePeriod(UnstHTSine, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstHTSine), u)
		for _, s := range []series{fixtures(300)[1], fixtures(300)[11], fixtures(300)[9]} {
			in := s.Real()
			last := len(in) - 1
			for _, rg := range [][2]int{{0, 0}, {0, last}, {3, 3}, {0, 62}, {0, 63}, {62, 62}, {63, 63},
				{64, 64}, {67, 69}, {10, 50}, {40, 100}, {64, 150}, {100, 299}, {last, last}, {62, 70}} {
				os, ol := make([]float64, len(in)), make([]float64, len(in))
				beg, nb, err := htsineRange(rg[0], rg[1], in, os, ol)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "HT_SINE", realIn(in), rg[0], rg[1])
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s u=%d range %v: go (%d,%d) c (%d,%d)", s.Name, u, rg, beg, nb, r.BegIdx, r.NbElement)
				}
				l := lbl(s.Name, " u=", u, " range ", rg)
				assertReal(t, "HT_SINE sine "+l, os[:nb], r.Outputs[0].Real, relTol)
				assertReal(t, "HT_SINE lead "+l, ol[:nb], r.Outputs[1].Real, relTol)
			}
		}
	}
}
