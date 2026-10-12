package ta

import (
	"errors"
	"math"
	"testing"
)

func TestMOMParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := MOM(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := MOMLookback(p); lb != -1 {
			t.Errorf("MOMLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := momRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 10, 100000} {
		if _, err := MOM(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := MOMLookback(p); lb != p {
			t.Errorf("MOMLookback(%d) = %d, want %d", p, lb, p)
		}
	}
	if out, err := MOM(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := MOM([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Bad parameter is reported even for empty input.
	if _, err := MOM(nil, 0); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input bad period: err = %v", err)
	}
}

func TestMOMLengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 10, 30} {
		lb := MOMLookback(period)
		if lb != period {
			t.Fatalf("MOMLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := MOM(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("MOM ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestMOMKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{10, 20, 15, 30, 0, 5}
	out, err := MOM(in, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "MOM p=1", out, []float64{nan, 10, -5, 15, -30, 5}, 0)

	out, _ = MOM(in, 2)
	// 15-10, 30-20, 0-15, 5-30.
	assertReal(t, "MOM p=2", out, []float64{nan, nan, 5, 10, -15, -25}, 0)

	out, _ = MOM(in, 5)
	assertReal(t, "MOM p=5", out, []float64{nan, nan, nan, nan, nan, -5}, 0)

	// Constant input has zero momentum.
	c := fixtures(40)[5].Real()
	out, _ = MOM(c, 7)
	for i := 7; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}
	// Linear trend with step 1: momentum equals the formula exactly and the
	// period approximately (the fixture accumulates its step in floating point).
	up := fixtures(60)[7].Real()
	out, _ = MOM(up, 5)
	for i := 5; i < len(out); i++ {
		if out[i] != up[i]-up[i-5] || !closeEnough(out[i], 5, 1e-12) {
			t.Fatalf("up: [%d]=%g", i, out[i])
		}
	}
	down := fixtures(60)[8].Real()
	out, _ = MOM(down, 5)
	for i := 5; i < len(out); i++ {
		if out[i] != down[i]-down[i-5] || !closeEnough(out[i], -5, 1e-12) {
			t.Fatalf("down: [%d]=%g", i, out[i])
		}
	}
	// Period equal to the series length: all NaN.
	out, _ = MOM(in, 6)
	assertLeadingNaN(t, "period==len", out, 6)
}

func TestMOMRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := momRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := momRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := momRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := momRange(0, 2, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Start inside the lookback is moved up to the lookback.
	beg, nb, err = momRange(1, 10, in, 3, out)
	if err != nil || beg != 3 || nb != 8 {
		t.Errorf("start in lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if out[0] != in[3]-in[0] || out[7] != in[10]-in[7] {
		t.Errorf("start in lookback values: %g %g", out[0], out[7])
	}
	// Short buffers.
	if _, _, err := momRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := momRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// In-place computation is allowed.
	for _, p := range []int{1, 4} {
		buf := append([]float64(nil), in...)
		want, _ := MOM(in, p)
		beg, nb, _ = momRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
