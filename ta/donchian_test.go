package ta

import (
	"errors"
	"math"
	"testing"
)

func TestDONCHIANParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, _, _, err := DONCHIAN(s.H, s.L, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := DONCHIANLookback(p); lb != -1 {
			t.Errorf("DONCHIANLookback(%d) = %d, want -1", p, lb)
		}
	}
	for _, p := range []int{2, 3, 20, 100000} {
		if _, _, _, err := DONCHIAN(s.H, s.L, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := DONCHIANLookback(p); lb != p-1 {
			t.Errorf("DONCHIANLookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	if _, _, _, err := DONCHIAN(s.H, s.L[:49], 20); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, _, _, err := DONCHIAN(nil, s.L, 20); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil high, non-empty low: err = %v, want ErrBadParam", err)
	}
	if _, _, _, err := DONCHIAN(s.H, nil, 20); !errors.Is(err, ErrBadParam) {
		t.Errorf("non-empty high, nil low: err = %v, want ErrBadParam", err)
	}
	if u, m, l, err := DONCHIAN(nil, nil, 5); err != nil || len(u) != 0 || len(m) != 0 || len(l) != 0 {
		t.Errorf("nil input: u=%v m=%v l=%v err=%v, want empty and nil", u, m, l, err)
	}
	if u, m, l, err := DONCHIAN([]float64{}, []float64{}, 5); err != nil || len(u) != 0 || len(m) != 0 || len(l) != 0 {
		t.Errorf("empty input: u=%v m=%v l=%v err=%v, want empty and nil", u, m, l, err)
	}
}

func TestDONCHIANLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 20, 30} {
		lb := DONCHIANLookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				u, m, l, err := DONCHIAN(s.H, s.L, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(u) != n || len(m) != n || len(l) != n {
					t.Fatalf("%s n=%d p=%d: len(u)=%d len(m)=%d len(l)=%d", s.Name, n, period, len(u), len(m), len(l))
				}
				assertLeadingNaN(t, lbl("DONCHIAN upper ", s.Name, " n=", n, " p=", period), u, lb)
				assertLeadingNaN(t, lbl("DONCHIAN middle ", s.Name, " n=", n, " p=", period), m, lb)
				assertLeadingNaN(t, lbl("DONCHIAN lower ", s.Name, " n=", n, " p=", period), l, lb)
			}
		}
	}
}

// donchianBrute is a direct evaluation of the formula in donchian.md.
func donchianBrute(high, low []float64, p int) (u, m, l []float64) {
	n := len(high)
	u, m, l = nanSlice(n), nanSlice(n), nanSlice(n)
	for i := p - 1; i < n; i++ {
		hi, lo := high[i-p+1], low[i-p+1]
		for j := i - p + 2; j <= i; j++ {
			hi = cMax(hi, high[j])
			lo = cMin(lo, low[j])
		}
		u[i], l[i], m[i] = hi, lo, (hi+lo)/2
	}
	return u, m, l
}

func TestDONCHIANKnownValues(t *testing.T) {
	nan := math.NaN()
	// Period 3, window = the 3 bars ending at the current bar.
	//  bar 2: high {3,5,4} -> 5, low {1,2,0} -> 0, middle 2.5
	//  bar 3: high {5,4,2} -> 5, low {2,0,3} -> 0, middle 2.5
	//  bar 4: high {4,2,1} -> 4 (cached 5 dropped out: rescan), low {0,3,4} -> 0, middle 2
	//  bar 5: high {2,1,6} -> 6, low {3,4,2} -> 2 (cached 0 dropped out: rescan), middle 4
	high := []float64{3, 5, 4, 2, 1, 6}
	low := []float64{1, 2, 0, 3, 4, 2}
	u, m, l, err := DONCHIAN(high, low, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "DONCHIAN upper", u, []float64{nan, nan, 5, 5, 4, 6}, 0)
	assertReal(t, "DONCHIAN middle", m, []float64{nan, nan, 2.5, 2.5, 2, 4}, 0)
	assertReal(t, "DONCHIAN lower", l, []float64{nan, nan, 0, 0, 0, 2}, 0)

	// Constant input: all three bands equal the constant.
	c := fixtures(40)[5]
	u, m, l, _ = DONCHIAN(c.H, c.L, 7)
	for i := 6; i < len(u); i++ {
		if u[i] != 100 || m[i] != 100 || l[i] != 100 {
			t.Fatalf("constant: [%d] u=%g m=%g l=%g", i, u[i], m[i], l[i])
		}
	}

	// Strictly rising: upper is the current high, lower the oldest low in the window.
	r := fixtures(60)[7]
	u, _, l, _ = DONCHIAN(r.H, r.L, 10)
	for i := 9; i < len(u); i++ {
		if u[i] != r.H[i] || l[i] != r.L[i-9] {
			t.Fatalf("up trend: [%d] u=%g l=%g", i, u[i], l[i])
		}
	}

	// Every fixture matches the brute-force formula exactly, and on
	// well-formed price data lower <= middle <= upper.
	for _, s := range fixtures(300) {
		for _, p := range []int{2, 3, 14, 20, 55} {
			u, m, l, _ := DONCHIAN(s.H, s.L, p)
			wu, wm, wl := donchianBrute(s.H, s.L, p)
			assertReal(t, lbl("brute upper ", s.Name, " p=", p), u, wu, 0)
			assertReal(t, lbl("brute middle ", s.Name, " p=", p), m, wm, 0)
			assertReal(t, lbl("brute lower ", s.Name, " p=", p), l, wl, 0)
			for i := p - 1; i < len(u); i++ {
				if !(l[i] <= m[i] && m[i] <= u[i]) {
					t.Fatalf("%s p=%d: [%d] l=%g m=%g u=%g not ordered", s.Name, p, i, l[i], m[i], u[i])
				}
			}
		}
	}
}

func TestDONCHIANRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	u := make([]float64, 30)
	m := make([]float64, 30)
	l := make([]float64, 30)
	if _, _, err := donchianRange(-1, 5, s.H, s.L, 3, u, m, l); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := donchianRange(5, 4, s.H, s.L, 3, u, m, l); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := donchianRange(0, -1, s.H, s.L, 3, u, m, l); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := donchianRange(0, 10, s.H, s.L, 1, u, m, l); !errors.Is(err, ErrBadParam) {
		t.Errorf("period 1: %v", err)
	}
	if _, _, err := donchianRange(0, 10, s.H, s.L, 100001, u, m, l); !errors.Is(err, ErrBadParam) {
		t.Errorf("period 100001: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := donchianRange(0, 1, s.H, s.L, 3, u, m, l)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := donchianRange(0, 10, s.H[:5], s.L, 3, u, m, l); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high: %v", err)
	}
	if _, _, err := donchianRange(0, 10, s.H, s.L[:5], 3, u, m, l); !errors.Is(err, ErrBadParam) {
		t.Errorf("short low: %v", err)
	}
	if _, _, err := donchianRange(0, 10, s.H, s.L, 3, u[:5], m, l); !errors.Is(err, ErrBadParam) {
		t.Errorf("short upper: %v", err)
	}
	if _, _, err := donchianRange(0, 10, s.H, s.L, 3, u, m[:5], l); !errors.Is(err, ErrBadParam) {
		t.Errorf("short middle: %v", err)
	}
	if _, _, err := donchianRange(0, 10, s.H, s.L, 3, u, m, l[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short lower: %v", err)
	}
	// Any two outputs sharing a buffer are rejected, as in C.
	if _, _, err := donchianRange(0, 10, s.H, s.L, 3, u, u, l); !errors.Is(err, ErrBadParam) {
		t.Errorf("upper == middle: %v", err)
	}
	if _, _, err := donchianRange(0, 10, s.H, s.L, 3, u, m, u); !errors.Is(err, ErrBadParam) {
		t.Errorf("upper == lower: %v", err)
	}
	if _, _, err := donchianRange(0, 10, s.H, s.L, 3, u, m, m); !errors.Is(err, ErrBadParam) {
		t.Errorf("middle == lower: %v", err)
	}
	// Sub-range: values match the full computation at the same bars.
	wu, wm, wl, _ := DONCHIAN(s.H, s.L, 4)
	beg, nb, err = donchianRange(1, 20, s.H, s.L, 4, u, m, l)
	if err != nil || beg != 3 || nb != 18 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range upper", u[:nb], wu[3:21], 0)
	assertReal(t, "sub-range middle", m[:nb], wm[3:21], 0)
	assertReal(t, "sub-range lower", l[:nb], wl[3:21], 0)
	// Outputs may alias the inputs.
	h := append([]float64(nil), s.H...)
	lo := append([]float64(nil), s.L...)
	mid := make([]float64, len(h))
	beg, nb, _ = donchianRange(0, len(h)-1, h, lo, 4, h, mid, lo)
	alignReal(h, beg, nb)
	alignReal(mid, beg, nb)
	alignReal(lo, beg, nb)
	assertReal(t, "in-place upper", h, wu, 0)
	assertReal(t, "in-place middle", mid, wm, 0)
	assertReal(t, "in-place lower", lo, wl, 0)
}
