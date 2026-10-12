package ta

import (
	"errors"
	"math"
	"testing"
)

// tanhSpecial holds saturation boundaries, tiny values and IEEE special values.
var tanhSpecial = []float64{
	0, math.Copysign(0, -1), 1, -1, 0.5, -0.5, 0.625, -0.625, 0.6249999, 0.6250001,
	1e-8, -1e-8, 1e-300, -1e-300, 5e-324, -5e-324, 2, -2, 10, -10, 18, 19, 20, -20,
	22, 44, 44.014845965556525, 44.0149, -44.0149, 45, 100, -100, 710, -710, 1e300, -1e300,
	math.MaxFloat64, -math.MaxFloat64, math.Inf(1), math.Inf(-1), math.NaN(),
}

func TestTANHParams(t *testing.T) {
	if lb := TANHLookback(); lb != 0 {
		t.Errorf("TANHLookback() = %d, want 0", lb)
	}
	if out, err := TANH(nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := TANH([]float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	in := fixtures(20)[0].Real()
	if _, _, err := tanhRange(0, 10, in[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := tanhRange(0, 10, in, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := tanhRange(0, 10, nil, make([]float64, 11)); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := tanhRange(0, 10, in, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := tanhRange(5, 10, in, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestTANHLengths(t *testing.T) {
	lb := TANHLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			in := s.Real()
			out, err := TANH(in)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			// No lookback; NaN only for NaN input.
			for i, v := range in {
				if math.IsNaN(v) != math.IsNaN(out[i]) {
					t.Fatalf("%s n=%d [%d]: in=%g out=%g", s.Name, n, i, v, out[i])
				}
			}
		}
	}
}

func TestTANHKnownValues(t *testing.T) {
	nan := math.NaN()
	e := math.E
	// tanh(x) = (e^x - e^-x) / (e^x + e^-x); tanh(ln 2) = (2 - 1/2)/(2 + 1/2) = 0.6.
	in := []float64{0, 1, -1, 2, math.Log(2), -math.Log(3), nan, math.Inf(1), math.Inf(-1), 1000, -1000}
	want := []float64{0, (e - 1/e) / (e + 1/e), -(e - 1/e) / (e + 1/e), (e*e - 1/(e*e)) / (e*e + 1/(e*e)),
		0.6, -0.8, nan, 1, -1, 1, -1}
	out, err := TANH(in)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "TANH known", out, want, 1e-15)
	if out[0] != 0 || math.Signbit(out[0]) {
		t.Errorf("tanh(0) = %g, want +0", out[0])
	}
	o, _ := TANH([]float64{math.Copysign(0, -1), 1e-300, -1e-300})
	if o[0] != 0 || !math.Signbit(o[0]) {
		t.Errorf("tanh(-0) = %g, want -0", o[0])
	}
	if o[1] != 1e-300 || o[2] != -1e-300 {
		t.Errorf("tanh(tiny) = %g %g, want x", o[1], o[2])
	}
	// Saturation: exactly ±1 for large magnitudes, never past it.
	for _, x := range []float64{45, 100, 710, 1e300, math.MaxFloat64} {
		o, _ := TANH([]float64{x, -x})
		if o[0] != 1 || o[1] != -1 {
			t.Errorf("tanh(±%g) = %g %g, want ±1", x, o[0], o[1])
		}
	}

	// Constant input gives a constant output.
	c := fixtures(40)[5].Real()
	out, _ = TANH(c)
	for i := range out {
		if out[i] != libmTanh(100) {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Bounds and odd symmetry: tanh(x) in [-1, 1] and tanh(-x) == -tanh(x).
	for _, s := range fixtures(500) {
		in := s.Real()
		neg := make([]float64, len(in))
		for i, v := range in {
			neg[i] = -v
		}
		a, _ := TANH(in)
		b, _ := TANH(neg)
		for i := range a {
			if math.IsNaN(in[i]) {
				continue
			}
			if a[i] < -1 || a[i] > 1 || a[i] != -b[i] {
				t.Fatalf("%s [%d]: tanh(%g)=%g tanh(-x)=%g", s.Name, i, in[i], a[i], b[i])
			}
		}
	}
	// Monotonically non-decreasing on [-50, 50].
	prev := -1.0
	for x := -50.0; x <= 50; x += 1.0 / 64 {
		o, _ := TANH([]float64{x})
		if o[0] < prev {
			t.Fatalf("not increasing at %g", x)
		}
		prev = o[0]
	}
}

func TestTANHRangeForm(t *testing.T) {
	in := fixtures(30)[2].Real()
	out := make([]float64, len(in))
	if _, _, err := tanhRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := tanhRange(taMaxIdx+1, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := tanhRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := tanhRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := tanhRange(0, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end > max: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := tanhRange(7, 7, in, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != libmTanh(in[7]) {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	beg, nb, err = tanhRange(3, 12, in, out)
	if err != nil || beg != 3 || nb != 10 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	for k := range nb {
		if out[k] != libmTanh(in[3+k]) {
			t.Errorf("sub-range [%d]", k)
		}
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := TANH(in)
	beg, nb, _ = tanhRange(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
