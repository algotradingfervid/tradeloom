package ta

import (
	"errors"
	"math"
	"testing"
)

// sqrtSpecial holds exact values, extremes, subnormals and IEEE special values.
var sqrtSpecial = []float64{
	0, math.Copysign(0, -1), 1, 4, 0.25, 2, 1e-300, 5e-324, 2.2250738585072014e-308, 1e300,
	math.MaxFloat64, 1 + 1e-15, 1 - 1e-16, -1e-300, -5e-324, -1, -math.MaxFloat64,
	math.Inf(1), math.Inf(-1), math.NaN(),
}

// sqrtExpectNaN reports whether IEEE sqrt(x) is NaN: NaN, or strictly negative
// (including -Inf). -0 is not negative here: sqrt(-0) = -0.
func sqrtExpectNaN(x float64) bool {
	return math.IsNaN(x) || x < 0
}

func TestSQRTParams(t *testing.T) {
	if lb := SQRTLookback(); lb != 0 {
		t.Errorf("SQRTLookback() = %d, want 0", lb)
	}
	if out, err := SQRT(nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := SQRT([]float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	in := fixtures(20)[0].Real()
	if _, _, err := sqrtRange(0, 10, in[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := sqrtRange(0, 10, nil, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := sqrtRange(0, 10, in, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := sqrtRange(0, 10, in, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := sqrtRange(5, 10, in, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
	if _, _, err := sqrtRange(0, 10, in[:11], make([]float64, 11)); err != nil {
		t.Errorf("exact input buffer: err = %v", err)
	}
}

func TestSQRTLengths(t *testing.T) {
	lb := SQRTLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			in := s.Real()
			out, err := SQRT(in)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			// nan_inf_output: lookback is 0, so NaN may appear only where the
			// input is negative or NaN; finite inputs give finite outputs.
			for i, v := range in {
				var ok bool
				if sqrtExpectNaN(v) {
					ok = math.IsNaN(out[i])
				} else {
					ok = !math.IsNaN(out[i]) && !math.IsInf(out[i], 0)
				}
				if !ok {
					t.Fatalf("%s n=%d [%d]: in=%g out=%g", s.Name, n, i, v, out[i])
				}
			}
		}
	}
}

func TestSQRTKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{0, 1, 4, 9, 0.25, 2, 1e-300 * 1e-8, 1e300, -1, math.Inf(1), math.Inf(-1), nan}
	want := []float64{0, 1, 2, 3, 0.5, math.Sqrt2, 1e-154, 1e150, nan, math.Inf(1), nan, nan}
	out, err := SQRT(in)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "SQRT known", out, want, 1e-15)
	// Perfect squares are exact.
	if out[0] != 0 || out[1] != 1 || out[2] != 2 || out[3] != 3 || out[4] != 0.5 {
		t.Errorf("exact values: %v", out)
	}
	// sqrt(-0) = -0 (IEEE 754), not NaN.
	out, _ = SQRT([]float64{math.Copysign(0, -1)})
	if out[0] != 0 || !math.Signbit(out[0]) {
		t.Errorf("sqrt(-0) = %g, want -0", out[0])
	}

	// Constant input gives a constant output: sqrt(100) = 10.
	c := fixtures(40)[5].Real()
	out, _ = SQRT(c)
	for i := range out {
		if out[i] != 10 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}
	// The zeros fixture gives 0 everywhere.
	out, _ = SQRT(fixtures(40)[6].Real())
	for i := range out {
		if out[i] != 0 {
			t.Fatalf("zeros: [%d]=%g", i, out[i])
		}
	}

	// Inverse of squaring, monotonic, and sqrt(x^2) = x on a long positive fixture.
	x := fixtures(500)[0].Real()
	out, _ = SQRT(x)
	sq := make([]float64, len(x))
	for i, v := range x {
		sq[i] = v * v
	}
	outSq, _ := SQRT(sq)
	for i, v := range out {
		if math.Abs(v*v-x[i]) > 1e-12*x[i] {
			t.Fatalf("sqrt(x)^2: [%d] %g vs %g", i, v*v, x[i])
		}
		if math.Abs(outSq[i]-x[i]) > 1e-15*x[i] {
			t.Fatalf("sqrt(x^2)=x: [%d] %g vs %g", i, outSq[i], x[i])
		}
		if i > 0 && x[i] != x[i-1] && (x[i] > x[i-1]) != (v > out[i-1]) {
			t.Fatalf("monotonic: [%d]", i)
		}
	}

	// The negative fixture gives NaN exactly where the input is negative.
	neg := fixtures(200)[13].Real()
	out, _ = SQRT(neg)
	for i, v := range neg {
		if sqrtExpectNaN(v) != math.IsNaN(out[i]) {
			t.Fatalf("negative fixture [%d]: in=%g out=%g", i, v, out[i])
		}
	}

	// Special values follow the IEEE classes.
	out, _ = SQRT(sqrtSpecial)
	for i, v := range sqrtSpecial {
		switch {
		case sqrtExpectNaN(v):
			if !math.IsNaN(out[i]) {
				t.Errorf("special [%d] %g: got %g want NaN", i, v, out[i])
			}
		case math.IsInf(v, 1):
			if !math.IsInf(out[i], 1) {
				t.Errorf("special [%d] %g: got %g want +Inf", i, v, out[i])
			}
		default:
			if math.IsNaN(out[i]) || math.IsInf(out[i], 0) || out[i] < 0 {
				t.Errorf("special [%d] %g: got %g want finite non-negative", i, v, out[i])
			}
		}
	}
}

func TestSQRTRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := sqrtRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := sqrtRange(taMaxIdx+1, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := sqrtRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := sqrtRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := sqrtRange(0, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end > max: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := sqrtRange(7, 7, in, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != math.Sqrt(in[7]) {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	beg, nb, err = sqrtRange(3, 12, in, out)
	if err != nil || beg != 3 || nb != 10 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	for k := range nb {
		if out[k] != math.Sqrt(in[3+k]) {
			t.Errorf("sub-range [%d]", k)
		}
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := SQRT(in)
	beg, nb, _ = sqrtRange(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
