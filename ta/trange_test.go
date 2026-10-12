package ta

import (
	"errors"
	"math"
	"testing"
)

func TestTRANGEParams(t *testing.T) {
	s := fixtures(50)[0]
	if lb := TRANGELookback(); lb != 1 {
		t.Errorf("TRANGELookback() = %d, want 1", lb)
	}
	if _, err := TRANGE(s.H, s.L, s.C); err != nil {
		t.Errorf("unexpected err %v", err)
	}
	// Mismatched input lengths.
	if _, err := TRANGE(s.H, s.L[:49], s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: err = %v", err)
	}
	if _, err := TRANGE(s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v", err)
	}
	if _, err := TRANGE(s.H[:1], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v", err)
	}
	if _, err := TRANGE(nil, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inHigh with non-empty others: err = %v", err)
	}
	// nil / empty input.
	if out, err := TRANGE(nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := TRANGE([]float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Single bar: entirely inside the lookback.
	out, err := TRANGE([]float64{2}, []float64{1}, []float64{1.5})
	if err != nil || len(out) != 1 || !math.IsNaN(out[0]) {
		t.Errorf("single bar: out=%v err=%v, want [NaN]", out, err)
	}
}

func TestTRANGELengths(t *testing.T) {
	lb := TRANGELookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := TRANGE(s.H, s.L, s.C)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			assertLeadingNaN(t, lbl("TRANGE ", s.Name, " n=", n), out, lb)
		}
	}
}

func TestTRANGEKnownValues(t *testing.T) {
	nan := math.NaN()
	// Bar 1: H-L=2, |C0-H|=|10-11|=1, |C0-L|=|10-9|=1 -> 2 (high-low wins).
	// Bar 2: gap up: H=15 L=14, prevClose=10.5 -> |10.5-15|=4.5 wins.
	// Bar 3: gap down: H=8 L=7, prevClose=14.5 -> |14.5-7|=7.5 wins.
	// Bar 4: H=9 L=8.5, prevClose=7.2 -> |7.2-9|=1.8 wins.
	h := []float64{10.5, 11, 15, 8, 9}
	l := []float64{9.5, 9, 14, 7, 8.5}
	c := []float64{10, 10.5, 14.5, 7.2, 8.8}
	out, err := TRANGE(h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "TRANGE hand", out, []float64{nan, 2, 4.5, 7.5, 1.8}, 1e-12)

	// Constant series: zero range, no gaps.
	s := fixtures(40)[5]
	out, _ = TRANGE(s.H, s.L, s.C)
	for i := 1; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Bounds: TR >= high-low and >= both gaps, and equals one of them.
	for _, s := range fixtures(300) {
		out, _ := TRANGE(s.H, s.L, s.C)
		for i := 1; i < len(out); i++ {
			v1 := s.H[i] - s.L[i]
			v2 := math.Abs(s.C[i-1] - s.H[i])
			v3 := math.Abs(s.C[i-1] - s.L[i])
			if out[i] < v1 || out[i] < v2 || out[i] < v3 || (out[i] != v1 && out[i] != v2 && out[i] != v3) {
				t.Fatalf("%s [%d]: tr=%g v=(%g,%g,%g)", s.Name, i, out[i], v1, v2, v3)
			}
		}
	}

	// NaN propagation follows C's comparisons: a NaN high-low is replaced
	// by any non-NaN gap only if the gap compares greater (it never does),
	// so NaN in high or low yields NaN; NaN prevClose leaves high-low.
	out, _ = TRANGE([]float64{2, nan, 3}, []float64{1, 1, 1}, []float64{1.5, 1.5, nan})
	if !math.IsNaN(out[1]) || out[2] != 2 {
		t.Errorf("NaN handling: %v", out)
	}
}

func TestTRANGERangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := trangeRange(-1, 5, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := trangeRange(5, 4, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := trangeRange(0, -1, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := trangeRange(0, 0, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Start inside the lookback is moved up to 1.
	beg, nb, err = trangeRange(0, 5, s.H, s.L, s.C, out)
	if err != nil || beg != 1 || nb != 5 {
		t.Errorf("start 0: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range matches the public form.
	full, _ := TRANGE(s.H, s.L, s.C)
	beg, nb, err = trangeRange(7, 20, s.H, s.L, s.C, out)
	if err != nil || beg != 7 || nb != 14 {
		t.Fatalf("7..20: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[7:21], 0)
	// Too-short buffers.
	if _, _, err := trangeRange(0, 10, s.H, s.L[:5], s.C, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v", err)
	}
	if _, _, err := trangeRange(0, 10, s.H, s.L, s.C[:10], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose buffer: err = %v", err)
	}
	if _, _, err := trangeRange(0, 10, s.H, s.L, s.C, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v", err)
	}
	if _, _, err := trangeRange(5, 10, s.H, s.L, s.C, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}
