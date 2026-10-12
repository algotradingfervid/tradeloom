package ta

import (
	"errors"
	"math"
	"testing"
)

func TestERIParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{0, -1, 100001, math.MinInt32} {
		if _, _, err := ERI(s.H, s.L, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := ERILookback(p); lb != -1 {
			t.Errorf("ERILookback(%d) = %d, want -1", p, lb)
		}
	}
	for _, p := range []int{1, 2, 13, 100000} {
		if _, _, err := ERI(s.H, s.L, s.C, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
	}
	// Mismatched input lengths.
	if _, _, err := ERI(s.H, s.L[:49], s.C, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: err = %v", err)
	}
	if _, _, err := ERI(s.H, s.L, s.C[:10], 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v", err)
	}
	if _, _, err := ERI(nil, s.L, s.C, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inHigh with data: err = %v", err)
	}
	// nil / empty input -> empty outputs.
	if bu, be, err := ERI(nil, nil, nil, 5); err != nil || len(bu) != 0 || len(be) != 0 {
		t.Errorf("nil input: %v %v %v", bu, be, err)
	}
	if bu, be, err := ERI([]float64{}, []float64{}, []float64{}, 5); err != nil || len(bu) != 0 || len(be) != 0 {
		t.Errorf("empty input: %v %v %v", bu, be, err)
	}
}

func TestERILengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 13, 30} {
		lb := ERILookback(period)
		if lb != period-1 {
			t.Fatalf("ERILookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				bu, be, err := ERI(s.H, s.L, s.C, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(bu) != n || len(be) != n {
					t.Fatalf("%s n=%d p=%d: len %d %d", s.Name, n, period, len(bu), len(be))
				}
				assertLeadingNaN(t, lbl("ERI bull ", s.Name, " n=", n, " p=", period), bu, lb)
				assertLeadingNaN(t, lbl("ERI bear ", s.Name, " n=", n, " p=", period), be, lb)
			}
		}
	}
}

func TestERIKnownValues(t *testing.T) {
	// Period 3: k = 0.5, seed = mean(c[0..2]).
	h := []float64{11, 13, 15, 17, 19}
	l := []float64{9, 11, 13, 15, 17}
	c := []float64{10, 12, 14, 16, 18}
	bu, be, err := ERI(h, l, c, 3)
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	// EMA: [2]=12, [3]=12+0.5*(16-12)=14, [4]=14+0.5*(18-14)=16.
	assertReal(t, "ERI bull", bu, []float64{nan, nan, 3, 3, 3}, 0)
	assertReal(t, "ERI bear", be, []float64{nan, nan, 1, 1, 1}, 0)

	// Period 1: EMA is the close itself.
	s := fixtures(60)[0]
	bu, be, _ = ERI(s.H, s.L, s.C, 1)
	for i := range s.C {
		if bu[i] != s.H[i]-s.C[i] || be[i] != s.L[i]-s.C[i] {
			t.Fatalf("period 1 [%d]: %g %g", i, bu[i], be[i])
		}
	}

	// Equals High/Low minus the Go EMA bit for bit, and bull >= bear.
	for _, p := range []int{2, 5, 13, 30} {
		for _, s := range fixtures(200) {
			bu, be, _ := ERI(s.H, s.L, s.C, p)
			ema, _ := EMA(s.C, p)
			for i := p - 1; i < len(s.C); i++ {
				if bu[i] != s.H[i]-ema[i] || be[i] != s.L[i]-ema[i] {
					t.Fatalf("%s p=%d [%d]: bull %g bear %g ema %g", s.Name, p, i, bu[i], be[i], ema[i])
				}
				if bu[i] < be[i] {
					t.Fatalf("%s p=%d [%d]: bull %g < bear %g", s.Name, p, i, bu[i], be[i])
				}
			}
		}
	}

	// Constant series: EMA equals the constant, both powers are 0.
	cs := fixtures(40)[5]
	bu, be, _ = ERI(cs.H, cs.L, cs.C, 7)
	for i := 6; i < len(bu); i++ {
		if bu[i] != 0 || be[i] != 0 {
			t.Fatalf("constant [%d]: %g %g", i, bu[i], be[i])
		}
	}
}

func TestERIUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	SetUnstablePeriod(UnstEMA, 4)
	if lb := ERILookback(5); lb != 8 {
		t.Fatalf("lookback with unstable 4: %d", lb)
	}
	s := fixtures(50)[0]
	for _, p := range []int{1, 5} {
		bu, be, _ := ERI(s.H, s.L, s.C, p)
		assertLeadingNaN(t, lbl("unst bull p=", p), bu, p-1+4)
		assertLeadingNaN(t, lbl("unst bear p=", p), be, p-1+4)
	}
}

func TestERIRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	bu := make([]float64, 30)
	be := make([]float64, 30)
	if _, _, err := eriRange(-1, 5, s.H, s.L, s.C, 3, bu, be); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := eriRange(5, 4, s.H, s.L, s.C, 3, bu, be); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := eriRange(0, -1, s.H, s.L, s.C, 3, bu, be); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := eriRange(0, 5, s.H, s.L, s.C, 0, bu, be); !errors.Is(err, ErrBadParam) {
		t.Errorf("period 0: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := eriRange(0, 1, s.H, s.L, s.C, 3, bu, be)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := eriRange(0, 10, s.H, s.L[:5], s.C, 3, bu, be); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: %v", err)
	}
	if _, _, err := eriRange(0, 29, s.H, s.L, s.C, 3, bu, be[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: %v", err)
	}
	// Same output buffer twice: rejected like C.
	if _, _, err := eriRange(0, 29, s.H, s.L, s.C, 3, bu, bu); !errors.Is(err, ErrBadParam) {
		t.Errorf("aliased outputs: %v", err)
	}
	// Outputs may alias inputs.
	wantBu, wantBe, _ := ERI(s.H, s.L, s.C, 4)
	h := append([]float64(nil), s.H...)
	l := append([]float64(nil), s.L...)
	c := append([]float64(nil), s.C...)
	beg, nb, _ = eriRange(0, 29, h, l, c, 4, h, l)
	alignReal(h, beg, nb)
	alignReal(l, beg, nb)
	assertReal(t, "in-place bull", h, wantBu, 0)
	assertReal(t, "in-place bear", l, wantBe, 0)
	// Sub-range: like C, the EMA is re-seeded at startIdx-lookback, so the
	// values equal a full computation over the slice starting there.
	beg, nb, err = eriRange(10, 20, s.H, s.L, s.C, 4, bu, be)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	sub, subBe, _ := ERI(s.H[7:21], s.L[7:21], s.C[7:21], 4)
	assertReal(t, "sub-range bull", bu[:nb], sub[3:], 0)
	assertReal(t, "sub-range bear", be[:nb], subBe[3:], 0)
}
