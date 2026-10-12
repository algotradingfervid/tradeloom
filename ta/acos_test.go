package ta

import (
	"errors"
	"math"
	"testing"
)

// acosUnit maps a fixture's close series into [-1, 1] (via sin), so ACOS is
// exercised inside its domain; the raw fixtures are mostly outside it.
func acosUnit(in []float64) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = math.Sin(v)
	}
	return out
}

// acosSpecial holds domain boundaries and IEEE special values.
var acosSpecial = []float64{
	-1, 1, 0, math.Copysign(0, -1), 0.5, -0.5,
	math.Nextafter(1, 2), math.Nextafter(-1, -2), math.Nextafter(1, 0), math.Nextafter(-1, 0),
	1e-300, -1e-300, 5e-324, 2, -2, 1e300,
	math.Inf(1), math.Inf(-1), math.NaN(),
}

func TestACOSParams(t *testing.T) {
	if lb := ACOSLookback(); lb != 0 {
		t.Errorf("ACOSLookback() = %d, want 0", lb)
	}
	if out, err := ACOS(nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := ACOS([]float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	in := fixtures(20)[0].Real()
	if _, _, err := acosRange(0, 10, in[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := acosRange(0, 10, in, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := acosRange(5, 10, in, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestACOSLengths(t *testing.T) {
	lb := ACOSLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			for _, in := range [][]float64{s.Real(), acosUnit(s.Real())} {
				out, err := ACOS(in)
				if err != nil {
					t.Fatalf("%s n=%d: %v", s.Name, n, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
				}
				// ACOS is flagged nan_inf_output: out-of-domain inputs are NaN,
				// so check element-wise rather than assertLeadingNaN.
				for i, v := range in {
					inDomain := v >= -1 && v <= 1
					if inDomain == math.IsNaN(out[i]) {
						t.Fatalf("%s n=%d [%d]: in=%g out=%g", s.Name, n, i, v, out[i])
					}
				}
			}
		}
	}
}

func TestACOSKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{1, 0, -1, 0.5, -0.5, math.Sqrt2 / 2, 1.5, -1.5, nan, math.Inf(1), math.Inf(-1)}
	want := []float64{0, math.Pi / 2, math.Pi, math.Pi / 3, 2 * math.Pi / 3, math.Pi / 4, nan, nan, nan, nan, nan}
	out, err := ACOS(in)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "ACOS known", out, want, 1e-15)
	if out[0] != 0 || out[2] != math.Pi || out[1] != math.Pi/2 {
		t.Errorf("exact endpoints: %v", out[:3])
	}

	// Constant input gives a constant output.
	c := make([]float64, 40)
	for i := range c {
		c[i] = 0.25
	}
	out, _ = ACOS(c)
	for i := range out {
		if out[i] != libmAcos(0.25) {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Bounds: in-domain output is in [0, pi]; cos(acos(x)) == x.
	u := acosUnit(fixtures(500)[0].Real())
	out, _ = ACOS(u)
	for i, v := range out {
		if v < 0 || v > math.Pi {
			t.Fatalf("bounds: [%d]=%g", i, v)
		}
		if math.Abs(math.Cos(v)-u[i]) > 1e-12 {
			t.Fatalf("inverse: [%d] cos(%g)=%g want %g", i, v, math.Cos(v), u[i])
		}
	}
	// Monotonically non-increasing on [-1, 1].
	prev := math.Inf(1)
	for x := -1.0; x <= 1; x += 1.0 / 64 {
		o, _ := ACOS([]float64{x})
		if o[0] > prev {
			t.Fatalf("not decreasing at %g", x)
		}
		prev = o[0]
	}
}

func TestACOSRangeForm(t *testing.T) {
	in := acosUnit(fixtures(30)[0].Real())
	out := make([]float64, len(in))
	if _, _, err := acosRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := acosRange(taMaxIdx+1, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := acosRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := acosRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := acosRange(7, 7, in, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != libmAcos(in[7]) {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	beg, nb, err = acosRange(3, 12, in, out)
	if err != nil || beg != 3 || nb != 10 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	for k := range nb {
		if out[k] != libmAcos(in[3+k]) {
			t.Errorf("sub-range [%d]", k)
		}
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := ACOS(in)
	beg, nb, _ = acosRange(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
