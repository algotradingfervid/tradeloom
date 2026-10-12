package ta

import (
	"errors"
	"math"
	"testing"
)

func TestSUMParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := SUM(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := SUMLookback(p); lb != -1 {
			t.Errorf("SUMLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := sumRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 30, 100000} {
		if _, err := SUM(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := SUMLookback(p); lb != p-1 {
			t.Errorf("SUMLookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	if out, err := SUM(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := SUM([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestSUMLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 30, 200} {
		lb := SUMLookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := SUM(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("SUM ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestSUMKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{1, 2, 3, 4, 5, 6}
	out, err := SUM(in, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "SUM(1..6,3)", out, []float64{nan, nan, 6, 9, 12, 15}, 0)

	out, _ = SUM(in, 2)
	assertReal(t, "SUM(1..6,2)", out, []float64{nan, 3, 5, 7, 9, 11}, 0)

	// Period equal to the length: one output, the whole sum.
	out, _ = SUM(in, 6)
	assertReal(t, "SUM(1..6,6)", out, []float64{nan, nan, nan, nan, nan, 21}, 0)

	// Period longer than the input: all NaN.
	out, _ = SUM(in, 7)
	assertReal(t, "SUM(1..6,7)", out, []float64{nan, nan, nan, nan, nan, nan}, 0)

	// Negative values.
	out, _ = SUM([]float64{-1, 2, -3, 4}, 2)
	assertReal(t, "SUM negative", out, []float64{nan, 1, -1, 1}, 0)

	// A constant series sums to period * value; SUM equals SMA * period.
	c := fixtures(40)[5].Real()
	out, _ = SUM(c, 7)
	for i := 6; i < len(out); i++ {
		if out[i] != 700 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}
	w := fixtures(200)[0].Real()
	sum, _ := SUM(w, 14)
	sma, _ := SMA(w, 14)
	for i := 13; i < len(w); i++ {
		if !closeEnough(sum[i]/14, sma[i], 1e-12) {
			t.Fatalf("SUM/14 vs SMA at %d: %g vs %g", i, sum[i]/14, sma[i])
		}
	}
}

func TestSUMRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := sumRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := sumRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := sumRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := sumRange(0, 1, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := sumRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := sumRange(0, 10, in, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := sumRange(0, 10, nil, 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input buffer: err = %v, want ErrBadParam", err)
	}
	// Single bar after the lookback.
	beg, nb, err = sumRange(5, 5, in, 3, out)
	if err != nil || beg != 5 || nb != 1 || out[0] != in[3]+in[4]+in[5] {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := SUM(in, 4)
	beg, nb, _ = sumRange(0, len(buf)-1, buf, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
