//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length. ASIN has no optional inputs. Each fixture is tested raw
// (mostly out of domain, so NaN must match NaN) and mapped into [-1, 1].

func TestASINOracle(t *testing.T) {
	lb := ASINLookback()
	if c := mustLookback(t, "ASIN"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			for k, in := range [][]float64{s.Real(), asinUnit(s.Real())} {
				got, err := ASIN(in)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "ASIN", realIn(in))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("ASIN ", s.Name, " n=", n, " unit=", k), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestASINOracleSpecial(t *testing.T) {
	in := asinSpecial
	got, err := ASIN(in)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "ASIN", realIn(in))
	if r.RetCode != oracle.Success {
		t.Fatalf("C retcode %d", r.RetCode)
	}
	assertReal(t, "ASIN special", got, r.AlignedReal(0, len(in)), relTol)
}

// ASIN has no optional inputs, so C can only reject the index range; Go must
// reject the same ranges.
func TestASINOracleBadParams(t *testing.T) {
	in := asinUnit(fixtures(20)[0].Real())
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}, {3, 2}} {
		r := mustCallRange(t, "ASIN", realIn(in), rg[0], rg[1])
		_, _, err := asinRange(rg[0], rg[1], in, make([]float64, len(in)))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestASINOracleRanges(t *testing.T) {
	in := asinUnit(fixtures(120)[1].Real())
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {118, 119}} {
		out := make([]float64, len(in))
		beg, nb, err := asinRange(rg[0], rg[1], in, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "ASIN", realIn(in), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertReal(t, lbl("ASIN range ", rg), out[:nb], r.Outputs[0].Real, relTol)
	}
}
