package ta

import (
	"errors"
	"math"
	"testing"
)

func TestVARParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32} {
		if _, err := VAR(in, p, 1); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := VARLookback(p, 1); lb != -1 {
			t.Errorf("VARLookback(%d) = %d, want -1", p, lb)
		}
	}
	for _, d := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 3.0000001e37, -3.0000001e37, -4e37} {
		if _, err := VAR(in, 5, d); !errors.Is(err, ErrBadParam) {
			t.Errorf("nbDev %g: err = %v, want ErrBadParam", d, err)
		}
		if lb := VARLookback(5, d); lb != -1 {
			t.Errorf("VARLookback(5, %g) = %d, want -1", d, lb)
		}
		if _, _, err := varRange(0, 49, in, 5, d, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("varRange nbDev %g: err = %v, want ErrBadParam", d, err)
		}
	}
	for _, p := range []int{1, 2, 100000} {
		if _, err := VAR(in, p, 1); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
	}
	for _, d := range []float64{-3e37, 3e37, 0, -2, 2} {
		if _, err := VAR(in, 5, d); err != nil {
			t.Errorf("nbDev %g: unexpected err %v", d, err)
		}
		if lb := VARLookback(5, d); lb != 4 {
			t.Errorf("VARLookback(5, %g) = %d, want 4", d, lb)
		}
	}
	if out, err := VAR(nil, 5, 1); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := VAR([]float64{}, 5, 1); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := varRange(0, 10, in[:5], 3, 1, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := varRange(0, 10, in, 3, 1, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
}

func TestVARLengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 30} {
		lb := VARLookback(period, 1)
		if lb != period-1 {
			t.Fatalf("VARLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := VAR(s.Real(), period, 1)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("VAR ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestVARKnownValues(t *testing.T) {
	nan := math.NaN()
	// Window {k, k+1, k+2}: mean k+1, deviations -1,0,1 -> variance 2/3.
	in := []float64{1, 2, 3, 4, 5, 6}
	out, err := VAR(in, 3, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "VAR(1..6,3)", out, []float64{nan, nan, 2.0 / 3, 2.0 / 3, 2.0 / 3, 2.0 / 3}, 1e-12)

	// {2,4,4,4,5,5,7,9}: population variance 4.
	out, _ = VAR([]float64{2, 4, 4, 4, 5, 5, 7, 9}, 8, 1)
	assertReal(t, "VAR classic", out, []float64{nan, nan, nan, nan, nan, nan, nan, 4}, 1e-12)

	// Period 1: every window is a single value, variance exactly 0.
	out, _ = VAR(fixtures(40)[0].Real(), 1, 1)
	for i, v := range out {
		if v != 0 {
			t.Fatalf("period 1: [%d]=%g, want 0", i, v)
		}
	}

	// A constant series has zero variance.
	out, _ = VAR(fixtures(40)[5].Real(), 7, 1)
	for i := 6; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// optInNbDev does not affect the result.
	base, _ := VAR(fixtures(100)[0].Real(), 10, 1)
	for _, d := range []float64{-2, 0, 2.5, 3e37} {
		got, _ := VAR(fixtures(100)[0].Real(), 10, d)
		assertReal(t, lbl("nbDev ", d), got, base, 0)
	}

	// Variance is never negative, on any fixture.
	for _, s := range fixtures(500) {
		for _, p := range []int{2, 5, 30} {
			out, _ := VAR(s.Real(), p, 1)
			for i := p - 1; i < len(out); i++ {
				if !(out[i] >= 0) {
					t.Fatalf("%s p=%d: [%d]=%g negative or NaN", s.Name, p, i, out[i])
				}
			}
		}
	}

	// Large offset with tiny spread: no catastrophic cancellation.
	// Window {1e9, 1e9+1}: variance 0.25.
	big := []float64{1e9, 1e9 + 1, 1e9, 1e9 + 1, 1e9}
	out, _ = VAR(big, 2, 1)
	assertReal(t, "large offset", out, []float64{nan, 0.25, 0.25, 0.25, 0.25}, 1e-12)
}

func TestVARRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := varRange(-1, 5, in, 3, 1, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := varRange(5, 4, in, 3, 1, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := varRange(0, -1, in, 3, 1, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := varRange(0, 1, in, 3, 1, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Start inside the lookback is moved up to it.
	beg, nb, err = varRange(1, 10, in, 3, 1, out)
	if err != nil || beg != 2 || nb != 9 {
		t.Errorf("start in lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// In-place computation is allowed (long enough to hit periodic reseeds).
	long := fixtures(1000)[4].Real()
	buf := append([]float64(nil), long...)
	want, _ := VAR(long, 4, 1)
	beg, nb, _ = varRange(0, len(buf)-1, buf, 4, 1, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
