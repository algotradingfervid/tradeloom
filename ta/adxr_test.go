package ta

import (
	"errors"
	"math"
	"testing"
)

func TestADXRParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := ADXR(s.H, s.L, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := ADXRLookback(p); lb != -1 {
			t.Errorf("ADXRLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := adxrRange(0, 49, s.H, s.L, s.C, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, err := ADXR(s.H, s.L, s.C, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := ADXRLookback(p); lb != 3*p-2 {
			t.Errorf("ADXRLookback(%d) = %d, want %d", p, lb, 3*p-2)
		}
	}
	// Mismatched lengths.
	if _, err := ADXR(s.H, s.L[:49], s.C, 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: err = %v", err)
	}
	if _, err := ADXR(s.H[:10], s.L, s.C, 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v", err)
	}
	if _, err := ADXR(s.H, s.L, nil, 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inClose with others set: err = %v", err)
	}
	// nil / empty input.
	if out, err := ADXR(nil, nil, nil, 14); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := ADXR([]float64{}, []float64{}, []float64{}, 14); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestADXRLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30} {
		lb := ADXRLookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := ADXR(s.H, s.L, s.C, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("ADXR ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestADXRKnownValues(t *testing.T) {
	// Period 2, lookback 4. ADX (hand-derived in TestADXKnownValues) is 100
	// at bar 3 and 1200/19 at bar 4, so
	// ADXR[4] = (ADX[4] + ADX[3]) / 2 = (1200/19 + 100) / 2 = 1550/19.
	h := []float64{10, 12, 14, 16, 15}
	l := []float64{8, 10, 12, 14, 11}
	c := []float64{9, 11, 13, 15, 12}
	out, err := ADXR(h, l, c, 2)
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	assertReal(t, "ADXR hand", out, []float64{nan, nan, nan, nan, 1550.0 / 19.0}, 1e-13)
	// Formula: ADXR[i] = (ADX[i] + ADX[i-(p-1)]) / 2.
	for _, p := range []int{2, 5, 14} {
		s := fixtures(200)[0]
		a, _ := ADX(s.H, s.L, s.C, p)
		r, _ := ADXR(s.H, s.L, s.C, p)
		for i := ADXRLookback(p); i < len(r); i++ {
			want := (a[i] + a[i-(p-1)]) / 2
			if r[i] != want {
				t.Fatalf("p=%d [%d]=%g, want %g", p, i, r[i], want)
			}
		}
	}

	// A strictly rising market: ADX is 100, so ADXR is 100.
	up := fixtures(80)[7]
	out, _ = ADXR(up.H, up.L, up.C, 14)
	for i := ADXRLookback(14); i < len(out); i++ {
		if math.Abs(out[i]-100) > 1e-9 {
			t.Fatalf("up trend: [%d]=%g, want 100", i, out[i])
		}
	}

	// Constant and all-zero series: ADX stays 0, so ADXR is 0.
	for _, s := range []series{fixtures(60)[5], fixtures(60)[6]} {
		out, _ = ADXR(s.H, s.L, s.C, 7)
		for i := ADXRLookback(7); i < len(out); i++ {
			if out[i] != 0 {
				t.Fatalf("%s: [%d]=%g, want 0", s.Name, i, out[i])
			}
		}
	}

	// Bounds: ADXR is in [0, 100] on every fixture.
	for _, s := range fixtures(300) {
		for _, p := range []int{2, 14, 50} {
			out, _ = ADXR(s.H, s.L, s.C, p)
			for i, v := range out[ADXRLookback(p):] {
				if v < 0 || v > 100+1e-9 {
					t.Fatalf("%s p=%d: [%d]=%g out of [0,100]", s.Name, p, i, v)
				}
			}
		}
	}
}

func TestADXRRangeForm(t *testing.T) {
	s := fixtures(60)[0]
	out := make([]float64, 60)
	if _, _, err := adxrRange(-1, 5, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := adxrRange(5, 4, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := adxrRange(0, -1, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (7 for period 3): no output, no error.
	beg, nb, err := adxrRange(0, 6, s.H, s.L, s.C, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := adxrRange(0, 59, s.H, s.L[:30], s.C, 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: %v", err)
	}
	if _, _, err := adxrRange(0, 59, s.H, s.L, s.C, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outReal: %v", err)
	}
	// A range starting inside the lookback is clamped to it and matches the
	// full series.
	want, _ := ADXR(s.H, s.L, s.C, 3)
	beg, nb, err = adxrRange(2, 20, s.H, s.L, s.C, 3, out)
	if err != nil || beg != 7 || nb != 14 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], want[7:21], 0)
	// Single bar at the lookback.
	beg, nb, err = adxrRange(7, 7, s.H, s.L, s.C, 3, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != want[7] {
		t.Fatalf("single bar: beg=%d nb=%d err=%v v=%g want %g", beg, nb, err, out[0], want[7])
	}
	// In-place: output may alias an input.
	c := append([]float64(nil), s.C...)
	beg, nb, _ = adxrRange(0, 59, s.H, s.L, c, 3, c)
	alignReal(c, beg, nb)
	assertReal(t, "in-place", c, want, 0)
}

func TestADXRUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstADX, 5); err != nil {
		t.Fatal(err)
	}
	if lb := ADXRLookback(14); lb != 3*14-2+5 {
		t.Fatalf("lookback with unstable 5: %d", lb)
	}
	s := fixtures(100)[0]
	out, _ := ADXR(s.H, s.L, s.C, 14)
	assertLeadingNaN(t, "unstable", out, 45)
}
