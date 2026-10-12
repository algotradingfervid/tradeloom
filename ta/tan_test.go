package ta

import (
	"errors"
	"math"
	"testing"
)

// tanSmall maps a fixture's close series into (-1.2, 1.2), well inside
// (-pi/2, pi/2), where tan is far from its poles; the raw fixtures are large
// prices whose tangent wanders over the whole real line.
func tanSmall(in []float64) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = 1.2 * math.Sin(v)
	}
	return out
}

// tanSpecial holds exact angles, values next to the poles, large arguments
// and IEEE special values.
var tanSpecial = []float64{
	0, math.Copysign(0, -1), math.Pi / 4, -math.Pi / 4, math.Pi / 3, -math.Pi / 6, math.Pi, -math.Pi,
	math.Pi / 2, -math.Pi / 2, math.Nextafter(math.Pi/2, 0), 1.5707963, 1.57, 3 * math.Pi / 2,
	1, -1, 1e-300, -1e-300, 5e-324, 1e-8, 1e6, -1e6, 1 << 29, 1<<29 + 1, 1e15, 1e22, 1e300, -1e300,
	math.MaxFloat64, -math.MaxFloat64,
	math.Inf(1), math.Inf(-1), math.NaN(),
}

func TestTANParams(t *testing.T) {
	if lb := TANLookback(); lb != 0 {
		t.Errorf("TANLookback() = %d, want 0", lb)
	}
	if out, err := TAN(nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := TAN([]float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	in := fixtures(20)[0].Real()
	if _, _, err := tanRange(0, 10, in[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := tanRange(0, 10, nil, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := tanRange(0, 10, in, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := tanRange(0, 10, in, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := tanRange(5, 10, in, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestTANLengths(t *testing.T) {
	lb := TANLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			for _, in := range [][]float64{s.Real(), tanSmall(s.Real())} {
				out, err := TAN(in)
				if err != nil {
					t.Fatalf("%s n=%d: %v", s.Name, n, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
				}
				// Lookback is 0 and finite inputs never give NaN; NaN/Inf
				// inputs (if a fixture has any) must give NaN.
				for i, v := range in {
					finite := !math.IsNaN(v) && !math.IsInf(v, 0)
					if finite == math.IsNaN(out[i]) {
						t.Fatalf("%s n=%d [%d]: in=%g out=%g", s.Name, n, i, v, out[i])
					}
				}
			}
		}
	}
}

func TestTANKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{0, math.Pi / 4, -math.Pi / 4, math.Pi / 3, -math.Pi / 3, math.Pi / 6, math.Pi, math.Inf(1), math.Inf(-1), nan}
	want := []float64{0, 1, -1, math.Sqrt(3), -math.Sqrt(3), 1 / math.Sqrt(3), 0, nan, nan, nan}
	out, err := TAN(in)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "TAN known", out, want, 1e-15)
	if out[0] != 0 {
		t.Errorf("tan(0) = %g", out[0])
	}
	// Signed zero is preserved; tiny inputs are returned unchanged.
	out, _ = TAN([]float64{math.Copysign(0, -1), 1e-300, -5e-324})
	if out[0] != 0 || !math.Signbit(out[0]) {
		t.Errorf("tan(-0) = %g (signbit %v), want -0", out[0], math.Signbit(out[0]))
	}
	if out[1] != 1e-300 || out[2] != -5e-324 {
		t.Errorf("tiny: %v", out[1:])
	}
	// Next to the pole the value is huge, with the sign of the side.
	out, _ = TAN([]float64{math.Pi / 2, math.Nextafter(math.Pi/2, 4)})
	if out[0] < 1e15 || out[1] > -1e15 {
		t.Errorf("pole: %v", out)
	}

	// Constant input gives a constant output.
	c := fixtures(40)[5].Real()
	out, _ = TAN(c)
	for i := range out {
		if out[i] != libmTan(c[i]) || out[i] != out[0] {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Odd symmetry, sin/cos identity and inverse on a long fixture.
	u := tanSmall(fixtures(500)[0].Real())
	out, _ = TAN(u)
	neg := make([]float64, len(u))
	for i, v := range u {
		neg[i] = -v
	}
	nout, _ := TAN(neg)
	for i, v := range out {
		if nout[i] != -v {
			t.Fatalf("odd: [%d] tan(%g)=%g tan(-x)=%g", i, u[i], v, nout[i])
		}
		if r := math.Sin(u[i]) / math.Cos(u[i]); math.Abs(r-v) > 1e-14*math.Max(1, math.Abs(v)) {
			t.Fatalf("sin/cos: [%d] %g vs %g", i, v, r)
		}
		if math.Abs(math.Atan(v)-u[i]) > 1e-14 {
			t.Fatalf("inverse: [%d] atan(%g)=%g want %g", i, v, math.Atan(v), u[i])
		}
	}
	// Strictly increasing on (-pi/2, pi/2).
	prev := math.Inf(-1)
	for x := -1.5; x <= 1.5; x += 1.0 / 64 {
		o, _ := TAN([]float64{x})
		if o[0] <= prev {
			t.Fatalf("not increasing at %g", x)
		}
		prev = o[0]
	}
	// Period pi: tan(x+pi) ~ tan(x) on the small series.
	sh := make([]float64, len(u))
	for i, v := range u {
		sh[i] = v + math.Pi
	}
	sout, _ := TAN(sh)
	for i, v := range out {
		if math.Abs(sout[i]-v) > 1e-12*math.Max(1, math.Abs(v)) {
			t.Fatalf("period: [%d] %g vs %g", i, v, sout[i])
		}
	}
}

func TestTANRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := tanRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := tanRange(taMaxIdx+1, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := tanRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := tanRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := tanRange(0, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end > max: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := tanRange(7, 7, in, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != libmTan(in[7]) {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	beg, nb, err = tanRange(3, 12, in, out)
	if err != nil || beg != 3 || nb != 10 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	for k := range nb {
		if out[k] != libmTan(in[3+k]) {
			t.Errorf("sub-range [%d]", k)
		}
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := TAN(in)
	beg, nb, _ = tanRange(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
