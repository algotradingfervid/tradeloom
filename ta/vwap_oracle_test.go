//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length. VWAP has no optional inputs.

func TestVWAPOracle(t *testing.T) {
	lb := VWAPLookback()
	if c := mustLookback(t, "VWAP"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := VWAP(s.H, s.L, s.C, s.V)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "VWAP", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("VWAP ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

// Non-finite bars are skipped by both; overflowing products reach the sums.
func TestVWAPOracleNonFinite(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	base := fixtures(64)[0]
	cases := []struct {
		name string
		mod  func(s *series)
	}{
		{"nan-close", func(s *series) { s.C[0], s.C[5], s.C[63] = nan, nan, nan }},
		{"inf-high", func(s *series) { s.H[3] = inf }},
		{"-inf-low", func(s *series) { s.L[10] = -inf }},
		{"nan-vol", func(s *series) { s.V[0], s.V[20] = nan, nan }},
		{"inf-vol", func(s *series) { s.V[7] = inf }},
		{"lead-zero-vol", func(s *series) {
			for i := 0; i < 10; i++ {
				s.V[i] = 0
			}
		}},
		{"neg-vol", func(s *series) { s.V[0], s.V[1] = -5, 2 }},
		{"overflow", func(s *series) { s.H[30], s.L[30], s.C[30], s.V[30] = 1e300, 1e300, 1e300, 1e300 }},
	}
	for _, tc := range cases {
		s := series{Name: tc.name,
			O: append([]float64(nil), base.O...), H: append([]float64(nil), base.H...),
			L: append([]float64(nil), base.L...), C: append([]float64(nil), base.C...),
			V: append([]float64(nil), base.V...)}
		tc.mod(&s)
		got, err := VWAP(s.H, s.L, s.C, s.V)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "VWAP", s.ohlcv())
		if r.RetCode != oracle.Success {
			t.Fatalf("%s: C retcode %d", tc.name, r.RetCode)
		}
		assertReal(t, lbl("VWAP ", tc.name), got, r.AlignedReal(0, 64), relTol)
	}
}

// VWAP has no optional inputs, so C can only reject the index range; Go must
// reject the same ranges.
func TestVWAPOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	out := make([]float64, 20)
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}, {0, 19}} {
		r := mustCallRange(t, "VWAP", s.ohlcv(), rg[0], rg[1])
		_, _, err := vwapRange(rg[0], rg[1], s.H, s.L, s.C, s.V, out)
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestVWAPOracleRanges(t *testing.T) {
	f := fixtures(120)
	for _, s := range []series{f[1], f[4], f[11], f[12], f[13]} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {1, 1}, {50, 119}} {
			out := make([]float64, 120)
			beg, nb, err := vwapRange(rg[0], rg[1], s.H, s.L, s.C, s.V, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "VWAP", s.ohlcv(), rg[0], rg[1])
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("VWAP range ", s.Name, " ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
