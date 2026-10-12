//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length. CEIL has no optional inputs. Each fixture is tested raw
// and mapped to fractional values of both signs.

func TestCEILOracle(t *testing.T) {
	lb := CEILLookback()
	if c := mustLookback(t, "CEIL"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			for k, in := range [][]float64{s.Real(), ceilFrac(s.Real())} {
				got, err := CEIL(in)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "CEIL", realIn(in))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("CEIL ", s.Name, " n=", n, " frac=", k), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestCEILOracleSpecial(t *testing.T) {
	in := ceilSpecial
	got, err := CEIL(in)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CEIL", realIn(in))
	if r.RetCode != oracle.Success {
		t.Fatalf("C retcode %d", r.RetCode)
	}
	want := r.AlignedReal(0, len(in))
	// ceil is exact, so demand bit-identical results (signed zeros included).
	for i := range in {
		if math.Float64bits(got[i]) != math.Float64bits(want[i]) && !(math.IsNaN(got[i]) && math.IsNaN(want[i])) {
			t.Errorf("special [%d] ceil(%g): go %g (%x) c %g (%x)", i, in[i], got[i], math.Float64bits(got[i]), want[i], math.Float64bits(want[i]))
		}
	}
}

// CEIL has no optional inputs, so C can only reject the index range; Go must
// reject the same ranges.
func TestCEILOracleBadParams(t *testing.T) {
	in := ceilFrac(fixtures(20)[0].Real())
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}, {3, 2}} {
		r := mustCallRange(t, "CEIL", realIn(in), rg[0], rg[1])
		_, _, err := ceilRange(rg[0], rg[1], in, make([]float64, len(in)))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCEILOracleRanges(t *testing.T) {
	in := ceilFrac(fixtures(120)[1].Real())
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {118, 119}} {
		out := make([]float64, len(in))
		beg, nb, err := ceilRange(rg[0], rg[1], in, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CEIL", realIn(in), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertReal(t, lbl("CEIL range ", rg), out[:nb], r.Outputs[0].Real, relTol)
	}
}
