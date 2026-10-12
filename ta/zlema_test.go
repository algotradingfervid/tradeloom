package ta

import (
	"errors"
	"math"
	"testing"
)

func TestZLEMAParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32} {
		if _, err := ZLEMA(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := ZLEMALookback(p); lb != -1 {
			t.Errorf("ZLEMALookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := zlemaRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 100000} {
		if _, err := ZLEMA(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
	}
	if lb := ZLEMALookback(100000); lb != 49999+99999 {
		t.Errorf("ZLEMALookback(100000) = %d", lb)
	}
	if out, err := ZLEMA(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := ZLEMA([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := zlemaRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := zlemaRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
}

func TestZLEMALengths(t *testing.T) {
	for _, period := range []int{1, 2, 3, 4, 5, 30} {
		lb := ZLEMALookback(period)
		if lb != (period-1)/2+period-1 {
			t.Fatalf("ZLEMALookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := ZLEMA(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("ZLEMA ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestZLEMAKnownValues(t *testing.T) {
	nan := math.NaN()
	// Period 3: lag 1, k 0.5, lookback 3. De-lagged d[i] = 2x[i]-x[i-1]:
	// x = 1,2,4,8,16 -> d[1..4] = 3,6,12,24. Seed = (3+6+12)/3 = 7 at bar 3;
	// bar 4 = 7 + 0.5*(24-7) = 15.5.
	out, err := ZLEMA([]float64{1, 2, 4, 8, 16}, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "ZLEMA(1,2,4,8,16;3)", out, []float64{nan, nan, nan, 7, 15.5}, 0)

	// Period 1 is the identity, even with values far apart.
	in := []float64{1, 1e300, -3, 0, 1e-300, 7}
	out, _ = ZLEMA(in, 1)
	assertReal(t, "ZLEMA period 1", out, in, 0)

	// A constant series stays constant.
	c := fixtures(80)[5].Real()
	out, _ = ZLEMA(c, 7)
	for i := ZLEMALookback(7); i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// A linear trend has zero lag once the seed has been forgotten:
	// 2x[i]-x[i-lag] = x[i]+lag*step, so ZLEMA tracks the trend closely.
	up := fixtures(400)[7].Real()
	out, _ = ZLEMA(up, 9)
	if d := math.Abs(out[399] - up[399]); d > 1e-6 {
		t.Errorf("trend: ZLEMA lags by %g", d)
	}

	// ZLEMA is bit-for-bit an EMA over the materialised de-lagged series.
	for _, s := range fixtures(300) {
		for _, p := range []int{2, 3, 6, 14, 30} {
			lag := (p - 1) / 2
			x := s.Real()
			d := make([]float64, len(x)-lag)
			for i := range d {
				d[i] = 2.0*x[i+lag] - x[i]
			}
			ema, _ := EMA(d, p)
			got, _ := ZLEMA(x, p)
			assertReal(t, lbl("ZLEMA==EMA(delag) ", s.Name, " p=", p), got[lag:], ema, 0)
		}
	}
}

func TestZLEMARangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := zlemaRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := zlemaRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := zlemaRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (3 for period 3): no output, no error.
	beg, nb, err := zlemaRange(0, 2, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Start inside the lookback is moved up to it.
	beg, nb, err = zlemaRange(1, 10, in, 3, out)
	if err != nil || beg != 3 || nb != 8 {
		t.Errorf("start in lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// A sub-range matches the full series at the same bars (EMA depends on
	// history, so only the first bar of a later start is re-seeded; check the
	// start-at-lookback case, which is identical).
	full, _ := ZLEMA(in, 3)
	assertReal(t, "sub-range", out[:nb], full[3:11], 0)
	// In-place computation is allowed.
	for _, p := range []int{1, 2, 4, 9} {
		buf := append([]float64(nil), in...)
		want, _ := ZLEMA(in, p)
		beg, nb, _ = zlemaRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}

func TestZLEMAUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	SetUnstablePeriod(UnstEMA, 5)
	if lb := ZLEMALookback(4); lb != 1+3+5 {
		t.Errorf("lookback with EMA unstable 5: %d", lb)
	}
	out, _ := ZLEMA(fixtures(40)[0].Real(), 4)
	assertLeadingNaN(t, "unstable", out, 9)
}
