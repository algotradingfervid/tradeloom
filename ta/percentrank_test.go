package ta

import (
	"errors"
	"math"
	"testing"
)

func TestPERCENTRANKParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := PERCENTRANK(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := PERCENTRANKLookback(p); lb != -1 {
			t.Errorf("PERCENTRANKLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := percentrankRange(0, len(in)-1, in, p, make([]float64, len(in))); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 100, 100000} {
		if _, err := PERCENTRANK(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := PERCENTRANKLookback(p); lb != p {
			t.Errorf("PERCENTRANKLookback(%d) = %d, want %d", p, lb, p)
		}
	}
	if out, err := PERCENTRANK(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := PERCENTRANK([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Bad period is reported even on empty input.
	if _, err := PERCENTRANK(nil, 1); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input bad period: err = %v, want ErrBadParam", err)
	}
	if _, _, err := percentrankRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := percentrankRange(0, 10, in, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
}

func TestPERCENTRANKLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 20, 100} {
		lb := PERCENTRANKLookback(period)
		if lb != period {
			t.Fatalf("PERCENTRANKLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := PERCENTRANK(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("PERCENTRANK ", s.Name, " n=", n, " p=", period), out, lb)
				for i := lb; i < n; i++ {
					if out[i] < 0 || out[i] > 100 {
						t.Fatalf("%s n=%d p=%d: [%d]=%g out of [0,100]", s.Name, n, period, i, out[i])
					}
				}
			}
		}
	}
}

func TestPERCENTRANKKnownValues(t *testing.T) {
	nan := math.NaN()
	// Period 4. Bar 4 (value 3) vs {5,1,4,2}: 1 and 2 below → 50.
	// Bar 5 (value 6) vs {1,4,2,3}: all four below → 100.
	// Bar 6 (value 0) vs {4,2,3,6}: none below → 0.
	// Bar 7 (value 3) vs {2,3,6,0}: 2 and 0 below (tie with 3 not counted) → 50.
	in := []float64{5, 1, 4, 2, 3, 6, 0, 3}
	out, err := PERCENTRANK(in, 4)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "PERCENTRANK p=4", out, []float64{nan, nan, nan, nan, 50, 100, 0, 50}, 0)

	// Period 3: the rounding is (count/3)*100, exactly as C computes it.
	// Bar 5 (2.5) vs {3,4,0}: one below. Bar 6 (3.5) vs {4,0,2.5}: two below.
	// Use float64 variables: Go folds constant expressions exactly.
	in = []float64{1, 2, 3, 4, 0, 2.5, 3.5}
	out, _ = PERCENTRANK(in, 3)
	one, two, three := 1.0, 2.0, 3.0
	assertReal(t, "PERCENTRANK p=3", out,
		[]float64{nan, nan, nan, 100, 0, one / three * 100, two / three * 100}, 0)

	// Period 2 is the minimum.
	// Bar 3 (1.5) vs {2,3}: none below. Bar 4 (2.5) vs {3,1.5}: one below.
	out, _ = PERCENTRANK([]float64{1, 2, 3, 1.5, 2.5}, 2)
	assertReal(t, "PERCENTRANK p=2", out, []float64{nan, nan, 100, 0, 50}, 0)

	// A constant series has every predecessor tied: rank 0 everywhere.
	c := fixtures(40)[5].Real()
	out, _ = PERCENTRANK(c, 7)
	for i := 7; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}
	// A strictly rising series ranks 100, a strictly falling one 0.
	up := fixtures(40)[7].Real()
	out, _ = PERCENTRANK(up, 10)
	for i := 10; i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("up: [%d]=%g", i, out[i])
		}
	}
	down := fixtures(40)[8].Real()
	out, _ = PERCENTRANK(down, 10)
	for i := 10; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("down: [%d]=%g", i, out[i])
		}
	}
	// A NaN in the window fails every comparison: output stays finite.
	// Bar 2 (3) vs {1,NaN}: only 1 counts. Bar 3 (2) vs {NaN,3}: none.
	// Bar 4 (NaN) vs {3,2}: NaN compares false with everything.
	out, _ = PERCENTRANK([]float64{1, nan, 3, 2, nan}, 2)
	assertReal(t, "PERCENTRANK NaN", out, []float64{nan, nan, 50, 0, 0}, 0)
	// -0 and +0 compare equal.
	out, _ = PERCENTRANK([]float64{math.Copysign(0, -1), 0, 0}, 2)
	assertReal(t, "PERCENTRANK signed zero", out, []float64{nan, nan, 0}, 0)
}

func TestPERCENTRANKRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := percentrankRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := percentrankRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := percentrankRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := percentrankRange(0, 2, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Single bar at the end of the series.
	want, _ := PERCENTRANK(in, 5)
	beg, nb, err = percentrankRange(29, 29, in, 5, out)
	if err != nil || beg != 29 || nb != 1 || out[0] != want[29] {
		t.Errorf("last bar: beg=%d nb=%d err=%v v=%g want %g", beg, nb, err, out[0], want[29])
	}
	// Start inside the lookback is clamped to it.
	beg, nb, _ = percentrankRange(2, 29, in, 5, out)
	if beg != 5 || nb != 25 {
		t.Errorf("clamped: beg=%d nb=%d", beg, nb)
	}
	assertReal(t, "clamped values", out[:nb], want[5:], 0)
}
