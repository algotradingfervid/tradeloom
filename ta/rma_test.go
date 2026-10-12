package ta

import (
	"errors"
	"math"
	"testing"
)

func TestRMAParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := RMA(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := RMALookback(p); lb != -1 {
			t.Errorf("RMALookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := rmaRange(0, len(in)-1, in, p, make([]float64, len(in))); !errors.Is(err, ErrBadParam) {
			t.Errorf("rmaRange period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 30, 100000} {
		if _, err := RMA(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := RMALookback(p); lb != p-1 {
			t.Errorf("RMALookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	// Period at max on a short input: valid, all NaN.
	out, err := RMA(in, 100000)
	if err != nil || len(out) != len(in) {
		t.Fatalf("period max: len=%d err=%v", len(out), err)
	}
	assertLeadingNaN(t, "RMA p=100000", out, len(in))

	if out, err := RMA(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := RMA([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := rmaRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := rmaRange(0, 10, in, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Output buffer exactly endIdx-startIdx+1 long is enough.
	if _, _, err := rmaRange(5, 10, in, 3, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestRMALengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 14, 30} {
		lb := RMALookback(period)
		if lb != period-1 {
			t.Fatalf("RMALookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := RMA(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("RMA ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestRMAKnownValues(t *testing.T) {
	nan := math.NaN()

	// Period 2: alpha = beta = 0.5 exactly. Seed (2+4)/2 = 3 at bar 1, then
	// 0.5*3+0.5*6 = 4.5, 0.5*4.5+0.5*8 = 6.25.
	out, err := RMA([]float64{2, 4, 6, 8}, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "RMA(2,4,6,8;2)", out, []float64{nan, 3, 4.5, 6.25}, 0)

	// Period 3: alpha = 1/3. Seed SMA(1,2,3) = 2, then
	// 2/3*2 + 4/3 = 8/3, 2/3*8/3 + 5/3 = 31/9, 2/3*31/9 + 6/3 = 116/27.
	out, _ = RMA([]float64{1, 2, 3, 4, 5, 6}, 3)
	assertReal(t, "RMA(1..6;3)", out, []float64{nan, nan, 2, 8.0 / 3, 31.0 / 9, 116.0 / 27}, 1e-14)

	// Period 1 is the identity (alpha, beta are exactly 1, 0).
	for _, s := range fixtures(100) {
		in := s.Real()
		out, _ = RMA(in, 1)
		assertReal(t, "RMA period 1 "+s.Name, out, in, 0)
	}

	// A constant series smooths to itself.
	c := fixtures(40)[5].Real()
	out, _ = RMA(c, 7)
	for i := 6; i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// The output stays within the running min/max of the input seen so far,
	// and tracks a strictly rising series from below while rising itself.
	up := fixtures(200)[7].Real()
	out, _ = RMA(up, 10)
	for i := 10; i < len(out); i++ {
		if !(out[i] < up[i]) || !(out[i] > out[i-1]) {
			t.Fatalf("up trend [%d]: rma %g in %g prev %g", i, out[i], up[i], out[i-1])
		}
	}
	w := fixtures(300)[0].Real()
	out, _ = RMA(w, 14)
	lo, hi := math.Inf(1), math.Inf(-1)
	for i, v := range w {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
		if i >= 13 && (out[i] < lo || out[i] > hi) {
			t.Fatalf("bounds [%d]: %g not in [%g, %g]", i, out[i], lo, hi)
		}
	}
}

// TA-Lib's contract: RMA over the true range is ATR, bit for bit.
func TestRMAOverTrueRangeIsATR(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	for _, unst := range []int{0, 4} {
		SetUnstablePeriod(UnstRMA, unst)
		SetUnstablePeriod(UnstATR, unst)
		for _, s := range fixtures(300) {
			tr := make([]float64, len(s.C)-1)
			for j := 1; j < len(s.C); j++ {
				tr[j-1] = trueRange(s.H[j], s.L[j], s.C[j-1])
			}
			for _, p := range []int{2, 5, 14, 30} {
				atr, err := ATR(s.H, s.L, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				rma, err := RMA(tr, p)
				if err != nil {
					t.Fatal(err)
				}
				assertReal(t, lbl("RMA(TR)==ATR ", s.Name, " p=", p, " unst=", unst), rma, atr[1:], 0)
			}
		}
	}
}

func TestRMAUnstablePeriod(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstRMA, 2); err != nil {
		t.Fatal(err)
	}
	if lb := RMALookback(2); lb != 3 {
		t.Fatalf("RMALookback(2) with unstable 2 = %d, want 3", lb)
	}
	nan := math.NaN()
	// Seed is still (2+4)/2 = 3 at bar 1; bars 2 and 3 are absorbed
	// (4.5, 6.25), first output at bar 3.
	out, _ := RMA([]float64{2, 4, 6, 8, 10}, 2)
	assertReal(t, "RMA unstable 2", out, []float64{nan, nan, nan, 6.25, 8.125}, 0)
	// Period 1 copies the input but still honours the unstable period.
	out, _ = RMA([]float64{1, 2, 3, 4}, 1)
	assertReal(t, "RMA p1 unstable 2", out, []float64{nan, nan, 3, 4}, 0)
	// Lengths around the shifted lookback.
	for _, p := range []int{1, 3, 14} {
		lb := RMALookback(p)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := RMA(s.Real(), p)
				if err != nil || len(out) != n {
					t.Fatalf("%s n=%d p=%d: len=%d err=%v", s.Name, n, p, len(out), err)
				}
				assertLeadingNaN(t, lbl("RMA unst ", s.Name, " n=", n, " p=", p), out, lb)
			}
		}
	}
	// Unrelated unstable ids do not affect RMA.
	SetAllUnstablePeriods(0)
	SetUnstablePeriod(UnstEMA, 5)
	if lb := RMALookback(3); lb != 2 {
		t.Fatalf("RMALookback(3) with EMA unstable = %d, want 2", lb)
	}
}

func TestRMARangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := rmaRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := rmaRange(taMaxIdx+1, taMaxIdx+1, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := rmaRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := rmaRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := rmaRange(0, 1, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Start inside the lookback is moved to the lookback.
	beg, nb, err = rmaRange(0, 10, in, 3, out)
	if err != nil || beg != 2 || nb != 9 {
		t.Errorf("start in lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Single bar.
	beg, nb, err = rmaRange(20, 20, in, 3, out)
	if err != nil || beg != 20 || nb != 1 {
		t.Errorf("single bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// A sub-range re-seeds at startIdx-lookback: its first value is the
	// plain average of the period bars ending at startIdx, summed in order.
	if want := (0 + in[18] + in[19] + in[20]) / 3; out[0] != want {
		t.Errorf("sub-range seed: got %.17g want %.17g", out[0], want)
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	full, _ := RMA(in, 4)
	beg, nb, _ = rmaRange(0, len(buf)-1, buf, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, full, 0)
}
