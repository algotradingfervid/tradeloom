package ta

import (
	"errors"
	"math"
	"testing"
)

// cosSpecial holds exact angles, large arguments and IEEE special values.
var cosSpecial = []float64{
	0, math.Copysign(0, -1), math.Pi, -math.Pi, math.Pi / 2, -math.Pi / 2, math.Pi / 3, 2 * math.Pi,
	1, -1, 1e-300, -1e-300, 5e-324, 1e-8, 1e6, -1e6, 1 << 29, 1<<29 + 1, 1e15, 1e22, 1e300, -1e300,
	math.MaxFloat64, -math.MaxFloat64,
	math.Inf(1), math.Inf(-1), math.NaN(),
}

func TestCOSParams(t *testing.T) {
	if lb := COSLookback(); lb != 0 {
		t.Errorf("COSLookback() = %d, want 0", lb)
	}
	if out, err := COS(nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := COS([]float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	in := fixtures(20)[0].Real()
	if _, _, err := cosRange(0, 10, in[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := cosRange(0, 10, nil, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := cosRange(0, 10, in, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := cosRange(5, 10, in, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestCOSLengths(t *testing.T) {
	lb := COSLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			in := s.Real()
			out, err := COS(in)
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

func TestCOSKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{0, math.Pi, math.Pi / 2, math.Pi / 3, 2 * math.Pi / 3, math.Pi / 4, -math.Pi / 3, math.Inf(1), math.Inf(-1), nan}
	want := []float64{1, -1, 0, 0.5, -0.5, math.Sqrt2 / 2, 0.5, nan, nan, nan}
	out, err := COS(in)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "COS known", out, want, 1e-15)
	if out[0] != 1 || out[1] != -1 {
		t.Errorf("exact values: %v", out[:2])
	}

	// Constant input gives a constant output.
	c := fixtures(40)[5].Real()
	out, _ = COS(c)
	for i := range out {
		if out[i] != libmCos(c[i]) || out[i] != out[0] {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Bounds, evenness and periodicity on a long fixture.
	x := fixtures(500)[0].Real()
	out, _ = COS(x)
	neg := make([]float64, len(x))
	shifted := make([]float64, len(x))
	for i, v := range x {
		neg[i] = -v
		shifted[i] = v + 2*math.Pi
	}
	outNeg, _ := COS(neg)
	outShift, _ := COS(shifted)
	for i, v := range out {
		if v < -1 || v > 1 {
			t.Fatalf("bounds: [%d]=%g", i, v)
		}
		if outNeg[i] != v {
			t.Fatalf("even: [%d] cos(%g)=%g cos(-x)=%g", i, x[i], v, outNeg[i])
		}
		if math.Abs(outShift[i]-v) > 1e-12 {
			t.Fatalf("period: [%d] %g vs %g", i, v, outShift[i])
		}
		if s := math.Sin(x[i]); math.Abs(v*v+s*s-1) > 1e-12 {
			t.Fatalf("pythagorean: [%d]", i)
		}
	}

	// Special values stay in [-1, 1] unless the input is NaN/Inf.
	out, _ = COS(cosSpecial)
	for i, v := range cosSpecial {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			if !math.IsNaN(out[i]) {
				t.Errorf("special [%d] %g: got %g want NaN", i, v, out[i])
			}
		} else if out[i] < -1 || out[i] > 1 {
			t.Errorf("special [%d] %g: got %g", i, v, out[i])
		}
	}
}

func TestCOSRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := cosRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cosRange(taMaxIdx+1, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := cosRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cosRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := cosRange(0, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end > max: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := cosRange(7, 7, in, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != libmCos(in[7]) {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	beg, nb, err = cosRange(3, 12, in, out)
	if err != nil || beg != 3 || nb != 10 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	for k := range nb {
		if out[k] != libmCos(in[3+k]) {
			t.Errorf("sub-range [%d]", k)
		}
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := COS(in)
	beg, nb, _ = cosRange(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
