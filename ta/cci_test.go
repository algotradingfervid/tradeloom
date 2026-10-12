package ta

import (
	"errors"
	"math"
	"testing"
)

func TestCCIParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := CCI(s.H, s.L, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := CCILookback(p); lb != -1 {
			t.Errorf("CCILookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := cciRange(0, 49, s.H, s.L, s.C, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, err := CCI(s.H, s.L, s.C, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := CCILookback(p); lb != p-1 {
			t.Errorf("CCILookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	// Mismatched input lengths.
	if _, err := CCI(s.H, s.L[:49], s.C, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched low: err = %v", err)
	}
	if _, err := CCI(s.H, s.L, s.C[:10], 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched close: err = %v", err)
	}
	if _, err := CCI(nil, s.L, s.C, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil high with non-empty others: err = %v", err)
	}
	// nil / empty input -> empty output.
	if out, err := CCI(nil, nil, nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CCI([]float64{}, []float64{}, []float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestCCILengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30, 31, 50} {
		lb := CCILookback(period)
		if lb != period-1 {
			t.Fatalf("CCILookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := CCI(s.H, s.L, s.C, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("CCI ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestCCIKnownValues(t *testing.T) {
	nan := math.NaN()
	// H == L == C, so TP == close. Window {1,2,3}: avg 2, meanDev 2/3,
	// CCI = (3-2)/(0.015*2/3) = 100. Every later window of a unit-step ramp
	// is the same shape.
	up := []float64{1, 2, 3, 4, 5}
	out, err := CCI(up, up, up, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "CCI ramp up", out, []float64{nan, nan, 100, 100, 100}, 1e-12)

	down := []float64{5, 4, 3, 2, 1}
	out, _ = CCI(down, down, down, 3)
	assertReal(t, "CCI ramp down", out, []float64{nan, nan, -100, -100, -100}, 1e-12)

	// Typical price from distinct H/L/C: TP = {2, 4, 6}, window avg 4,
	// meanDev 4/3, CCI = 2/(0.015*4/3) = 100.
	h := []float64{3, 5, 9}
	l := []float64{1, 3, 3}
	c := []float64{2, 4, 6}
	out, _ = CCI(h, l, c, 3)
	assertReal(t, "CCI typical price", out, []float64{nan, nan, 100}, 1e-12)

	// Last value equal to the average (window {1,3,2}) -> 0 even though the
	// deviation is non-zero.
	m := []float64{1, 3, 2}
	out, _ = CCI(m, m, m, 3)
	assertReal(t, "CCI last==avg", out, []float64{nan, nan, 0}, 0)

	// Period 2: window {a, b} gives (b-avg)/(0.015*|b-a|/2) = ±1/0.015.
	two := []float64{10, 12, 11}
	out, _ = CCI(two, two, two, 2)
	assertReal(t, "CCI period 2", out, []float64{nan, 1 / 0.015, -1 / 0.015}, 1e-12)

	// A constant series (and an all-zero one) is flat: CCI is 0, not NaN.
	for _, idx := range []int{5, 6} {
		s := fixtures(40)[idx]
		out, _ = CCI(s.H, s.L, s.C, 7)
		for i := 6; i < len(out); i++ {
			if out[i] != 0 {
				t.Fatalf("%s: [%d]=%g, want 0", s.Name, i, out[i])
			}
		}
	}

	// A low-priced instrument is not mistaken for flat (TA-Lib #253): the
	// zero tests are relative to the price level.
	tiny := make([]float64, len(up))
	for i, v := range up {
		tiny[i] = v * 1e-9
	}
	out, _ = CCI(tiny, tiny, tiny, 3)
	assertReal(t, "CCI tiny ramp", out, []float64{nan, nan, 100, 100, 100}, 1e-9)

	// Bound: |TP - avg| <= N/2 * meanDev, so |CCI| <= N/(2*0.015).
	for _, s := range fixtures(500) {
		for _, p := range []int{2, 5, 14, 40} {
			out, _ = CCI(s.H, s.L, s.C, p)
			limit := float64(p)/(2*0.015)*(1+1e-9) + 1e-9
			for i := p - 1; i < len(out); i++ {
				if math.IsNaN(out[i]) || math.Abs(out[i]) > limit {
					t.Fatalf("%s p=%d: [%d]=%g exceeds bound %g", s.Name, p, i, out[i], limit)
				}
			}
		}
	}
}

func TestCCIRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := cciRange(-1, 5, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cciRange(5, 4, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cciRange(0, -1, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cciRange(0, 1, s.H, s.L, s.C, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := cciRange(0, 10, s.H[:5], s.L, s.C, 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high: %v", err)
	}
	if _, _, err := cciRange(0, 10, s.H, s.L, s.C[:10], 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short close: %v", err)
	}
	if _, _, err := cciRange(0, 10, s.H, s.L, s.C, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Sub-range matches the full computation. Not bit-exact: the circular
	// buffer's rotation, and so the summation order, depends on startIdx
	// (as in C), so allow last-ulp differences.
	full, _ := CCI(s.H, s.L, s.C, 5)
	beg, nb, err = cciRange(10, 20, s.H, s.L, s.C, 5, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[10:21], relTol)
	// Output may alias an input.
	buf := append([]float64(nil), s.C...)
	beg, nb, _ = cciRange(0, len(buf)-1, s.H, s.L, buf, 5, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, full, 0)
}
