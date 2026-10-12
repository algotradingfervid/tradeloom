package ta

import (
	"errors"
	"math"
	"testing"
)

func TestROCParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := ROC(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := ROCLookback(p); lb != -1 {
			t.Errorf("ROCLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := rocRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 10, 100000} {
		if _, err := ROC(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := ROCLookback(p); lb != p {
			t.Errorf("ROCLookback(%d) = %d, want %d", p, lb, p)
		}
	}
	if out, err := ROC(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := ROC([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Bad parameter is reported even for empty input.
	if _, err := ROC(nil, 0); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input bad period: err = %v", err)
	}
}

func TestROCLengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 10, 30} {
		lb := ROCLookback(period)
		if lb != period {
			t.Fatalf("ROCLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := ROC(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("ROC ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestROCKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{10, 20, 15, 30, 0, 5}
	out, err := ROC(in, 1)
	if err != nil {
		t.Fatal(err)
	}
	// (20/10-1)*100=100, (15/20-1)*100=-25, (30/15-1)*100=100, (0/30-1)*100=-100, prev 0 -> 0.
	assertReal(t, "ROC p=1", out, []float64{nan, 100, -25, 100, -100, 0}, 0)

	out, _ = ROC(in, 2)
	// (15/10-1)*100=50, (30/20-1)*100=50, (0/15-1)*100=-100, (5/30-1)*100=-250/3.
	assertReal(t, "ROC p=2", out, []float64{nan, nan, 50, 50, -100, -250.0 / 3.0}, 1e-12)

	// ROC == 100*ROCP up to rounding (the TA-Lib equivalence ROC = ((p/prev)-1)*100).
	w := fixtures(80)[0].Real()
	r1, _ := ROC(w, 9)
	r2, _ := ROCP(w, 9)
	for i := 9; i < len(w); i++ {
		if d := math.Abs(r1[i] - 100*r2[i]); d > 1e-9*math.Max(1, math.Abs(r1[i])) {
			t.Fatalf("ROC vs 100*ROCP [%d]: %g vs %g", i, r1[i], 100*r2[i])
		}
	}
	// ROC is bounded below by -100 for non-negative prices.
	for _, s := range fixtures(80)[:9] {
		o, _ := ROC(s.Real(), 4)
		for i := 4; i < len(o); i++ {
			if o[i] < -100 {
				t.Fatalf("%s [%d]=%g < -100", s.Name, i, o[i])
			}
		}
	}

	// Constant input has zero rate of change.
	c := fixtures(40)[5].Real()
	out, _ = ROC(c, 7)
	for i := 7; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}
	// All-zero input: division guarded, output 0.
	z := fixtures(40)[6].Real()
	out, _ = ROC(z, 3)
	for i := 3; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("zeros: [%d]=%g", i, out[i])
		}
	}
	// Strictly rising positive series: strictly positive ROC; falling: negative.
	up := fixtures(60)[7].Real()
	out, _ = ROC(up, 5)
	for i := 5; i < len(out); i++ {
		if !(out[i] > 0) {
			t.Fatalf("up: [%d]=%g", i, out[i])
		}
	}
	down := fixtures(60)[8].Real()
	out, _ = ROC(down, 5)
	for i := 5; i < len(out); i++ {
		if !(out[i] < 0) {
			t.Fatalf("down: [%d]=%g", i, out[i])
		}
	}
	// Period larger than the series: all NaN.
	out, _ = ROC(in, 6)
	assertLeadingNaN(t, "period==len", out, 6)
}

func TestROCRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := rocRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := rocRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := rocRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := rocRange(0, 2, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Start inside the lookback is moved up to the lookback.
	beg, nb, err = rocRange(1, 10, in, 3, out)
	if err != nil || beg != 3 || nb != 8 {
		t.Errorf("start in lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := rocRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := rocRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// In-place computation is allowed.
	for _, p := range []int{1, 4} {
		buf := append([]float64(nil), in...)
		want, _ := ROC(in, p)
		beg, nb, _ = rocRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
