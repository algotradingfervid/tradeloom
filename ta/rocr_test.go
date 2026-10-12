package ta

import (
	"errors"
	"math"
	"testing"
)

func TestROCRParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := ROCR(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := ROCRLookback(p); lb != -1 {
			t.Errorf("ROCRLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := rocrRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 10, 100000} {
		if _, err := ROCR(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := ROCRLookback(p); lb != p {
			t.Errorf("ROCRLookback(%d) = %d, want %d", p, lb, p)
		}
	}
	if out, err := ROCR(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := ROCR([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, err := ROCR(nil, 0); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input bad period: err = %v, want ErrBadParam", err)
	}
}

func TestROCRLengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 10, 30} {
		lb := ROCRLookback(period)
		if lb != period {
			t.Fatalf("ROCRLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := ROCR(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("ROCR ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestROCRKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{1, 2, 4, 8, 4, 2}
	out, err := ROCR(in, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "ROCR p=1", out, []float64{nan, 2, 2, 2, 0.5, 0.5}, 0)

	out, _ = ROCR(in, 2)
	assertReal(t, "ROCR p=2", out, []float64{nan, nan, 4, 4, 1, 0.25}, 0)

	// A zero previous price gives 0, not Inf/NaN; a zero current price gives 0.
	out, _ = ROCR([]float64{0, 5, 0, 3}, 1)
	assertReal(t, "ROCR zero prev", out, []float64{nan, 0, 0, 0}, 0)
	out, _ = ROCR([]float64{0, 0, 0}, 1)
	assertReal(t, "ROCR all zero", out, []float64{nan, 0, 0}, 0)

	// Negative values divide normally.
	out, _ = ROCR([]float64{-2, 4}, 1)
	assertReal(t, "ROCR negative", out, []float64{nan, -2}, 0)

	// A constant non-zero series has ratio 1.
	c := fixtures(40)[5].Real()
	out, _ = ROCR(c, 7)
	for i := 7; i < len(out); i++ {
		if out[i] != 1 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Rising positive series > 1, falling < 1.
	up := fixtures(60)[7].Real()
	out, _ = ROCR(up, 3)
	for i := 3; i < len(out); i++ {
		if !(out[i] > 1) {
			t.Fatalf("up: [%d]=%g", i, out[i])
		}
	}
	down := fixtures(60)[8].Real()
	out, _ = ROCR(down, 3)
	for i := 3; i < len(out); i++ {
		if !(out[i] < 1 && out[i] > 0) {
			t.Fatalf("down: [%d]=%g", i, out[i])
		}
	}

	// Period equal to the length: everything inside the lookback.
	out, _ = ROCR(in, len(in))
	assertReal(t, "ROCR p=n", out, nanSlice(len(in)), 0)
}

func TestROCRRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := rocrRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := rocrRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := rocrRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := rocrRange(0, 2, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// First bar after the lookback.
	beg, nb, err = rocrRange(0, 3, in, 3, out)
	if err != nil || beg != 3 || nb != 1 || out[0] != in[3]/in[0] {
		t.Errorf("first bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	// Short buffers.
	if _, _, err := rocrRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := rocrRange(0, 10, in, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// In-place computation is allowed (trailingIdx < inIdx, outIdx <= trailingIdx).
	buf := append([]float64(nil), in...)
	want, _ := ROCR(in, 4)
	beg, nb, _ = rocrRange(0, len(buf)-1, buf, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
