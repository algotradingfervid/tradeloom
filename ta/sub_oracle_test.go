//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture pair
// and every edge length.

func TestSUBOracle(t *testing.T) {
	lb := SUBLookback()
	if c := mustLookback(t, "SUB"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		fs := fixtures(n)
		for i, s := range fs {
			for _, j := range []int{i, (i + 1) % len(fs), (i + 7) % len(fs)} {
				other := fs[j]
				got, err := SUB(s.Real(), other.Real())
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "SUB", realIn(s.Real(), other.Real()))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("SUB ", s.Name, "-", other.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
			}
			// Other components of the same fixture as the second input.
			if n > 0 {
				got, _ := SUB(s.H, s.L)
				r := mustCall(t, "SUB", realIn(s.H, s.L))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("SUB H-L ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestSUBOracleBadParams(t *testing.T) {
	// SUB has no optional inputs; the only C-side rejections are index errors,
	// which Go reports the same way through the range form.
	a := fixtures(20)[0].Real()
	b := fixtures(20)[1].Real()
	r := mustCall(t, "SUB", realIn(a, b))
	_, err := SUB(a, b)
	if (r.RetCode == oracle.BadParam) != (err != nil) {
		t.Errorf("C retcode %d, Go err %v", r.RetCode, err)
	}
}

func TestSUBOracleRanges(t *testing.T) {
	fs := fixtures(120)
	for _, pair := range [][2]int{{1, 13}, {6, 4}, {3, 3}} {
		a, b := fs[pair[0]].Real(), fs[pair[1]].Real()
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {5, 119}} {
			out := make([]float64, len(a))
			beg, nb, err := subRange(rg[0], rg[1], a, b, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "SUB", realIn(a, b), rg[0], rg[1])
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%v range %v: go (%d,%d) c (%d,%d)", pair, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("SUB range ", pair, " ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
