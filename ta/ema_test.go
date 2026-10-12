package ta

import (
	"errors"
	"math"
	"testing"
)

func TestEMAParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := EMA(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := EMALookback(p); lb != -1 {
			t.Errorf("EMALookback(%d) = %d, want -1", p, lb)
		}
		out := make([]float64, len(in))
		if _, _, err := emaRange(0, len(in)-1, in, p, out); !errors.Is(err, ErrBadParam) {
			t.Errorf("emaRange period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 30, 100000} {
		if _, err := EMA(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := EMALookback(p); lb != p-1 {
			t.Errorf("EMALookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	if out, err := EMA(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := EMA([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Period 100000 on a short series: all NaN, no error.
	out, err := EMA(in, 100000)
	if err != nil || len(out) != len(in) {
		t.Fatalf("p=100000: len=%d err=%v", len(out), err)
	}
	assertLeadingNaN(t, "EMA p=100000", out, len(in))
}

func TestEMALengths(t *testing.T) {
	for _, period := range []int{1, 2, 3, 5, 30} {
		lb := EMALookback(period)
		if lb != period-1 {
			t.Fatalf("EMALookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := EMA(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("EMA ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestEMAKnownValues(t *testing.T) {
	nan := math.NaN()
	// k = 2/(3+1) = 0.5; seed = SMA(1,2,3) = 2; then 2+(4-2)/2 = 3, ...
	in := []float64{1, 2, 3, 4, 5, 6}
	out, err := EMA(in, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "EMA(1..6,3)", out, []float64{nan, nan, 2, 3, 4, 5}, 0)

	// seed SMA(2,4,6)=4; 4+(8-4)/2=6; 6+(4-6)/2=5.
	out, _ = EMA([]float64{2, 4, 6, 8, 4}, 3)
	assertReal(t, "EMA(2,4,6,8,4;3)", out, []float64{nan, nan, 4, 6, 5}, 0)

	// k = 2/(4+1) = 0.4; seed SMA(10,10,10,10)=10; 10+(20-10)*0.4 = 14.
	out, _ = EMA([]float64{10, 10, 10, 10, 20}, 4)
	assertReal(t, "EMA period 4", out, []float64{nan, nan, nan, 10, 14}, 1e-15)

	// Period 1 is the identity, even across moves larger than 2x (where the
	// k=1 recursion would round).
	id := []float64{0.1, 1234.56, 0.003, -7.25, 1e12, 3.3, 0}
	out, _ = EMA(id, 1)
	assertReal(t, "EMA period 1", out, id, 0)
	for _, s := range fixtures(200) {
		out, _ = EMA(s.Real(), 1)
		assertReal(t, "EMA period 1 "+s.Name, out, s.Real(), 0)
	}

	// A constant series averages to itself.
	c := fixtures(40)[5].Real()
	out, _ = EMA(c, 7)
	for i := 6; i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// An EMA is a convex combination of inputs: it stays within the running
	// [min, max] of the input seen so far (up to rounding).
	for _, s := range fixtures(300) {
		in := s.Real()
		for _, p := range []int{2, 5, 30} {
			out, _ := EMA(in, p)
			lo, hi := math.Inf(1), math.Inf(-1)
			for i, v := range in {
				lo, hi = math.Min(lo, v), math.Max(hi, v)
				if i < p-1 {
					continue
				}
				tol := 1e-12 * math.Max(1, math.Max(math.Abs(lo), math.Abs(hi)))
				if out[i] < lo-tol || out[i] > hi+tol {
					t.Fatalf("%s p=%d [%d]=%g outside [%g,%g]", s.Name, p, i, out[i], lo, hi)
				}
			}
		}
	}

	// Rising input: EMA lags below the input and is itself rising.
	up := fixtures(100)[7].Real()
	out, _ = EMA(up, 10)
	for i := 10; i < len(out); i++ {
		if !(out[i] < up[i]) || !(out[i] > out[i-1]) {
			t.Fatalf("up trend [%d]: ema %g in %g prev %g", i, out[i], up[i], out[i-1])
		}
	}
}

func TestEMAUnstablePeriod(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstEMA, 2); err != nil {
		t.Fatal(err)
	}
	if lb := EMALookback(3); lb != 4 {
		t.Fatalf("EMALookback(3) with unstable 2 = %d, want 4", lb)
	}
	nan := math.NaN()
	// Seed is still SMA(1,2,3)=2 at bar 2; bars 3 and 4 are absorbed, first
	// output at bar 4.
	out, _ := EMA([]float64{1, 2, 3, 4, 5, 6}, 3)
	assertReal(t, "EMA unstable 2", out, []float64{nan, nan, nan, nan, 4, 5}, 0)
	// Period 1 copies the input but still honours the unstable period.
	out, _ = EMA([]float64{1, 2, 3, 4}, 1)
	assertReal(t, "EMA p1 unstable 2", out, []float64{nan, nan, 3, 4}, 0)
	// Unrelated unstable ids do not affect EMA.
	SetAllUnstablePeriods(0)
	SetUnstablePeriod(UnstRSI, 5)
	if lb := EMALookback(3); lb != 2 {
		t.Fatalf("EMALookback(3) with RSI unstable = %d, want 2", lb)
	}
}

func TestEMARangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := emaRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := emaRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := emaRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := emaRange(0, 1, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := emaRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := emaRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// A sub-range equals the matching slice of the full series: EMA re-seeds
	// from the SMA at startIdx-lookback, so it does NOT equal the full-series
	// values in general; check the seed instead.
	beg, nb, err = emaRange(10, 10, in, 3, out)
	if err != nil || beg != 10 || nb != 1 {
		t.Fatalf("single bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if want := (in[8] + in[9] + in[10]) / 3; math.Abs(out[0]-want) > 1e-12*math.Abs(want) {
		t.Errorf("single bar: got %g want %g", out[0], want)
	}
	// In-place computation is allowed.
	for _, p := range []int{1, 4} {
		buf := append([]float64(nil), in...)
		want, _ := EMA(in, p)
		beg, nb, _ = emaRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
