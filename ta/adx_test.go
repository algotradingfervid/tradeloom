package ta

import (
	"errors"
	"math"
	"testing"
)

func TestADXParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := ADX(s.H, s.L, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := ADXLookback(p); lb != -1 {
			t.Errorf("ADXLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := adxRange(0, 49, s.H, s.L, s.C, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, err := ADX(s.H, s.L, s.C, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := ADXLookback(p); lb != 2*p-1 {
			t.Errorf("ADXLookback(%d) = %d, want %d", p, lb, 2*p-1)
		}
	}
	// Mismatched lengths.
	if _, err := ADX(s.H, s.L[:49], s.C, 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: err = %v", err)
	}
	if _, err := ADX(s.H, s.L, s.C[:10], 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v", err)
	}
	if _, err := ADX(nil, s.L, s.C, 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inHigh with others set: err = %v", err)
	}
	// nil / empty input.
	if out, err := ADX(nil, nil, nil, 14); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := ADX([]float64{}, []float64{}, []float64{}, 14); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestADXLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30} {
		lb := ADXLookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := ADX(s.H, s.L, s.C, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("ADX ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestADXKnownValues(t *testing.T) {
	// Period 2, lookback 3. Hand-derived:
	//  bar1: +DM1=2, TR1=3                  -> +DM=2,  TR=3
	//  bar2: +DM=2-1+2=3,  TR=3-1.5+3=4.5   -> DX=100
	//  bar3: +DM=3-1.5+2=3.5, TR=4.5-2.25+3=5.25 -> DX=100; ADX=(100+100)/2=100
	//  bar4: -DM1=3, +DM=1.75, -DM=3, TR=5.25-2.625+4=6.625
	//        DX=100*1.25/4.75=500/19; ADX=(100*1+500/19)/2=1200/19
	h := []float64{10, 12, 14, 16, 15}
	l := []float64{8, 10, 12, 14, 11}
	c := []float64{9, 11, 13, 15, 12}
	out, err := ADX(h, l, c, 2)
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	assertReal(t, "ADX hand", out, []float64{nan, nan, nan, 100, 1200.0 / 19.0}, 1e-13)

	// A strictly rising market has only +DM: ADX is 100.
	up := fixtures(80)[7]
	out, _ = ADX(up.H, up.L, up.C, 14)
	for i := ADXLookback(14); i < len(out); i++ {
		if math.Abs(out[i]-100) > 1e-9 {
			t.Fatalf("up trend: [%d]=%g, want 100", i, out[i])
		}
	}

	// Constant and all-zero series: TR is zero, DX is skipped, ADX stays 0.
	for _, s := range []series{fixtures(60)[5], fixtures(60)[6]} {
		out, _ = ADX(s.H, s.L, s.C, 7)
		for i := ADXLookback(7); i < len(out); i++ {
			if out[i] != 0 {
				t.Fatalf("%s: [%d]=%g, want 0", s.Name, i, out[i])
			}
		}
	}

	// Bounds: ADX is in [0, 100] on every fixture.
	for _, s := range fixtures(300) {
		for _, p := range []int{2, 14, 50} {
			out, _ = ADX(s.H, s.L, s.C, p)
			for i, v := range out[ADXLookback(p):] {
				if v < 0 || v > 100+1e-9 {
					t.Fatalf("%s p=%d: [%d]=%g out of [0,100]", s.Name, p, i, v)
				}
			}
		}
	}
}

func TestADXRangeForm(t *testing.T) {
	s := fixtures(60)[0]
	out := make([]float64, 60)
	if _, _, err := adxRange(-1, 5, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := adxRange(5, 4, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := adxRange(0, -1, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (5 for period 3): no output, no error.
	beg, nb, err := adxRange(0, 4, s.H, s.L, s.C, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := adxRange(0, 59, s.H, s.L[:30], s.C, 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: %v", err)
	}
	if _, _, err := adxRange(0, 59, s.H, s.L, s.C, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outReal: %v", err)
	}
	// A range starting inside the lookback is clamped to it and matches the
	// full series. (Like C, a range starting later re-seeds the smoothing at
	// startIdx-lookback, so it does not equal a slice of the full series;
	// the oracle tests cover that case.)
	want, _ := ADX(s.H, s.L, s.C, 3)
	beg, nb, err = adxRange(2, 20, s.H, s.L, s.C, 3, out)
	if err != nil || beg != 5 || nb != 16 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], want[5:21], 0)
	// In-place: output may alias an input.
	c := append([]float64(nil), s.C...)
	beg, nb, _ = adxRange(0, 59, s.H, s.L, c, 3, c)
	alignReal(c, beg, nb)
	assertReal(t, "in-place", c, want, 0)
}

func TestADXUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstADX, 5); err != nil {
		t.Fatal(err)
	}
	if lb := ADXLookback(14); lb != 2*14-1+5 {
		t.Fatalf("lookback with unstable 5: %d", lb)
	}
	s := fixtures(100)[0]
	out, _ := ADX(s.H, s.L, s.C, 14)
	assertLeadingNaN(t, "unstable", out, 32)
}
