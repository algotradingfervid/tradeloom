package ta

import (
	"errors"
	"math"
	"testing"
)

func TestLINEARREGANGLEParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := LINEARREGANGLE(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := LINEARREGANGLELookback(p); lb != -1 {
			t.Errorf("LINEARREGANGLELookback(%d) = %d, want -1", p, lb)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, err := LINEARREGANGLE(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := LINEARREGANGLELookback(p); lb != p-1 {
			t.Errorf("LINEARREGANGLELookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	if out, err := LINEARREGANGLE(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := LINEARREGANGLE([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Bad parameter wins over empty input, as in C.
	if _, err := LINEARREGANGLE(nil, 1); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input bad period: err = %v, want ErrBadParam", err)
	}
	if _, _, err := linearregangleRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := linearregangleRange(0, 10, in, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := linearregangleRange(0, 10, in, 1, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("range bad period: err = %v, want ErrBadParam", err)
	}
}

func TestLINEARREGANGLELengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30} {
		lb := LINEARREGANGLELookback(period)
		if lb != period-1 {
			t.Fatalf("LINEARREGANGLELookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := LINEARREGANGLE(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("LINEARREGANGLE ", s.Name, " n=", n, " p=", period), out, lb)
				for i := lb; i < n; i++ {
					if !(out[i] > -90 && out[i] < 90) && !(math.Abs(out[i]) == 90) {
						t.Fatalf("%s n=%d p=%d: [%d]=%g outside [-90, 90]", s.Name, n, period, i, out[i])
					}
				}
			}
		}
	}
}

func TestLINEARREGANGLEKnownValues(t *testing.T) {
	nan := math.NaN()
	deg := func(m float64) float64 { return math.Atan(m) * 180 / math.Pi }

	// Period 2: the slope is simply the last difference.
	in := []float64{1, 2, 4, 4, 3}
	out, err := LINEARREGANGLE(in, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "period 2", out, []float64{nan, 45, deg(2), 0, -45}, 1e-12)

	// Period 3 on {1, 2, 6}: x = 0,1,2; mean x 1, mean y 3;
	// m = sum((x-1)(y-3)) / sum((x-1)^2) = ((-1)(-2) + 0 + (1)(3)) / 2 = 2.5.
	out, _ = LINEARREGANGLE([]float64{1, 2, 6}, 3)
	assertReal(t, "period 3", out, []float64{nan, nan, deg(2.5)}, 1e-12)

	// A line of slope 1 has angle 45; slope -1 has -45; slope 3 atan(3).
	for _, period := range []int{2, 5, 14, 30} {
		up := make([]float64, 200)
		down := make([]float64, 200)
		steep := make([]float64, 200)
		for i := range up {
			up[i] = 10 + float64(i)
			down[i] = 500 - float64(i)
			steep[i] = 3 * float64(i)
		}
		for name, c := range map[string]struct {
			in   []float64
			want float64
		}{"up": {up, 45}, "down": {down, -45}, "steep": {steep, deg(3)}} {
			out, _ := LINEARREGANGLE(c.in, period)
			for i := period - 1; i < len(out); i++ {
				if math.Abs(out[i]-c.want) > 1e-9 {
					t.Fatalf("%s p=%d: [%d]=%.17g want %.17g", name, period, i, out[i], c.want)
				}
			}
		}
	}

	// A constant series has zero slope.
	c := fixtures(400)[5].Real()
	out, _ = LINEARREGANGLE(c, 7)
	for i := 6; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Scaling the input by k > 0 cannot flip the sign of the angle.
	s := fixtures(300)[0].Real()
	a, _ := LINEARREGANGLE(s, 10)
	scaled := make([]float64, len(s))
	for i, v := range s {
		scaled[i] = -v
	}
	b, _ := LINEARREGANGLE(scaled, 10)
	for i := 9; i < len(a); i++ {
		if math.Abs(a[i]+b[i]) > 1e-9*math.Max(1, math.Abs(a[i])) {
			t.Fatalf("negation: [%d] %g vs %g", i, a[i], b[i])
		}
	}

	// A single huge print forces the reseed path; values afterwards must match
	// a fresh computation over each window.
	spike := append([]float64(nil), fixtures(200)[1].Real()...)
	spike[50] = 1e15
	out, _ = LINEARREGANGLE(spike, 5)
	for i := 60; i < len(spike); i++ {
		fresh, _ := LINEARREGANGLE(spike[i-4:i+1], 5)
		if math.Abs(out[i]-fresh[4]) > 1e-9*math.Max(1, math.Abs(fresh[4])) {
			t.Fatalf("after spike: [%d]=%.17g fresh %.17g", i, out[i], fresh[4])
		}
	}
}

func TestLINEARREGANGLERangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := linearregangleRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := linearregangleRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := linearregangleRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := linearregangleRange(0, 1, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// A sub-range matches the full computation.
	full, _ := LINEARREGANGLE(in, 4)
	beg, nb, err = linearregangleRange(10, 20, in, 4, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[10:21], 1e-12)
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := LINEARREGANGLE(in, 4)
	beg, nb, _ = linearregangleRange(0, len(buf)-1, buf, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
