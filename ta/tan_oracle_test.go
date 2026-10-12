//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length. TAN has no optional inputs. Each fixture is tested raw
// (large prices, tangent anywhere on the real line) and mapped into
// (-1.2, 1.2) (far from the poles).

func TestTANOracle(t *testing.T) {
	lb := TANLookback()
	if c := mustLookback(t, "TAN"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			for k, in := range [][]float64{s.Real(), tanSmall(s.Real())} {
				got, err := TAN(in)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "TAN", realIn(in))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("TAN ", s.Name, " n=", n, " small=", k), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestTANOracleSpecial(t *testing.T) {
	in := tanSpecial
	got, err := TAN(in)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "TAN", realIn(in))
	if r.RetCode != oracle.Success {
		t.Fatalf("C retcode %d", r.RetCode)
	}
	want := r.AlignedReal(0, len(in))
	assertReal(t, "TAN special", got, want, relTol)
	// Tiny magnitudes fall under closeEnough's absolute floor; demand a
	// relative match there too, including the sign of zero.
	for i := range got {
		if got[i] == 0 && want[i] == 0 && (1/got[i] > 0) != (1/want[i] > 0) {
			t.Errorf("special [%d]: signed zero go %g c %g", i, got[i], want[i])
		}
		if v := in[i]; v != 0 && v > -1e-8 && v < 1e-8 && !closeEnough(got[i]/want[i], 1, relTol) {
			t.Errorf("special [%d]: tiny go %.17g c %.17g", i, got[i], want[i])
		}
	}
}

// TAN has no optional inputs, so C can only reject the index range; Go must
// reject the same ranges.
func TestTANOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}, {3, 2}} {
		r := mustCallRange(t, "TAN", realIn(in), rg[0], rg[1])
		_, _, err := tanRange(rg[0], rg[1], in, make([]float64, len(in)))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestTANOracleRanges(t *testing.T) {
	for k, in := range [][]float64{fixtures(120)[1].Real(), tanSmall(fixtures(120)[1].Real())} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {118, 119}} {
			out := make([]float64, len(in))
			beg, nb, err := tanRange(rg[0], rg[1], in, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "TAN", realIn(in), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("TAN range ", rg, " small=", k), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
