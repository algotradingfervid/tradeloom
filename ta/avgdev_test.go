package ta

import (
	"errors"
	"math"
	"testing"
)

func TestAVGDEVParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := AVGDEV(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := AVGDEVLookback(p); lb != -1 {
			t.Errorf("AVGDEVLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := avgdevRange(0, len(in)-1, in, p, make([]float64, len(in))); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, err := AVGDEV(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := AVGDEVLookback(p); lb != p-1 {
			t.Errorf("AVGDEVLookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	if out, err := AVGDEV(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := AVGDEV([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := avgdevRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := avgdevRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
}

func TestAVGDEVLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30} {
		lb := AVGDEVLookback(period)
		if lb != period-1 {
			t.Fatalf("AVGDEVLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := AVGDEV(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("AVGDEV ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestAVGDEVKnownValues(t *testing.T) {
	nan := math.NaN()
	// Window {1,2,3}: mean 2, deviations 1,0,1 -> 2/3.
	// Window {2,3,7}: mean 4, deviations 2,1,3 -> 2.
	// Window {3,7,1}: mean 11/3, deviations 2/3,10/3,8/3 -> 20/9.
	in := []float64{1, 2, 3, 7, 1}
	out, err := AVGDEV(in, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "AVGDEV(3)", out, []float64{nan, nan, 2.0 / 3, 2, 20.0 / 9}, 1e-15)

	// Period 2: |a-b|/2.
	out, _ = AVGDEV([]float64{4, 10, 7}, 2)
	assertReal(t, "AVGDEV(2)", out, []float64{nan, 3, 1.5}, 0)

	// A constant series has zero deviation.
	c := fixtures(40)[5].Real()
	out, _ = AVGDEV(c, 7)
	for i := 6; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Non-negative everywhere; a linear trend of step 1 over period 4
	// (window {x,x+1,x+2,x+3}, mean x+1.5) gives (1.5+0.5+0.5+1.5)/4 = 1.
	for _, s := range fixtures(200) {
		out, _ := AVGDEV(s.Real(), 10)
		for i := 9; i < len(out); i++ {
			if !(out[i] >= 0) {
				t.Fatalf("%s: [%d]=%g, want >= 0", s.Name, i, out[i])
			}
		}
	}
	out, _ = AVGDEV(fixtures(30)[7].Real(), 4)
	for i := 3; i < len(out); i++ {
		if math.Abs(out[i]-1) > 1e-12 {
			t.Fatalf("trend: [%d]=%g, want 1", i, out[i])
		}
	}
}

func TestAVGDEVRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := avgdevRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := avgdevRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := avgdevRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := avgdevRange(0, 1, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range matches the full computation.
	full, _ := AVGDEV(in, 5)
	beg, nb, err = avgdevRange(10, 20, in, 5, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[10:21], 0)
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := AVGDEV(in, 4)
	beg, nb, _ = avgdevRange(0, len(buf)-1, buf, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
