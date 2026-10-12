//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture pair
// and every edge length.

func TestDIVOracle(t *testing.T) {
	lb := DIVLookback()
	if c := mustLookback(t, "DIV"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		fs := fixtures(n)
		for i, s := range fs {
			// j == i includes zeros/zeros (0/0 = NaN); i+1 includes
			// constant/zeros (100/0 = +Inf) and zeros/up (0/x = 0).
			for _, j := range []int{i, (i + 1) % len(fs), (i + 7) % len(fs)} {
				other := fs[j]
				got, err := DIV(s.Real(), other.Real())
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "DIV", realIn(s.Real(), other.Real()))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("DIV ", s.Name, "/", other.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
			}
			// Other components of the same fixture as the second input.
			if n > 0 {
				got, _ := DIV(s.H, s.L)
				r := mustCall(t, "DIV", realIn(s.H, s.L))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("DIV H/L ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

// Special IEEE cases: signed zeros, infinities and NaN must come out of C and
// Go identically.
func TestDIVOracleSpecial(t *testing.T) {
	nan := math.NaN()
	inf := math.Inf(1)
	negZero := math.Copysign(0, -1)
	a := []float64{1, -1, 0, 1, nan, 2, inf, 5, -3, 1e308, 1e-308}
	b := []float64{0, 0, 0, negZero, 2, nan, -inf, inf, 7, 1e-308, 1e308}
	got, err := DIV(a, b)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "DIV", realIn(a, b))
	if r.RetCode != oracle.Success {
		t.Fatalf("C retcode %d", r.RetCode)
	}
	assertReal(t, "DIV special", got, r.AlignedReal(0, len(a)), 0)
}

func TestDIVOracleBadParams(t *testing.T) {
	// DIV has no optional inputs; the only C-side rejections are index errors,
	// which Go reports the same way through the range form.
	a := fixtures(20)[0].Real()
	b := fixtures(20)[1].Real()
	r := mustCall(t, "DIV", realIn(a, b))
	_, err := DIV(a, b)
	if (r.RetCode == oracle.BadParam) != (err != nil) {
		t.Errorf("C retcode %d, Go err %v", r.RetCode, err)
	}
}

func TestDIVOracleRanges(t *testing.T) {
	fs := fixtures(120)
	for _, pair := range [][2]int{{1, 13}, {6, 4}, {3, 3}, {5, 6}} {
		a, b := fs[pair[0]].Real(), fs[pair[1]].Real()
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {5, 119}} {
			out := make([]float64, len(a))
			beg, nb, err := divRange(rg[0], rg[1], a, b, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "DIV", realIn(a, b), rg[0], rg[1])
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%v range %v: go (%d,%d) c (%d,%d)", pair, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("DIV range ", pair, " ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
