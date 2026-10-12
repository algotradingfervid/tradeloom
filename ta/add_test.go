package ta

import (
	"errors"
	"math"
	"testing"
)

func TestADDParams(t *testing.T) {
	if lb := ADDLookback(); lb != 0 {
		t.Errorf("ADDLookback() = %d, want 0", lb)
	}
	a := fixtures(50)[0].Real()
	b := fixtures(50)[1].Real()
	// Mismatched input lengths, each input in turn.
	if _, err := ADD(a[:49], b); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inReal0: err = %v, want ErrBadParam", err)
	}
	if _, err := ADD(a, b[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inReal1: err = %v, want ErrBadParam", err)
	}
	if _, err := ADD(nil, b); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inReal0 with non-empty inReal1: err = %v, want ErrBadParam", err)
	}
	if _, err := ADD(a, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inReal1 with non-empty inReal0: err = %v, want ErrBadParam", err)
	}
	// nil / empty input -> empty output, nil error.
	if out, err := ADD(nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := ADD([]float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestADDLengths(t *testing.T) {
	lb := ADDLookback()
	for _, n := range edgeLengths(lb) {
		fs := fixtures(n)
		for i, s := range fs {
			other := fs[(i+1)%len(fs)].Real()
			out, err := ADD(s.Real(), other)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			assertLeadingNaN(t, lbl("ADD ", s.Name, " n=", n), out, lb)
		}
	}
}

func TestADDKnownValues(t *testing.T) {
	a := []float64{1, -2, 3.5, 0, 1e308, -0.25}
	b := []float64{10, 2, -1.5, 0, 1e308, 0.125}
	out, err := ADD(a, b)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "ADD hand", out, []float64{11, 0, 2, 0, math.Inf(1), -0.125}, 0)

	// NaN in either input propagates to that bar only.
	nan := math.NaN()
	out, _ = ADD([]float64{1, nan, 3}, []float64{nan, 2, 4})
	assertReal(t, "ADD nan", out, []float64{nan, nan, 7}, 0)

	// Commutative; adding zeros is the identity; x + (-x) = 0.
	for _, s := range fixtures(300) {
		x := s.Real()
		y := fixtures(300)[13].Real()
		ab, _ := ADD(x, y)
		ba, _ := ADD(y, x)
		assertReal(t, lbl("commutative ", s.Name), ab, ba, 0)
		z, _ := ADD(x, make([]float64, len(x)))
		assertReal(t, lbl("identity ", s.Name), z, x, 0)
		neg := make([]float64, len(x))
		for i := range x {
			neg[i] = -x[i]
		}
		zero, _ := ADD(x, neg)
		for i, v := range zero {
			if v != 0 {
				t.Fatalf("%s x+(-x): [%d]=%g", s.Name, i, v)
			}
		}
	}
	// Constant + constant is constant.
	c := fixtures(40)[5].Real()
	out, _ = ADD(c, c)
	for i, v := range out {
		if v != 200 {
			t.Fatalf("constant: [%d]=%g", i, v)
		}
	}
}

func TestADDRangeForm(t *testing.T) {
	a := fixtures(30)[0].Real()
	b := fixtures(30)[1].Real()
	out := make([]float64, len(a))
	if _, _, err := addRange(-1, 5, a, b, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := addRange(5, 4, a, b, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := addRange(0, -1, a, b, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := addRange(7, 7, a, b, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != a[7]+b[7] {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	beg, nb, err = addRange(3, 20, a, b, out)
	if err != nil || beg != 3 || nb != 18 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := addRange(0, 10, a[:5], b, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inReal0 buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := addRange(0, 10, a, b[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inReal1 buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := addRange(0, 10, a, b, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := addRange(2, 10, a, b, make([]float64, 9)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
	// In-place computation into either input is allowed.
	want, _ := ADD(a, b)
	buf := append([]float64(nil), a...)
	beg, nb, _ = addRange(0, len(buf)-1, buf, b, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place inReal0", buf, want, 0)
	buf = append([]float64(nil), b...)
	beg, nb, _ = addRange(0, len(buf)-1, a, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place inReal1", buf, want, 0)
}
