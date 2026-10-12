//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture pair
// and every edge length.

func TestADDOracle(t *testing.T) {
	lb := ADDLookback()
	if c := mustLookback(t, "ADD"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		fs := fixtures(n)
		for i, s := range fs {
			for _, j := range []int{i, (i + 1) % len(fs), (i + 7) % len(fs)} {
				other := fs[j]
				got, err := ADD(s.Real(), other.Real())
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "ADD", realIn(s.Real(), other.Real()))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("ADD ", s.Name, "+", other.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
			}
			// Other components of the same fixture as the second input.
			if n > 0 {
				got, _ := ADD(s.H, s.L)
				r := mustCall(t, "ADD", realIn(s.H, s.L))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("ADD H+L ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestADDOracleBadParams(t *testing.T) {
	// ADD has no optional inputs; the only C-side rejections are index errors,
	// which Go reports the same way through the range form.
	a := fixtures(20)[0].Real()
	b := fixtures(20)[1].Real()
	r := mustCall(t, "ADD", realIn(a, b))
	_, err := ADD(a, b)
	if (r.RetCode == oracle.BadParam) != (err != nil) {
		t.Errorf("C retcode %d, Go err %v", r.RetCode, err)
	}
}

func TestADDOracleRanges(t *testing.T) {
	fs := fixtures(120)
	for _, pair := range [][2]int{{1, 13}, {6, 4}, {3, 3}} {
		a, b := fs[pair[0]].Real(), fs[pair[1]].Real()
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {5, 119}} {
			out := make([]float64, len(a))
			beg, nb, err := addRange(rg[0], rg[1], a, b, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "ADD", realIn(a, b), rg[0], rg[1])
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%v range %v: go (%d,%d) c (%d,%d)", pair, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("ADD range ", pair, " ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
