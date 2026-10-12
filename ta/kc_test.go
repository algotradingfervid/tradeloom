package ta

import (
	"errors"
	"math"
	"testing"
)

func TestKCParams(t *testing.T) {
	s := fixtures(50)[0]
	h, l, c := s.H, s.L, s.C
	buf := func() []float64 { return make([]float64, 50) }
	bad := []struct {
		p, a int
		d    float64
	}{
		{1, 10, 2}, {0, 10, 2}, {-1, 10, 2}, {100001, 10, 2}, {math.MinInt32, 10, 2},
		{20, 0, 2}, {20, -1, 2}, {20, 100001, 2}, {20, math.MinInt32, 2},
		{20, 10, math.NaN()}, {20, 10, math.Inf(1)}, {20, 10, math.Inf(-1)},
		{20, 10, 3.0000001e37}, {20, 10, -3.0000001e37}, {20, 10, -4e37},
	}
	for _, b := range bad {
		if _, _, _, err := KC(h, l, c, b.p, b.a, b.d); !errors.Is(err, ErrBadParam) {
			t.Errorf("KC(%d,%d,%g): err = %v, want ErrBadParam", b.p, b.a, b.d, err)
		}
		if lb := KCLookback(b.p, b.a, b.d); lb != -1 {
			t.Errorf("KCLookback(%d,%d,%g) = %d, want -1", b.p, b.a, b.d, lb)
		}
		if _, _, err := kcRange(0, 49, h, l, c, b.p, b.a, b.d, buf(), buf(), buf()); !errors.Is(err, ErrBadParam) {
			t.Errorf("kcRange(%d,%d,%g): err = %v, want ErrBadParam", b.p, b.a, b.d, err)
		}
	}
	good := []struct {
		p, a int
		d    float64
		lb   int
	}{
		{2, 1, 2, 1}, {2, 100000, 2, 100000}, {100000, 1, 2, 99999}, {100000, 100000, 2, 100000},
		{20, 10, 2, 19}, {5, 14, 2, 14}, {20, 10, 3e37, 19}, {20, 10, -3e37, 19}, {20, 10, 0, 19},
	}
	for _, g := range good {
		if _, _, _, err := KC(h, l, c, g.p, g.a, g.d); err != nil {
			t.Errorf("KC(%d,%d,%g): unexpected err %v", g.p, g.a, g.d, err)
		}
		if lb := KCLookback(g.p, g.a, g.d); lb != g.lb {
			t.Errorf("KCLookback(%d,%d,%g) = %d, want %d", g.p, g.a, g.d, lb, g.lb)
		}
	}
	for _, in := range [][]float64{nil, {}} {
		u, m, lo, err := KC(in, in, in, 20, 10, 2)
		if err != nil || len(u) != 0 || len(m) != 0 || len(lo) != 0 {
			t.Errorf("empty input: %v %v %v err=%v", u, m, lo, err)
		}
	}
	if _, _, _, err := KC(h, l[:49], c, 20, 10, 2); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, _, _, err := KC(h, l, nil, 20, 10, 2); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil close: err = %v, want ErrBadParam", err)
	}
}

func TestKCLengths(t *testing.T) {
	for _, ps := range []struct {
		p, a int
		d    float64
	}{{2, 1, 2}, {20, 10, 2}, {5, 14, 1.5}, {14, 5, -1}, {30, 30, 0}} {
		lb := KCLookback(ps.p, ps.a, ps.d)
		if want := max(ps.p-1, ps.a); lb != want {
			t.Fatalf("KCLookback(%v) = %d, want %d", ps, lb, want)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				u, m, l, err := KC(s.H, s.L, s.C, ps.p, ps.a, ps.d)
				if err != nil {
					t.Fatalf("%s n=%d %v: %v", s.Name, n, ps, err)
				}
				if len(u) != n || len(m) != n || len(l) != n {
					t.Fatalf("%s n=%d %v: lens %d %d %d", s.Name, n, ps, len(u), len(m), len(l))
				}
				tag := lbl(" ", s.Name, " n=", n, " ", ps)
				assertLeadingNaN(t, "KC upper"+tag, u, lb)
				assertLeadingNaN(t, "KC middle"+tag, m, lb)
				assertLeadingNaN(t, "KC lower"+tag, l, lb)
			}
		}
	}
}

