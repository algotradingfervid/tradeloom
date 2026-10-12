package ta

import (
	"errors"
	"math"
	"testing"
)

func TestSTDDEVParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := STDDEV(in, p, 1); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := STDDEVLookback(p, 1); lb != -1 {
			t.Errorf("STDDEVLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := stddevRange(0, 49, in, p, 1, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("stddevRange period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, d := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 3.0000001e37, -3.0000001e37, -4e37} {
		if _, err := STDDEV(in, 5, d); !errors.Is(err, ErrBadParam) {
			t.Errorf("nbDev %g: err = %v, want ErrBadParam", d, err)
		}
		if lb := STDDEVLookback(5, d); lb != -1 {
			t.Errorf("STDDEVLookback(5, %g) = %d, want -1", d, lb)
		}
		if _, _, err := stddevRange(0, 49, in, 5, d, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("stddevRange nbDev %g: err = %v, want ErrBadParam", d, err)
		}
	}
	for _, p := range []int{2, 3, 100000} {
		if _, err := STDDEV(in, p, 1); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := STDDEVLookback(p, 1); lb != p-1 {
			t.Errorf("STDDEVLookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	for _, d := range []float64{-3e37, 3e37, 0, -2, 2} {
		if _, err := STDDEV(in, 5, d); err != nil {
			t.Errorf("nbDev %g: unexpected err %v", d, err)
		}
		if lb := STDDEVLookback(5, d); lb != 4 {
			t.Errorf("STDDEVLookback(5, %g) = %d, want 4", d, lb)
		}
	}
	if out, err := STDDEV(nil, 5, 1); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := STDDEV([]float64{}, 5, 1); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := stddevRange(0, 10, in[:5], 3, 1, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := stddevRange(0, 10, in, 3, 1, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
}

func TestSTDDEVLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 30} {
		for _, d := range []float64{1, -2} {
			lb := STDDEVLookback(period, d)
			if lb != period-1 {
				t.Fatalf("STDDEVLookback(%d) = %d", period, lb)
			}
			for _, n := range edgeLengths(lb) {
				for _, s := range fixtures(n) {
					out, err := STDDEV(s.Real(), period, d)
					if err != nil {
						t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
					}
					if len(out) != n {
						t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
					}
					assertLeadingNaN(t, lbl("STDDEV ", s.Name, " n=", n, " p=", period, " d=", d), out, lb)
				}
			}
		}
	}
}

func TestSTDDEVKnownValues(t *testing.T) {
	nan := math.NaN()
	// {2,4,4,4,5,5,7,9}: population variance 4, standard deviation 2.
	in := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	out, err := STDDEV(in, 8, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "STDDEV classic", out, []float64{nan, nan, nan, nan, nan, nan, nan, 2}, 1e-12)
	out, _ = STDDEV(in, 8, 2.5)
	assertReal(t, "STDDEV classic x2.5", out, []float64{nan, nan, nan, nan, nan, nan, nan, 5}, 1e-12)
	out, _ = STDDEV(in, 8, -1)
	assertReal(t, "STDDEV classic x-1", out, []float64{nan, nan, nan, nan, nan, nan, nan, -2}, 1e-12)

	// Period 2 on {k, k+2}: deviations +-1 -> stddev 1.
	out, _ = STDDEV([]float64{1, 3, 5, 3, 1}, 2, 1)
	assertReal(t, "STDDEV p=2", out, []float64{nan, 1, 1, 1, 1}, 1e-12)

	// A constant series has zero deviation.
	out, _ = STDDEV(fixtures(40)[5].Real(), 7, 1)
	for i := 6; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// STDDEV == sqrt(VAR) * nbDev, and is non-negative when nbDev >= 0.
	for _, s := range fixtures(500) {
		for _, p := range []int{2, 5, 30} {
			v, _ := VAR(s.Real(), p, 1)
			sd, _ := STDDEV(s.Real(), p, 1)
			sd2, _ := STDDEV(s.Real(), p, 2)
			for i := p - 1; i < len(sd); i++ {
				if !(sd[i] >= 0) || sd[i] != math.Sqrt(v[i]) || sd2[i] != math.Sqrt(v[i])*2 {
					t.Fatalf("%s p=%d: [%d] sd=%g sd2=%g var=%g", s.Name, p, i, sd[i], sd2[i], v[i])
				}
			}
		}
	}

	// nbDev 0 zeroes every output.
	out, _ = STDDEV(fixtures(60)[0].Real(), 5, 0)
	for i := 4; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("nbDev 0: [%d]=%g", i, out[i])
		}
	}

	// Tiny-tick series is not suppressed to 0 (no TA_EPSILON cliff).
	tiny := []float64{100, 100 + 1e-8, 100, 100 + 1e-8, 100}
	out, _ = STDDEV(tiny, 2, 1)
	for i := 1; i < len(out); i++ {
		if !(out[i] > 0) {
			t.Fatalf("tiny ticks: [%d]=%g, want > 0", i, out[i])
		}
	}
}

func TestSTDDEVRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := stddevRange(-1, 5, in, 3, 1, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := stddevRange(5, 4, in, 3, 1, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := stddevRange(0, -1, in, 3, 1, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := stddevRange(0, 1, in, 3, 1, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Start inside the lookback is moved up to it.
	beg, nb, err = stddevRange(1, 10, in, 3, 1, out)
	if err != nil || beg != 2 || nb != 9 {
		t.Errorf("start in lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// In-place computation is allowed.
	long := fixtures(1000)[4].Real()
	buf := append([]float64(nil), long...)
	want, _ := STDDEV(long, 4, 1.5)
	beg, nb, _ = stddevRange(0, len(buf)-1, buf, 4, 1.5, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
