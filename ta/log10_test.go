package ta

import (
	"errors"
	"math"
	"testing"
)

// log10Pos maps a fixture's close series to strictly positive values
// (|x| + 1e-3), so LOG10 is exercised inside its domain; the raw fixtures
// include zeros (-Inf) and negatives (NaN).
func log10Pos(in []float64) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = math.Abs(v) + 1e-3
	}
	return out
}

// log10Special holds domain boundaries, powers of ten, subnormals, extremes
// and IEEE special values.
var log10Special = []float64{
	0, math.Copysign(0, -1), 1, 10, 100, 1000, 0.1, 0.01, 1e-10, 1e22, 1e23, 1e300, 1e-300, 1e-308,
	math.Nextafter(1, 2), math.Nextafter(1, 0), math.Nextafter(10, 11), math.Nextafter(10, 9),
	5e-324, 2.2250738585072014e-308, math.MaxFloat64, 2, 0.5, 3, math.E,
	-1, -5e-324, -1e300, math.Inf(1), math.Inf(-1), math.NaN(),
}

func TestLOG10Params(t *testing.T) {
	if lb := LOG10Lookback(); lb != 0 {
		t.Errorf("LOG10Lookback() = %d, want 0", lb)
	}
	if out, err := LOG10(nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := LOG10([]float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	in := fixtures(20)[0].Real()
	if _, _, err := log10Range(0, 10, in[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := log10Range(0, 10, in, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := log10Range(0, 10, nil, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := log10Range(5, 10, in, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestLOG10Lengths(t *testing.T) {
	lb := LOG10Lookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			for _, in := range [][]float64{s.Real(), log10Pos(s.Real())} {
				out, err := LOG10(in)
				if err != nil {
					t.Fatalf("%s n=%d: %v", s.Name, n, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
				}
				// LOG10 is flagged nan_inf_output: negative inputs give NaN
				// and zeros give -Inf, so check element-wise rather than
				// with assertLeadingNaN.
				for i, v := range in {
					switch {
					case v < 0:
						if !math.IsNaN(out[i]) {
							t.Fatalf("%s n=%d [%d]: log10(%g)=%g, want NaN", s.Name, n, i, v, out[i])
						}
					case v == 0:
						if !math.IsInf(out[i], -1) {
							t.Fatalf("%s n=%d [%d]: log10(0)=%g, want -Inf", s.Name, n, i, out[i])
						}
					default:
						if math.IsNaN(out[i]) || math.IsInf(out[i], 0) {
							t.Fatalf("%s n=%d [%d]: log10(%g)=%g, want finite", s.Name, n, i, v, out[i])
						}
					}
				}
			}
		}
	}
}

func TestLOG10KnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{1, 10, 100, 1000, 0.1, 0.01, 1e-6, 1e9, 2, 0, math.Copysign(0, -1), -1, nan, math.Inf(1), math.Inf(-1)}
	want := []float64{0, 1, 2, 3, -1, -2, -6, 9, math.Log2(2) * math.Ln2 / math.Ln10, math.Inf(-1), math.Inf(-1), nan, nan, math.Inf(1), nan}
	out, err := LOG10(in)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "LOG10 known", out, want, 1e-15)
	if out[0] != 0 {
		t.Errorf("log10(1) = %g, want exactly 0", out[0])
	}

	// Constant input gives a constant output.
	c := fixtures(40)[5].Real()
	out, _ = LOG10(c)
	for i := range out {
		if out[i] != 2 {
			t.Fatalf("constant 100: [%d]=%.17g", i, out[i])
		}
	}

	// Properties: 10^log10(x) == x, and monotonically increasing.
	u := log10Pos(fixtures(500)[0].Real())
	out, _ = LOG10(u)
	for i, v := range out {
		if math.Abs(math.Pow(10, v)-u[i]) > 1e-12*u[i] {
			t.Fatalf("inverse: [%d] 10^%g=%g want %g", i, v, math.Pow(10, v), u[i])
		}
	}
	prev := math.Inf(-1)
	for x := 1.0 / 64; x <= 100; x += 1.0 / 64 {
		o, _ := LOG10([]float64{x})
		if o[0] <= prev {
			t.Fatalf("not increasing at %g", x)
		}
		prev = o[0]
	}
	// log10(a*b) == log10(a) + log10(b).
	ab, _ := LOG10([]float64{3, 7, 21})
	if math.Abs(ab[0]+ab[1]-ab[2]) > 1e-15 {
		t.Errorf("product rule: %v", ab)
	}
}

func TestLOG10RangeForm(t *testing.T) {
	in := log10Pos(fixtures(30)[0].Real())
	out := make([]float64, len(in))
	if _, _, err := log10Range(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := log10Range(taMaxIdx+1, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := log10Range(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := log10Range(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := log10Range(7, 7, in, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != libmLog10(in[7]) {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	beg, nb, err = log10Range(3, 12, in, out)
	if err != nil || beg != 3 || nb != 10 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	for k := range nb {
		if out[k] != libmLog10(in[3+k]) {
			t.Errorf("sub-range [%d]", k)
		}
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := LOG10(in)
	beg, nb, _ = log10Range(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
