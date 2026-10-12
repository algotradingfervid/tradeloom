package ta

import (
	"errors"
	"math"
	"testing"
)

func TestCVIParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := CVI(s.H, s.L, p, 10); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := CVILookback(p, 10); lb != -1 {
			t.Errorf("CVILookback(%d,10) = %d, want -1", p, lb)
		}
	}
	for _, p := range []int{0, -1, 100001, math.MinInt32} {
		if _, err := CVI(s.H, s.L, 10, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("roc %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := CVILookback(10, p); lb != -1 {
			t.Errorf("CVILookback(10,%d) = %d, want -1", p, lb)
		}
	}
	for _, pp := range [][2]int{{2, 1}, {2, 100000}, {100000, 1}, {100000, 100000}, {10, 10}} {
		if _, err := CVI(s.H, s.L, pp[0], pp[1]); err != nil {
			t.Errorf("params %v: unexpected err %v", pp, err)
		}
		if lb := CVILookback(pp[0], pp[1]); lb != pp[0]-1+pp[1] {
			t.Errorf("CVILookback%v = %d", pp, lb)
		}
	}
	if _, err := CVI(s.H, s.L[:49], 10, 10); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, err := CVI(s.H, nil, 10, 10); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil low with non-empty high: err = %v, want ErrBadParam", err)
	}
	if out, err := CVI(nil, nil, 10, 10); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CVI([]float64{}, []float64{}, 10, 10); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestCVILengths(t *testing.T) {
	for _, pp := range [][2]int{{2, 1}, {2, 5}, {5, 1}, {10, 10}, {14, 40}, {30, 3}} {
		lb := CVILookback(pp[0], pp[1])
		if lb != pp[0]-1+pp[1] {
			t.Fatalf("CVILookback%v = %d", pp, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := CVI(s.H, s.L, pp[0], pp[1])
				if err != nil {
					t.Fatalf("%s n=%d p=%v: %v", s.Name, n, pp, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%v: len(out)=%d", s.Name, n, pp, len(out))
				}
				assertLeadingNaN(t, lbl("CVI ", s.Name, " n=", n, " p=", pp), out, lb)
			}
		}
	}
}

func TestCVIKnownValues(t *testing.T) {
	// Spreads 1..6. EMA(2): k = 2/3, seed (1+2)/2 = 1.5 at bar 1, then
	// 2.5, 3.5, 4.5, 5.5. ROC period 1: 100*(E[t]-E[t-1])/E[t-1].
	hi := []float64{11, 12, 13, 14, 15, 16}
	lo := []float64{10, 10, 10, 10, 10, 10}
	out, err := CVI(hi, lo, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	assertReal(t, "CVI known", out, []float64{nan, nan, 100.0 / 1.5, 100.0 / 2.5, 100.0 / 3.5, 100.0 / 4.5}, 1e-12)

	// Constant spread: the EMA is constant, so CVI is 0.
	s := fixtures(60)[9] // zigzag: high-low = 7 on every bar
	out, _ = CVI(s.H, s.L, 5, 3)
	for i := 7; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant spread: [%d]=%g", i, out[i])
		}
	}
	// All flat bars: the lagged EMA is exactly 0, and the guard yields 0.
	c := fixtures(40)[5]
	out, _ = CVI(c.H, c.L, 4, 2)
	for i := 5; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("flat bars: [%d]=%g", i, out[i])
		}
	}
	// The guard is an exact zero test, not an epsilon band: tiny spreads
	// still produce a non-zero reading.
	tiny := fixtures(80)[3]
	out, _ = CVI(tiny.H, tiny.L, 5, 5)
	nonZero := false
	for _, v := range out[9:] {
		if v != 0 {
			nonZero = true
		}
	}
	if !nonZero {
		t.Error("tiny prices: CVI is identically 0")
	}
	// Bounds: CVI > -100 whenever the spread is non-negative and the lagged
	// EMA is positive.
	for _, s := range fixtures(300) {
		out, _ = CVI(s.H, s.L, 10, 10)
		for i := 19; i < len(out); i++ {
			if out[i] < -100 {
				t.Fatalf("%s: [%d]=%g below -100", s.Name, i, out[i])
			}
		}
	}
}

// TestCVIComposition checks the fused pass against TA_SUB + TA_EMA + TA_ROCP
// composed from emaRange and rocpRange, which TA-Lib holds to bit equality.
func TestCVIComposition(t *testing.T) {
	for _, s := range fixtures(200) {
		for _, pp := range [][2]int{{2, 1}, {5, 3}, {10, 10}, {14, 40}} {
			n := len(s.H)
			hl := make([]float64, n)
			for i := range hl {
				hl[i] = s.H[i] - s.L[i]
			}
			ema := make([]float64, n)
			eb, en, err := emaRange(0, n-1, hl, pp[0], ema)
			if err != nil {
				t.Fatal(err)
			}
			roc := make([]float64, n)
			rb, rn, err := rocpRange(0, en-1, ema[:en], pp[1], roc)
			if err != nil {
				t.Fatal(err)
			}
			want := make([]float64, n)
			for i := range want {
				want[i] = math.NaN()
			}
			for i := 0; i < rn; i++ {
				want[eb+rb+i] = 100.0 * roc[i]
			}
			got, _ := CVI(s.H, s.L, pp[0], pp[1])
			assertReal(t, lbl("composition ", s.Name, " p=", pp), got, want, 0)
		}
	}
}

func TestCVIRangeForm(t *testing.T) {
	s := fixtures(40)[0]
	out := make([]float64, len(s.H))
	if _, _, err := cviRange(-1, 5, s.H, s.L, 3, 2, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cviRange(5, 4, s.H, s.L, 3, 2, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cviRange(0, 5, s.H, s.L, 1, 2, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("period 1: %v", err)
	}
	if _, _, err := cviRange(0, 5, s.H, s.L, 3, 0, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("roc 0: %v", err)
	}
	// Range entirely inside the lookback (4): no output, no error.
	beg, nb, err := cviRange(0, 3, s.H, s.L, 3, 2, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if _, _, err := cviRange(0, 20, s.H, s.L[:10], 3, 2, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: %v", err)
	}
	if _, _, err := cviRange(0, 20, s.H, s.L, 3, 2, out[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: %v", err)
	}
	// Like TA_EMA, a sub-range re-seeds the EMA lookback (4) bars before
	// startIdx, so it equals the full computation over inputs [6, 30].
	full, _ := CVI(s.H, s.L, 3, 2)
	sub, _ := CVI(s.H[6:31], s.L[6:31], 3, 2)
	beg, nb, err = cviRange(10, 30, s.H, s.L, 3, 2, out)
	if err != nil || beg != 10 || nb != 21 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], sub[4:], 0)
	// In-place: output may alias an input.
	h := append([]float64(nil), s.H...)
	beg, nb, _ = cviRange(0, len(h)-1, h, s.L, 3, 2, h)
	alignReal(h, beg, nb)
	assertReal(t, "in-place", h, full, 0)
}

func TestCVIUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstEMA, 5); err != nil {
		t.Fatal(err)
	}
	if lb := CVILookback(10, 10); lb != 9+5+10 {
		t.Fatalf("lookback with unstable 5: %d", lb)
	}
	s := fixtures(100)[0]
	out, _ := CVI(s.H, s.L, 10, 10)
	assertLeadingNaN(t, "unstable", out, 24)
}
