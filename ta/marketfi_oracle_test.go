//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length. MARKETFI has no optional inputs.

func TestMARKETFIOracle(t *testing.T) {
	lb := MARKETFILookback()
	if c := mustLookback(t, "MARKETFI"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := MARKETFI(s.H, s.L, s.V)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "MARKETFI", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("MARKETFI ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

// Special volumes: exact zero, negative zero, negative, tiny, NaN and a mix
// with the regular fixture data.
func TestMARKETFIOracleSpecial(t *testing.T) {
	s := fixtures(12)[0]
	s.V = []float64{0, math.Copysign(0, -1), -5, 1e-300, math.NaN(), 1e-15, 1, 0, 2, -0.5, 1e300, 3}
	s.H[2], s.L[2] = s.L[2], s.H[2] // malformed bar: high < low
	got, err := MARKETFI(s.H, s.L, s.V)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "MARKETFI", s.ohlcv())
	if r.RetCode != oracle.Success {
		t.Fatalf("C retcode %d", r.RetCode)
	}
	assertReal(t, "MARKETFI special", got, r.AlignedReal(0, len(s.V)), relTol)
}

// MARKETFI has no optional inputs, so C can only reject the index range;
// Go must reject the same ranges.
func TestMARKETFIOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	out := make([]float64, 20)
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}, {0, 19}, {19, 19}} {
		r := mustCallRange(t, "MARKETFI", s.ohlcv(), rg[0], rg[1])
		_, _, err := marketfiRange(rg[0], rg[1], s.H, s.L, s.V, out)
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestMARKETFIOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[4], fixtures(120)[6], fixtures(120)[12], fixtures(120)[13]} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {1, 1}, {50, 119}} {
			out := make([]float64, 120)
			beg, nb, err := marketfiRange(rg[0], rg[1], s.H, s.L, s.V, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "MARKETFI", s.ohlcv(), rg[0], rg[1])
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("MARKETFI range ", s.Name, " ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
