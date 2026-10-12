//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length. TANH has no optional inputs.

func TestTANHOracle(t *testing.T) {
	lb := TANHLookback()
	if c := mustLookback(t, "TANH"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			in := s.Real()
			got, err := TANH(in)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "TANH", realIn(in))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("TANH ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

func TestTANHOracleSpecial(t *testing.T) {
	in := append([]float64(nil), tanhSpecial...)
	// A dense sweep of [-50, 50] covering the small-argument polynomial
	// region (|x| < 0.625) and the saturation point (~44.01).
	for x := -50.0; x <= 50; x += 0.0137 {
		in = append(in, x)
	}
	for x := 1e-12; x < 1; x *= 1.7 {
		in = append(in, x, -x)
	}
	got, err := TANH(in)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "TANH", realIn(in))
	if r.RetCode != oracle.Success {
		t.Fatalf("C retcode %d", r.RetCode)
	}
	assertReal(t, "TANH special", got, r.AlignedReal(0, len(in)), relTol)
}

// TANH has no optional inputs, so C can only reject the index range; Go must
// reject the same ranges.
func TestTANHOracleBadParams(t *testing.T) {
	in := fixtures(20)[2].Real()
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}, {3, 2}} {
		r := mustCallRange(t, "TANH", realIn(in), rg[0], rg[1])
		_, _, err := tanhRange(rg[0], rg[1], in, make([]float64, len(in)))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestTANHOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[2], fixtures(120)[13]} {
		in := s.Real()
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {118, 119}} {
			out := make([]float64, len(in))
			beg, nb, err := tanhRange(rg[0], rg[1], in, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "TANH", realIn(in), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("TANH range ", s.Name, " ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
