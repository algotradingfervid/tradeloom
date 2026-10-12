package ta

import (
	"errors"
	"math"
	"testing"
)

func TestSMAParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32} {
		if _, err := SMA(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := SMALookback(p); lb != -1 {
			t.Errorf("SMALookback(%d) = %d, want -1", p, lb)
		}
	}
	for _, p := range []int{1, 2, 100000} {
		if _, err := SMA(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
	}
	if out, err := SMA(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := smaRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
}

func TestSMALengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 30} {
		lb := SMALookback(period)
		if lb != period-1 {
			t.Fatalf("SMALookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := SMA(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("SMA ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestSMAKnownValues(t *testing.T) {
	in := []float64{1, 2, 3, 4, 5, 6}
	out, err := SMA(in, 3)
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	assertReal(t, "SMA(1..6,3)", out, []float64{nan, nan, 2, 3, 4, 5}, 0)

	// Period 1 is the identity.
	out, _ = SMA(in, 1)
	assertReal(t, "SMA period 1", out, in, 0)

	// A constant series averages to itself.
	c := fixtures(40)[5].Real()
	out, _ = SMA(c, 7)
	for i := 6; i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}
}

func TestSMARangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := smaRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := smaRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := smaRange(0, 1, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := SMA(in, 4)
	beg, nb, _ = smaRange(0, len(buf)-1, buf, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
