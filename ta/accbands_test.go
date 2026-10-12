package ta

import (
	"errors"
	"math"
	"testing"
)

func TestACCBANDSParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, _, _, err := ACCBANDS(s.H, s.L, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := ACCBANDSLookback(p); lb != -1 {
			t.Errorf("ACCBANDSLookback(%d) = %d, want -1", p, lb)
		}
	}
	for _, p := range []int{2, 3, 20, 100000} {
		if _, _, _, err := ACCBANDS(s.H, s.L, s.C, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := ACCBANDSLookback(p); lb != p-1 {
			t.Errorf("ACCBANDSLookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	// Mismatched input lengths.
	if _, _, _, err := ACCBANDS(s.H[:49], s.L, s.C, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high: err = %v", err)
	}
	if _, _, _, err := ACCBANDS(s.H, s.L[:49], s.C, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short low: err = %v", err)
	}
	if _, _, _, err := ACCBANDS(s.H, s.L, s.C[:49], 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short close: err = %v", err)
	}
	if _, _, _, err := ACCBANDS(nil, s.L, s.C, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil high with non-empty others: err = %v", err)
	}
	// nil / empty input → empty outputs.
	for _, in := range [][]float64{nil, {}} {
		u, m, l, err := ACCBANDS(in, in, in, 5)
		if err != nil || len(u) != 0 || len(m) != 0 || len(l) != 0 {
			t.Errorf("empty input: u=%v m=%v l=%v err=%v", u, m, l, err)
		}
	}
}

func TestACCBANDSLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 20, 30} {
		lb := ACCBANDSLookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				u, m, l, err := ACCBANDS(s.H, s.L, s.C, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				for k, out := range [][]float64{u, m, l} {
					if len(out) != n {
						t.Fatalf("%s n=%d p=%d out%d: len=%d", s.Name, n, period, k, len(out))
					}
					assertLeadingNaN(t, lbl("ACCBANDS out", k, " ", s.Name, " n=", n, " p=", period), out, lb)
				}
			}
		}
	}
}

func TestACCBANDSKnownValues(t *testing.T) {
	// factor = 4(H-L)/(H+L): bar0 H=3,L=1 → 2 (upper 9, lower -1);
	// bar1 H=5,L=3 → 1 (upper 10, lower 0); bar2 H=7,L=1 → 3 (upper 28, lower -2).
	h := []float64{3, 5, 7}
	l := []float64{1, 3, 1}
	c := []float64{2, 4, 6}
	u, m, lo, err := ACCBANDS(h, l, c, 2)
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	assertReal(t, "upper", u, []float64{nan, 9.5, 19}, 0)
	assertReal(t, "middle", m, []float64{nan, 3, 5}, 0)
	assertReal(t, "lower", lo, []float64{nan, -0.5, -1}, 0)

	// H+L cancels to zero: the bar enters the sums unscaled (H and L).
	h = []float64{1, 3, 1}
	l = []float64{-1, 1, -1}
	c = []float64{0, 2, 0}
	u, m, lo, _ = ACCBANDS(h, l, c, 2)
	assertReal(t, "zero-sum upper", u, []float64{nan, (1 + 9) / 2.0, (9 + 1) / 2.0}, 0)
	assertReal(t, "zero-sum middle", m, []float64{nan, 1, 1}, 0)
	assertReal(t, "zero-sum lower", lo, []float64{nan, (-1 - 1) / 2.0, (-1 - 1) / 2.0}, 0)

	// A constant bar (H == L == C) has factor 0: all three bands equal it.
	cs := fixtures(40)[5]
	u, m, lo, _ = ACCBANDS(cs.H, cs.L, cs.C, 7)
	for i := 6; i < len(u); i++ {
		if u[i] != 100 || m[i] != 100 || lo[i] != 100 {
			t.Fatalf("constant: [%d]=%g,%g,%g", i, u[i], m[i], lo[i])
		}
	}
	// All-zero bars take the degenerate branch and stay zero.
	zs := fixtures(40)[6]
	u, m, lo, _ = ACCBANDS(zs.H, zs.L, zs.C, 5)
	for i := 4; i < len(u); i++ {
		if u[i] != 0 || m[i] != 0 || lo[i] != 0 {
			t.Fatalf("zeros: [%d]=%g,%g,%g", i, u[i], m[i], lo[i])
		}
	}

	// Middle band is SMA(close).
	s := fixtures(200)[0]
	_, m, _, _ = ACCBANDS(s.H, s.L, s.C, 14)
	want, _ := SMA(s.C, 14)
	assertReal(t, "middle == SMA(close)", m, want, 1e-12)

	// Bounds: for positive prices with L <= C <= H, upper >= middle >= lower.
	for _, s := range fixtures(300)[:5] {
		u, m, lo, _ := ACCBANDS(s.H, s.L, s.C, 10)
		for i := 9; i < len(u); i++ {
			if !(u[i] >= m[i]*(1-1e-12) && m[i] >= lo[i]*(1-1e-12)) {
				t.Fatalf("%s bounds [%d]: %g %g %g", s.Name, i, u[i], m[i], lo[i])
			}
		}
	}
}

func TestACCBANDSRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	n := len(s.C)
	u, m, l := make([]float64, n), make([]float64, n), make([]float64, n)
	if _, _, err := accbandsRange(-1, 5, s.H, s.L, s.C, 3, u, m, l); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := accbandsRange(5, 4, s.H, s.L, s.C, 3, u, m, l); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := accbandsRange(0, -1, s.H, s.L, s.C, 3, u, m, l); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := accbandsRange(0, 10, s.H, s.L, s.C, 1, u, m, l); !errors.Is(err, ErrBadParam) {
		t.Errorf("period 1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := accbandsRange(0, 1, s.H, s.L, s.C, 3, u, m, l)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := accbandsRange(0, 10, s.H[:5], s.L, s.C, 3, u, m, l); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high: %v", err)
	}
	if _, _, err := accbandsRange(0, 10, s.H, s.L, s.C[:5], 3, u, m, l); !errors.Is(err, ErrBadParam) {
		t.Errorf("short close: %v", err)
	}
	if _, _, err := accbandsRange(0, 10, s.H, s.L, s.C, 3, u, m[:5], l); !errors.Is(err, ErrBadParam) {
		t.Errorf("short middle out: %v", err)
	}
	// Aliased outputs are rejected, like C.
	if _, _, err := accbandsRange(0, 10, s.H, s.L, s.C, 3, u, u, l); !errors.Is(err, ErrBadParam) {
		t.Errorf("aliased outputs: %v", err)
	}
	if _, _, err := accbandsRange(0, 10, s.H, s.L, s.C, 3, u, m, m); !errors.Is(err, ErrBadParam) {
		t.Errorf("aliased outputs: %v", err)
	}
	// An output may alias an input (every trailing bar is read before it is overwritten).
	wu, wm, wl, _ := ACCBANDS(s.H, s.L, s.C, 4)
	hb := append([]float64(nil), s.H...)
	lb := append([]float64(nil), s.L...)
	cb := append([]float64(nil), s.C...)
	beg, nb, _ = accbandsRange(0, n-1, hb, lb, cb, 4, hb, cb, lb)
	alignReal(hb, beg, nb)
	alignReal(cb, beg, nb)
	alignReal(lb, beg, nb)
	assertReal(t, "in-place upper", hb, wu, 0)
	assertReal(t, "in-place middle", cb, wm, 0)
	assertReal(t, "in-place lower", lb, wl, 0)
}
