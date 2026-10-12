//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length. OBV has no optional inputs.

func obvIn(s series) oracle.Inputs {
	return oracle.Inputs{Real: [][]float64{s.C}, Volume: s.V}
}

func TestOBVOracle(t *testing.T) {
	lb := OBVLookback()
	if c := mustLookback(t, "OBV"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := OBV(s.C, s.V)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "OBV", obvIn(s))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("OBV ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
			// Also drive OBV with the open series as inReal.
			got, _ = OBV(s.O, s.V)
			r = mustCall(t, "OBV", oracle.Inputs{Real: [][]float64{s.O}, Volume: s.V})
			assertReal(t, lbl("OBV open ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

// OBV has no optional inputs, so C can only reject the index range; Go must
// reject the same ranges.
func TestOBVOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	out := make([]float64, 20)
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}, {0, 19}} {
		r := mustCallRange(t, "OBV", obvIn(s), rg[0], rg[1])
		_, _, err := obvRange(rg[0], rg[1], s.C, s.V, out)
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestOBVOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[4], fixtures(120)[9], fixtures(120)[13]} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {1, 1}, {50, 119}} {
			out := make([]float64, 120)
			beg, nb, err := obvRange(rg[0], rg[1], s.C, s.V, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "OBV", obvIn(s), rg[0], rg[1])
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("OBV range ", s.Name, " ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