func TestKCKnownValues(t *testing.T) {
	// TP = 2, 3, 4, 4. EMA(2): k = 2/3, seed SMA(2,3) = 2.5 at bar 1, then
	// 2.5+(4-2.5)*2/3 = 3.5, 3.5+(4-3.5)*2/3 = 23/6.
	// ATR(1) = true range: bar1 4-2 = 2, bar2 5-3 = 2, bar3 6-2 = 4.
	h := []float64{3, 4, 5, 6}
	l := []float64{1, 2, 3, 2}
	c := []float64{2, 3, 4, 4}
	u, m, lo, err := KC(h, l, c, 2, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	assertReal(t, "KC known middle", m, []float64{nan, 2.5, 3.5, 23.0 / 6}, 1e-12)
	assertReal(t, "KC known upper", u, []float64{nan, 6.5, 7.5, 23.0/6 + 8}, 1e-12)
	assertReal(t, "KC known lower", lo, []float64{nan, -1.5, -0.5, 23.0/6 - 8}, 1e-12)

	// Constant series: zero true range, so all three bands equal the price.
	k := fixtures(60)[5]
	u, m, lo, _ = KC(k.H, k.L, k.C, 7, 5, 2)
	for i := 6; i < 60; i++ {
		if u[i] != 100 || m[i] != 100 || lo[i] != 100 {
			t.Fatalf("constant [%d]: %g %g %g", i, u[i], m[i], lo[i])
		}
	}

	s := fixtures(300)[0]
	// When the EMA leg has the longer lookback, the middle band is exactly
	// EMA(TYPPRICE) over the whole series.
	tp, _ := TYPPRICE(s.H, s.L, s.C)
	ema, _ := EMA(tp, 20)
	_, m, _, _ = KC(s.H, s.L, s.C, 20, 10, 2)
	assertReal(t, "KC middle == EMA(TYPPRICE)", m, ema, 0)
	// When the ATR leg has the longer lookback, upper-middle is ATR*nbDev.
	atr, _ := ATR(s.H, s.L, s.C, 30)
	u, m, lo, _ = KC(s.H, s.L, s.C, 5, 30, 1.5)
	for i := 30; i < 300; i++ {
		if !closeEnough(u[i]-m[i], atr[i]*1.5, 1e-9) || !closeEnough(m[i]-lo[i], atr[i]*1.5, 1e-9) {
			t.Fatalf("band width [%d]: %g %g want %g", i, u[i]-m[i], m[i]-lo[i], atr[i]*1.5)
		}
		if !(lo[i] <= m[i] && m[i] <= u[i]) {
			t.Fatalf("ordering [%d]: %g %g %g", i, lo[i], m[i], u[i])
		}
	}
	// nbDev 0 collapses the channel; a negative nbDev swaps the bands.
	u, m, lo, _ = KC(s.H, s.L, s.C, 10, 10, 0)
	assertReal(t, "nbDev 0 upper", u, m, 0)
	assertReal(t, "nbDev 0 lower", lo, m, 0)
	u1, _, l1, _ := KC(s.H, s.L, s.C, 10, 10, 2)
	u2, _, l2, _ := KC(s.H, s.L, s.C, 10, 10, -2)
	assertReal(t, "nbDev -2 upper", u2, l1, 0)
	assertReal(t, "nbDev -2 lower", l2, u1, 0)
}

func TestKCRangeForm(t *testing.T) {
	s := fixtures(40)[0]
	u, m, l := make([]float64, 40), make([]float64, 40), make([]float64, 40)
	if _, _, err := kcRange(-1, 5, s.H, s.L, s.C, 5, 3, 2, u, m, l); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := kcRange(5, 4, s.H, s.L, s.C, 5, 3, 2, u, m, l); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := kcRange(0, -1, s.H, s.L, s.C, 5, 3, 2, u, m, l); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (lb = 4): no output, no error.
	beg, nb, err := kcRange(0, 3, s.H, s.L, s.C, 5, 3, 2, u, m, l)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if _, _, err := kcRange(0, 10, s.H[:5], s.L, s.C, 5, 3, 2, u, m, l); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: %v", err)
	}
	if _, _, err := kcRange(0, 10, s.H, s.L, s.C, 5, 3, 2, u, m[:5], l); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: %v", err)
	}
	if _, _, err := kcRange(0, 10, s.H, s.L, s.C, 5, 3, 2, u, u, l); !errors.Is(err, ErrBadParam) {
		t.Errorf("aliased outputs: %v", err)
	}
	// Sub-range values match the full series.
	fu, fm, fl, _ := KC(s.H, s.L, s.C, 5, 3, 2)
	beg, nb, err = kcRange(10, 30, s.H, s.L, s.C, 5, 3, 2, u, m, l)
	if err != nil || beg != 10 || nb != 21 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Both legs are re-seeded inside the sub-range (as in C), so exact
	// equality is checked on the full range; sub-range values are compared
	// with C in the oracle tests.
	beg, nb, _ = kcRange(0, 39, s.H, s.L, s.C, 5, 3, 2, u, m, l)
	alignReal(u, beg, nb)
	alignReal(m, beg, nb)
	alignReal(l, beg, nb)
	assertReal(t, "range upper", u, fu, 0)
	assertReal(t, "range middle", m, fm, 0)
	assertReal(t, "range lower", l, fl, 0)
	// The middle band may alias an input (C consumes inputs before writing it).
	hc := append([]float64(nil), s.H...)
	u2, l2 := make([]float64, 40), make([]float64, 40)
	beg, nb, _ = kcRange(0, 39, hc, s.L, s.C, 5, 3, 2, u2, hc, l2)
	alignReal(hc, beg, nb)
	assertReal(t, "aliased middle", hc, fm, 0)
}
