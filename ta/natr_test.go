package ta

import (
	"errors"
	"math"
	"testing"
)

func TestNATRParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := NATR(s.H, s.L, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := NATRLookback(p); lb != -1 {
			t.Errorf("NATRLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := natrRange(0, 49, s.H, s.L, s.C, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("natrRange period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 14, 100000} {
		if _, err := NATR(s.H, s.L, s.C, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := NATRLookback(p); lb != p {
			t.Errorf("NATRLookback(%d) = %d, want %d", p, lb, p)
		}
	}
	// Mismatched input lengths.
	if _, err := NATR(s.H, s.L[:49], s.C, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: err = %v", err)
	}
	if _, err := NATR(s.H, s.L, s.C[:10], 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v", err)
	}
	if _, err := NATR(nil, s.L, s.C, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inHigh with non-empty others: err = %v", err)
	}
	// nil / empty input.
	if out, err := NATR(nil, nil, nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := NATR([]float64{}, []float64{}, []float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Short buffers in the range form.
	if _, _, err := natrRange(0, 10, s.H, s.L[:5], s.C, 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := natrRange(0, 10, s.H, s.L, s.C, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := natrRange(5, 10, s.H, s.L, s.C, 3, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestNATRLengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 14, 30} {
		lb := NATRLookback(period)
		if lb != period {
			t.Fatalf("NATRLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := NATR(s.H, s.L, s.C, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("NATR ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestNATRKnownValues(t *testing.T) {
	h := []float64{10, 11, 13, 12.5, 11}
	l := []float64{8, 9, 12, 10, 7}
	c := []float64{9, 10, 12, 11, 8}
	// True ranges from bar 1: 2, 3 (|10-13|), 2.5 (h-l), 4 (h-l = |c1-l|).
	nan := math.NaN()

	out, err := NATR(h, l, c, 2)
	if err != nil {
		t.Fatal(err)
	}
	// ATR: seed (2+3)/2 = 2.5; then 2.5; then 0.5*2.5+0.5*4 = 3.25. NATR = ATR/close*100.
	// norm evaluates in float64 (Go would fold constant expressions exactly).
	norm := func(atr, close float64) float64 { return atr / close * 100 }
	assertReal(t, "NATR p=2", out, []float64{nan, nan, norm(2.5, 12), norm(2.5, 11), norm(3.25, 8)}, 0)

	// Period 1: raw true range, deliberately not normalized (TA-Lib behaviour).
	out, _ = NATR(h, l, c, 1)
	assertReal(t, "NATR p=1", out, []float64{nan, 2, 3, 2.5, 4}, 0)

	// Period 4 with exactly lookback+1 bars: a single seed value.
	out, _ = NATR(h, l, c, 4)
	assertReal(t, "NATR p=4", out, []float64{nan, nan, nan, nan, norm((2+3+2.5+4)/4.0, 8)}, 0)

	// Period 5 with 5 bars: all inside the lookback.
	out, _ = NATR(h, l, c, 5)
	assertReal(t, "NATR p=5", out, []float64{nan, nan, nan, nan, nan}, 0)

	// A close of exactly zero yields 0 (both the first output and later ones);
	// a negative close yields a negative NATR.
	cz := []float64{9, 10, 0, 11, -8}
	out, _ = NATR(h, l, cz, 2)
	if out[2] != 0 || out[3] == 0 || math.IsNaN(out[3]) || !(out[4] < 0) {
		t.Fatalf("zero/negative close: %v", out)
	}
	cz2 := []float64{9, 10, 12, 0, 8}
	out, _ = NATR(h, l, cz2, 2)
	if out[3] != 0 || out[2] <= 0 {
		t.Fatalf("zero close later: %v", out)
	}
	// Tiny but non-zero close is normalized (exact zero test, not an epsilon band).
	ct := []float64{1e-12, 1e-12, 1e-12, 1e-12, 1e-12}
	out, _ = NATR(h, l, ct, 2)
	if !(out[2] > 1e12) {
		t.Fatalf("tiny close: %v", out)
	}

	// Relationship to ATR: for period > 1, NATR == ATR / close * 100 exactly.
	for _, s := range fixtures(300) {
		for _, p := range []int{1, 2, 14} {
			a, _ := ATR(s.H, s.L, s.C, p)
			nt, _ := NATR(s.H, s.L, s.C, p)
			for i := p; i < len(nt); i++ {
				want := a[i]
				if p > 1 {
					if s.C[i] != 0 {
						want = a[i] / s.C[i] * 100
					} else {
						want = 0
					}
				}
				if nt[i] != want {
					t.Fatalf("%s p=%d [%d]: NATR %.17g want %.17g", s.Name, p, i, nt[i], want)
				}
			}
		}
	}

	// Constant series: zero range, so zero NATR.
	cs := fixtures(40)[5]
	out, _ = NATR(cs.H, cs.L, cs.C, 7)
	for i := 7; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}
	// All-zero series: close is zero, output is 0 (not NaN).
	zs := fixtures(40)[6]
	out, _ = NATR(zs.H, zs.L, zs.C, 7)
	for i := 7; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("zeros: [%d]=%g", i, out[i])
		}
	}
	// Zigzag: every true range is 7, so NATR is 700/close.
	zz := fixtures(60)[9]
	out, _ = NATR(zz.H, zz.L, zz.C, 9)
	for i := 9; i < len(out); i++ {
		if math.Abs(out[i]-700/zz.C[i]) > 1e-9 {
			t.Fatalf("zigzag: [%d]=%.17g", i, out[i])
		}
	}
	// Non-negative for positive closes.
	for _, s := range fixtures(300) {
		if s.Name == "negative" || s.Name == "zeros" {
			continue
		}
		out, _ := NATR(s.H, s.L, s.C, 10)
		for i := 10; i < len(out); i++ {
			if out[i] < 0 || math.IsNaN(out[i]) {
				t.Fatalf("%s [%d]=%g", s.Name, i, out[i])
			}
		}
	}
}

func TestNATRUnstablePeriod(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	s := fixtures(100)[1]
	base, _ := NATR(s.H, s.L, s.C, 14)
	if err := SetUnstablePeriod(UnstNATR, 5); err != nil {
		t.Fatal(err)
	}
	if lb := NATRLookback(14); lb != 19 {
		t.Fatalf("lookback with unstable 5 = %d, want 19", lb)
	}
	out, _ := NATR(s.H, s.L, s.C, 14)
	assertLeadingNaN(t, "unstable 5", out, 19)
	assertReal(t, "unstable tail", out[19:], base[19:], 0)
	out, _ = NATR(s.H[:19], s.L[:19], s.C[:19], 14)
	assertLeadingNaN(t, "unstable short", out, 19)
	// The ATR unstable period does not affect NATR.
	SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstATR, 7); err != nil {
		t.Fatal(err)
	}
	if lb := NATRLookback(14); lb != 14 {
		t.Fatalf("NATR lookback with ATR unstable 7 = %d, want 14", lb)
	}
}

func TestNATRRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := natrRange(-1, 5, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := natrRange(5, 4, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := natrRange(0, -1, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	beg, nb, err := natrRange(0, 2, s.H, s.L, s.C, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	beg, nb, err = natrRange(3, 3, s.H, s.L, s.C, 3, out)
	if err != nil || beg != 3 || nb != 1 {
		t.Errorf("at lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	want, _ := NATR(s.H, s.L, s.C, 4)
	beg, nb, _ = natrRange(0, 29, s.H, s.L, s.C, 4, out)
	alignReal(out, beg, nb)
	assertReal(t, "full range", out, want, 0)
	// In-place into each input is allowed.
	for k := range 3 {
		h := append([]float64(nil), s.H...)
		l := append([]float64(nil), s.L...)
		c := append([]float64(nil), s.C...)
		buf := [][]float64{h, l, c}[k]
		beg, nb, _ := natrRange(0, 29, h, l, c, 4, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place ", k), buf, want, 0)
	}
}
