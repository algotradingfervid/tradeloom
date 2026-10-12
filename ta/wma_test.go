package ta

import (
	"errors"
	"math"
	"testing"
)

func TestWMAParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := WMA(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := WMALookback(p); lb != -1 {
			t.Errorf("WMALookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := wmaRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 30, 100000} {
		if _, err := WMA(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := WMALookback(p); lb != p-1 {
			t.Errorf("WMALookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	if out, err := WMA(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := WMA([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestWMALengths(t *testing.T) {
	for _, period := range []int{1, 2, 3, 5, 30} {
		lb := WMALookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := WMA(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("WMA ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestWMAKnownValues(t *testing.T) {
	nan := math.NaN()
	// Period 3, weights 1,2,3, divider 6:
	// bar2: (1+4+9)/6 = 14/6; bar3: (2+6+12)/6 = 20/6; bar4: (3+8+15)/6 = 26/6;
	// bar5: (4+10+18)/6 = 32/6.
	in := []float64{1, 2, 3, 4, 5, 6}
	out, err := WMA(in, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "WMA(1..6,3)", out, []float64{nan, nan, 14.0 / 6, 20.0 / 6, 26.0 / 6, 32.0 / 6}, 1e-12)

	// Non-linear series, period 2 (weights 1,2, divider 3).
	in2 := []float64{10, 4, 7, 1}
	out, _ = WMA(in2, 2)
	assertReal(t, "WMA period 2", out, []float64{nan, (10 + 8) / 3.0, (4 + 14) / 3.0, (7 + 2) / 3.0}, 1e-12)

	// Period 1 is the identity.
	out, _ = WMA(in, 1)
	assertReal(t, "WMA period 1", out, in, 0)

	// A constant series averages to itself, also across re-anchor points
	// (every 8*period bars).
	c := fixtures(300)[5].Real()
	out, _ = WMA(c, 7)
	for i := 6; i < len(out); i++ {
		if math.Abs(out[i]-100) > 1e-12 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Long series crossing many re-anchor points matches a direct computation.
	w := fixtures(1000)[0].Real()
	for _, p := range []int{2, 5, 14} {
		out, _ = WMA(w, p)
		div := float64(p*(p+1)) / 2
		for i := p - 1; i < len(w); i++ {
			s := 0.0
			for k := 0; k < p; k++ {
				s += w[i-p+1+k] * float64(k+1)
			}
			if !closeEnough(out[i], s/div, 1e-10) {
				t.Fatalf("direct p=%d [%d]: got %g want %g", p, i, out[i], s/div)
			}
		}
	}

	// Output stays within the window's min/max (convex combination).
	for i := 13; i < len(w); i++ {
		lo, hi := w[i], w[i]
		for k := i - 13; k <= i; k++ {
			lo, hi = math.Min(lo, w[k]), math.Max(hi, w[k])
		}
		o, _ := WMA(w[i-13:i+1], 14)
		if o[13] < lo-1e-9 || o[13] > hi+1e-9 {
			t.Fatalf("bounds [%d]: %g not in [%g,%g]", i, o[13], lo, hi)
		}
	}
}

func TestWMARangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := wmaRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := wmaRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := wmaRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := wmaRange(0, 1, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := wmaRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := wmaRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Sub-range values equal the full computation.
	full, _ := WMA(in, 4)
	beg, nb, _ = wmaRange(10, 20, in, 4, out)
	if beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d", beg, nb)
	}
	assertReal(t, "sub-range", out[:nb], full[10:21], 1e-12)
	// In-place computation is allowed (both period 1 and >1).
	for _, p := range []int{1, 4} {
		long := fixtures(200)[1].Real()
		buf := append([]float64(nil), long...)
		want, _ := WMA(long, p)
		beg, nb, _ = wmaRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
