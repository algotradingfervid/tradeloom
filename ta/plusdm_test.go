package ta

import (
	"errors"
	"math"
	"testing"
)

func TestPLUSDMParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := PLUSDM(s.H, s.L, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := PLUSDMLookback(p); lb != -1 {
			t.Errorf("PLUSDMLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := plusdmRange(0, 49, s.H, s.L, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("plusdmRange period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, c := range []struct{ p, lb int }{{1, 1}, {2, 1}, {3, 2}, {14, 13}, {100000, 99999}} {
		if _, err := PLUSDM(s.H, s.L, c.p); err != nil {
			t.Errorf("period %d: unexpected err %v", c.p, err)
		}
		if lb := PLUSDMLookback(c.p); lb != c.lb {
			t.Errorf("PLUSDMLookback(%d) = %d, want %d", c.p, lb, c.lb)
		}
	}
	// Mismatched input lengths.
	if _, err := PLUSDM(s.H, s.L[:49], 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: err = %v", err)
	}
	if _, err := PLUSDM(s.H[:10], s.L, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v", err)
	}
	if _, err := PLUSDM(nil, s.L, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inHigh with non-empty inLow: err = %v", err)
	}
	// nil / empty input.
	if out, err := PLUSDM(nil, nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := PLUSDM([]float64{}, []float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Short buffers in the range form.
	if _, _, err := plusdmRange(0, 10, s.H, s.L[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := plusdmRange(0, 10, s.H, s.L, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := plusdmRange(5, 10, s.H, s.L, 3, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestPLUSDMLengths(t *testing.T) {
	for _, period := range []int{1, 2, 3, 5, 14, 30} {
		lb := PLUSDMLookback(period)
		want := period - 1
		if period == 1 {
			want = 1
		}
		if lb != want {
			t.Fatalf("PLUSDMLookback(%d) = %d, want %d", period, lb, want)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := PLUSDM(s.H, s.L, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("PLUSDM ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestPLUSDMKnownValues(t *testing.T) {
	// One-bar +DM by hand (diffP = high-prevHigh, diffM = prevLow-low):
	//   bar1: diffP=2,  diffM=-1 -> +DM1 = 2 (case 1)
	//   bar2: diffP=-1, diffM=2  -> 0 (case 2)
	//   bar3: diffP=2,  diffM=-3 -> +DM1 = 2 (outside bar, diffP wins: case 3)
	//   bar4: diffP=1,  diffM=2  -> 0 (outside bar, diffM wins: case 4)
	//   bar5: diffP=1,  diffM=1  -> 0 (equal deltas: case 7)
	h := []float64{10, 12, 11, 13, 14, 15}
	l := []float64{8, 9, 7, 10, 8, 7}
	nan := math.NaN()

	out, err := PLUSDM(h, l, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "PLUSDM p=1", out, []float64{nan, 2, 0, 2, 0, 0}, 0)

	// Period 2: seed = +DM1(bar1) = 2 at bar 1, then
	// Wilder: prev - prev/2 + DM1.
	out, err = PLUSDM(h, l, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "PLUSDM p=2", out, []float64{nan, 2, 1, 2.5, 1.25, 0.625}, 0)

	// Period 3: seed = +DM1(bar1)+DM1(bar2) = 2 at bar 2.
	v3 := 2 - 2.0/3 + 2
	v4 := v3 - v3/3
	v5 := v4 - v4/3
	out, _ = PLUSDM(h, l, 3)
	assertReal(t, "PLUSDM p=3", out, []float64{nan, nan, 2, v3, v4, v5}, 0)

	// A constant series has no directional movement.
	c := fixtures(40)[5]
	for _, p := range []int{1, 7} {
		out, _ = PLUSDM(c.H, c.L, p)
		for i := PLUSDMLookback(p); i < len(out); i++ {
			if out[i] != 0 {
				t.Fatalf("constant p=%d: [%d]=%g", p, i, out[i])
			}
		}
	}

	// A strictly falling series (lower highs, lower lows) has no +DM.
	down := fixtures(60)[8]
	out, _ = PLUSDM(down.H, down.L, 14)
	for i := 13; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("down p=14: [%d]=%g", i, out[i])
		}
	}

	// A strictly rising series: every bar has +DM1 = 1 (up to rounding: the
	// highs are start+i+1.3, not exactly representable), so the period-1
	// output is 1 after the first bar.
	up := fixtures(60)[7]
	out, _ = PLUSDM(up.H, up.L, 1)
	for i := 1; i < len(out); i++ {
		if !closeEnough(out[i], 1, 1e-12) {
			t.Fatalf("up p=1: [%d]=%g", i, out[i])
		}
	}

	// +DM is never negative.
	for _, s := range fixtures(300) {
		for _, p := range []int{1, 2, 14} {
			out, _ = PLUSDM(s.H, s.L, p)
			for i := PLUSDMLookback(p); i < len(out); i++ {
				if !(out[i] >= 0) {
					t.Fatalf("%s p=%d: [%d]=%g < 0", s.Name, p, i, out[i])
				}
			}
		}
	}
}

func TestPLUSDMUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstPlusDM, 5); err != nil {
		t.Fatal(err)
	}
	if lb := PLUSDMLookback(14); lb != 18 {
		t.Errorf("unst=5 lookback(14) = %d, want 18", lb)
	}
	if lb := PLUSDMLookback(1); lb != 1 {
		t.Errorf("unst=5 lookback(1) = %d, want 1", lb)
	}
	s := fixtures(100)[0]
	out, err := PLUSDM(s.H, s.L, 14)
	if err != nil {
		t.Fatal(err)
	}
	assertLeadingNaN(t, "PLUSDM unst=5", out, 18)
}

func TestPLUSDMRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := plusdmRange(-1, 5, s.H, s.L, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := plusdmRange(5, 4, s.H, s.L, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := plusdmRange(0, -1, s.H, s.L, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	for _, c := range []struct{ p, end int }{{3, 1}, {1, 0}, {14, 12}} {
		beg, nb, err := plusdmRange(0, c.end, s.H, s.L, c.p, out)
		if err != nil || beg != 0 || nb != 0 {
			t.Errorf("inside lookback p=%d end=%d: beg=%d nb=%d err=%v", c.p, c.end, beg, nb, err)
		}
	}
	// Like C, a sub-range restarts the Wilder seed at startIdx-lookback, so
	// it equals the full computation over inputs sliced from that bar.
	for _, p := range []int{1, 4} {
		lb := PLUSDMLookback(p)
		sub, _ := PLUSDM(s.H[10-lb:21], s.L[10-lb:21], p)
		beg, nb, err := plusdmRange(10, 20, s.H, s.L, p, out)
		if err != nil || beg != 10 || nb != 11 {
			t.Fatalf("p=%d sub-range: beg=%d nb=%d err=%v", p, beg, nb, err)
		}
		assertReal(t, lbl("sub-range p=", p), out[:nb], sub[lb:], 0)
	}
	// In-place computation over inHigh or inLow is allowed.
	for _, p := range []int{1, 4} {
		buf := append([]float64(nil), s.H...)
		want, _ := PLUSDM(s.H, s.L, p)
		beg, nb, _ := plusdmRange(0, len(buf)-1, buf, s.L, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place inHigh p=", p), buf, want, 0)
	}
	for _, p := range []int{1, 4} {
		buf := append([]float64(nil), s.L...)
		want, _ := PLUSDM(s.H, s.L, p)
		beg, nb, _ := plusdmRange(0, len(buf)-1, s.H, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
