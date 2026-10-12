package ta

import (
	"errors"
	"math"
	"testing"
)

// ceilFrac maps a fixture's series to fractional values of both signs
// (x*0.37 - 30), so CEIL is exercised across zero and on non-integers.
func ceilFrac(in []float64) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = v*0.37 - 30
	}
	return out
}

// ceilSpecial holds integer boundaries, signed zeros, values around the
// 2^52/2^53 precision limits and IEEE special values.
var ceilSpecial = []float64{
	0, math.Copysign(0, -1), 0.5, -0.5, 1, -1, 1.5, -1.5, 2.0000001, -2.0000001,
	math.Nextafter(0, 1), math.Nextafter(0, -1), math.Nextafter(1, 2), math.Nextafter(1, 0),
	math.Nextafter(-1, -2), math.Nextafter(-1, 0), 5e-324, -5e-324, 1e-300, -1e-300,
	4503599627370495.5, -4503599627370495.5, 4503599627370496, 9007199254740993, 1e300, -1e300,
	math.MaxFloat64, -math.MaxFloat64, math.Inf(1), math.Inf(-1), math.NaN(),
}

func TestCEILParams(t *testing.T) {
	if lb := CEILLookback(); lb != 0 {
		t.Errorf("CEILLookback() = %d, want 0", lb)
	}
	if out, err := CEIL(nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CEIL([]float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	in := fixtures(20)[0].Real()
	if _, _, err := ceilRange(0, 10, in[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := ceilRange(0, 10, in, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := ceilRange(0, 10, nil, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := ceilRange(5, 10, in, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestCEILLengths(t *testing.T) {
	lb := CEILLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			for _, in := range [][]float64{s.Real(), ceilFrac(s.Real())} {
				out, err := CEIL(in)
				if err != nil {
					t.Fatalf("%s n=%d: %v", s.Name, n, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
				}
				// No lookback and finite inputs: no NaN anywhere.
				assertLeadingNaN(t, lbl("CEIL ", s.Name, " n=", n), out, lb)
			}
		}
	}
}

func TestCEILKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{1.2, 1, 0.0001, -0.9, -1, -1.5, 2.999, 100.5, -100.5, nan, math.Inf(1), math.Inf(-1)}
	want := []float64{2, 1, 1, 0, -1, -1, 3, 101, -100, nan, math.Inf(1), math.Inf(-1)}
	out, err := CEIL(in)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "CEIL known", out, want, 0)
	// ceil of a value in (-1, 0) is negative zero, as in C.
	if !math.Signbit(out[3]) || out[3] != 0 {
		t.Errorf("ceil(-0.9) = %g signbit=%v, want -0", out[3], math.Signbit(out[3]))
	}

	// Constant input gives a constant output.
	c := fixtures(40)[5].Real()
	out, _ = CEIL(c)
	for i := range out {
		if out[i] != math.Ceil(c[i]) || out[i] != out[0] {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Properties: integer-valued, x <= ceil(x) < x+1, idempotent.
	u := ceilFrac(fixtures(500)[0].Real())
	out, _ = CEIL(u)
	again, _ := CEIL(out)
	for i, v := range out {
		if v != math.Trunc(v) || v < u[i] || v >= u[i]+1 {
			t.Fatalf("bounds: [%d] ceil(%g)=%g", i, u[i], v)
		}
		if again[i] != v {
			t.Fatalf("idempotent: [%d] %g -> %g", i, v, again[i])
		}
	}

	// Integers (including the exact-integer fixtures) are the identity.
	ints := []float64{-3, -2, -1, 0, 1, 2, 3, 4503599627370496}
	out, _ = CEIL(ints)
	assertReal(t, "CEIL integers", out, ints, 0)
}

func TestCEILRangeForm(t *testing.T) {
	in := ceilFrac(fixtures(30)[0].Real())
	out := make([]float64, len(in))
	if _, _, err := ceilRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := ceilRange(taMaxIdx+1, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := ceilRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := ceilRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := ceilRange(7, 7, in, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != math.Ceil(in[7]) {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	beg, nb, err = ceilRange(3, 12, in, out)
	if err != nil || beg != 3 || nb != 10 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	for k := range nb {
		if out[k] != math.Ceil(in[3+k]) {
			t.Errorf("sub-range [%d]", k)
		}
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := CEIL(in)
	beg, nb, _ = ceilRange(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
