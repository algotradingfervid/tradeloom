package ta

import (
	"errors"
	"math"
	"testing"
)

func TestCMOUParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := CMOU(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := CMOULookback(p); lb != -1 {
			t.Errorf("CMOULookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := cmouRange(0, len(in)-1, in, p, make([]float64, len(in))); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, err := CMOU(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := CMOULookback(p); lb != p {
			t.Errorf("CMOULookback(%d) = %d, want %d", p, lb, p)
		}
	}
	if out, err := CMOU(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CMOU([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := cmouRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := cmouRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
}

func TestCMOULengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30} {
		lb := CMOULookback(period)
		if lb != period {
			t.Fatalf("CMOULookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := CMOU(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("CMOU ", s.Name, " n=", n, " p=", period), out, lb)
				// Bounded in [-100, 100] up to rounding: the sliding sums keep
				// add-then-subtract residue (C does the same), so allow a few ulps.
				for i := lb; i < n; i++ {
					if out[i] < -100*(1+1e-12) || out[i] > 100*(1+1e-12) {
						t.Fatalf("%s n=%d p=%d: [%d]=%g out of [-100,100]", s.Name, n, period, i, out[i])
					}
				}
			}
		}
	}
}

func TestCMOUKnownValues(t *testing.T) {
	nan := math.NaN()
	// Changes: +2, -1, +3, -4, 0.
	in := []float64{10, 12, 11, 14, 10, 10}
	out, err := CMOU(in, 3)
	if err != nil {
		t.Fatal(err)
	}
	// [3]: d=+2,-1,+3 -> Su=5 Sd=1 -> 100*4/6
	// [4]: d=-1,+3,-4 -> Su=3 Sd=5 -> 100*-2/8 = -25
	// [5]: d=+3,-4,0  -> Su=3 Sd=4 -> 100*-1/7
	assertReal(t, "CMOU p=3", out, []float64{nan, nan, nan, 400.0 / 6, -25, -100.0 / 7}, 1e-12)

	// Strictly rising -> +100; strictly falling -> -100.
	up := fixtures(40)[7].Real()
	out, _ = CMOU(up, 5)
	for i := 5; i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("up: [%d]=%g", i, out[i])
		}
	}
	down := fixtures(40)[8].Real()
	out, _ = CMOU(down, 5)
	for i := 5; i < len(out); i++ {
		if out[i] != -100 {
			t.Fatalf("down: [%d]=%g", i, out[i])
		}
	}

	// A constant series is exactly flat -> 0.
	c := fixtures(40)[5].Real()
	out, _ = CMOU(c, 7)
	for i := 7; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// After a full flat window following movement, the residue is cleared: exactly 0.
	flat := []float64{1.1, 2.3, 0.7, 5.9, 3.3, 3.3, 3.3, 3.3, 3.3}
	out, _ = CMOU(flat, 3)
	for i := 7; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("flat tail: [%d]=%g", i, out[i])
		}
	}

	// Zigzag with period 2: one +5 and one -5 in every window -> 0.
	z := fixtures(20)[9].Real()
	out, _ = CMOU(z, 2)
	for i := 2; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("zigzag: [%d]=%g", i, out[i])
		}
	}
}

func TestCMOURangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := cmouRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cmouRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cmouRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cmouRange(0, 2, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range agrees with the full series.
	full, _ := CMOU(in, 4)
	beg, nb, err = cmouRange(10, 20, in, 4, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[10:21], 0)
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	beg, nb, _ = cmouRange(0, len(buf)-1, buf, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, full, 0)
}
