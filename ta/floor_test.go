package ta

import (
	"errors"
	"math"
	"testing"
)

// floorFrac maps a fixture's series to fractional values of both signs
// (x*0.37 - 30), so FLOOR is exercised across zero and on non-integers.
func floorFrac(in []float64) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = v*0.37 - 30
	}
	return out
}

// floorSpecial holds integer boundaries, signed zeros, values around the
// 2^52/2^53 precision limits and IEEE special values.
var floorSpecial = []float64{
	0, math.Copysign(0, -1), 0.5, -0.5, 1, -1, 1.5, -1.5, 2.0000001, -2.0000001,
	math.Nextafter(0, 1), math.Nextafter(0, -1), math.Nextafter(1, 2), math.Nextafter(1, 0),
	math.Nextafter(-1, -2), math.Nextafter(-1, 0), 5e-324, -5e-324, 1e-300, -1e-300,
	4503599627370495.5, -4503599627370495.5, 4503599627370496, 9007199254740993, 1e300, -1e300,
	math.MaxFloat64, -math.MaxFloat64, math.Inf(1), math.Inf(-1), math.NaN(),
}

func TestFLOORParams(t *testing.T) {
	if lb := FLOORLookback(); lb != 0 {
		t.Errorf("FLOORLookback() = %d, want 0", lb)
	}
	if out, err := FLOOR(nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := FLOOR([]float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	in := fixtures(20)[0].Real()
	if _, _, err := floorRange(0, 10, in[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := floorRange(0, 10, in, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := floorRange(0, 10, nil, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := floorRange(5, 10, in, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestFLOORLengths(t *testing.T) {
	lb := FLOORLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			for _, in := range [][]float64{s.Real(), floorFrac(s.Real())} {
				out, err := FLOOR(in)
				if err != nil {
					t.Fatalf("%s n=%d: %v", s.Name, n, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
				}
				// No lookback and finite inputs: no NaN anywhere.
				assertLeadingNaN(t, lbl("FLOOR ", s.Name, " n=", n), out, lb)
			}
		}
	}
}

func TestFLOORKnownValues(t *testing.T) {
	nan := math.NaN()
	negZero := math.Copysign(0, -1)
	in := []float64{1.2, 1, 0.9999, -0.1, -1, -1.5, 2.999, 100.5, -100.5, nan, math.Inf(1), math.Inf(-1), negZero, 0.5}
	want := []float64{1, 1, 0, -1, -1, -2, 2, 100, -101, nan, math.Inf(1), math.Inf(-1), 0, 0}
	out, err := FLOOR(in)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "FLOOR known", out, want, 0)
	// floor(-0) is -0 and floor of a value in (0, 1) is +0, as in C.
	if !math.Signbit(out[12]) || out[12] != 0 {
		t.Errorf("floor(-0) = %g signbit=%v, want -0", out[12], math.Signbit(out[12]))
	}
	if math.Signbit(out[13]) || out[13] != 0 {
		t.Errorf("floor(0.5) = %g signbit=%v, want +0", out[13], math.Signbit(out[13]))
	}

	// Constant input gives a constant output.
	c := fixtures(40)[5].Real()
	out, _ = FLOOR(c)
	for i := range out {
		if out[i] != math.Floor(c[i]) || out[i] != out[0] {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Properties: integer-valued, x-1 < floor(x) <= x, idempotent.
	u := floorFrac(fixtures(500)[0].Real())
	out, _ = FLOOR(u)
	again, _ := FLOOR(out)
	for i, v := range out {
		if v != math.Trunc(v) || v > u[i] || v <= u[i]-1 {
			t.Fatalf("bounds: [%d] floor(%g)=%g", i, u[i], v)
		}
		if again[i] != v {
			t.Fatalf("idempotent: [%d] %g -> %g", i, v, again[i])
		}
	}

	// Integers (including the exact-integer fixtures) are the identity.
	ints := []float64{-3, -2, -1, 0, 1, 2, 3, 4503599627370496}
	out, _ = FLOOR(ints)
	assertReal(t, "FLOOR integers", out, ints, 0)
}

func TestFLOORRangeForm(t *testing.T) {
	in := floorFrac(fixtures(30)[0].Real())
	out := make([]float64, len(in))
	if _, _, err := floorRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := floorRange(taMaxIdx+1, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := floorRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := floorRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := floorRange(7, 7, in, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != math.Floor(in[7]) {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	beg, nb, err = floorRange(3, 12, in, out)
	if err != nil || beg != 3 || nb != 10 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	for k := range nb {
		if out[k] != math.Floor(in[3+k]) {
			t.Errorf("sub-range [%d]", k)
		}
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := FLOOR(in)
	beg, nb, _ = floorRange(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
