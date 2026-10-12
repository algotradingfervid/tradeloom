package ta

import (
	"errors"
	"math"
	"testing"
)

func TestRSIParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := RSI(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := RSILookback(p); lb != -1 {
			t.Errorf("RSILookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := rsiRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, err := RSI(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := RSILookback(p); lb != p {
			t.Errorf("RSILookback(%d) = %d, want %d", p, lb, p)
		}
	}
	if out, err := RSI(nil, 14); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := RSI([]float64{}, 14); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestRSILengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30} {
		lb := RSILookback(period)
		if lb != period {
			t.Fatalf("RSILookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := RSI(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("RSI ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestRSIKnownValues(t *testing.T) {
	// Deltas +1,+1,-1,+1 with period 2:
	//   bar 2: avgGain=(1+1)/2=1, avgLoss=0          -> 100
	//   bar 3: avgGain=(1*1+0)/2=0.5, avgLoss=(0+1)/2=0.5 -> 50
	//   bar 4: avgGain=(0.5+1)/2=0.75, avgLoss=(0.5+0)/2=0.25 -> 75
	in := []float64{1, 2, 3, 2, 3}
	out, err := RSI(in, 2)
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	assertReal(t, "RSI(1,2,3,2,3; 2)", out, []float64{nan, nan, 100, 50, 75}, 0)

	// Period 3 on 10,11,10,12,11: deltas +1,-1,+2,-1.
	//   bar 3: gain=3/3=1, loss=1/3            -> 100*1/(4/3) = 75
	//   bar 4: gain=(1*2+0)/3=2/3, loss=(1/3*2+1)/3=5/9 -> 100*(2/3)/(11/9) = 600/11
	out, _ = RSI([]float64{10, 11, 10, 12, 11}, 3)
	assertReal(t, "RSI period 3", out, []float64{nan, nan, nan, 75, 600.0 / 11}, 1e-12)

	// Constant series: no movement -> 0 (C's tempValue1 > 0 guard).
	c := fixtures(40)[5].Real()
	out, _ = RSI(c, 7)
	for i := 7; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g, want 0", i, out[i])
		}
	}
	// Strictly rising -> 100, strictly falling -> 0.
	out, _ = RSI(fixtures(60)[7].Real(), 14)
	for i := 14; i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("up: [%d]=%g, want 100", i, out[i])
		}
	}
	out, _ = RSI(fixtures(60)[8].Real(), 14)
	for i := 14; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("down: [%d]=%g, want 0", i, out[i])
		}
	}
	// Bounds: every value in [0, 100] on every fixture.
	for _, s := range fixtures(300) {
		for _, p := range []int{2, 14, 50} {
			out, _ := RSI(s.Real(), p)
			for i := p; i < len(out); i++ {
				if !(out[i] >= 0 && out[i] <= 100) {
					t.Fatalf("%s p=%d: [%d]=%g outside [0,100]", s.Name, p, i, out[i])
				}
			}
		}
	}
	// Scale invariance: RSI of a*x equals RSI of x for a > 0.
	w := fixtures(200)[0].Real()
	w2 := make([]float64, len(w))
	for i := range w {
		w2[i] = w[i] * 4
	}
	a, _ := RSI(w, 14)
	b, _ := RSI(w2, 14)
	assertReal(t, "scale invariance", b, a, 1e-12)
}

func TestRSIUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	SetUnstablePeriod(UnstRSI, 5)
	if lb := RSILookback(10); lb != 15 {
		t.Fatalf("lookback with unstable 5 = %d", lb)
	}
	in := fixtures(80)[0].Real()
	out, _ := RSI(in, 10)
	assertLeadingNaN(t, "unstable", out, 15)
	// Unstable bars are still processed: values after the lookback match the
	// stable run.
	SetAllUnstablePeriods(0)
	ref, _ := RSI(in, 10)
	assertReal(t, "unstable tail", out[15:], ref[15:], 0)
	// Another function's unstable period does not affect RSI.
	SetUnstablePeriod(UnstEMA, 7)
	if lb := RSILookback(10); lb != 10 {
		t.Fatalf("EMA unstable leaked into RSI lookback: %d", lb)
	}
}

func TestRSIRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := rsiRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := rsiRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := rsiRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := rsiRange(0, 2, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := rsiRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := rsiRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Like C, a sub-range re-seeds Wilder's averages at startIdx-lookback,
	// so it equals a full run over in[startIdx-lookback : endIdx+1].
	sub, _ := RSI(in[7:21], 3)
	beg, nb, err = rsiRange(10, 20, in, 3, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], sub[3:], 0)
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := RSI(in, 4)
	beg, nb, _ = rsiRange(0, len(buf)-1, buf, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
