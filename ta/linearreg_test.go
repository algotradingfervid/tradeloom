package ta

import (
	"errors"
	"math"
	"testing"
)

func TestLINEARREGParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := LINEARREG(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := LINEARREGLookback(p); lb != -1 {
			t.Errorf("LINEARREGLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := linearregRange(0, len(in)-1, in, p, make([]float64, len(in))); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, err := LINEARREG(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := LINEARREGLookback(p); lb != p-1 {
			t.Errorf("LINEARREGLookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	if out, err := LINEARREG(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := LINEARREG([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Bad parameter is reported even on empty input.
	if _, err := LINEARREG(nil, 1); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input bad period: err = %v, want ErrBadParam", err)
	}
}

func TestLINEARREGLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30} {
		lb := LINEARREGLookback(period)
		if lb != period-1 {
			t.Fatalf("LINEARREGLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := LINEARREG(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("LINEARREG ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestLINEARREGKnownValues(t *testing.T) {
	nan := math.NaN()

	// Fit over x=0,1,2 of y=1,2,4: slope 1.5, mean 7/3, so the line at x=2 is
	// 7/3 + 1.5 = 23/6. Next window y=2,4,4: slope 1, mean 10/3 -> 13/3.
	out, err := LINEARREG([]float64{1, 2, 4, 4}, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "LINEARREG(1,2,4,4;3)", out, []float64{nan, nan, 23.0 / 6, 13.0 / 3}, 1e-12)

	// A line through two points passes through both: period 2 is the identity
	// from bar 1 on.
	in := fixtures(100)[0].Real()
	out, _ = LINEARREG(in, 2)
	assertReal(t, "LINEARREG period 2", out[1:], in[1:], 1e-12)

	// A perfectly linear series is reproduced exactly (up to rounding).
	lin := make([]float64, 200)
	for i := range lin {
		lin[i] = 3 + 0.5*float64(i)
	}
	out, _ = LINEARREG(lin, 14)
	assertReal(t, "LINEARREG linear", out[13:], lin[13:], 1e-12)

	// A constant series regresses to itself.
	c := fixtures(400)[5].Real()
	out, _ = LINEARREG(c, 7)
	for i := 6; i < len(out); i++ {
		if !closeEnough(out[i], 100, 1e-12) {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// All zeros stay zero.
	z := fixtures(100)[6].Real()
	out, _ = LINEARREG(z, 5)
	for i := 4; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("zeros: [%d]=%g", i, out[i])
		}
	}

	// The incremental path and a fresh priming scan agree: output at bar k of
	// a full call equals the first output of a call starting at k.
	w := fixtures(300)[1].Real()
	full, _ := LINEARREG(w, 10)
	for _, k := range []int{9, 10, 57, 150, 299} {
		buf := make([]float64, 1)
		beg, nb, err := linearregRange(k, k, w, 10, buf)
		if err != nil || beg != k || nb != 1 {
			t.Fatalf("single bar %d: beg=%d nb=%d err=%v", k, beg, nb, err)
		}
		if !closeEnough(buf[0], full[k], 1e-9) {
			t.Errorf("bar %d: single %g full %g", k, buf[0], full[k])
		}
	}

	// A large outlier leaving the window triggers a re-anchor; the values
	// after it must match a fresh computation.
	spike := make([]float64, 60)
	for i := range spike {
		spike[i] = 1 + 0.01*float64(i%3)
	}
	spike[10] = 1e12
	out, _ = LINEARREG(spike, 5)
	for k := 15; k < 60; k++ {
		buf := make([]float64, 1)
		linearregRange(k, k, spike, 5, buf)
		if !closeEnough(out[k], buf[0], 1e-9) {
			t.Errorf("after spike bar %d: %g vs fresh %g", k, out[k], buf[0])
		}
	}
}

func TestLINEARREGRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := linearregRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := linearregRange(taMaxIdx+1, taMaxIdx+1, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := linearregRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := linearregRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := linearregRange(0, 1, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := linearregRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := linearregRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Start inside the lookback is clamped.
	beg, nb, err = linearregRange(1, 20, in, 5, out)
	if err != nil || beg != 4 || nb != 17 {
		t.Errorf("clamped: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// In-place computation is allowed, including with a re-anchor.
	long := fixtures(500)[0].Real()
	for _, p := range []int{2, 4, 14} {
		buf := append([]float64(nil), long...)
		want, _ := LINEARREG(long, p)
		beg, nb, _ = linearregRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
