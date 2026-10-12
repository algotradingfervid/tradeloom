//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length. MEDPRICE has no optional inputs.

func TestMEDPRICEOracle(t *testing.T) {
	lb := MEDPRICELookback()
	if c := mustLookback(t, "MEDPRICE"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := MEDPRICE(s.H, s.L)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "MEDPRICE", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("MEDPRICE ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

// MEDPRICE has no optional inputs, so C can only reject the index range;
// Go must reject the same ranges.
func TestMEDPRICEOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	out := make([]float64, 20)
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, 19}} {
		r := mustCallRange(t, "MEDPRICE", s.ohlcv(), rg[0], rg[1])
		_, _, err := medpriceRange(rg[0], rg[1], s.H, s.L, out)
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestMEDPRICEOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[4], fixtures(120)[13]} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {1, 1}, {50, 119}} {
			out := make([]float64, 120)
			beg, nb, err := medpriceRange(rg[0], rg[1], s.H, s.L, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "MEDPRICE", s.ohlcv(), rg[0], rg[1])
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("MEDPRICE range ", s.Name, " ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
