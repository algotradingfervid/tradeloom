package ta

import (
	"errors"
	"math"
	"testing"
)

func TestKAMAParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := KAMA(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := KAMALookback(p); lb != -1 {
			t.Errorf("KAMALookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := kamaRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 30, 100000} {
		if _, err := KAMA(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
	}
	if lb := KAMALookback(1); lb != 0 {
		t.Errorf("KAMALookback(1) = %d, want 0", lb)
	}
	if lb := KAMALookback(100000); lb != 100000 {
		t.Errorf("KAMALookback(100000) = %d, want 100000", lb)
	}
	if out, err := KAMA(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := KAMA([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestKAMALengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 10, 30} {
		lb := KAMALookback(period)
		want := period
		if period == 1 {
			want = 0
		}
		if lb != want {
			t.Fatalf("KAMALookback(%d) = %d, want %d", period, lb, want)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := KAMA(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("KAMA ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestKAMAKnownValues(t *testing.T) {
	nan := math.NaN()
	fast := 2.0 / 3.0
	slow := 2.0 / 31.0

	// Strictly rising series: efficiency ratio is 1, so sc = fast^2.
	// Period 2, input 1,2,3,4: first KAMA at bar 2 = 2 + (3-2)*sc.
	in := []float64{1, 2, 3, 4}
	out, err := KAMA(in, 2)
	if err != nil {
		t.Fatal(err)
	}
	sc := fast * fast
	k2 := 2 + (3-2)*sc
	k3 := k2 + (4-k2)*sc
	assertReal(t, "KAMA rising p=2", out, []float64{nan, nan, k2, k3}, 1e-15)

	// Period 2, input 1,3,2: change 2, volatility |3-1|+|2-3| = 3, ER = 1/3.
	in = []float64{1, 3, 2}
	out, _ = KAMA(in, 2)
	er := 1.0 / 3.0
	sc = (er*(fast-slow) + slow) * (er*(fast-slow) + slow)
	assertReal(t, "KAMA ER=1/3", out, []float64{nan, nan, 3 + (2-3)*sc}, 1e-15)

	// Period 1 is the identity.
	in = fixtures(40)[0].Real()
	out, _ = KAMA(in, 1)
	assertReal(t, "KAMA period 1", out, in, 0)

	// A constant series stays constant.
	c := fixtures(60)[5].Real()
	out, _ = KAMA(c, 10)
	for i := 10; i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Bounds: KAMA is a convex combination, so it stays within the input's
	// running min/max.
	for _, s := range fixtures(300) {
		in := s.Real()
		out, _ := KAMA(in, 14)
		lo, hi := math.Inf(1), math.Inf(-1)
		for i, v := range in {
			lo, hi = math.Min(lo, v), math.Max(hi, v)
			if i >= 14 {
				tol := 1e-12 * math.Max(1, math.Max(math.Abs(lo), math.Abs(hi)))
				if out[i] < lo-tol || out[i] > hi+tol {
					t.Fatalf("%s: [%d]=%g outside [%g,%g]", s.Name, i, out[i], lo, hi)
				}
			}
		}
	}
}

func TestKAMAUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	SetUnstablePeriod(UnstKAMA, 5)
	if lb := KAMALookback(10); lb != 15 {
		t.Fatalf("lookback with unstable 5 = %d", lb)
	}
	if lb := KAMALookback(1); lb != 5 {
		t.Fatalf("lookback p=1 with unstable 5 = %d", lb)
	}
	in := fixtures(60)[0].Real()
	out, _ := KAMA(in, 10)
	assertLeadingNaN(t, "unstable", out, 15)
	out, _ = KAMA(in, 1)
	assertLeadingNaN(t, "unstable p=1", out, 5)
	for i := 5; i < len(in); i++ {
		if out[i] != in[i] {
			t.Fatalf("p=1 unstable: [%d]=%g want %g", i, out[i], in[i])
		}
	}
}

func TestKAMARangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := kamaRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := kamaRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := kamaRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := kamaRange(0, 2, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := kamaRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := kamaRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// A sub-range restarts the recurrence at startIdx-lookback (like C), so
	// its values differ from the full series; only the indices are checked
	// here (values are checked against C in the oracle tests). A range that
	// starts at the lookback equals the full computation.
	full, _ := KAMA(in, 4)
	beg, nb, err = kamaRange(10, 20, in, 4, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	beg, nb, err = kamaRange(0, 20, in, 4, out)
	if err != nil || beg != 4 || nb != 17 {
		t.Fatalf("range from 0: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "range from 0", out[:nb], full[4:21], 0)
	// In-place computation is allowed.
	for _, p := range []int{1, 2, 4} {
		buf := append([]float64(nil), in...)
		want, _ := KAMA(in, p)
		beg, nb, _ = kamaRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
