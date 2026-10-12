package ta

import (
	"errors"
	"math"
	"testing"
)

func TestSUPERTRENDParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, _, err := SUPERTREND(s.H, s.L, s.C, p, 3); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := SUPERTRENDLookback(p, 3); lb != -1 {
			t.Errorf("SUPERTRENDLookback(%d, 3) = %d, want -1", p, lb)
		}
	}
	for _, m := range []float64{-1e-300, -1, taRealMax * 1.0000001, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, _, err := SUPERTREND(s.H, s.L, s.C, 10, m); !errors.Is(err, ErrBadParam) {
			t.Errorf("multiplier %g: err = %v, want ErrBadParam", m, err)
		}
		if lb := SUPERTRENDLookback(10, m); lb != -1 {
			t.Errorf("SUPERTRENDLookback(10, %g) = %d, want -1", m, lb)
		}
	}
	for _, p := range []int{2, 10, 100000} {
		for _, m := range []float64{0, 3, taRealMax} {
			if _, _, err := SUPERTREND(s.H, s.L, s.C, p, m); err != nil {
				t.Errorf("period %d mult %g: unexpected err %v", p, m, err)
			}
			if lb := SUPERTRENDLookback(p, m); lb != p {
				t.Errorf("SUPERTRENDLookback(%d, %g) = %d, want %d", p, m, lb, p)
			}
		}
	}
	// Mismatched input lengths.
	if _, _, err := SUPERTREND(s.H, s.L[:49], s.C, 10, 3); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: err = %v", err)
	}
	if _, _, err := SUPERTREND(s.H, s.L, s.C[:10], 10, 3); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v", err)
	}
	// nil / empty input: empty outputs, nil error.
	for _, in := range [][]float64{nil, {}} {
		st, tr, err := SUPERTREND(in, in, in, 10, 3)
		if err != nil || len(st) != 0 || len(tr) != 0 {
			t.Errorf("empty input: st=%v tr=%v err=%v", st, tr, err)
		}
	}
	// Buffer checks in the range form.
	if _, _, err := supertrendRange(0, 40, s.H, s.L[:5], s.C, 10, 3, make([]float64, 50), make([]int, 50)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v", err)
	}
	if _, _, err := supertrendRange(0, 40, s.H, s.L, s.C, 10, 3, make([]float64, 40), make([]int, 50)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short real output buffer: err = %v", err)
	}
	if _, _, err := supertrendRange(0, 40, s.H, s.L, s.C, 10, 3, make([]float64, 50), make([]int, 40)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short int output buffer: err = %v", err)
	}
	if _, nb, err := supertrendRange(0, 40, s.H, s.L, s.C, 10, 3, make([]float64, 41), make([]int, 41)); err != nil || nb != 41-10 {
		t.Errorf("exact output buffers: nb=%d err=%v", nb, err)
	}
}

func TestSUPERTRENDLengths(t *testing.T) {
	for _, ps := range []struct {
		p int
		m float64
	}{{2, 0}, {2, 3}, {10, 3}, {14, 1.5}, {30, 4}} {
		lb := SUPERTRENDLookback(ps.p, ps.m)
		if lb != ps.p {
			t.Fatalf("SUPERTRENDLookback(%d, %g) = %d", ps.p, ps.m, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				st, tr, err := SUPERTREND(s.H, s.L, s.C, ps.p, ps.m)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, ps.p, err)
				}
				if len(st) != n || len(tr) != n {
					t.Fatalf("%s n=%d p=%d: len=%d,%d", s.Name, n, ps.p, len(st), len(tr))
				}
				label := lbl("SUPERTREND ", s.Name, " n=", n, " p=", ps.p, " m=", ps.m)
				assertLeadingNaN(t, label, st, lb)
				for i, v := range tr {
					if i < lb && v != 0 {
						t.Fatalf("%s: trend[%d]=%d inside lookback", label, i, v)
					}
					if i >= lb && v != 1 && v != -1 {
						t.Fatalf("%s: trend[%d]=%d, want +-1", label, i, v)
					}
				}
			}
		}
	}
}

