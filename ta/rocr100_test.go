package ta

import (
	"errors"
	"math"
	"testing"
)

func TestROCR100Params(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := ROCR100(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := ROCR100Lookback(p); lb != -1 {
			t.Errorf("ROCR100Lookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := rocr100Range(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 10, 100000} {
		if _, err := ROCR100(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := ROCR100Lookback(p); lb != p {
			t.Errorf("ROCR100Lookback(%d) = %d, want %d", p, lb, p)
		}
	}
	if out, err := ROCR100(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := ROCR100([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, err := ROCR100(nil, 0); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input bad period: err = %v, want ErrBadParam", err)
	}
}

func TestROCR100Lengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 10, 30} {
		lb := ROCR100Lookback(period)
		if lb != period {
			t.Fatalf("ROCR100Lookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := ROCR100(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("ROCR100 ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestROCR100KnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{1, 2, 4, 8, 4, 2}
	out, err := ROCR100(in, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "ROCR100 p=1", out, []float64{nan, 200, 200, 200, 50, 50}, 0)

	out, _ = ROCR100(in, 2)
	assertReal(t, "ROCR100 p=2", out, []float64{nan, nan, 400, 400, 100, 25}, 0)

	// A zero previous price gives 0, not Inf/NaN; a zero current price gives 0.
	out, _ = ROCR100([]float64{0, 5, 0, 3}, 1)
	assertReal(t, "ROCR100 zero prev", out, []float64{nan, 0, 0, 0}, 0)
	out, _ = ROCR100([]float64{0, 0, 0}, 1)
	assertReal(t, "ROCR100 all zero", out, []float64{nan, 0, 0}, 0)

	// Negative values divide normally.
	out, _ = ROCR100([]float64{-2, 4}, 1)
	assertReal(t, "ROCR100 negative", out, []float64{nan, -200}, 0)

	// Operation order is (price/prevPrice)*100, as in C.
	// Runtime floats (Go would fold constants exactly): fl(fl(1/3)*100)
	// = 33.333333333333329, whereas fl(fl(1*100)/3) = 33.333333333333336.
	one, three := 1.0, 3.0
	divFirst, mulFirst := one/three*100.0, one*100.0/three
	if divFirst == mulFirst {
		t.Fatalf("op order case does not discriminate: %v", divFirst)
	}
	out, _ = ROCR100([]float64{3, 1}, 1)
	assertReal(t, "ROCR100 op order", out, []float64{nan, divFirst}, 0)
	out, _ = ROCR100([]float64{7, 3.5, 1.75}, 1)
	assertReal(t, "ROCR100 halves", out, []float64{nan, 50, 50}, 0)

	// A constant non-zero series has ratio 100.
	c := fixtures(40)[5].Real()
	out, _ = ROCR100(c, 7)
	for i := 7; i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Rising positive series > 100, falling in (0, 100).
	up := fixtures(60)[7].Real()
	out, _ = ROCR100(up, 3)
	for i := 3; i < len(out); i++ {
		if !(out[i] > 100) {
			t.Fatalf("up: [%d]=%g", i, out[i])
		}
	}
	down := fixtures(60)[8].Real()
	out, _ = ROCR100(down, 3)
	for i := 3; i < len(out); i++ {
		if !(out[i] < 100 && out[i] > 0) {
			t.Fatalf("down: [%d]=%g", i, out[i])
		}
	}

	// Period equal to the length: everything inside the lookback.
	out, _ = ROCR100(in, len(in))
	assertReal(t, "ROCR100 p=n", out, nanSlice(len(in)), 0)
}

func TestROCR100RangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := rocr100Range(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := rocr100Range(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := rocr100Range(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := rocr100Range(0, 2, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// First bar after the lookback.
	beg, nb, err = rocr100Range(0, 3, in, 3, out)
	if err != nil || beg != 3 || nb != 1 || out[0] != in[3]/in[0]*100 {
		t.Errorf("first bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	// Short buffers.
	if _, _, err := rocr100Range(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := rocr100Range(0, 10, in, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// In-place computation is allowed (trailingIdx < inIdx, outIdx <= trailingIdx).
	buf := append([]float64(nil), in...)
	want, _ := ROCR100(in, 4)
	beg, nb, _ = rocr100Range(0, len(buf)-1, buf, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
