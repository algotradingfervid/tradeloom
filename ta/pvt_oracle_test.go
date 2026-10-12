//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length. PVT has no optional inputs.

func pvtIn(s series) oracle.Inputs {
	return oracle.Inputs{Close: s.C, Volume: s.V}
}

func TestPVTOracle(t *testing.T) {
	lb := PVTLookback()
	if c := mustLookback(t, "PVT"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := PVT(s.C, s.V)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "PVT", pvtIn(s))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("PVT ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
			// Also drive PVT with the open series as the close input.
			got, _ = PVT(s.O, s.V)
			r = mustCall(t, "PVT", oracle.Inputs{Close: s.O, Volume: s.V})
			assertReal(t, lbl("PVT open ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

// PVT has no optional inputs, so C can only reject the index range; Go must
// reject the same ranges.
func TestPVTOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	out := make([]float64, 20)
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}, {0, 19}, {19, 19}} {
		r := mustCallRange(t, "PVT", pvtIn(s), rg[0], rg[1])
		_, _, err := pvtRange(rg[0], rg[1], s.C, s.V, out)
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestPVTOracleRanges(t *testing.T) {
	all := fixtures(120)
	for _, s := range []series{all[1], all[4], all[6], all[9], all[13]} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {1, 1}, {50, 119}} {
			out := make([]float64, 120)
			beg, nb, err := pvtRange(rg[0], rg[1], s.C, s.V, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "PVT", pvtIn(s), rg[0], rg[1])
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("PVT range ", s.Name, " ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
