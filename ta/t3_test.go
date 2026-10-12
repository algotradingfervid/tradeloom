package ta

import (
	"errors"
	"math"
	"testing"
)

func TestT3Params(t *testing.T) {
	in := fixtures(80)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32} {
		if _, err := T3(in, p, 0.7); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := T3Lookback(p, 0.7); lb != -1 {
			t.Errorf("T3Lookback(%d) = %d, want -1", p, lb)
		}
	}
	for _, v := range []float64{-1e-9, -0.01, 1.0000001, 2, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := T3(in, 5, v); !errors.Is(err, ErrBadParam) {
			t.Errorf("vfactor %g: err = %v, want ErrBadParam", v, err)
		}
		if lb := T3Lookback(5, v); lb != -1 {
			t.Errorf("T3Lookback(5, %g) = %d, want -1", v, lb)
		}
	}
	for _, p := range []int{1, 2, 100000} {
		for _, v := range []float64{0, 0.7, 1} {
			if _, err := T3(in, p, v); err != nil {
				t.Errorf("period %d vfactor %g: unexpected err %v", p, v, err)
			}
		}
	}
	if lb := T3Lookback(100000, 1); lb != 6*99999 {
		t.Errorf("T3Lookback(100000) = %d", lb)
	}
	if out, err := T3(nil, 5, 0.7); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := T3([]float64{}, 5, 0.7); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := t3Range(0, 10, in[:5], 3, 0.7, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := t3Range(0, 40, in, 3, 0.7, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := t3Range(0, 40, in, 0, 0.7, make([]float64, 80)); !errors.Is(err, ErrBadParam) {
		t.Errorf("range bad period: err = %v, want ErrBadParam", err)
	}
	if _, _, err := t3Range(0, 40, in, 3, math.NaN(), make([]float64, 80)); !errors.Is(err, ErrBadParam) {
		t.Errorf("range bad vfactor: err = %v, want ErrBadParam", err)
	}
}

func TestT3Lengths(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	for _, unst := range []int{0, 7} {
		SetUnstablePeriod(UnstT3, unst)
		for _, period := range []int{1, 2, 5, 14} {
			for _, v := range []float64{0, 0.7, 1} {
				lb := T3Lookback(period, v)
				if lb != 6*(period-1)+unst {
					t.Fatalf("T3Lookback(%d, %g) = %d (unst %d)", period, v, lb, unst)
				}
				for _, n := range edgeLengths(lb) {
					for _, s := range fixtures(n) {
						out, err := T3(s.Real(), period, v)
						if err != nil {
							t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
						}
						if len(out) != n {
							t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
						}
						assertLeadingNaN(t, lbl("T3 ", s.Name, " n=", n, " p=", period, " v=", v, " u=", unst), out, lb)
					}
				}
			}
		}
	}
}

func TestT3KnownValues(t *testing.T) {
	// Period 2 (k = 2/3), lookback 6. Zeros then a 6 on the last bar:
	// e1..e4 stay 0 through the e5 seeding; the e6 seeding loop consumes the
	// 6: e1=4, e2=8/3, e3=16/9, e4=32/27, e5=64/81, e6=(0+64/81)/2=32/81.
	in := []float64{0, 0, 0, 0, 0, 0, 6}
	nan := math.NaN()
	pre := []float64{nan, nan, nan, nan, nan, nan}

	// vFactor 0: c1=c2=c3=0, c4=1, so T3 = e3.
	out, err := T3(in, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "T3 v=0", out, append(append([]float64{}, pre...), 16.0/9), 1e-15)

	// vFactor 1: c1=-1, c2=6, c3=-12, c4=8:
	// -32/81 + 6*64/81 - 12*32/27 + 8*16/9 = 352/81.
	out, _ = T3(in, 2, 1)
	assertReal(t, "T3 v=1", out, append(append([]float64{}, pre...), 352.0/81), 1e-14)

	// Period 1 is the identity (exact copy), whatever vFactor.
	walk := fixtures(60)[0].Real()
	for _, v := range []float64{0, 0.7, 1} {
		out, _ = T3(walk, 1, v)
		assertReal(t, "T3 period 1", out, walk, 0)
	}

	// A constant series stays constant (coefficients sum to 1 up to rounding).
	c := fixtures(80)[5].Real()
	for _, v := range []float64{0, 0.3, 0.7, 1} {
		out, _ = T3(c, 4, v)
		for i := T3Lookback(4, v); i < len(out); i++ {
			if math.Abs(out[i]-100) > 1e-11 {
				t.Fatalf("constant v=%g: [%d]=%.17g", v, i, out[i])
			}
		}
	}

	// With vFactor 0 and a linear ramp, T3 = EMA3 lags the input but stays
	// below it and keeps rising.
	up := fixtures(100)[7].Real()
	out, _ = T3(up, 5, 0)
	for i := T3Lookback(5, 0) + 1; i < len(out); i++ {
		if !(out[i] > out[i-1] && out[i] < up[i]) {
			t.Fatalf("ramp: [%d]=%g prev %g in %g", i, out[i], out[i-1], up[i])
		}
	}
}

func TestT3RangeForm(t *testing.T) {
	in := fixtures(60)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := t3Range(-1, 5, in, 3, 0.7, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := t3Range(5, 4, in, 3, 0.7, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := t3Range(0, -1, in, 3, 0.7, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (12 for period 3): no output, no error.
	beg, nb, err := t3Range(0, 11, in, 3, 0.7, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// T3 is seeded at startIdx-lookback (unstable), so a sub-range equals a
	// fresh computation over in[startIdx-lookback : endIdx+1], not a slice of
	// the full-series result.
	full, _ := T3(in[20-12:41], 3, 0.7)
	beg, nb, err = t3Range(20, 40, in, 3, 0.7, out)
	if err != nil || beg != 20 || nb != 21 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[12:], 0)
	// In-place computation is allowed.
	for _, p := range []int{1, 4} {
		buf := append([]float64(nil), in...)
		want, _ := T3(in, p, 0.7)
		beg, nb, _ = t3Range(0, len(buf)-1, buf, p, 0.7, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
