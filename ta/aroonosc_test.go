package ta

import (
	"errors"
	"math"
	"testing"
)

func TestAROONOSCParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := AROONOSC(s.H, s.L, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := AROONOSCLookback(p); lb != -1 {
			t.Errorf("AROONOSCLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := aroonoscRange(0, 49, s.H, s.L, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("aroonoscRange period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, err := AROONOSC(s.H, s.L, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := AROONOSCLookback(p); lb != p {
			t.Errorf("AROONOSCLookback(%d) = %d, want %d", p, lb, p)
		}
	}
	// Mismatched input lengths.
	if _, err := AROONOSC(s.H, s.L[:49], 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: err = %v", err)
	}
	if _, err := AROONOSC(s.H[:10], s.L, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v", err)
	}
	if _, err := AROONOSC(nil, s.L, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inHigh with non-empty inLow: err = %v", err)
	}
	// nil / empty input.
	if out, err := AROONOSC(nil, nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := AROONOSC([]float64{}, []float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Short buffers in the range form.
	if _, _, err := aroonoscRange(0, 10, s.H, s.L[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := aroonoscRange(0, 10, s.H[:5], s.L, 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := aroonoscRange(0, 10, s.H, s.L, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := aroonoscRange(5, 10, s.H, s.L, 3, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestAROONOSCLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30} {
		lb := AROONOSCLookback(period)
		if lb != period {
			t.Fatalf("AROONOSCLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := AROONOSC(s.H, s.L, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("AROONOSC ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestAROONOSCKnownValues(t *testing.T) {
	nan := math.NaN()
	// Period 4, factor 25. Window for bar 4 is bars 0..4.
	high := []float64{5, 9, 4, 3, 2, 1, 10}
	low := []float64{4, 8, 1, 2, 1.5, 0.5, 9}
	out, err := AROONOSC(high, low, 4)
	if err != nil {
		t.Fatal(err)
	}
	// bar4: highest at 1, lowest at 2 → 25*(1-2) = -25
	// bar5: window 1..5: highest at 1, lowest at 5 → 25*(1-5) = -100
	// bar6: window 2..6: highest at 6, lowest at 5 → 25*(6-5) = 25
	assertReal(t, "AROONOSC hand", out, []float64{nan, nan, nan, nan, -25, -100, 25}, 0)

	// Ties go to the most recent bar: a constant series has highestIdx ==
	// lowestIdx == today, so the oscillator is 0.
	c := fixtures(40)[5]
	out, _ = AROONOSC(c.H, c.L, 7)
	for i := 7; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Strictly rising highs and lows: highest is today, lowest is the oldest
	// bar of the window → +100. Strictly falling → -100.
	up := fixtures(40)[7]
	out, _ = AROONOSC(up.H, up.L, 10)
	for i := 10; i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("up: [%d]=%g", i, out[i])
		}
	}
	down := fixtures(40)[8]
	out, _ = AROONOSC(down.H, down.L, 10)
	for i := 10; i < len(out); i++ {
		if out[i] != -100 {
			t.Fatalf("down: [%d]=%g", i, out[i])
		}
	}

	// Bounds: always within [-100, 100].
	for _, s := range fixtures(300) {
		for _, p := range []int{2, 14, 50} {
			out, _ := AROONOSC(s.H, s.L, p)
			for i := p; i < len(out); i++ {
				if out[i] < -100 || out[i] > 100 {
					t.Fatalf("%s p=%d: [%d]=%g out of bounds", s.Name, p, i, out[i])
				}
			}
		}
	}
}

func TestAROONOSCRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := aroonoscRange(-1, 5, s.H, s.L, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := aroonoscRange(5, 4, s.H, s.L, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := aroonoscRange(0, -1, s.H, s.L, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := aroonoscRange(0, 2, s.H, s.L, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Start inside the lookback is moved up to it.
	beg, nb, err = aroonoscRange(1, 10, s.H, s.L, 3, out)
	if err != nil || beg != 3 || nb != 8 {
		t.Errorf("start in lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// In-place computation (output aliases inLow) is allowed.
	want, _ := AROONOSC(s.H, s.L, 4)
	buf := append([]float64(nil), s.L...)
	beg, nb, _ = aroonoscRange(0, len(buf)-1, s.H, buf, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
