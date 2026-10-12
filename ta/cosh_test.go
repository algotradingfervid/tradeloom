package ta

import (
	"errors"
	"math"
	"testing"
)

// coshSpecial holds overflow boundaries, tiny values and IEEE special values.
var coshSpecial = []float64{
	0, math.Copysign(0, -1), 1, -1, 0.5, -0.5, 21, 21.0000001, -21, 22, 1e-8, -1e-8,
	1e-300, -1e-300, 5e-324, 20, 100, -100, 700, 709, 709.5, 709.78, -709.78,
	709.782712893384, 709.79, 710, -710, 710.4, 710.475, -710.475,
	710.4758600739439, 710.476, -710.476, 711, 1000, -1000, 1e300,
	math.Inf(1), math.Inf(-1), math.NaN(),
}

func TestCOSHParams(t *testing.T) {
	if lb := COSHLookback(); lb != 0 {
		t.Errorf("COSHLookback() = %d, want 0", lb)
	}
	if out, err := COSH(nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := COSH([]float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	in := fixtures(20)[0].Real()
	if _, _, err := coshRange(0, 10, in[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := coshRange(0, 10, in, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := coshRange(0, 10, nil, make([]float64, 11)); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := coshRange(5, 10, in, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestCOSHLengths(t *testing.T) {
	lb := COSHLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			in := s.Real()
			out, err := COSH(in)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			// No lookback; NaN only for NaN input, never for finite input
			// (large inputs give +Inf, not NaN).
			for i, v := range in {
				if math.IsNaN(v) != math.IsNaN(out[i]) {
					t.Fatalf("%s n=%d [%d]: in=%g out=%g", s.Name, n, i, v, out[i])
				}
			}
		}
	}
}

func TestCOSHKnownValues(t *testing.T) {
	nan := math.NaN()
	inf := math.Inf(1)
	e := math.E
	in := []float64{0, 1, -1, 2, math.Log(2), nan, inf, math.Inf(-1), 1000, -1000}
	want := []float64{1, (e + 1/e) / 2, (e + 1/e) / 2, (e*e + 1/(e*e)) / 2, 1.25, nan, inf, inf, inf, inf}
	out, err := COSH(in)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "COSH known", out, want, 1e-15)
	if out[0] != 1 {
		t.Errorf("cosh(0) = %g, want exactly 1", out[0])
	}

	// The overflow band (709.78, 710.48]: Exp overflows but cosh is finite.
	for _, x := range []float64{709.79, 710, 710.4, 710.475, -710.475} {
		o, _ := COSH([]float64{x})
		if math.IsInf(o[0], 0) || o[0] < 8e307 {
			t.Errorf("cosh(%g) = %g, want finite near 1e308", x, o[0])
		}
		// cosh(x) = 2*cosh(x-ln2)... check via exp(|x|-ln2) which is finite.
		ref := math.Exp(math.Abs(x) - math.Ln2)
		if math.Abs(o[0]-ref) > 1e-13*ref {
			t.Errorf("cosh(%g) = %.17g, want ~%.17g", x, o[0], ref)
		}
	}
	for _, x := range []float64{710.476, 711, -711} {
		o, _ := COSH([]float64{x})
		if !math.IsInf(o[0], 1) {
			t.Errorf("cosh(%g) = %g, want +Inf", x, o[0])
		}
	}

	// Constant input gives a constant output.
	c := fixtures(40)[5].Real()
	out, _ = COSH(c)
	for i := range out {
		if out[i] != libmCosh(100) {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Bounds and symmetry: cosh(x) >= 1 and cosh(-x) == cosh(x).
	for _, s := range fixtures(500) {
		in := s.Real()
		neg := make([]float64, len(in))
		for i, v := range in {
			neg[i] = -v
		}
		a, _ := COSH(in)
		b, _ := COSH(neg)
		for i := range a {
			if a[i] < 1 || a[i] != b[i] {
				t.Fatalf("%s [%d]: cosh(%g)=%g cosh(-x)=%g", s.Name, i, in[i], a[i], b[i])
			}
		}
	}
	// Monotonically non-decreasing on [0, 712].
	prev := 0.0
	for x := 0.0; x <= 712; x += 1.0 / 16 {
		o, _ := COSH([]float64{x})
		if o[0] < prev {
			t.Fatalf("not increasing at %g", x)
		}
		prev = o[0]
	}
}

func TestCOSHRangeForm(t *testing.T) {
	in := fixtures(30)[2].Real()
	out := make([]float64, len(in))
	if _, _, err := coshRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := coshRange(taMaxIdx+1, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := coshRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := coshRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := coshRange(0, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end > max: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := coshRange(7, 7, in, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != libmCosh(in[7]) {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	beg, nb, err = coshRange(3, 12, in, out)
	if err != nil || beg != 3 || nb != 10 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	for k := range nb {
		if out[k] != libmCosh(in[3+k]) {
			t.Errorf("sub-range [%d]", k)
		}
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := COSH(in)
	beg, nb, _ = coshRange(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
