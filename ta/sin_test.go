package ta

import (
	"errors"
	"math"
	"testing"
)

// sinSpecial holds exact angles, large arguments and IEEE special values.
var sinSpecial = []float64{
	0, math.Copysign(0, -1), math.Pi, -math.Pi, math.Pi / 2, -math.Pi / 2, math.Pi / 6, 2 * math.Pi,
	1, -1, 1e-300, -1e-300, 5e-324, -5e-324, 1e-8, 1e6, -1e6, 1 << 29, 1<<29 + 1, 1e15, 1e22, 1e300, -1e300,
	math.MaxFloat64, -math.MaxFloat64,
	math.Inf(1), math.Inf(-1), math.NaN(),
}

func TestSINParams(t *testing.T) {
	if lb := SINLookback(); lb != 0 {
		t.Errorf("SINLookback() = %d, want 0", lb)
	}
	if lb := sinLookback(); lb != 0 {
		t.Errorf("sinLookback() = %d, want 0", lb)
	}
	if out, err := SIN(nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := SIN([]float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	in := fixtures(20)[0].Real()
	if _, _, err := sinRange(0, 10, in[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := sinRange(0, 10, nil, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := sinRange(0, 10, in, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := sinRange(0, 10, in, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := sinRange(5, 10, in, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestSINLengths(t *testing.T) {
	lb := SINLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			in := s.Real()
			out, err := SIN(in)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			// Lookback is 0 and finite inputs never give NaN; NaN/Inf inputs
			// (if a fixture has any) must give NaN.
			for i, v := range in {
				finite := !math.IsNaN(v) && !math.IsInf(v, 0)
				if finite == math.IsNaN(out[i]) {
					t.Fatalf("%s n=%d [%d]: in=%g out=%g", s.Name, n, i, v, out[i])
				}
			}
		}
	}
}

func TestSINKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{0, math.Pi / 2, -math.Pi / 2, math.Pi / 6, 5 * math.Pi / 6, math.Pi / 4, -math.Pi / 6, math.Pi / 3, math.Pi, math.Inf(1), math.Inf(-1), nan}
	want := []float64{0, 1, -1, 0.5, 0.5, math.Sqrt2 / 2, -0.5, math.Sqrt(3) / 2, 0, nan, nan, nan}
	out, err := SIN(in)
	if err != nil {
		t.Fatal(err)
	}
	// sin(float64(Pi)) is ~1.2e-16, not 0; compare with an absolute tolerance.
	for i := range want {
		if math.IsNaN(want[i]) {
			if !math.IsNaN(out[i]) {
				t.Errorf("known [%d]: got %g want NaN", i, out[i])
			}
		} else if math.Abs(out[i]-want[i]) > 1e-15 {
			t.Errorf("known [%d]: sin(%g) = %.17g want %.17g", i, in[i], out[i], want[i])
		}
	}
	if out[0] != 0 || out[1] != 1 || out[2] != -1 {
		t.Errorf("exact values: %v", out[:3])
	}
	// Signed zero and tiny arguments: sin(x) == x.
	out, _ = SIN([]float64{math.Copysign(0, -1), 1e-300, 5e-324})
	if !math.Signbit(out[0]) || out[0] != 0 || out[1] != 1e-300 || out[2] != 5e-324 {
		t.Errorf("tiny args: %v", out)
	}

	// Constant input gives a constant output.
	c := fixtures(40)[5].Real()
	out, _ = SIN(c)
	for i := range out {
		if out[i] != libmSin(c[i]) || out[i] != out[0] {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Bounds, oddness, periodicity and the Pythagorean identity on a long fixture.
	x := fixtures(500)[0].Real()
	out, _ = SIN(x)
	neg := make([]float64, len(x))
	shifted := make([]float64, len(x))
	for i, v := range x {
		neg[i] = -v
		shifted[i] = v + 2*math.Pi
	}
	outNeg, _ := SIN(neg)
	outShift, _ := SIN(shifted)
	for i, v := range out {
		if v < -1 || v > 1 {
			t.Fatalf("bounds: [%d]=%g", i, v)
		}
		if outNeg[i] != -v {
			t.Fatalf("odd: [%d] sin(%g)=%g sin(-x)=%g", i, x[i], v, outNeg[i])
		}
		if math.Abs(outShift[i]-v) > 1e-12 {
			t.Fatalf("period: [%d] %g vs %g", i, v, outShift[i])
		}
		if c := math.Cos(x[i]); math.Abs(v*v+c*c-1) > 1e-12 {
			t.Fatalf("pythagorean: [%d]", i)
		}
	}

	// Special values stay in [-1, 1] unless the input is NaN/Inf.
	out, _ = SIN(sinSpecial)
	for i, v := range sinSpecial {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			if !math.IsNaN(out[i]) {
				t.Errorf("special [%d] %g: got %g want NaN", i, v, out[i])
			}
		} else if out[i] < -1 || out[i] > 1 {
			t.Errorf("special [%d] %g: got %g", i, v, out[i])
		}
	}
}

func TestSINRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := sinRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := sinRange(taMaxIdx+1, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := sinRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := sinRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := sinRange(0, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end > max: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := sinRange(7, 7, in, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != libmSin(in[7]) {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	beg, nb, err = sinRange(3, 12, in, out)
	if err != nil || beg != 3 || nb != 10 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	for k := range nb {
		if out[k] != libmSin(in[3+k]) {
			t.Errorf("sub-range [%d]", k)
		}
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := SIN(in)
	beg, nb, _ = sinRange(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
