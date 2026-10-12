package ta

import (
	"errors"
	"math"
	"testing"
)

func TestBETAParams(t *testing.T) {
	s := fixtures(50)
	in0, in1 := s[0].Real(), s[1].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32} {
		if _, err := BETA(in0, in1, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := BETALookback(p); lb != -1 {
			t.Errorf("BETALookback(%d) = %d, want -1", p, lb)
		}
	}
	for _, p := range []int{1, 2, 5, 100000} {
		if _, err := BETA(in0, in1, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := BETALookback(p); lb != p {
			t.Errorf("BETALookback(%d) = %d, want %d", p, lb, p)
		}
	}
	if _, err := BETA(in0, in1[:49], 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, err := BETA(in0, nil, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil second input: err = %v, want ErrBadParam", err)
	}
	if out, err := BETA(nil, nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := BETA([]float64{}, []float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := betaRange(0, 10, in0[:5], in1, 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inReal0: err = %v, want ErrBadParam", err)
	}
	if _, _, err := betaRange(0, 10, in0, in1[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inReal1: err = %v, want ErrBadParam", err)
	}
	if _, _, err := betaRange(0, 10, in0, in1, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outReal: err = %v, want ErrBadParam", err)
	}
	if _, _, err := betaRange(0, 10, in0, in1, 0, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("range bad period: err = %v, want ErrBadParam", err)
	}
}

func TestBETALengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 30} {
		lb := BETALookback(period)
		if lb != period {
			t.Fatalf("BETALookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			fx := fixtures(n)
			for k, s := range fx {
				other := fx[(k+1)%len(fx)]
				for _, pair := range [][2][]float64{{s.Real(), s.Real()}, {s.Real(), other.Real()}} {
					out, err := BETA(pair[0], pair[1], period)
					if err != nil {
						t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
					}
					if len(out) != n {
						t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
					}
					assertLeadingNaN(t, lbl("BETA ", s.Name, "/", other.Name, " n=", n, " p=", period), out, lb)
				}
			}
		}
	}
}

func TestBETAKnownValues(t *testing.T) {
	nan := math.NaN()
	// Period 2: returns x = (0.1, -0.1), y = (0.05, 0). The least-squares slope
	// through two points is (y1-y2)/(x1-x2) = 0.05/0.2 = 0.25.
	p0 := []float64{100, 110, 99}
	p1 := []float64{100, 105, 105}
	out, err := BETA(p0, p1, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "BETA hand case", out, []float64{nan, nan, 0.25}, 1e-12)

	// y returns are exactly twice x returns: beta is 2.
	// x: 100 -> 110 -> 99 -> 108.9 -> 87.12 (returns .1, -.1, .1, -.2)
	// y: 100 -> 120 -> 96 -> 115.2 -> 69.12 (returns .2, -.2, .2, -.4)
	x := []float64{100, 110, 99, 108.9, 87.12}
	y := []float64{100, 120, 96, 115.2, 69.12}
	out, _ = BETA(x, y, 3)
	assertReal(t, "BETA y=2x", out, []float64{nan, nan, nan, 2, 2}, 1e-12)
	// Swapping the axes gives the reciprocal.
	out, _ = BETA(y, x, 3)
	assertReal(t, "BETA x=y/2", out, []float64{nan, nan, nan, 0.5, 0.5}, 1e-12)

	// A series against itself has beta 1 where its returns vary.
	w := fixtures(200)[0].Real()
	out, _ = BETA(w, w, 10)
	for i := 10; i < len(out); i++ {
		if math.Abs(out[i]-1) > 1e-9 {
			t.Fatalf("self beta [%d]=%.17g", i, out[i])
		}
	}

	// A steadily rising series (near-identical returns) against itself must
	// still give 1, not 0: the shifted sums keep the denominator meaningful.
	up := fixtures(300)[7].Real()
	out, _ = BETA(up, up, 5)
	for i := 5; i < len(out); i++ {
		if math.Abs(out[i]-1) > 1e-6 {
			t.Fatalf("trend self beta [%d]=%.17g", i, out[i])
		}
	}

	// Constant regressor: zero variance, beta is defined as 0.
	c := fixtures(40)[5].Real()
	out, _ = BETA(c, w[:40], 7)
	for i := 7; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant x: [%d]=%g", i, out[i])
		}
	}
	// All-zero prices: no returns, beta 0.
	z := fixtures(40)[6].Real()
	out, _ = BETA(z, z, 5)
	for i := 5; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("zeros: [%d]=%g", i, out[i])
		}
	}
	// Period 1: a single point has no slope, so the output is always 0.
	out, _ = BETA(w, fixtures(200)[1].Real(), 1)
	for i := 1; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("period 1: [%d]=%g", i, out[i])
		}
	}
}

func TestBETARangeForm(t *testing.T) {
	s := fixtures(30)
	in0, in1 := s[0].Real(), s[1].Real()
	out := make([]float64, len(in0))
	if _, _, err := betaRange(-1, 5, in0, in1, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := betaRange(5, 4, in0, in1, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := betaRange(0, -1, in0, in1, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := betaRange(0, 2, in0, in1, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// First bar after the lookback.
	beg, nb, err = betaRange(0, 3, in0, in1, 3, out)
	if err != nil || beg != 3 || nb != 1 {
		t.Errorf("first bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// In-place computation is allowed with the output aliasing either input.
	want, _ := BETA(in0, in1, 4)
	buf := append([]float64(nil), in0...)
	beg, nb, _ = betaRange(0, len(buf)-1, buf, in1, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place inReal0", buf, want, 0)
	buf = append([]float64(nil), in1...)
	beg, nb, _ = betaRange(0, len(buf)-1, in0, buf, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place inReal1", buf, want, 0)
}
