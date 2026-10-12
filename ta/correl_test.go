package ta

import (
	"errors"
	"math"
	"testing"
)

func TestCORRELParams(t *testing.T) {
	s := fixtures(50)
	in0, in1 := s[0].Real(), s[1].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32} {
		if _, err := CORREL(in0, in1, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := CORRELLookback(p); lb != -1 {
			t.Errorf("CORRELLookback(%d) = %d, want -1", p, lb)
		}
	}
	for _, p := range []int{1, 2, 30, 100000} {
		if _, err := CORREL(in0, in1, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := CORRELLookback(p); lb != p-1 {
			t.Errorf("CORRELLookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	if _, err := CORREL(in0, in1[:49], 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, err := CORREL(in0, nil, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil second input: err = %v, want ErrBadParam", err)
	}
	if out, err := CORREL(nil, nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CORREL([]float64{}, []float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := correlRange(0, 10, in0[:5], in1, 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inReal0: err = %v, want ErrBadParam", err)
	}
	if _, _, err := correlRange(0, 10, in0, in1[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inReal1: err = %v, want ErrBadParam", err)
	}
	if _, _, err := correlRange(0, 10, in0, in1, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outReal: err = %v, want ErrBadParam", err)
	}
	if _, _, err := correlRange(0, 10, in0, in1, 0, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("range bad period: err = %v, want ErrBadParam", err)
	}
	if _, _, err := correlRange(0, 10, in0, in1, 100001, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("range period max+1: err = %v, want ErrBadParam", err)
	}
}

func TestCORRELLengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 30} {
		lb := CORRELLookback(period)
		if lb != period-1 {
			t.Fatalf("CORRELLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			fx := fixtures(n)
			for k, s := range fx {
				other := fx[(k+1)%len(fx)]
				for _, pair := range [][2][]float64{{s.Real(), s.Real()}, {s.Real(), other.Real()}, {s.O, s.C}} {
					out, err := CORREL(pair[0], pair[1], period)
					if err != nil {
						t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
					}
					if len(out) != n {
						t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
					}
					label := lbl("CORREL ", s.Name, "/", other.Name, " n=", n, " p=", period)
					assertLeadingNaN(t, label, out, lb)
					for i := lb; i < n; i++ {
						if out[i] < -1 || out[i] > 1 {
							t.Fatalf("%s: [%d]=%g outside [-1,1]", label, i, out[i])
						}
					}
				}
			}
		}
	}
}

func TestCORRELKnownValues(t *testing.T) {
	nan := math.NaN()
	// Perfect positive and negative linear relations.
	x := []float64{1, 2, 3}
	out, err := CORREL(x, []float64{2, 4, 6}, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "CORREL y=2x", out, []float64{nan, nan, 1}, 1e-12)
	out, _ = CORREL(x, []float64{6, 4, 2}, 3)
	assertReal(t, "CORREL y=-2x", out, []float64{nan, nan, -1}, 1e-12)

	// x = 1,2,3,4; y = 1,3,2,4: deviations (-1.5,-.5,.5,1.5) and
	// (-1.5,.5,-.5,1.5); sp = 4, ssX = ssY = 5, r = 0.8.
	out, _ = CORREL([]float64{1, 2, 3, 4}, []float64{1, 3, 2, 4}, 4)
	assertReal(t, "CORREL hand case", out, []float64{nan, nan, nan, 0.8}, 1e-12)
	// Rolling window of 2 on that pair: each pair of points is perfectly
	// (anti)correlated: (1,1)-(2,3) +, (2,3)-(3,2) -, (3,2)-(4,4) +.
	out, _ = CORREL([]float64{1, 2, 3, 4}, []float64{1, 3, 2, 4}, 2)
	assertReal(t, "CORREL p=2", out, []float64{nan, 1, -1, 1}, 1e-12)

	// A series against itself correlates at 1.
	w := fixtures(300)[0].Real()
	out, _ = CORREL(w, w, 10)
	for i := 9; i < len(out); i++ {
		if math.Abs(out[i]-1) > 1e-9 {
			t.Fatalf("self correl [%d]=%.17g", i, out[i])
		}
	}

	// Tiny spread at a $100 level (#242): perfectly correlated series must
	// still give 1, not 0 or -1.
	a := make([]float64, 200)
	b := make([]float64, 200)
	for i := range a {
		d := 1e-5 * math.Sin(float64(i))
		a[i] = 100 + d
		b[i] = 50 + 2*d
	}
	out, _ = CORREL(a, b, 14)
	for i := 13; i < len(out); i++ {
		if math.Abs(out[i]-1) > 1e-6 {
			t.Fatalf("tiny spread [%d]=%.17g", i, out[i])
		}
	}

	// Constant series: correlation undefined, output 0.
	c := fixtures(40)[5].Real()
	out, _ = CORREL(c, w[:40], 7)
	for i := 6; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant x: [%d]=%g", i, out[i])
		}
	}
	out, _ = CORREL(w[:40], c, 7)
	for i := 6; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant y: [%d]=%g", i, out[i])
		}
	}
	// Period 1: a single point has no spread, so the output is always 0.
	out, _ = CORREL(w, fixtures(300)[1].Real(), 1)
	for i := range out {
		if out[i] != 0 {
			t.Fatalf("period 1: [%d]=%g", i, out[i])
		}
	}
}

func TestCORRELRangeForm(t *testing.T) {
	s := fixtures(30)
	in0, in1 := s[0].Real(), s[1].Real()
	out := make([]float64, len(in0))
	if _, _, err := correlRange(-1, 5, in0, in1, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := correlRange(5, 4, in0, in1, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := correlRange(0, -1, in0, in1, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := correlRange(0, 1, in0, in1, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// First bar after the lookback.
	beg, nb, err = correlRange(0, 2, in0, in1, 3, out)
	if err != nil || beg != 2 || nb != 1 {
		t.Errorf("first bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// In-place computation is allowed with the output aliasing either input.
	want, _ := CORREL(in0, in1, 4)
	buf := append([]float64(nil), in0...)
	beg, nb, _ = correlRange(0, len(buf)-1, buf, in1, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place inReal0", buf, want, 0)
	buf = append([]float64(nil), in1...)
	beg, nb, _ = correlRange(0, len(buf)-1, in0, buf, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place inReal1", buf, want, 0)
}