func TestSUPERTRENDKnownValues(t *testing.T) {
	// Hand-derived, period 2, multiplier 1 (see supertrend.md):
	//   TR[1]=2, TR[2]=2 -> seed ATR 2 at bar 2; median 11 -> bands 13/9, up, line 9.
	//   bar 3: TR 5, ATR 3.5, median 8 -> basic 11.5/4.5; upper 11.5, lower stays 9;
	//          close 7 < 9 -> down, line 11.5.
	//   bar 4: TR 2, ATR 2.75, median 8 -> basic 10.75/5.25; upper 10.75; prev close
	//          7 < 9 releases lower to 5.25; close 8 <= 10.75 -> still down, line 10.75.
	//   bar 5: TR 6, ATR 4.375, median 13 -> basic 17.375/8.625; upper held 10.75,
	//          lower 8.625; close 13.5 > 10.75 -> up, line 8.625.
	h := []float64{10, 11, 12, 10, 9, 14}
	l := []float64{8, 9, 10, 6, 7, 12}
	c := []float64{9, 10, 11, 7, 8, 13.5}
	st, tr, err := SUPERTREND(h, l, c, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	assertReal(t, "SUPERTREND line", st, []float64{nan, nan, 9, 11.5, 10.75, 8.625}, 0)
	assertInt(t, "SUPERTREND trend", tr, []int{0, 0, 1, -1, -1, 1})

	// Constant input: no true range, bands collapse on the price, close never
	// below the lower band -> up throughout, line == price.
	s := fixtures(40)[5]
	st, tr, _ = SUPERTREND(s.H, s.L, s.C, 7, 3)
	for i := 7; i < len(st); i++ {
		if st[i] != 100 || tr[i] != 1 {
			t.Fatalf("constant: [%d]=%g trend %d", i, st[i], tr[i])
		}
	}

	// Strictly rising series stays up; the lower band never decreases.
	s = fixtures(200)[7]
	st, tr, _ = SUPERTREND(s.H, s.L, s.C, 10, 3)
	for i := 10; i < len(st); i++ {
		if tr[i] != 1 {
			t.Fatalf("up trend: trend[%d]=%d", i, tr[i])
		}
		if i > 10 && st[i] < st[i-1] {
			t.Fatalf("up trend: line fell at %d: %g < %g", i, st[i], st[i-1])
		}
	}

	// Strictly falling series flips down and stays down.
	s = fixtures(200)[8]
	_, tr, _ = SUPERTREND(s.H, s.L, s.C, 10, 3)
	if tr[len(tr)-1] != -1 {
		t.Fatalf("down trend: last trend %d", tr[len(tr)-1])
	}

	// The random walk flips in both directions at least once.
	s = fixtures(1000)[0]
	_, tr, _ = SUPERTREND(s.H, s.L, s.C, 10, 3)
	ups, downs := 0, 0
	for _, v := range tr {
		switch v {
		case 1:
			ups++
		case -1:
			downs++
		}
	}
	if ups == 0 || downs == 0 {
		t.Fatalf("walk: ups=%d downs=%d, want both", ups, downs)
	}
}

func TestSUPERTRENDUnstablePeriod(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstATR, 5); err != nil {
		t.Fatal(err)
	}
	if lb := SUPERTRENDLookback(10, 3); lb != 15 {
		t.Fatalf("lookback with ATR unstable 5 = %d, want 15", lb)
	}
	s := fixtures(60)[0]
	st, tr, err := SUPERTREND(s.H, s.L, s.C, 10, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertLeadingNaN(t, "unstable 5", st, 15)
	if tr[14] != 0 || tr[15] == 0 {
		t.Fatalf("unstable 5: trend[14]=%d trend[15]=%d", tr[14], tr[15])
	}
}

func TestSUPERTRENDRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	st := make([]float64, 30)
	tr := make([]int, 30)
	if _, _, err := supertrendRange(-1, 5, s.H, s.L, s.C, 3, 3, st, tr); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := supertrendRange(5, 4, s.H, s.L, s.C, 3, 3, st, tr); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := supertrendRange(0, -1, s.H, s.L, s.C, 3, 3, st, tr); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := supertrendRange(0, 2, s.H, s.L, s.C, 3, 3, st, tr)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Bad parameter in the range form.
	if _, _, err := supertrendRange(0, 29, s.H, s.L, s.C, 1, 3, st, tr); !errors.Is(err, ErrBadParam) {
		t.Errorf("period 1: %v", err)
	}
	if _, _, err := supertrendRange(0, 29, s.H, s.L, s.C, 3, -1, st, tr); !errors.Is(err, ErrBadParam) {
		t.Errorf("multiplier -1: %v", err)
	}
	// Full range equals the public form.
	wantSt, wantTr, _ := SUPERTREND(s.H, s.L, s.C, 4, 2)
	beg, nb, err = supertrendRange(0, 29, s.H, s.L, s.C, 4, 2, st, tr)
	if err != nil || beg != 4 || nb != 26 {
		t.Fatalf("full range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	alignReal(st, beg, nb)
	alignInt(tr, beg, nb)
	assertReal(t, "full range", st, wantSt, 0)
	assertInt(t, "full range trend", tr, wantTr)
	// A single bar after the lookback: one value, trend seeded up.
	beg, nb, err = supertrendRange(20, 20, s.H, s.L, s.C, 4, 2, st, tr)
	if err != nil || beg != 20 || nb != 1 || tr[0] != 1 {
		t.Errorf("single bar: beg=%d nb=%d err=%v trend=%d", beg, nb, err, tr[0])
	}
}
