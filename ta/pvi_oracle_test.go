//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length. PVI has no optional inputs.

func pviIn(close, volume []float64) oracle.Inputs {
	return oracle.Inputs{Close: close, Volume: volume}
}

func TestPVIOracle(t *testing.T) {
	lb := PVILookback()
	if c := mustLookback(t, "PVI"); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := PVI(s.C, s.V)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "PVI", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("PVI ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
			// Also drive PVI with the open series as close and the high
			// series as volume (non-monotone, non-integer "volume").
			got, _ = PVI(s.O, s.H)
			r = mustCall(t, "PVI", pviIn(s.O, s.H))
			assertReal(t, lbl("PVI open/high ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

// Degenerate inputs exercising the zero-close, NaN, Inf and overflow guards.
func TestPVIOracleDegenerate(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	cases := []struct {
		name          string
		close, volume []float64
	}{
		{"known", []float64{10, 15, 15, 30, 15, 99}, []float64{1, 2, 1, 3, 4, 4}},
		{"zero close", []float64{0, 5, 10, 0, 0, 3}, []float64{1, 2, 3, 4, 5, 6}},
		{"nan close", []float64{10, nan, 20, 40}, []float64{1, 2, 3, 4}},
		{"inf close", []float64{10, inf, 20, 40}, []float64{1, 2, 3, 4}},
		{"-inf close", []float64{10, -inf, 20, 40}, []float64{1, 2, 3, 4}},
		{"nan volume", []float64{10, 20, 40, 80}, []float64{1, nan, 3, 4}},
		{"overflow", []float64{1, 1e300, 1e308, 5e307, 1e308}, []float64{1, 2, 3, 4, 5}},
		{"negative close", []float64{-10, -5, -20, 4}, []float64{1, 2, 3, 4}},
	}
	// Alternating x1e6 gains and near-total losses: each loss nearly cancels
	// the index, so any FMA fusion in Go would show up as large drift.
	var gc, gv []float64
	for i := range 400 {
		if i%2 == 0 {
			gc = append(gc, 1)
		} else {
			gc = append(gc, 1e6)
		}
		gv = append(gv, float64(i))
	}
	cases = append(cases, struct {
		name          string
		close, volume []float64
	}{"compound", gc, gv})
	for _, c := range cases {
		got, err := PVI(c.close, c.volume)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "PVI", pviIn(c.close, c.volume))
		if r.RetCode != oracle.Success {
			t.Fatalf("%s: C retcode %d", c.name, r.RetCode)
		}
		assertReal(t, "PVI "+c.name, got, r.AlignedReal(0, len(c.close)), relTol)
	}
}

// PVI has no optional inputs, so C can only reject the index range; Go must
// reject the same ranges.
func TestPVIOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	out := make([]float64, 20)
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}, {0, 19}, {19, 19}} {
		r := mustCallRange(t, "PVI", s.ohlcv(), rg[0], rg[1])
		_, _, err := pviRange(rg[0], rg[1], s.C, s.V, out)
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
		if err == nil && r.RetCode != oracle.Success {
			t.Errorf("range %v: C rejected, Go accepted", rg)
		}
	}
}

func TestPVIOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[4], fixtures(120)[9], fixtures(120)[11], fixtures(120)[13]} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {1, 1}, {50, 119}, {118, 119}} {
			out := make([]float64, 120)
			beg, nb, err := pviRange(rg[0], rg[1], s.C, s.V, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "PVI", s.ohlcv(), rg[0], rg[1])
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("PVI range ", s.Name, " ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
