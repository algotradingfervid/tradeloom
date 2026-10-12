package ta

import (
	"errors"
	"math"
	"testing"
)

func TestMULTParams(t *testing.T) {
	if lb := MULTLookback(); lb != 0 {
		t.Errorf("MULTLookback() = %d, want 0", lb)
	}
	a := fixtures(50)[0].Real()
	b := fixtures(50)[1].Real()
	// Mismatched input lengths, each input in turn.
	if _, err := MULT(a[:49], b); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inReal0: err = %v, want ErrBadParam", err)
	}
	if _, err := MULT(a, b[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inReal1: err = %v, want ErrBadParam", err)
	}
	if _, err := MULT(nil, b); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inReal0 with non-empty inReal1: err = %v, want ErrBadParam", err)
	}
	if _, err := MULT(a, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inReal1 with non-empty inReal0: err = %v, want ErrBadParam", err)
	}
	// nil / empty input -> empty output, nil error.
	if out, err := MULT(nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := MULT([]float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestMULTLengths(t *testing.T) {
	lb := MULTLookback()
	for _, n := range edgeLengths(lb) {
		fs := fixtures(n)
		for i, s := range fs {
			other := fs[(i+1)%len(fs)].Real()
			out, err := MULT(s.Real(), other)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			assertLeadingNaN(t, lbl("MULT ", s.Name, " n=", n), out, lb)
		}
	}
}

func TestMULTKnownValues(t *testing.T) {
	a := []float64{1, -2, 3.5, 0, 1e308, -0.25, 1e-200}
	b := []float64{10, 2, -1.5, -7, 10, 0.125, 1e-200}
	out, err := MULT(a, b)
	if err != nil {
		t.Fatal(err)
	}
	// 0 * -7 is -0 in IEEE; compare by value. 1e308*10 overflows; 1e-400 underflows to 0.
	assertReal(t, "MULT hand", out, []float64{10, -4, -5.25, 0, math.Inf(1), -0.03125, 0}, 0)

	// NaN in either input propagates to that bar only; 0 * Inf is NaN.
	nan := math.NaN()
	out, _ = MULT([]float64{1, nan, 3, 0}, []float64{nan, 2, 4, math.Inf(1)})
	assertReal(t, "MULT nan", out, []float64{nan, nan, 12, nan}, 0)

	// Commutative; multiplying by ones is the identity; by zeros gives zero.
	ones := make([]float64, 300)
	for i := range ones {
		ones[i] = 1
	}
	for _, s := range fixtures(300) {
		x := s.Real()
		y := fixtures(300)[13].Real()
		ab, _ := MULT(x, y)
		ba, _ := MULT(y, x)
		assertReal(t, lbl("commutative ", s.Name), ab, ba, 0)
		id, _ := MULT(x, ones)
		assertReal(t, lbl("identity ", s.Name), id, x, 0)
		zero, _ := MULT(x, make([]float64, len(x)))
		for i, v := range zero {
			if v != 0 && !(math.IsNaN(x[i]) || math.IsInf(x[i], 0)) {
				t.Fatalf("%s x*0: [%d]=%g", s.Name, i, v)
			}
		}
		// Squares are non-negative.
		sq, _ := MULT(x, x)
		for i, v := range sq {
			if v < 0 {
				t.Fatalf("%s x*x: [%d]=%g", s.Name, i, v)
			}
		}
	}
	// Constant * constant is constant.
	c := fixtures(40)[5].Real()
	out, _ = MULT(c, c)
	for i, v := range out {
		if v != 10000 {
			t.Fatalf("constant: [%d]=%g", i, v)
		}
	}
}

func TestMULTRangeForm(t *testing.T) {
	a := fixtures(30)[0].Real()
	b := fixtures(30)[1].Real()
	out := make([]float64, len(a))
	if _, _, err := multRange(-1, 5, a, b, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := multRange(5, 4, a, b, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := multRange(0, -1, a, b, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := multRange(7, 7, a, b, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != a[7]*b[7] {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	beg, nb, err = multRange(3, 20, a, b, out)
	if err != nil || beg != 3 || nb != 18 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := multRange(0, 10, a[:5], b, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inReal0 buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := multRange(0, 10, a, b[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inReal1 buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := multRange(0, 10, a, b, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := multRange(2, 10, a, b, make([]float64, 9)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
	// In-place computation into either input is allowed.
	want, _ := MULT(a, b)
	buf := append([]float64(nil), a...)
	beg, nb, _ = multRange(0, len(buf)-1, buf, b, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place inReal0", buf, want, 0)
	buf = append([]float64(nil), b...)
	beg, nb, _ = multRange(0, len(buf)-1, a, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place inReal1", buf, want, 0)
}
