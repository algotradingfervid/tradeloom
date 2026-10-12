package ta

import (
	"errors"
	"math"
	"testing"
)

func TestBOPParams(t *testing.T) {
	if lb := BOPLookback(); lb != 0 {
		t.Errorf("BOPLookback() = %d, want 0", lb)
	}
	s := fixtures(50)[0]
	// Mismatched input lengths, each input in turn.
	for k := range 4 {
		in := [][]float64{s.O, s.H, s.L, s.C}
		in[k] = in[k][:49]
		if _, err := BOP(in[0], in[1], in[2], in[3]); !errors.Is(err, ErrBadParam) {
			t.Errorf("short input %d: err = %v, want ErrBadParam", k, err)
		}
	}
	if _, err := BOP(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := BOP(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := BOP([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestBOPLengths(t *testing.T) {
	lb := BOPLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := BOP(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			assertLeadingNaN(t, lbl("BOP ", s.Name, " n=", n), out, lb)
		}
	}
}

func TestBOPKnownValues(t *testing.T) {
	o := []float64{10, 10, 12, 5, 7, 3}
	h := []float64{14, 12, 12, 5, 6, 4}
	l := []float64{8, 9, 12, 5, 8, 2}
	c := []float64{13, 9, 12, 5, 7, 2}
	out, err := BOP(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	// (13-10)/6 = 0.5; (9-10)/3; zero range -> 0; zero range -> 0;
	// negative range (malformed) -> 0; (2-3)/2 = -0.5.
	assertReal(t, "BOP known", out, []float64{0.5, -1.0 / 3.0, 0, 0, 0, -0.5}, 0)

	// The exact (non-epsilon) zero test: a tiny positive range is still divided.
	out, _ = BOP([]float64{1e-12}, []float64{2e-12}, []float64{0}, []float64{2e-12})
	assertReal(t, "BOP tiny range", out, []float64{0.5}, 1e-15)

	// Constant series: zero range everywhere -> 0.
	for _, idx := range []int{5, 6} {
		s := fixtures(40)[idx]
		out, _ = BOP(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%g, want 0", s.Name, i, v)
			}
		}
	}

	// Bounds: with well-formed bars (low <= open,close <= high) BOP is in [-1, 1].
	for _, s := range fixtures(500) {
		out, _ = BOP(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if math.IsNaN(v) || v < -1 || v > 1 {
				t.Fatalf("%s: [%d]=%g outside [-1,1]", s.Name, i, v)
			}
		}
	}
}

func TestBOPRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, len(s.C))
	if _, _, err := bopRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := bopRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := bopRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := bopRange(0, 10, s.O, s.H, s.L[:5], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := bopRange(0, 10, s.O, s.H, s.L, s.C, out[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := bopRange(7, 7, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 7 || nb != 1 {
		t.Errorf("single bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	full, _ := BOP(s.O, s.H, s.L, s.C)
	if out[0] != full[7] {
		t.Errorf("single bar value %g, want %g", out[0], full[7])
	}
	// In-place computation over the close buffer is allowed.
	buf := append([]float64(nil), s.C...)
	beg, nb, _ = bopRange(0, len(buf)-1, s.O, s.H, s.L, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, full, 0)
}
