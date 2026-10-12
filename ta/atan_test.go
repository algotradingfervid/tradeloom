package ta

import (
	"errors"
	"math"
	"testing"
)

// atanSmall maps a fixture's close series into a range around 0 (via sin, then
// scaled), where atan is far from saturated; the raw fixtures are mostly large
// positive prices whose atan is close to pi/2.
func atanSmall(in []float64) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = 3 * math.Sin(v)
	}
	return out
}

// atanSpecial holds signed zeros, tiny and huge magnitudes, and IEEE specials.
var atanSpecial = []float64{
	0, math.Copysign(0, -1), 1, -1, 0.5, -0.5, math.Sqrt(3), -math.Sqrt(3),
	1e-300, -1e-300, 5e-324, -5e-324, 1e-8, 1e8, -1e8, 1e300, -1e300,
	math.MaxFloat64, -math.MaxFloat64,
	math.Inf(1), math.Inf(-1), math.NaN(),
}

func TestATANParams(t *testing.T) {
	if lb := ATANLookback(); lb != 0 {
		t.Errorf("ATANLookback() = %d, want 0", lb)
	}
	if out, err := ATAN(nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := ATAN([]float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	in := fixtures(20)[0].Real()
	if _, _, err := atanRange(0, 10, in[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := atanRange(0, 10, in, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := atanRange(0, 10, nil, make([]float64, 11)); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := atanRange(0, 10, in, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := atanRange(5, 10, in, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestATANLengths(t *testing.T) {
	lb := ATANLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			for _, in := range [][]float64{s.Real(), atanSmall(s.Real())} {
				out, err := ATAN(in)
				if err != nil {
					t.Fatalf("%s n=%d: %v", s.Name, n, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
				}
				// ATAN is defined on all reals: no NaN for non-NaN input.
				assertLeadingNaN(t, lbl("ATAN ", s.Name, " n=", n), out, lb)
			}
		}
	}
}

func TestATANKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{0, 1, -1, math.Sqrt(3), -math.Sqrt(3), 1 / math.Sqrt(3), math.Inf(1), math.Inf(-1), nan}
	want := []float64{0, math.Pi / 4, -math.Pi / 4, math.Pi / 3, -math.Pi / 3, math.Pi / 6, math.Pi / 2, -math.Pi / 2, nan}
	out, err := ATAN(in)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "ATAN known", out, want, 1e-15)
	if out[0] != 0 || out[6] != math.Pi/2 || out[7] != -math.Pi/2 {
		t.Errorf("exact values: %v", out)
	}
	// Signed zero is preserved; tiny inputs are returned unchanged.
	out, _ = ATAN([]float64{math.Copysign(0, -1), 1e-300, -5e-324})
	if out[0] != 0 || !math.Signbit(out[0]) {
		t.Errorf("atan(-0) = %g (signbit %v), want -0", out[0], math.Signbit(out[0]))
	}
	if out[1] != 1e-300 || out[2] != -5e-324 {
		t.Errorf("tiny: %v", out[1:])
	}

	// Constant input gives a constant output.
	c := fixtures(40)[5].Real()
	out, _ = ATAN(c)
	for i := range out {
		if out[i] != libmAtan(c[0]) {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Bounds, odd symmetry and inverse: |atan(x)| <= pi/2, atan(-x) == -atan(x),
	// tan(atan(x)) == x.
	u := atanSmall(fixtures(500)[0].Real())
	out, _ = ATAN(u)
	neg := make([]float64, len(u))
	for i, v := range u {
		neg[i] = -v
	}
	nout, _ := ATAN(neg)
	for i, v := range out {
		if math.Abs(v) > math.Pi/2 {
			t.Fatalf("bounds: [%d]=%g", i, v)
		}
		if nout[i] != -v {
			t.Fatalf("odd: [%d] atan(%g)=%g atan(-x)=%g", i, u[i], v, nout[i])
		}
		if math.Abs(math.Tan(v)-u[i]) > 1e-12*math.Max(1, math.Abs(u[i])) {
			t.Fatalf("inverse: [%d] tan(%g)=%g want %g", i, v, math.Tan(v), u[i])
		}
	}
	// Monotonically non-decreasing.
	prev := math.Inf(-1)
	for x := -50.0; x <= 50; x += 1.0 / 16 {
		o, _ := ATAN([]float64{x})
		if o[0] < prev {
			t.Fatalf("not increasing at %g", x)
		}
		prev = o[0]
	}
}

func TestATANRangeForm(t *testing.T) {
	in := atanSmall(fixtures(30)[0].Real())
	out := make([]float64, len(in))
	if _, _, err := atanRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := atanRange(taMaxIdx+1, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := atanRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := atanRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := atanRange(0, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end > max: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := atanRange(7, 7, in, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != libmAtan(in[7]) {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	beg, nb, err = atanRange(3, 12, in, out)
	if err != nil || beg != 3 || nb != 10 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	for k := range nb {
		if out[k] != libmAtan(in[3+k]) {
			t.Errorf("sub-range [%d]", k)
		}
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := ATAN(in)
	beg, nb, _ = atanRange(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
