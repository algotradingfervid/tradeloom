//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and several unstable periods. HA has no optional inputs.

func haOracleCompare(t *testing.T, prefix string) {
	t.Helper()
	lb := HALookback()
	if c := mustLookback(t, "HA"); lb != c {
		t.Fatalf("%slookback: go %d c %d", prefix, lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			o, h, l, c, err := HA(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "HA", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			for k, got := range [][]float64{o, h, l, c} {
				assertReal(t, lbl(prefix, "HA ", s.Name, " n=", n, " out#", k), got, r.AlignedReal(k, n), relTol)
			}
		}
	}
}

func haOracleRanges(t *testing.T, prefix string) {
	t.Helper()
	for _, s := range []series{fixtures(120)[1], fixtures(120)[4], fixtures(120)[11], fixtures(120)[13]} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {1, 1}, {0, 8}, {9, 9}, {37, 40}, {50, 119}} {
			outs := [4][]float64{make([]float64, 120), make([]float64, 120), make([]float64, 120), make([]float64, 120)}
			beg, nb, err := haRange(rg[0], rg[1], s.O, s.H, s.L, s.C, outs[0], outs[1], outs[2], outs[3])
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "HA", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("%s%s range %v: C retcode %d", prefix, s.Name, rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s%s range %v: go (%d,%d) c (%d,%d)", prefix, s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			for k := range outs {
				assertReal(t, lbl(prefix, "HA range ", s.Name, " ", rg, " out#", k), outs[k][:nb], r.Outputs[k].Real, relTol)
			}
		}
	}
}

func TestHAOracle(t *testing.T) {
	haOracleCompare(t, "")
}

func TestHAOracleRanges(t *testing.T) {
	haOracleRanges(t, "")
}

// HA has no optional inputs, so C can only reject the index range; Go must
// reject the same ranges.
func TestHAOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	out := func() []float64 { return make([]float64, 20) }
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}, {0, 19}, {19, 19}} {
		r := mustCallRange(t, "HA", s.ohlcv(), rg[0], rg[1])
		_, _, err := haRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out(), out(), out(), out())
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestHAOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 2, 5, 37} {
		SetUnstablePeriod(UnstHA, u)
		oracle.SetUnstablePeriod(int(UnstHA), u)
		haOracleCompare(t, lbl("unst=", u, " "))
		haOracleRanges(t, lbl("unst=", u, " "))
	}
	// Another function's unstable period must not leak into HA.
	SetAllUnstablePeriods(0)
	oracle.ResetUnstablePeriods()
	SetUnstablePeriod(UnstEMA, 9)
	oracle.SetUnstablePeriod(int(UnstEMA), 9)
	haOracleCompare(t, "ema-unst ")
}

// Signed zeros must come out with the same sign bits as C: the extremum uses
// plain comparisons, so ties keep the raw high/low.
func TestHAOracleSignedZeros(t *testing.T) {
	nz := math.Copysign(0, -1)
	s := series{
		Name: "signed-zeros",
		O:    []float64{nz, 0, nz, 0, nz},
		H:    []float64{0, 0, nz, 0, 0},
		L:    []float64{nz, nz, nz, 0, nz},
		C:    []float64{nz, 0, nz, nz, 0},
		V:    []float64{1, 1, 1, 1, 1},
	}
	o, h, l, c, err := HA(s.O, s.H, s.L, s.C)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "HA", s.ohlcv())
	if r.RetCode != oracle.Success {
		t.Fatalf("C retcode %d", r.RetCode)
	}
	for k, got := range [][]float64{o, h, l, c} {
		want := r.AlignedReal(k, len(s.O))
		for i := range got {
			if math.Float64bits(got[i]) != math.Float64bits(want[i]) {
				t.Errorf("out#%d [%d]: go %v (bits %x) c %v (bits %x)", k, i, got[i], math.Float64bits(got[i]), want[i], math.Float64bits(want[i]))
			}
		}
	}
}
