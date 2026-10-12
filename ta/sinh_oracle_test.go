//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length. SINH has no optional inputs. Large fixtures overflow to
// +/-Inf in both; the special-value test covers the overflow band exactly.

func TestSINHOracle(t *testing.T) {
	lb := SINHLookback()
	if c := mustLookback(t, "SINH"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			in := s.Real()
			got, err := SINH(in)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "SINH", realIn(in))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("SINH ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

func TestSINHOracleSpecial(t *testing.T) {
	in := append([]float64(nil), sinhSpecial...)
	// A dense sweep of [-712, 712], including the band where exp overflows
	// but sinh does not.
	for x := -712.0; x <= 712; x += 0.37 {
		in = append(in, x)
	}
	for x := 709.0; x <= 711; x += 1.0 / 1024 {
		in = append(in, x, -x)
	}
	got, err := SINH(in)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "SINH", realIn(in))
	if r.RetCode != oracle.Success {
		t.Fatalf("C retcode %d", r.RetCode)
	}
	assertReal(t, "SINH special", got, r.AlignedReal(0, len(in)), relTol)
}

// SINH has no optional inputs, so C can only reject the index range; Go must
// reject the same ranges.
func TestSINHOracleBadParams(t *testing.T) {
	in := fixtures(20)[2].Real()
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}, {3, 2}} {
		r := mustCallRange(t, "SINH", realIn(in), rg[0], rg[1])
		_, _, err := sinhRange(rg[0], rg[1], in, make([]float64, len(in)))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestSINHOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[2], fixtures(120)[13]} {
		in := s.Real()
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {118, 119}} {
			out := make([]float64, len(in))
			beg, nb, err := sinhRange(rg[0], rg[1], in, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "SINH", realIn(in), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("SINH range ", s.Name, " ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}

// Signed zero must survive: C's sinh(-0) is -0.
func TestSINHOracleSignedZero(t *testing.T) {
	in := []float64{0, math.Copysign(0, -1)}
	got, _ := SINH(in)
	r := mustCall(t, "SINH", realIn(in))
	want := r.AlignedReal(0, len(in))
	for i := range in {
		if math.Signbit(got[i]) != math.Signbit(want[i]) {
			t.Errorf("[%d]: go %g c %g", i, got[i], want[i])
		}
	}
}
