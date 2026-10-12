package ta

import (
	"errors"
	"math"
	"testing"
)

func TestMINUSDIParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := MINUSDI(s.H, s.L, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := MINUSDILookback(p); lb != -1 {
			t.Errorf("MINUSDILookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := minusdiRange(0, 49, s.H, s.L, s.C, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("minusdiRange period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 3, 14, 100000} {
		if _, err := MINUSDI(s.H, s.L, s.C, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := MINUSDILookback(p); lb != p {
			t.Errorf("MINUSDILookback(%d) = %d, want %d", p, lb, p)
		}
	}
	// Mismatched input lengths.
	if _, err := MINUSDI(s.H, s.L[:49], s.C, 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: err = %v", err)
	}
	if _, err := MINUSDI(s.H, s.L, s.C[:10], 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v", err)
	}
	if _, err := MINUSDI(s.H[:1], s.L, s.C, 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v", err)
	}
	if _, err := MINUSDI(nil, s.L, s.C, 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inHigh with others set: err = %v", err)
	}
	// nil / empty input.
	if out, err := MINUSDI(nil, nil, nil, 14); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := MINUSDI([]float64{}, []float64{}, []float64{}, 1); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Short buffers in the range form.
	if _, _, err := minusdiRange(0, 10, s.H, s.L, s.C[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := minusdiRange(0, 10, s.H[:10], s.L, s.C, 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := minusdiRange(0, 10, s.H, s.L, s.C, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := minusdiRange(5, 10, s.H, s.L, s.C, 3, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestMINUSDILengths(t *testing.T) {
	for _, period := range []int{1, 2, 3, 5, 14, 30} {
		lb := MINUSDILookback(period)
		if lb != period { // period 1 also has lookback 1
			t.Fatalf("MINUSDILookback(%d) = %d, want %d", period, lb, period)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := MINUSDI(s.H, s.L, s.C, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("MINUSDI ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestMINUSDIKnownValues(t *testing.T) {
	// Period 2 by hand:
	//   bar1: diffP=2,  diffM=-1 -> -DM1=0; TR1=3. Seed: -DM=0, TR=3.
	//   bar2: diffP=-1, diffM=2  -> -DM1=2; TR1=4.
	//         -DM=0-0+2=2, TR=3-1.5+4=5.5 -> -DI=100*2/5.5
	//   bar3: diffP=2,  diffM=-1 -> -DM1=0; TR1=5.
	//         -DM=2-1=1, TR=5.5-2.75+5=7.75 -> -DI=100*1/7.75
	h := []float64{10, 12, 11, 13}
	l := []float64{8, 9, 7, 8}
	c := []float64{9, 11, 8, 12}
	nan := math.NaN()
	out, err := MINUSDI(h, l, c, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "MINUSDI p=2", out, []float64{nan, nan, 100 * 2 / 5.5, 100 / 7.75}, 1e-12)

	// Period 1 is the unsmoothed ratio -DM1/TR1 (no x100):
	//   bar1: -DM1=0 -> 0; bar2: -DM1=2, TR1=4 -> 0.5; bar3: -DM1=0 -> 0.
	out, err = MINUSDI(h, l, c, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "MINUSDI p=1", out, []float64{nan, 0, 0.5, 0}, 0)

	// Period 1 with a down move but zero true range is impossible (TR >= diffM),
	// but a zero true range with no move gives 0.
	out, _ = MINUSDI([]float64{5, 5}, []float64{5, 5}, []float64{5, 5}, 1)
	assertReal(t, "p=1 zero range", out, []float64{nan, 0}, 0)

	// A constant series has zero true range: -DI is 0.
	for _, idx := range []int{5, 6} {
		cs := fixtures(40)[idx]
		for _, p := range []int{1, 7} {
			out, _ = MINUSDI(cs.H, cs.L, cs.C, p)
			for i := MINUSDILookback(p); i < len(out); i++ {
				if out[i] != 0 {
					t.Fatalf("%s p=%d: [%d]=%g", cs.Name, p, i, out[i])
				}
			}
		}
	}

	// A strictly rising series has no downward movement: -DI is 0.
	up := fixtures(60)[7]
	out, _ = MINUSDI(up.H, up.L, up.C, 14)
	for i := 14; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("up: [%d]=%g, want 0", i, out[i])
		}
	}
	// A strictly falling series: every bar's -DM1 is 1 and TR1 is 1.8, so
	// -DI = 100/1.8 once seeded (the seed is equal-weighted, so exact at once).
	down := fixtures(60)[8]
	out, _ = MINUSDI(down.H, down.L, down.C, 14)
	for i := 14; i < len(out); i++ {
		if math.Abs(out[i]-100/1.8) > 1e-9 {
			t.Fatalf("down: [%d]=%.17g, want %g", i, out[i], 100/1.8)
		}
	}

	// -DI is in [0, 100] (period 1: [0, 1]).
	for _, s := range fixtures(300) {
		for _, p := range []int{1, 2, 14} {
			hi := 100 + 1e-9
			if p == 1 {
				hi = 1 + 1e-12
			}
			out, _ = MINUSDI(s.H, s.L, s.C, p)
			for i := MINUSDILookback(p); i < len(out); i++ {
				if !(out[i] >= 0 && out[i] <= hi) {
					t.Fatalf("%s p=%d: [%d]=%g out of [0,%g]", s.Name, p, i, out[i], hi)
				}
			}
		}
	}
}

func TestMINUSDIUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstMinusDI, 5); err != nil {
		t.Fatal(err)
	}
	if lb := MINUSDILookback(14); lb != 19 {
		t.Errorf("unst=5 lookback(14) = %d, want 19", lb)
	}
	// Period 1 ignores the unstable period, like C.
	if lb := MINUSDILookback(1); lb != 1 {
		t.Errorf("unst=5 lookback(1) = %d, want 1", lb)
	}
	s := fixtures(100)[0]
	out, err := MINUSDI(s.H, s.L, s.C, 14)
	if err != nil {
		t.Fatal(err)
	}
	assertLeadingNaN(t, "MINUSDI unst=5", out, 19)
}

func TestMINUSDIRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := minusdiRange(-1, 5, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := minusdiRange(5, 4, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := minusdiRange(0, -1, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	for _, c := range []struct{ p, end int }{{3, 2}, {2, 1}, {1, 0}, {14, 13}} {
		beg, nb, err := minusdiRange(0, c.end, s.H, s.L, s.C, c.p, out)
		if err != nil || beg != 0 || nb != 0 {
			t.Errorf("inside lookback p=%d end=%d: beg=%d nb=%d err=%v", c.p, c.end, beg, nb, err)
		}
	}
	// Like C, a sub-range restarts the Wilder seed at startIdx-lookback, so
	// it equals the full computation over inputs sliced from that bar.
	for _, p := range []int{1, 2, 4} {
		lb := MINUSDILookback(p)
		sub, _ := MINUSDI(s.H[10-lb:21], s.L[10-lb:21], s.C[10-lb:21], p)
		beg, nb, err := minusdiRange(10, 20, s.H, s.L, s.C, p, out)
		if err != nil || beg != 10 || nb != 11 {
			t.Fatalf("p=%d sub-range: beg=%d nb=%d err=%v", p, beg, nb, err)
		}
		assertReal(t, lbl("sub-range p=", p), out[:nb], sub[lb:], 0)
	}
	// In-place computation over inClose is allowed.
	for _, p := range []int{1, 2, 4} {
		buf := append([]float64(nil), s.C...)
		want, _ := MINUSDI(s.H, s.L, s.C, p)
		beg, nb, _ := minusdiRange(0, len(buf)-1, s.H, s.L, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
