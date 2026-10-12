package ta

import (
	"errors"
	"math"
	"testing"
)

// asinUnit maps a fixture's close series into [-1, 1] (via sin), so ASIN is
// exercised inside its domain; the raw fixtures are mostly outside it.
func asinUnit(in []float64) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = math.Sin(v)
	}
	return out
}

// asinSpecial holds domain boundaries and IEEE special values.
var asinSpecial = []float64{
	-1, 1, 0, math.Copysign(0, -1), 0.5, -0.5,
	math.Nextafter(1, 2), math.Nextafter(-1, -2), math.Nextafter(1, 0), math.Nextafter(-1, 0),
	1e-300, -1e-300, 5e-324, 2, -2, 1e300,
	math.Inf(1), math.Inf(-1), math.NaN(),
}

func TestASINParams(t *testing.T) {
	if lb := ASINLookback(); lb != 0 {
		t.Errorf("ASINLookback() = %d, want 0", lb)
	}
	if out, err := ASIN(nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := ASIN([]float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	in := fixtures(20)[0].Real()
	if _, _, err := asinRange(0, 10, in[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := asinRange(0, 10, in, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := asinRange(5, 10, in, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestASINLengths(t *testing.T) {
	lb := ASINLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			for _, in := range [][]float64{s.Real(), asinUnit(s.Real())} {
				out, err := ASIN(in)
				if err != nil {
					t.Fatalf("%s n=%d: %v", s.Name, n, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
				}
				// ASIN is flagged nan_inf_output: out-of-domain inputs are NaN,
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

func TestASINKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{1, 0, -1, 0.5, -0.5, math.Sqrt2 / 2, 1.5, -1.5, nan, math.Inf(1), math.Inf(-1)}
	want := []float64{math.Pi / 2, 0, -math.Pi / 2, math.Pi / 6, -math.Pi / 6, math.Pi / 4, nan, nan, nan, nan, nan}
	out, err := ASIN(in)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "ASIN known", out, want, 1e-15)
	if out[0] != math.Pi/2 || out[1] != 0 || out[2] != -math.Pi/2 {
		t.Errorf("exact endpoints: %v", out[:3])
	}
	// asin preserves the sign of zero and is exact for tiny arguments.
	z, _ := ASIN([]float64{math.Copysign(0, -1), 5e-324, -1e-300})
	if z[0] != 0 || !math.Signbit(z[0]) || z[1] != 5e-324 || z[2] != -1e-300 {
		t.Errorf("signed zero / tiny: %v", z)
	}

	// Constant input gives a constant output.
	c := make([]float64, 40)
	for i := range c {
		c[i] = 0.25
	}
	out, _ = ASIN(c)
	for i := range out {
		if out[i] != libmAsin(0.25) {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Bounds: in-domain output is in [-pi/2, pi/2]; sin(asin(x)) == x; odd function.
	u := asinUnit(fixtures(500)[0].Real())
	out, _ = ASIN(u)
	neg := make([]float64, len(u))
	for i, v := range u {
		neg[i] = -v
	}
	outNeg, _ := ASIN(neg)
	for i, v := range out {
		if v < -math.Pi/2 || v > math.Pi/2 {
			t.Fatalf("bounds: [%d]=%g", i, v)
		}
		if math.Abs(math.Sin(v)-u[i]) > 1e-12 {
			t.Fatalf("inverse: [%d] sin(%g)=%g want %g", i, v, math.Sin(v), u[i])
		}
		if outNeg[i] != -v {
			t.Fatalf("odd: [%d] asin(-x)=%g, -asin(x)=%g", i, outNeg[i], -v)
		}
	}
	// Monotonically non-decreasing on [-1, 1].
	prev := math.Inf(-1)
	for x := -1.0; x <= 1; x += 1.0 / 64 {
		o, _ := ASIN([]float64{x})
		if o[0] < prev {
			t.Fatalf("not increasing at %g", x)
		}
		prev = o[0]
	}
}

func TestASINRangeForm(t *testing.T) {
	in := asinUnit(fixtures(30)[0].Real())
	out := make([]float64, len(in))
	if _, _, err := asinRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := asinRange(taMaxIdx+1, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := asinRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := asinRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := asinRange(7, 7, in, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != libmAsin(in[7]) {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	beg, nb, err = asinRange(3, 12, in, out)
	if err != nil || beg != 3 || nb != 10 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	for k := range nb {
		if out[k] != libmAsin(in[3+k]) {
			t.Errorf("sub-range [%d]", k)
		}
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := ASIN(in)
	beg, nb, _ = asinRange(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
