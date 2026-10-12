package ta

import (
	"errors"
	"math"
	"testing"
)

// lnSpecial holds exact values, extremes, subnormals and IEEE special values.
var lnSpecial = []float64{
	1, math.E, 1 / math.E, 2, 0.5, 10, 1e-300, 5e-324, 2.2250738585072014e-308, 1e300,
	math.MaxFloat64, 1 + 1e-15, 1 - 1e-16, 0, math.Copysign(0, -1), -1e-300, -1, -math.MaxFloat64,
	math.Inf(1), math.Inf(-1), math.NaN(),
}

// lnExpect is the IEEE expectation for log(x) by input class.
func lnExpect(x float64) (wantNaN bool, wantInf int) {
	switch {
	case math.IsNaN(x) || x < 0:
		return true, 0
	case x == 0:
		return false, -1
	case math.IsInf(x, 1):
		return false, 1
	}
	return false, 0
}

func TestLNParams(t *testing.T) {
	if lb := LNLookback(); lb != 0 {
		t.Errorf("LNLookback() = %d, want 0", lb)
	}
	if out, err := LN(nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := LN([]float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	in := fixtures(20)[0].Real()
	if _, _, err := lnRange(0, 10, in[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := lnRange(0, 10, nil, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := lnRange(0, 10, in, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := lnRange(0, 10, in, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := lnRange(5, 10, in, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestLNLengths(t *testing.T) {
	lb := LNLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			in := s.Real()
			out, err := LN(in)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			// nan_inf_output: lookback is 0, so NaN/Inf may appear only where
			// the input class dictates (negative/NaN -> NaN, 0 -> -Inf).
			for i, v := range in {
				wantNaN, wantInf := lnExpect(v)
				var ok bool
				switch {
				case wantNaN:
					ok = math.IsNaN(out[i])
				case wantInf != 0:
					ok = math.IsInf(out[i], wantInf)
				default:
					ok = !math.IsNaN(out[i]) && !math.IsInf(out[i], 0)
				}
				if !ok {
					t.Fatalf("%s n=%d [%d]: in=%g out=%g", s.Name, n, i, v, out[i])
				}
			}
		}
	}
}

func TestLNKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{1, math.E, 1 / math.E, 2, 0.5, 4, 0, -1, math.Inf(1), math.Inf(-1), nan}
	want := []float64{0, 1, -1, math.Ln2, -math.Ln2, 2 * math.Ln2, math.Inf(-1), nan, math.Inf(1), nan, nan}
	out, err := LN(in)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "LN known", out, want, 1e-15)
	if out[0] != 0 || !math.IsInf(out[6], -1) {
		t.Errorf("exact values: %v", out)
	}
	// -0 is a zero: log(-0) = -Inf, not NaN.
	out, _ = LN([]float64{math.Copysign(0, -1)})
	if !math.IsInf(out[0], -1) {
		t.Errorf("log(-0) = %g, want -Inf", out[0])
	}

	// Constant input gives a constant output: log(100).
	c := fixtures(40)[5].Real()
	out, _ = LN(c)
	for i := range out {
		if out[i] != libmLog(100) {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}
	// The zeros fixture gives -Inf everywhere.
	out, _ = LN(fixtures(40)[6].Real())
	for i := range out {
		if !math.IsInf(out[i], -1) {
			t.Fatalf("zeros: [%d]=%g", i, out[i])
		}
	}

	// Inverse of exp, monotonic, and the product rule on a long positive fixture.
	x := fixtures(500)[0].Real()
	out, _ = LN(x)
	sq := make([]float64, len(x))
	for i, v := range x {
		sq[i] = v * v
	}
	outSq, _ := LN(sq)
	for i, v := range out {
		if math.Abs(math.Exp(v)-x[i]) > 1e-12*x[i] {
			t.Fatalf("exp(ln x): [%d] %g vs %g", i, math.Exp(v), x[i])
		}
		if math.Abs(outSq[i]-2*v) > 1e-12*math.Max(1, math.Abs(v)) {
			t.Fatalf("ln(x^2)=2ln(x): [%d]", i)
		}
		if i > 0 && (x[i] > x[i-1]) != (v > out[i-1]) && x[i] != x[i-1] {
			t.Fatalf("monotonic: [%d]", i)
		}
	}

	// Special values follow the IEEE classes.
	out, _ = LN(lnSpecial)
	for i, v := range lnSpecial {
		wantNaN, wantInf := lnExpect(v)
		switch {
		case wantNaN:
			if !math.IsNaN(out[i]) {
				t.Errorf("special [%d] %g: got %g want NaN", i, v, out[i])
			}
		case wantInf != 0:
			if !math.IsInf(out[i], wantInf) {
				t.Errorf("special [%d] %g: got %g want %dInf", i, v, out[i], wantInf)
			}
		default:
			if math.IsNaN(out[i]) || math.IsInf(out[i], 0) {
				t.Errorf("special [%d] %g: got %g want finite", i, v, out[i])
			}
		}
	}
}

func TestLNRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := lnRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := lnRange(taMaxIdx+1, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := lnRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := lnRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := lnRange(0, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end > max: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := lnRange(7, 7, in, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != libmLog(in[7]) {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	beg, nb, err = lnRange(3, 12, in, out)
	if err != nil || beg != 3 || nb != 10 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	for k := range nb {
		if out[k] != libmLog(in[3+k]) {
			t.Errorf("sub-range [%d]", k)
		}
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := LN(in)
	beg, nb, _ = lnRange(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
