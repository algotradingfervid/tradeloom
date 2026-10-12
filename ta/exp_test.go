package ta

import (
	"errors"
	"math"
	"testing"
)

// expSpecial holds overflow/underflow boundaries, tiny values and IEEE
// special values.
var expSpecial = []float64{
	0, math.Copysign(0, -1), 1, -1, 0.5, -0.5, 1e-8, -1e-8, 1e-300, -1e-300, 5e-324,
	math.Ln2, -math.Ln2, 20, -20, 100, -100, 700, 709, 709.78, 709.782712893384,
	709.7827128933841, 709.79, 710, 1000, 1e300,
	-700, -708, -708.39, -708.4, -709, -720, -740, -744, -745, -745.1, -745.13,
	-745.1332191019411, -745.14, -746, -1000, -1e300,
	math.Inf(1), math.Inf(-1), math.NaN(),
}

func TestEXPParams(t *testing.T) {
	if lb := EXPLookback(); lb != 0 {
		t.Errorf("EXPLookback() = %d, want 0", lb)
	}
	if out, err := EXP(nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := EXP([]float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	in := fixtures(20)[2].Real()
	if _, _, err := expRange(0, 10, in[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := expRange(0, 10, in, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := expRange(0, 10, nil, make([]float64, 11)); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := expRange(0, 10, in, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := expRange(5, 10, in, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestEXPLengths(t *testing.T) {
	lb := EXPLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			in := s.Real()
			out, err := EXP(in)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			// No lookback; NaN only for NaN input (overflow gives +Inf).
			for i, v := range in {
				if math.IsNaN(v) != math.IsNaN(out[i]) {
					t.Fatalf("%s n=%d [%d]: in=%g out=%g", s.Name, n, i, v, out[i])
				}
			}
		}
	}
}

func TestEXPKnownValues(t *testing.T) {
	nan := math.NaN()
	inf := math.Inf(1)
	e := math.E
	in := []float64{0, math.Copysign(0, -1), 1, -1, 2, math.Ln2, math.Log(10), nan, inf, math.Inf(-1), 710, 1000, -746, -1000}
	want := []float64{1, 1, e, 1 / e, e * e, 2, 10, nan, inf, 0, inf, inf, 0, 0}
	out, err := EXP(in)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "EXP known", out, want, 1e-15)
	if out[0] != 1 || out[1] != 1 {
		t.Errorf("exp(±0) = %g,%g, want exactly 1", out[0], out[1])
	}
	for i, x := range []float64{-746, -1000, math.Inf(-1)} {
		o, _ := EXP([]float64{x})
		if o[0] != 0 || math.Signbit(o[0]) {
			t.Errorf("case %d: exp(%g) = %g, want +0", i, x, o[0])
		}
	}
	// Largest finite result just below the overflow threshold; subnormal just
	// above the underflow threshold.
	o, _ := EXP([]float64{709.78, -745})
	if math.IsInf(o[0], 0) || o[0] < 1.7e308 {
		t.Errorf("exp(709.78) = %g, want finite ~1.79e308", o[0])
	}
	if o[1] <= 0 || o[1] >= 2.2250738585072014e-308 {
		t.Errorf("exp(-745) = %g, want positive subnormal", o[1])
	}

	// Constant input gives a constant output.
	c := fixtures(40)[5].Real()
	out, _ = EXP(c)
	for i := range out {
		if out[i] != libmExp(100) {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}
	// All-zero input gives all ones.
	z := fixtures(40)[6].Real()
	out, _ = EXP(z)
	for i := range out {
		if out[i] != 1 {
			t.Fatalf("zeros: [%d]=%g", i, out[i])
		}
	}

	// Bounds and identities: exp(x) >= 0, exp(x)*exp(-x) ~ 1 for moderate x.
	for _, s := range fixtures(500) {
		in := s.Real()
		neg := make([]float64, len(in))
		for i, v := range in {
			neg[i] = -v
		}
		a, _ := EXP(in)
		b, _ := EXP(neg)
		for i := range a {
			if a[i] < 0 || math.IsNaN(a[i]) {
				t.Fatalf("%s [%d]: exp(%g)=%g", s.Name, i, in[i], a[i])
			}
			if math.Abs(in[i]) < 300 && math.Abs(a[i]*b[i]-1) > 1e-12 {
				t.Fatalf("%s [%d]: exp(x)*exp(-x)=%g", s.Name, i, a[i]*b[i])
			}
		}
	}
	// Monotonically non-decreasing on [-750, 712].
	prev := 0.0
	for x := -750.0; x <= 712; x += 1.0 / 16 {
		o, _ := EXP([]float64{x})
		if o[0] < prev {
			t.Fatalf("not increasing at %g", x)
		}
		prev = o[0]
	}
}

func TestEXPRangeForm(t *testing.T) {
	in := fixtures(30)[2].Real()
	out := make([]float64, len(in))
	if _, _, err := expRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := expRange(taMaxIdx+1, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := expRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := expRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := expRange(0, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end > max: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := expRange(7, 7, in, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != libmExp(in[7]) {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	beg, nb, err = expRange(3, 12, in, out)
	if err != nil || beg != 3 || nb != 10 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	for k := range nb {
		if out[k] != libmExp(in[3+k]) {
			t.Errorf("sub-range [%d]", k)
		}
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := EXP(in)
	beg, nb, _ = expRange(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
