package ta

import (
	"errors"
	"math"
	"testing"
)

func TestMASSIParams(t *testing.T) {
	s := fixtures(80)[0]
	for _, p := range [][2]int{{1, 25}, {0, 25}, {-1, 25}, {100001, 25}, {9, 1}, {9, 0}, {9, 100001}, {math.MinInt32, 25}, {9, math.MaxInt32}} {
		if _, err := MASSI(s.H, s.L, p[0], p[1]); !errors.Is(err, ErrBadParam) {
			t.Errorf("params %v: err = %v, want ErrBadParam", p, err)
		}
		if lb := MASSILookback(p[0], p[1]); lb != -1 {
			t.Errorf("MASSILookback(%v) = %d, want -1", p, lb)
		}
		out := make([]float64, len(s.H))
		if _, _, err := massiRange(0, len(s.H)-1, s.H, s.L, p[0], p[1], out); !errors.Is(err, ErrBadParam) {
			t.Errorf("range params %v: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range [][2]int{{2, 2}, {2, 100000}, {100000, 2}, {100000, 100000}, {9, 25}} {
		if _, err := MASSI(s.H, s.L, p[0], p[1]); err != nil {
			t.Errorf("params %v: unexpected err %v", p, err)
		}
		if lb := MASSILookback(p[0], p[1]); lb != 2*(p[0]-1)+p[1]-1 {
			t.Errorf("MASSILookback(%v) = %d", p, lb)
		}
	}
	if _, err := MASSI(s.H, s.L[:10], 9, 25); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, err := MASSI(nil, s.L, 9, 25); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil high, non-empty low: err = %v, want ErrBadParam", err)
	}
	if out, err := MASSI(nil, nil, 9, 25); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := MASSI([]float64{}, []float64{}, 9, 25); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestMASSILengths(t *testing.T) {
	for _, p := range [][2]int{{2, 2}, {3, 5}, {9, 25}, {25, 9}, {14, 40}} {
		lb := MASSILookback(p[0], p[1])
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := MASSI(s.H, s.L, p[0], p[1])
				if err != nil {
					t.Fatalf("%s n=%d p=%v: %v", s.Name, n, p, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%v: len(out)=%d", s.Name, n, p, len(out))
				}
				assertLeadingNaN(t, lbl("MASSI ", s.Name, " n=", n, " p=", p), out, lb)
			}
		}
	}
}

func TestMASSIKnownValues(t *testing.T) {
	// fast=2 (k=2/3), slow=2: lookback = 2*1 + 1 = 3.
	// HL = 2, 4, 1, 4, 3.
	// ema1: seed (2+4)/2 = 3 at bar 1; bar 2: 3+(1-3)*2/3 = 5/3; bar 3: 5/3+(4-5/3)*2/3 = 29/9.
	// ema2: seed (3+5/3)/2 = 7/3 at bar 2; bar 3: 7/3+(29/9-7/3)*2/3 = 79/27.
	// ratio bar 2 = 5/7, bar 3 = (29/9)/(79/27) = 87/79. out[3] = 5/7 + 87/79.
	// bar 4: ema1 = 29/9+(3-29/9)*2/3 = 83/27; ema2 = 79/27+(83/27-79/27)*2/3 = 245/81;
	// ratio = (83/27)/(245/81) = 249/245. out[4] = 87/79 + 249/245.
	high := []float64{12, 14, 11, 14, 13}
	low := []float64{10, 10, 10, 10, 10}
	out, err := MASSI(high, low, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	want := []float64{nan, nan, nan, 5.0/7 + 87.0/79, 87.0/79 + 249.0/245}
	assertReal(t, "MASSI known", out, want, 1e-13)

	// Flat market (high == low): ema2 == 0, every ratio is 1, so the index is
	// exactly optInSlowPeriod.
	for _, s := range []series{fixtures(100)[5], fixtures(100)[6]} {
		out, _ = MASSI(s.H, s.L, 9, 25)
		for i := MASSILookback(9, 25); i < len(out); i++ {
			if out[i] != 25 {
				t.Fatalf("%s flat: [%d]=%g, want 25", s.Name, i, out[i])
			}
		}
	}

	// Constant non-zero range: both EMAs equal the range, ratio 1.
	h := make([]float64, 60)
	l := make([]float64, 60)
	for i := range h {
		h[i], l[i] = 105, 100
	}
	out, _ = MASSI(h, l, 5, 10)
	for i := MASSILookback(5, 10); i < len(out); i++ {
		if math.Abs(out[i]-10) > 1e-12 {
			t.Fatalf("constant range: [%d]=%g, want 10", i, out[i])
		}
	}

	// Bounds: with high >= low every ratio is positive, so the index lies in
	// (0, +inf); with a smooth trend it stays near optInSlowPeriod.
	for _, s := range fixtures(300) {
		out, _ = MASSI(s.H, s.L, 9, 25)
		for i := MASSILookback(9, 25); i < len(out); i++ {
			if !(out[i] > 0) || math.IsInf(out[i], 0) {
				t.Fatalf("%s: [%d]=%g not in (0, inf)", s.Name, i, out[i])
			}
		}
	}

	// The periods are not interchangeable.
	s := fixtures(200)[0]
	a, _ := MASSI(s.H, s.L, 9, 25)
	b, _ := MASSI(s.H, s.L, 25, 9)
	if a[len(a)-1] == b[len(b)-1] {
		t.Errorf("swapping periods gave the same value %g", a[len(a)-1])
	}
}

func TestMASSIRangeForm(t *testing.T) {
	s := fixtures(60)[0]
	out := make([]float64, len(s.H))
	if _, _, err := massiRange(-1, 5, s.H, s.L, 2, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := massiRange(5, 4, s.H, s.L, 2, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := massiRange(0, -1, s.H, s.L, 2, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (lb = 2*1 + 2 = 4): no output, no error.
	beg, nb, err := massiRange(0, 3, s.H, s.L, 2, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := massiRange(0, 30, s.H[:10], s.L, 2, 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high: %v", err)
	}
	if _, _, err := massiRange(0, 30, s.H, s.L[:10], 2, 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short low: %v", err)
	}
	if _, _, err := massiRange(0, 30, s.H, s.L, 2, 3, out[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short out: %v", err)
	}
	// A sub-range starting at the lookback (lb = 2*2 + 4 = 8) matches the full
	// computation. A later startIdx re-seeds both EMAs at startIdx-lookback, as
	// in C, so its values legitimately differ (the oracle tests cover those).
	full, _ := MASSI(s.H, s.L, 3, 5)
	beg, nb, err = massiRange(0, 40, s.H, s.L, 3, 5, out)
	if err != nil || beg != 8 || nb != 33 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[8:41], 0)
	beg, nb, err = massiRange(20, 40, s.H, s.L, 3, 5, out)
	if err != nil || beg != 20 || nb != 21 {
		t.Fatalf("late sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// In-place: output may alias an input.
	buf := append([]float64(nil), s.H...)
	want, _ := MASSI(s.H, s.L, 3, 5)
	beg, nb, _ = massiRange(0, len(buf)-1, buf, s.L, 3, 5, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}

func TestMASSIUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	SetUnstablePeriod(UnstEMA, 4)
	if lb := MASSILookback(9, 25); lb != 2*(8+4)+24 {
		t.Fatalf("lookback with EMA unstable 4 = %d", lb)
	}
	s := fixtures(200)[0]
	out, _ := MASSI(s.H, s.L, 9, 25)
	assertLeadingNaN(t, "MASSI unst", out, MASSILookback(9, 25))
}
