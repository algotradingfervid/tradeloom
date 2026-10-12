package ta

import (
	"errors"
	"math"
	"testing"
)

func TestATRParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := ATR(s.H, s.L, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := ATRLookback(p); lb != -1 {
			t.Errorf("ATRLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := atrRange(0, 49, s.H, s.L, s.C, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("atrRange period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 14, 100000} {
		if _, err := ATR(s.H, s.L, s.C, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := ATRLookback(p); lb != p {
			t.Errorf("ATRLookback(%d) = %d, want %d", p, lb, p)
		}
	}
	// Mismatched input lengths.
	if _, err := ATR(s.H, s.L[:49], s.C, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: err = %v", err)
	}
	if _, err := ATR(s.H, s.L, s.C[:10], 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v", err)
	}
	if _, err := ATR(nil, s.L, s.C, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inHigh with non-empty others: err = %v", err)
	}
	// nil / empty input.
	if out, err := ATR(nil, nil, nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := ATR([]float64{}, []float64{}, []float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Short buffers in the range form.
	if _, _, err := atrRange(0, 10, s.H, s.L[:5], s.C, 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := atrRange(0, 10, s.H, s.L, s.C, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := atrRange(5, 10, s.H, s.L, s.C, 3, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestATRLengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 14, 30} {
		lb := ATRLookback(period)
		if lb != period {
			t.Fatalf("ATRLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := ATR(s.H, s.L, s.C, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("ATR ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestATRKnownValues(t *testing.T) {
	h := []float64{10, 11, 13, 12.5, 11}
	l := []float64{8, 9, 12, 10, 7}
	c := []float64{9, 10, 12, 11, 8}
	// True ranges from bar 1: 2, 3 (|10-13|), 2.5 (h-l), 4 (h-l = |c1-l|).
	nan := math.NaN()

	out, err := ATR(h, l, c, 2)
	if err != nil {
		t.Fatal(err)
	}
	// Seed (2+3)/2 = 2.5; then 0.5*2.5+0.5*2.5 = 2.5; 0.5*2.5+0.5*4 = 3.25.
	assertReal(t, "ATR p=2", out, []float64{nan, nan, 2.5, 2.5, 3.25}, 0)

	// Period 1: wBeta = 0, wAlpha = 1, so ATR is the true range itself.
	out, _ = ATR(h, l, c, 1)
	assertReal(t, "ATR p=1", out, []float64{nan, 2, 3, 2.5, 4}, 0)

	// Period 4 with exactly lookback+1 bars: a single seed value.
	out, _ = ATR(h, l, c, 4)
	assertReal(t, "ATR p=4", out, []float64{nan, nan, nan, nan, (2 + 3 + 2.5 + 4) / 4.0}, 0)

	// Period 5 with 5 bars: all inside the lookback.
	out, _ = ATR(h, l, c, 5)
	assertReal(t, "ATR p=5", out, []float64{nan, nan, nan, nan, nan}, 0)

	// Period-1 identity against the shared trueRange helper on a random walk.
	s := fixtures(200)[0]
	out, _ = ATR(s.H, s.L, s.C, 1)
	for i := 1; i < len(out); i++ {
		if want := trueRange(s.H[i], s.L[i], s.C[i-1]); out[i] != want {
			t.Fatalf("p=1 [%d]: got %g want %g", i, out[i], want)
		}
	}

	// Constant series: zero range everywhere.
	cs := fixtures(40)[5]
	out, _ = ATR(cs.H, cs.L, cs.C, 7)
	for i := 7; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}
	// Zigzag: every true range is 106-99 = 7, so ATR is 7.
	zz := fixtures(60)[9]
	out, _ = ATR(zz.H, zz.L, zz.C, 9)
	for i := 9; i < len(out); i++ {
		if math.Abs(out[i]-7) > 1e-12 {
			t.Fatalf("zigzag: [%d]=%.17g", i, out[i])
		}
	}

	// ATR is non-negative and bounded by the min/max true range seen so far.
	for _, s := range fixtures(300) {
		out, _ := ATR(s.H, s.L, s.C, 10)
		for i := 10; i < len(out); i++ {
			if out[i] < 0 || math.IsNaN(out[i]) {
				t.Fatalf("%s [%d]=%g", s.Name, i, out[i])
			}
		}
	}
}

func TestATRUnstablePeriod(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	s := fixtures(100)[1]
	base, _ := ATR(s.H, s.L, s.C, 14)
	if err := SetUnstablePeriod(UnstATR, 5); err != nil {
		t.Fatal(err)
	}
	if lb := ATRLookback(14); lb != 19 {
		t.Fatalf("lookback with unstable 5 = %d, want 19", lb)
	}
	out, _ := ATR(s.H, s.L, s.C, 14)
	assertLeadingNaN(t, "unstable 5", out, 19)
	// From index 0 the seed bars are unchanged, so later values are identical.
	assertReal(t, "unstable tail", out[19:], base[19:], 0)
	// Inputs shorter than the extended lookback yield all NaN.
	out, _ = ATR(s.H[:19], s.L[:19], s.C[:19], 14)
	assertLeadingNaN(t, "unstable short", out, 19)
}

func TestATRRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := atrRange(-1, 5, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := atrRange(5, 4, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := atrRange(0, -1, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := atrRange(0, 2, s.H, s.L, s.C, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Range starting exactly at the lookback.
	beg, nb, err = atrRange(3, 3, s.H, s.L, s.C, 3, out)
	if err != nil || beg != 3 || nb != 1 {
		t.Errorf("at lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Full range equals the public form.
	want, _ := ATR(s.H, s.L, s.C, 4)
	beg, nb, _ = atrRange(0, 29, s.H, s.L, s.C, 4, out)
	alignReal(out, beg, nb)
	assertReal(t, "full range", out, want, 0)
	// In-place into each input is allowed.
	for k := range 3 {
		h := append([]float64(nil), s.H...)
		l := append([]float64(nil), s.L...)
		c := append([]float64(nil), s.C...)
		buf := [][]float64{h, l, c}[k]
		beg, nb, _ := atrRange(0, 29, h, l, c, 4, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place ", k), buf, want, 0)
	}
}
