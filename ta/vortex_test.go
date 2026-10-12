package ta

import (
	"errors"
	"math"
	"testing"
)

func TestVORTEXParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{0, -1, 100001, math.MinInt32} {
		if _, _, err := VORTEX(s.H, s.L, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := VORTEXLookback(p); lb != -1 {
			t.Errorf("VORTEXLookback(%d) = %d, want -1", p, lb)
		}
	}
	for _, p := range []int{1, 2, 14, 100000} {
		if _, _, err := VORTEX(s.H, s.L, s.C, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := VORTEXLookback(p); lb != p {
			t.Errorf("VORTEXLookback(%d) = %d, want %d", p, lb, p)
		}
	}
	// Mismatched input lengths.
	if _, _, err := VORTEX(s.H, s.L[:49], s.C, 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: err = %v", err)
	}
	if _, _, err := VORTEX(s.H, s.L, s.C[:10], 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v", err)
	}
	if _, _, err := VORTEX(nil, s.L, s.C, 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inHigh with non-empty others: err = %v", err)
	}
	// Empty input.
	for _, in := range [][]float64{nil, {}} {
		p, m, err := VORTEX(in, in, in, 14)
		if err != nil || len(p) != 0 || len(m) != 0 {
			t.Errorf("empty input: p=%v m=%v err=%v", p, m, err)
		}
	}
}

func TestVORTEXLengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 14, 30} {
		lb := VORTEXLookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				p, m, err := VORTEX(s.H, s.L, s.C, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(p) != n || len(m) != n {
					t.Fatalf("%s n=%d p=%d: len=%d,%d", s.Name, n, period, len(p), len(m))
				}
				assertLeadingNaN(t, lbl("VORTEX+ ", s.Name, " n=", n, " p=", period), p, lb)
				assertLeadingNaN(t, lbl("VORTEX- ", s.Name, " n=", n, " p=", period), m, lb)
			}
		}
	}
}

func TestVORTEXKnownValues(t *testing.T) {
	// Hand-derived, period 2:
	// TR[1]=max(2,|2-4|,|2-2|)=2  VMP[1]=|4-1|=3  VMM[1]=|2-3|=1
	// TR[2]=max(2,|3-5|,|3-3|)=2  VMP[2]=|5-2|=3  VMM[2]=|3-4|=1
	// TR[3]=max(3,|4-4|,|4-1|)=3  VMP[3]=|4-3|=1  VMM[3]=|1-5|=4
	// bar 2: +VI=6/4=1.5, -VI=2/4=0.5; bar 3: +VI=4/5=0.8, -VI=5/5=1.
	h := []float64{3, 4, 5, 4}
	l := []float64{1, 2, 3, 1}
	c := []float64{2, 3, 4, 2}
	nan := math.NaN()
	p, m, err := VORTEX(h, l, c, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "VORTEX+ known", p, []float64{nan, nan, 1.5, 0.8}, 1e-15)
	assertReal(t, "VORTEX- known", m, []float64{nan, nan, 0.5, 1}, 1e-15)

	// Period 1: a single bar's ratio.
	p, m, _ = VORTEX(h, l, c, 1)
	assertReal(t, "VORTEX+ p1", p, []float64{nan, 3.0 / 2, 3.0 / 2, 1.0 / 3}, 1e-15)
	assertReal(t, "VORTEX- p1", m, []float64{nan, 1.0 / 2, 1.0 / 2, 4.0 / 3}, 1e-15)

	// Constant (flat) series: zero true range -> both lines 0.
	k := fixtures(40)[5]
	p, m, _ = VORTEX(k.H, k.L, k.C, 7)
	for i := 7; i < len(p); i++ {
		if p[i] != 0 || m[i] != 0 {
			t.Fatalf("constant: [%d]=%g,%g", i, p[i], m[i])
		}
	}

	// Spread bar then a halt: TR of the halted bars is 0 but the first
	// halted bar's vortex terms are alive; once the window is all flat the
	// lines are 0, never negative.
	h2 := []float64{10, 12, 11, 11, 11, 11, 11}
	l2 := []float64{9, 10, 11, 11, 11, 11, 11}
	c2 := []float64{9.5, 11, 11, 11, 11, 11, 11}
	p, m, _ = VORTEX(h2, l2, c2, 2)
	// bar 2: TR[1]=max(2,|9.5-12|,|9.5-10|)=2.5, TR[2]=0; VMP=|12-9|+|11-10|=4; VMM=|10-10|+|11-12|=1
	assertReal(t, "halt+", p, []float64{nan, nan, 4 / 2.5, 0, 0, 0, 0}, 1e-15)
	assertReal(t, "halt-", m, []float64{nan, nan, 1 / 2.5, 0, 0, 0, 0}, 1e-15)

	// Bounds: both lines are non-negative on well-formed data.
	for _, s := range fixtures(300) {
		p, m, _ := VORTEX(s.H, s.L, s.C, 14)
		for i := 14; i < len(p); i++ {
			if p[i] < 0 || m[i] < 0 || math.IsInf(p[i], 0) || math.IsInf(m[i], 0) {
				t.Fatalf("%s: [%d]=%g,%g", s.Name, i, p[i], m[i])
			}
		}
	}

	// Scale invariance (homogeneous of degree 0): multiply by a power of 2.
	s := fixtures(100)[0]
	sc := func(x []float64) []float64 {
		o := make([]float64, len(x))
		for i, v := range x {
			o[i] = v * 1024
		}
		return o
	}
	p1, m1, _ := VORTEX(s.H, s.L, s.C, 14)
	p2, m2, _ := VORTEX(sc(s.H), sc(s.L), sc(s.C), 14)
	assertReal(t, "scaled+", p2, p1, 0)
	assertReal(t, "scaled-", m2, m1, 0)
}

func TestVORTEXRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	p := make([]float64, 30)
	m := make([]float64, 30)
	if _, _, err := vortexRange(-1, 5, s.H, s.L, s.C, 3, p, m); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := vortexRange(5, 4, s.H, s.L, s.C, 3, p, m); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := vortexRange(0, 5, s.H, s.L, s.C, 0, p, m); !errors.Is(err, ErrBadParam) {
		t.Errorf("period 0: %v", err)
	}
	// Range entirely inside the lookback (lookback 3 -> bars 0..2).
	beg, nb, err := vortexRange(0, 2, s.H, s.L, s.C, 3, p, m)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := vortexRange(0, 20, s.H, s.L[:10], s.C, 3, p, m); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := vortexRange(0, 20, s.H, s.L, s.C, 3, p, m[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Same output buffer for both lines is rejected, as in C.
	if _, _, err := vortexRange(0, 20, s.H, s.L, s.C, 3, p, p); !errors.Is(err, ErrBadParam) {
		t.Errorf("aliased outputs: %v", err)
	}
	// Outputs may alias inputs (C writes outputs last).
	wantP, wantM, _ := VORTEX(s.H, s.L, s.C, 4)
	h := append([]float64(nil), s.H...)
	l := append([]float64(nil), s.L...)
	beg, nb, _ = vortexRange(0, 29, h, l, s.C, 4, h, l)
	alignReal(h, beg, nb)
	alignReal(l, beg, nb)
	assertReal(t, "in-place+", h, wantP, 0)
	assertReal(t, "in-place-", l, wantM, 0)
}
