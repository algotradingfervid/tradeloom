//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length. EXP has no optional inputs. Large fixtures overflow to
// +Inf in both; the special-value test covers the overflow and underflow
// bands exactly.

func TestEXPOracle(t *testing.T) {
	lb := EXPLookback()
	if c := mustLookback(t, "EXP"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			in := s.Real()
			got, err := EXP(in)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "EXP", realIn(in))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("EXP ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

func TestEXPOracleSpecial(t *testing.T) {
	in := append([]float64(nil), expSpecial...)
	// A dense sweep of [-750, 712], plus fine sweeps of both the overflow
	// threshold and the subnormal/underflow band.
	for x := -750.0; x <= 712; x += 0.37 {
		in = append(in, x)
	}
	for x := 709.0; x <= 711; x += 1.0 / 1024 {
		in = append(in, x)
	}
	for x := -746.0; x <= -707; x += 1.0 / 4096 {
		in = append(in, x)
	}
	got, err := EXP(in)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "EXP", realIn(in))
	if r.RetCode != oracle.Success {
		t.Fatalf("C retcode %d", r.RetCode)
	}
	want := r.AlignedReal(0, len(in))
	assertReal(t, "EXP special", got, want, relTol)
	// closeEnough is absolute below 1, which would hide any disagreement in
	// the tiny/subnormal results (including 0 versus a nonzero subnormal);
	// also compare relatively there. Subnormals carry fewer significant bits,
	// and libm and fdlibm round them independently, so allow one unit in
	// their last place, but zero must match zero exactly.
	for i := range got {
		g, w := got[i], want[i]
		if math.IsNaN(g) || math.IsNaN(w) || math.IsInf(g, 0) || math.IsInf(w, 0) || g == w {
			continue
		}
		if w >= 2.2250738585072014e-308 {
			if math.Abs(g-w) > relTol*math.Abs(w) {
				t.Errorf("exp(%.17g): go %.17g c %.17g", in[i], g, w)
			}
		} else if (g == 0) != (w == 0) || math.Abs(g-w) > 5e-324 {
			t.Errorf("subnormal exp(%.17g): go %.17g c %.17g", in[i], g, w)
		}
	}
}

// EXP has no optional inputs, so C can only reject the index range; Go must
// reject the same ranges.
func TestEXPOracleBadParams(t *testing.T) {
	in := fixtures(20)[2].Real()
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}, {3, 2}} {
		r := mustCallRange(t, "EXP", realIn(in), rg[0], rg[1])
		_, _, err := expRange(rg[0], rg[1], in, make([]float64, len(in)))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestEXPOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[2], fixtures(120)[13]} {
		in := s.Real()
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {118, 119}} {
			out := make([]float64, len(in))
			beg, nb, err := expRange(rg[0], rg[1], in, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "EXP", realIn(in), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("EXP range ", s.Name, " ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
