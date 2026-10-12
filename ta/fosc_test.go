package ta

import (
	"errors"
	"math"
	"testing"
)

func TestFOSCParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := FOSC(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := FOSCLookback(p); lb != -1 {
			t.Errorf("FOSCLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := foscRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 5, 100000} {
		if _, err := FOSC(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := FOSCLookback(p); lb != p {
			t.Errorf("FOSCLookback(%d) = %d, want %d", p, lb, p)
		}
	}
	if out, err := FOSC(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := FOSC([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestFOSCLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30} {
		lb := FOSCLookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := FOSC(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("FOSC ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestFOSCKnownValues(t *testing.T) {
	nan := math.NaN()
	// Period 2, window [1,2] -> line y = x+1 forecasts 3 for bar 2: 100*(4-3)/4 = 25.
	// Window [2,4] forecasts 6 for bar 3: 100*(4-6)/4 = -50.
	// Window [4,4] forecasts 4 for bar 4 whose value is 0: zero close -> 0.
	out, err := FOSC([]float64{1, 2, 4, 4, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "FOSC hand", out, []float64{nan, nan, 25, -50, 0}, 1e-12)

	// Period 3 on [1,2,3,10]: window [1,2,3] forecasts 4: 100*(10-4)/10 = 60.
	out, _ = FOSC([]float64{1, 2, 3, 10}, 3)
	assertReal(t, "FOSC p3", out, []float64{nan, nan, nan, 60}, 1e-12)

	// A straight line is forecast exactly, so the oscillator is zero.
	for _, p := range []int{2, 5, 14} {
		up := fixtures(100)[7].Real()
		out, _ = FOSC(up, p)
		for i := p; i < len(out); i++ {
			if math.Abs(out[i]) > 1e-9 {
				t.Fatalf("trend p=%d: [%d]=%g, want 0", p, i, out[i])
			}
		}
	}

	// Constant input: zero; all-zero input: zero (zero-close branch).
	for _, idx := range []int{5, 6} {
		out, _ = FOSC(fixtures(60)[idx].Real(), 5)
		for i := 5; i < len(out); i++ {
			if math.Abs(out[i]) > 1e-12 {
				t.Fatalf("fixture %d: [%d]=%g, want 0", idx, i, out[i])
			}
		}
	}
}

func TestFOSCRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := foscRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := foscRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := foscRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (lookback = period): no output, no error.
	beg, nb, err := foscRange(0, 2, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// First computable bar.
	beg, nb, err = foscRange(0, 3, in, 3, out)
	if err != nil || beg != 3 || nb != 1 {
		t.Errorf("first bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := foscRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := foscRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Sub-range values equal the full computation up to rounding: the running
	// sums start at a different bar, so accumulation order differs (as in C).
	full, _ := FOSC(in, 4)
	beg, nb, _ = foscRange(10, 20, in, 4, out)
	if beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d", beg, nb)
	}
	assertReal(t, "sub-range", out[:nb], full[10:21], relTol)
	// In-place computation is allowed (C issue #130).
	for _, p := range []int{2, 4} {
		buf := append([]float64(nil), in...)
		want, _ := FOSC(in, p)
		beg, nb, _ = foscRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
