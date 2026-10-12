package ta

import (
	"errors"
	"math"
	"testing"
)

func TestPLUSDIParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := PLUSDI(s.H, s.L, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := PLUSDILookback(p); lb != -1 {
			t.Errorf("PLUSDILookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := plusdiRange(0, 49, s.H, s.L, s.C, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("plusdiRange period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, c := range []struct{ p, lb int }{{1, 1}, {2, 2}, {3, 3}, {14, 14}, {100000, 100000}} {
		if _, err := PLUSDI(s.H, s.L, s.C, c.p); err != nil {
			t.Errorf("period %d: unexpected err %v", c.p, err)
		}
		if lb := PLUSDILookback(c.p); lb != c.lb {
			t.Errorf("PLUSDILookback(%d) = %d, want %d", c.p, lb, c.lb)
		}
	}
	// Mismatched input lengths.
	if _, err := PLUSDI(s.H, s.L[:49], s.C, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: err = %v", err)
	}
	if _, err := PLUSDI(s.H[:10], s.L, s.C, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v", err)
	}
	if _, err := PLUSDI(s.H, s.L, s.C[:1], 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v", err)
	}
	if _, err := PLUSDI(s.H, s.L, nil, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inClose with non-empty others: err = %v", err)
	}
	// nil / empty input.
	if out, err := PLUSDI(nil, nil, nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := PLUSDI([]float64{}, []float64{}, []float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Short buffers in the range form.
	if _, _, err := plusdiRange(0, 10, s.H, s.L, s.C[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := plusdiRange(0, 10, s.H, s.L, s.C, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := plusdiRange(5, 10, s.H, s.L, s.C, 3, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestPLUSDILengths(t *testing.T) {
	for _, period := range []int{1, 2, 3, 5, 14, 30} {
		lb := PLUSDILookback(period)
		want := period
		if period == 1 {
			want = 1
		}
		if lb != want {
			t.Fatalf("PLUSDILookback(%d) = %d, want %d", period, lb, want)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := PLUSDI(s.H, s.L, s.C, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("PLUSDI ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestPLUSDIKnownValues(t *testing.T) {
	// One-bar +DM and TR by hand:
	//   bar1: diffP=2,  diffM=-1 -> +DM1=2, TR=max(3,3,0)=3
	//   bar2: diffP=-1          -> +DM1=0, TR=max(4,0,4)=4
	//   bar3: diffP=3,  diffM=-3 -> +DM1=3, TR=max(4,6,2)=6
	//   bar4: diffP=-1          -> +DM1=0, TR=max(1,0,1)=1
	h := []float64{10, 12, 11, 14, 13}
	l := []float64{8, 9, 7, 10, 12}
	c := []float64{9, 11, 8, 13, 12.5}
	nan := math.NaN()

	// Period 1 is the unscaled ratio +DM1/TR1 (C does not multiply by 100).
	out, err := PLUSDI(h, l, c, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "PLUSDI p=1", out, []float64{nan, 2.0 / 3, 0, 0.5, 0}, 0)

	// Period 2: seed DM=2, TR=3 at bar 1, then one Wilder step per bar.
	out, err = PLUSDI(h, l, c, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "PLUSDI p=2", out, []float64{nan, nan, 100 * (1 / 5.5), 40, 100 * (1.75 / 5.375)}, 1e-15)

	// Period 3: seed DM=2, TR=7 over bars 1-2.
	dm, tr := 2.0, 7.0
	dm, tr = dm-dm/3+3, tr-tr/3+6
	w3 := 100 * (dm / tr)
	dm, tr = dm-dm/3, tr-tr/3+1
	w4 := 100 * (dm / tr)
	out, _ = PLUSDI(h, l, c, 3)
	assertReal(t, "PLUSDI p=3", out, []float64{nan, nan, nan, w3, w4}, 0)

	// A constant series has zero true range: the output is 0, not NaN.
	cs := fixtures(40)[5]
	for _, p := range []int{1, 7} {
		out, _ = PLUSDI(cs.H, cs.L, cs.C, p)
		for i := PLUSDILookback(p); i < len(out); i++ {
			if out[i] != 0 {
				t.Fatalf("constant p=%d: [%d]=%g", p, i, out[i])
			}
		}
	}

	// A strictly rising trend fixture: every bar has +DM1 = 1 and TR1 = 1.8,
	// so the smoothed ratio stays at 100/1.8 and period 1 gives 1/1.8.
	up := fixtures(60)[7]
	out, _ = PLUSDI(up.H, up.L, up.C, 1)
	for i := 1; i < len(out); i++ {
		if !closeEnough(out[i], 1/1.8, 1e-12) {
			t.Fatalf("up p=1: [%d]=%.17g", i, out[i])
		}
	}
	out, _ = PLUSDI(up.H, up.L, up.C, 14)
	for i := 14; i < len(out); i++ {
		if !closeEnough(out[i], 100/1.8, 1e-9) {
			t.Fatalf("up p=14: [%d]=%.17g", i, out[i])
		}
	}

	// A strictly falling series has no +DM.
	down := fixtures(60)[8]
	for _, p := range []int{1, 14} {
		out, _ = PLUSDI(down.H, down.L, down.C, p)
		for i := PLUSDILookback(p); i < len(out); i++ {
			if out[i] != 0 {
				t.Fatalf("down p=%d: [%d]=%g", p, i, out[i])
			}
		}
	}

	// Bounds: +DI is in [0, 100] (period 1: [0, 1]) because +DM1 <= TR1.
	for _, s := range fixtures(300) {
		for _, p := range []int{1, 2, 14} {
			hi := 100.0
			if p == 1 {
				hi = 1
			}
			out, _ = PLUSDI(s.H, s.L, s.C, p)
			for i := PLUSDILookback(p); i < len(out); i++ {
				if !(out[i] >= 0 && out[i] <= hi*(1+1e-12)) {
					t.Fatalf("%s p=%d: [%d]=%g out of [0,%g]", s.Name, p, i, out[i], hi)
				}
			}
		}
	}
}

func TestPLUSDIUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstPlusDI, 5); err != nil {
		t.Fatal(err)
	}
	if lb := PLUSDILookback(14); lb != 19 {
		t.Errorf("unst=5 lookback(14) = %d, want 19", lb)
	}
	if lb := PLUSDILookback(1); lb != 1 {
		t.Errorf("unst=5 lookback(1) = %d, want 1", lb)
	}
	s := fixtures(100)[0]
	out, err := PLUSDI(s.H, s.L, s.C, 14)
	if err != nil {
		t.Fatal(err)
	}
	assertLeadingNaN(t, "PLUSDI unst=5", out, 19)
}

func TestPLUSDIRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := plusdiRange(-1, 5, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := plusdiRange(5, 4, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := plusdiRange(0, -1, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	for _, c := range []struct{ p, end int }{{3, 2}, {1, 0}, {14, 13}} {
		beg, nb, err := plusdiRange(0, c.end, s.H, s.L, s.C, c.p, out)
		if err != nil || beg != 0 || nb != 0 {
			t.Errorf("inside lookback p=%d end=%d: beg=%d nb=%d err=%v", c.p, c.end, beg, nb, err)
		}
	}
	// Like C, a sub-range restarts the Wilder seed at startIdx-lookback, so
	// it equals the full computation over inputs sliced from that bar.
	for _, p := range []int{1, 4} {
		lb := PLUSDILookback(p)
		sub, _ := PLUSDI(s.H[10-lb:21], s.L[10-lb:21], s.C[10-lb:21], p)
		beg, nb, err := plusdiRange(10, 20, s.H, s.L, s.C, p, out)
		if err != nil || beg != 10 || nb != 11 {
			t.Fatalf("p=%d sub-range: beg=%d nb=%d err=%v", p, beg, nb, err)
		}
		assertReal(t, lbl("sub-range p=", p), out[:nb], sub[lb:], 0)
	}
	// In-place computation over each input is allowed.
	for _, p := range []int{1, 4} {
		want, _ := PLUSDI(s.H, s.L, s.C, p)
		for k := 0; k < 3; k++ {
			h := append([]float64(nil), s.H...)
			l := append([]float64(nil), s.L...)
			c := append([]float64(nil), s.C...)
			buf := [][]float64{h, l, c}[k]
			beg, nb, _ := plusdiRange(0, len(buf)-1, h, l, c, p, buf)
			alignReal(buf, beg, nb)
			assertReal(t, lbl("in-place p=", p, " input=", k), buf, want, 0)
		}
	}
}
