package ta

import (
	"errors"
	"math"
	"testing"
)

func TestDIVParams(t *testing.T) {
	if lb := DIVLookback(); lb != 0 {
		t.Errorf("DIVLookback() = %d, want 0", lb)
	}
	a := fixtures(50)[0].Real()
	b := fixtures(50)[1].Real()
	// Mismatched input lengths, each input in turn.
	if _, err := DIV(a[:49], b); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inReal0: err = %v, want ErrBadParam", err)
	}
	if _, err := DIV(a, b[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inReal1: err = %v, want ErrBadParam", err)
	}
	if _, err := DIV(nil, b); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inReal0 with non-empty inReal1: err = %v, want ErrBadParam", err)
	}
	if _, err := DIV(a, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inReal1 with non-empty inReal0: err = %v, want ErrBadParam", err)
	}
	// nil / empty input -> empty output, nil error.
	if out, err := DIV(nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := DIV([]float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

// DIV carries the nan_inf_output flag (0/0 = NaN on the "zeros" fixture), so
// instead of "no NaN after the lookback" each bar is checked to be exactly the
// IEEE quotient, NaN exactly where the quotient is NaN.
func TestDIVLengths(t *testing.T) {
	lb := DIVLookback()
	for _, n := range edgeLengths(lb) {
		fs := fixtures(n)
		for i, s := range fs {
			x := s.Real()
			y := fs[(i+1)%len(fs)].Real()
			out, err := DIV(x, y)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			want := make([]float64, n)
			for k := range want {
				want[k] = x[k] / y[k]
			}
			assertReal(t, lbl("DIV ", s.Name, " n=", n), out, want, 0)
			// A fixture divided by a strictly non-zero fixture never yields NaN.
			q, _ := DIV(x, fs[0].Real()) // "walk" around 1000, never zero
			assertLeadingNaN(t, lbl("DIV by walk ", s.Name, " n=", n), q, lb)
		}
	}
}

func TestDIVKnownValues(t *testing.T) {
	a := []float64{1, -2, 3.5, 7, 1e308, -0.25, 6}
	b := []float64{4, 2, -0.5, 7, 1e-308, 0.125, -3}
	out, err := DIV(a, b)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "DIV hand", out, []float64{0.25, -1, -7, 1, math.Inf(1), -2, -2}, 0)

	// Division by zero is not an error: x/+0 = ±Inf, x/-0 = ∓Inf, 0/0 = NaN.
	nan := math.NaN()
	negZero := math.Copysign(0, -1)
	out, err = DIV([]float64{1, -1, 0, 1, nan, 2}, []float64{0, 0, 0, negZero, 2, nan})
	if err != nil {
		t.Fatalf("divide by zero: err = %v", err)
	}
	assertReal(t, "DIV by zero / nan", out, []float64{math.Inf(1), math.Inf(-1), nan, math.Inf(-1), nan, nan}, 0)

	// Inf / Inf = NaN; finite / Inf = 0.
	out, _ = DIV([]float64{math.Inf(1), 5}, []float64{math.Inf(-1), math.Inf(1)})
	assertReal(t, "DIV inf", out, []float64{nan, 0}, 0)

	// x / 1 is the identity; x / x is 1 for non-zero x.
	for _, s := range fixtures(300) {
		x := s.Real()
		ones := make([]float64, len(x))
		for i := range ones {
			ones[i] = 1
		}
		id, _ := DIV(x, ones)
		assertReal(t, lbl("identity ", s.Name), id, x, 0)
		self, _ := DIV(x, x)
		for i, v := range self {
			if x[i] != 0 && v != 1 {
				t.Fatalf("%s x/x: [%d]=%g", s.Name, i, v)
			}
			if x[i] == 0 && !math.IsNaN(v) {
				t.Fatalf("%s 0/0: [%d]=%g, want NaN", s.Name, i, v)
			}
		}
	}
	// Constant / constant is constant.
	c := fixtures(40)[5].Real()
	out, _ = DIV(c, c)
	for i, v := range out {
		if v != 1 {
			t.Fatalf("constant: [%d]=%g", i, v)
		}
	}
}

func TestDIVRangeForm(t *testing.T) {
	a := fixtures(30)[0].Real()
	b := fixtures(30)[1].Real()
	out := make([]float64, len(a))
	if _, _, err := divRange(-1, 5, a, b, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := divRange(5, 4, a, b, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := divRange(0, -1, a, b, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := divRange(7, 7, a, b, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != a[7]/b[7] {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	beg, nb, err = divRange(3, 20, a, b, out)
	if err != nil || beg != 3 || nb != 18 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	for k := 0; k < nb; k++ {
		if out[k] != a[3+k]/b[3+k] {
			t.Fatalf("sub-range [%d]=%g want %g", k, out[k], a[3+k]/b[3+k])
		}
	}
	// Short buffers.
	if _, _, err := divRange(0, 10, a[:5], b, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inReal0 buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := divRange(0, 10, a, b[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inReal1 buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := divRange(0, 10, a, b, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := divRange(2, 10, a, b, make([]float64, 9)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
	// In-place computation into either input is allowed.
	want, _ := DIV(a, b)
	buf := append([]float64(nil), a...)
	beg, nb, _ = divRange(0, len(buf)-1, buf, b, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place inReal0", buf, want, 0)
	buf = append([]float64(nil), b...)
	beg, nb, _ = divRange(0, len(buf)-1, a, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place inReal1", buf, want, 0)
}
