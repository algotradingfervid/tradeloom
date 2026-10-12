package ta

import (
	"errors"
	"math"
	"testing"
)

func TestMINUSDMParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := MINUSDM(s.H, s.L, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := MINUSDMLookback(p); lb != -1 {
			t.Errorf("MINUSDMLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := minusdmRange(0, 49, s.H, s.L, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("minusdmRange period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, c := range []struct{ p, lb int }{{1, 1}, {2, 1}, {3, 2}, {14, 13}, {100000, 99999}} {
		if _, err := MINUSDM(s.H, s.L, c.p); err != nil {
			t.Errorf("period %d: unexpected err %v", c.p, err)
		}
		if lb := MINUSDMLookback(c.p); lb != c.lb {
			t.Errorf("MINUSDMLookback(%d) = %d, want %d", c.p, lb, c.lb)
		}
	}
	// Mismatched input lengths.
	if _, err := MINUSDM(s.H, s.L[:49], 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: err = %v", err)
	}
	if _, err := MINUSDM(s.H[:10], s.L, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v", err)
	}
	if _, err := MINUSDM(nil, s.L, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inHigh with non-empty inLow: err = %v", err)
	}
	// nil / empty input.
	if out, err := MINUSDM(nil, nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := MINUSDM([]float64{}, []float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Short buffers in the range form.
	if _, _, err := minusdmRange(0, 10, s.H, s.L[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := minusdmRange(0, 10, s.H, s.L, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := minusdmRange(5, 10, s.H, s.L, 3, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestMINUSDMLengths(t *testing.T) {
	for _, period := range []int{1, 2, 3, 5, 14, 30} {
		lb := MINUSDMLookback(period)
		want := period - 1
		if period == 1 {
			want = 1
		}
		if lb != want {
			t.Fatalf("MINUSDMLookback(%d) = %d, want %d", period, lb, want)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := MINUSDM(s.H, s.L, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("MINUSDM ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestMINUSDMKnownValues(t *testing.T) {
	// One-bar -DM by hand:
	//   bar1: diffP=1,    diffM=1  -> equal, 0 (case 7)
	//   bar2: diffP=-0.5, diffM=-1 -> inside bar, 0
	//   bar3: diffP=1.5,  diffM=2  -> -DM1 = 2
	//   bar4: diffP=-3,   diffM=1  -> -DM1 = 1
	h := []float64{10, 11, 10.5, 12, 9}
	l := []float64{8, 7, 8, 6, 5}
	nan := math.NaN()

	out, err := MINUSDM(h, l, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "MINUSDM p=1", out, []float64{nan, 0, 0, 2, 1}, 0)

	// Period 3: seed = -DM1(bar1)+-DM1(bar2) = 0 at bar 2, then Wilder.
	out, err = MINUSDM(h, l, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "MINUSDM p=3", out, []float64{nan, nan, 0, 2, 2 - 2.0/3 + 1}, 0)

	// Period 2: seed = -DM1(bar1) = 0 at bar 1.
	out, _ = MINUSDM(h, l, 2)
	assertReal(t, "MINUSDM p=2", out, []float64{nan, 0, 0, 2, 2 - 2.0/2 + 1}, 0)

	// A constant series has no directional movement.
	c := fixtures(40)[5]
	for _, p := range []int{1, 7} {
		out, _ = MINUSDM(c.H, c.L, p)
		for i := MINUSDMLookback(p); i < len(out); i++ {
			if out[i] != 0 {
				t.Fatalf("constant p=%d: [%d]=%g", p, i, out[i])
			}
		}
	}

	// A strictly rising series (higher highs, higher lows) has no -DM.
	up := fixtures(60)[7]
	out, _ = MINUSDM(up.H, up.L, 14)
	for i := 13; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("up p=14: [%d]=%g", i, out[i])
		}
	}

	// A strictly falling series: every bar has -DM1 = 1, so the period-1
	// output is exactly 1 after the first bar.
	down := fixtures(60)[8]
	out, _ = MINUSDM(down.H, down.L, 1)
	for i := 1; i < len(out); i++ {
		if out[i] != 1 {
			t.Fatalf("down p=1: [%d]=%g", i, out[i])
		}
	}

	// -DM is never negative.
	for _, s := range fixtures(300) {
		for _, p := range []int{1, 2, 14} {
			out, _ = MINUSDM(s.H, s.L, p)
			for i := MINUSDMLookback(p); i < len(out); i++ {
				if !(out[i] >= 0) {
					t.Fatalf("%s p=%d: [%d]=%g < 0", s.Name, p, i, out[i])
				}
			}
		}
	}
}

func TestMINUSDMUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstMinusDM, 5); err != nil {
		t.Fatal(err)
	}
	if lb := MINUSDMLookback(14); lb != 18 {
		t.Errorf("unst=5 lookback(14) = %d, want 18", lb)
	}
	if lb := MINUSDMLookback(1); lb != 1 {
		t.Errorf("unst=5 lookback(1) = %d, want 1", lb)
	}
	s := fixtures(100)[0]
	out, err := MINUSDM(s.H, s.L, 14)
	if err != nil {
		t.Fatal(err)
	}
	assertLeadingNaN(t, "MINUSDM unst=5", out, 18)
}

func TestMINUSDMRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := minusdmRange(-1, 5, s.H, s.L, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := minusdmRange(5, 4, s.H, s.L, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := minusdmRange(0, -1, s.H, s.L, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	for _, c := range []struct{ p, end int }{{3, 1}, {1, 0}, {14, 12}} {
		beg, nb, err := minusdmRange(0, c.end, s.H, s.L, c.p, out)
		if err != nil || beg != 0 || nb != 0 {
			t.Errorf("inside lookback p=%d end=%d: beg=%d nb=%d err=%v", c.p, c.end, beg, nb, err)
		}
	}
	// Like C, a sub-range restarts the Wilder seed at startIdx-lookback, so
	// it equals the full computation over inputs sliced from that bar.
	for _, p := range []int{1, 4} {
		lb := MINUSDMLookback(p)
		sub, _ := MINUSDM(s.H[10-lb:21], s.L[10-lb:21], p)
		beg, nb, err := minusdmRange(10, 20, s.H, s.L, p, out)
		if err != nil || beg != 10 || nb != 11 {
			t.Fatalf("p=%d sub-range: beg=%d nb=%d err=%v", p, beg, nb, err)
		}
		assertReal(t, lbl("sub-range p=", p), out[:nb], sub[lb:], 0)
	}
	// In-place computation over inLow is allowed.
	for _, p := range []int{1, 4} {
		buf := append([]float64(nil), s.L...)
		want, _ := MINUSDM(s.H, s.L, p)
		beg, nb, _ := minusdmRange(0, len(buf)-1, s.H, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
