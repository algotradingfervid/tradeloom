//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length. HT_PHASOR has no optional inputs; the only setting is
// its unstable period.

func htphasorOracleCompare(t *testing.T, tag string) {
	t.Helper()
	lb := HTPHASORLookback()
	if c := mustLookback(t, "HT_PHASOR"); lb != c {
		t.Fatalf("%slookback: go %d c %d", tag, lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			i, q, err := HTPHASOR(s.Real())
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "HT_PHASOR", realIn(s.Real()))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			l := lbl(tag, s.Name, " n=", n)
			assertReal(t, "InPhase "+l, i, r.AlignedReal(0, n), relTol)
			assertReal(t, "Quadrature "+l, q, r.AlignedReal(1, n), relTol)
		}
	}
}

func TestHTPHASOROracle(t *testing.T) {
	htphasorOracleCompare(t, "")
}

func TestHTPHASOROracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 2, 10, 50} {
		if err := SetUnstablePeriod(UnstHTPhasor, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstHTPhasor), u)
		htphasorOracleCompare(t, lbl("unstable=", u, " "))
	}
}

// HT_PHASOR has no optional inputs, so C can never return TA_BAD_PARAM for a
// parameter; both sides must succeed on any non-empty input.
func TestHTPHASOROracleBadParams(t *testing.T) {
	for _, n := range []int{1, 32, 33} {
		in := fixtures(n)[0].Real()
		r := mustCall(t, "HT_PHASOR", realIn(in))
		_, _, err := HTPHASOR(in)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("n=%d: C retcode %d, Go err %v", n, r.RetCode, err)
		}
		if err != nil || r.RetCode != oracle.Success {
			t.Errorf("n=%d: C retcode %d, Go err %v, want success", n, r.RetCode, err)
		}
	}
}

func TestHTPHASOROracleRanges(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{0, 5} {
		if err := SetUnstablePeriod(UnstHTPhasor, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstHTPhasor), u)
		for _, s := range []series{fixtures(300)[1], fixtures(300)[11], fixtures(300)[9]} {
			in := s.Real()
			last := len(in) - 1
			for _, rg := range [][2]int{{0, 0}, {0, last}, {3, 3}, {0, 31}, {0, 32}, {32, 32}, {33, 33},
				{10, 50}, {40, 41}, {41, 120}, {100, 299}, {last, last}, {31, 40}, {36, 37}, {37, 37}} {
				oi, oq := make([]float64, len(in)), make([]float64, len(in))
				beg, nb, err := htphasorRange(rg[0], rg[1], in, oi, oq)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "HT_PHASOR", realIn(in), rg[0], rg[1])
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s u=%d range %v: go (%d,%d) c (%d,%d)", s.Name, u, rg, beg, nb, r.BegIdx, r.NbElement)
				}
				l := lbl(s.Name, " u=", u, " range ", rg)
				assertReal(t, "InPhase "+l, oi[:nb], r.Outputs[0].Real, relTol)
				assertReal(t, "Quadrature "+l, oq[:nb], r.Outputs[1].Real, relTol)
			}
		}
	}
}
