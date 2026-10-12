package ta

import (
	"errors"
	"math"
	"testing"
)

func TestTSFParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := TSF(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := TSFLookback(p); lb != -1 {
			t.Errorf("TSFLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := tsfRange(0, len(in)-1, in, p, make([]float64, len(in))); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, err := TSF(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := TSFLookback(p); lb != p-1 {
			t.Errorf("TSFLookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	if out, err := TSF(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := TSF([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Bad parameter is reported even on empty input.
	if _, err := TSF(nil, 1); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input bad period: err = %v, want ErrBadParam", err)
	}
}

func TestTSFLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30} {
		lb := TSFLookback(period)
		if lb != period-1 {
			t.Fatalf("TSFLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := TSF(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("TSF ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestTSFKnownValues(t *testing.T) {
	nan := math.NaN()

	// Fit over x=0,1,2 of y=1,2,4: slope 1.5, mean 7/3 at x=1, so the
	// forecast at x=3 is 7/3 + 2*1.5 = 16/3. Next window y=2,4,4: slope 1,
	// mean 10/3 -> 10/3 + 2 = 16/3.
	out, err := TSF([]float64{1, 2, 4, 4}, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "TSF(1,2,4,4;3)", out, []float64{nan, nan, 16.0 / 3, 16.0 / 3}, 1e-12)

	// Period 2 extrapolates the line through the last two points:
	// 2*y[i] - y[i-1].
	in := fixtures(100)[0].Real()
	out, _ = TSF(in, 2)
	for i := 1; i < len(in); i++ {
		if want := 2*in[i] - in[i-1]; !closeEnough(out[i], want, 1e-12) {
			t.Fatalf("period 2: [%d]=%g want %g", i, out[i], want)
		}
	}

	// A perfectly linear series is forecast exactly: the value at bar i is
	// the series value at bar i+1.
	lin := make([]float64, 200)
	for i := range lin {
		lin[i] = 3 + 0.5*float64(i)
	}
	out, _ = TSF(lin, 14)
	for i := 13; i < len(lin); i++ {
		if want := 3 + 0.5*float64(i+1); !closeEnough(out[i], want, 1e-12) {
			t.Fatalf("linear: [%d]=%g want %g", i, out[i], want)
		}
	}

	// A constant series forecasts itself.
	c := fixtures(400)[5].Real()
	out, _ = TSF(c, 7)
	for i := 6; i < len(out); i++ {
		if !closeEnough(out[i], 100, 1e-12) {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// All zeros stay zero.
	z := fixtures(100)[6].Real()
	out, _ = TSF(z, 5)
	for i := 4; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("zeros: [%d]=%g", i, out[i])
		}
	}

	// The incremental path and a fresh priming scan agree: output at bar k of
	// a full call equals the first output of a call starting at k.
	w := fixtures(300)[1].Real()
	full, _ := TSF(w, 10)
	for _, k := range []int{9, 10, 57, 150, 299} {
		buf := make([]float64, 1)
		beg, nb, err := tsfRange(k, k, w, 10, buf)
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
	out, _ = TSF(spike, 5)
	for k := 15; k < 60; k++ {
		buf := make([]float64, 1)
		tsfRange(k, k, spike, 5, buf)
		if !closeEnough(out[k], buf[0], 1e-9) {
			t.Errorf("after spike bar %d: %g vs fresh %g", k, out[k], buf[0])
		}
	}
}

func TestTSFRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := tsfRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := tsfRange(taMaxIdx+1, taMaxIdx+1, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := tsfRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := tsfRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := tsfRange(0, 1, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := tsfRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := tsfRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Start inside the lookback is clamped.
	beg, nb, err = tsfRange(1, 20, in, 5, out)
	if err != nil || beg != 4 || nb != 17 {
		t.Errorf("clamped: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// In-place computation is allowed, including with a re-anchor.
	long := fixtures(500)[0].Real()
	for _, p := range []int{2, 4, 14} {
		buf := append([]float64(nil), long...)
		want, _ := TSF(long, p)
		beg, nb, _ = tsfRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
