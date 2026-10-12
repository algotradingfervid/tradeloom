package ta

import (
	"errors"
	"math"
	"testing"
)

func TestSARParams(t *testing.T) {
	s := fixtures(50)[0]
	bad := [][2]float64{
		{-1e-12, 0.2}, {-0.02, 0.2}, {math.NaN(), 0.2}, {math.Inf(1), 0.2}, {math.Inf(-1), 0.2}, {3.0000001e37, 0.2},
		{0.02, -1e-12}, {0.02, -0.2}, {0.02, math.NaN()}, {0.02, math.Inf(1)}, {0.02, 3.0000001e37},
	}
	for _, p := range bad {
		if _, err := SAR(s.H, s.L, p[0], p[1]); !errors.Is(err, ErrBadParam) {
			t.Errorf("params %v: err = %v, want ErrBadParam", p, err)
		}
		if lb := SARLookback(p[0], p[1]); lb != -1 {
			t.Errorf("SARLookback(%v) = %d, want -1", p, lb)
		}
		if _, _, err := sarRange(0, 49, s.H, s.L, p[0], p[1], make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range params %v: err = %v, want ErrBadParam", p, err)
		}
	}
	good := [][2]float64{{0, 0}, {0.02, 0.2}, {taRealMax, taRealMax}, {0, taRealMax}, {taRealMax, 0}, {0.5, 0.1}}
	for _, p := range good {
		if _, err := SAR(s.H, s.L, p[0], p[1]); err != nil {
			t.Errorf("params %v: unexpected err %v", p, err)
		}
		if lb := SARLookback(p[0], p[1]); lb != 1 {
			t.Errorf("SARLookback(%v) = %d, want 1", p, lb)
		}
	}
	if _, err := SAR(s.H, s.L[:49], 0.02, 0.2); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, err := SAR(nil, s.L, 0.02, 0.2); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil high, non-empty low: err = %v, want ErrBadParam", err)
	}
	if out, err := SAR(nil, nil, 0.02, 0.2); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := SAR([]float64{}, []float64{}, 0.02, 0.2); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestSARLengths(t *testing.T) {
	for _, p := range [][2]float64{{0.02, 0.2}, {0, 0}, {0.01, 0.4}, {0.5, 0.1}, {0.2, 0.2}} {
		lb := SARLookback(p[0], p[1])
		if lb != 1 {
			t.Fatalf("SARLookback(%v) = %d", p, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := SAR(s.H, s.L, p[0], p[1])
				if err != nil {
					t.Fatalf("%s n=%d p=%v: %v", s.Name, n, p, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%v: len(out)=%d", s.Name, n, p, len(out))
				}
				assertLeadingNaN(t, lbl("SAR ", s.Name, " n=", n, " p=", p), out, lb)
			}
		}
	}
}

func TestSARKnownValues(t *testing.T) {
	nan := math.NaN()

	// Rising bars: +DM wins, so long. ep = H[1] = 11, SAR = L[0] = 9.
	// Bar 1 emits 9; then SAR = 9 + 0.02*(11-9) = 9.04 (no new ep).
	// Bar 2 emits 9.04; new high 12 -> ep 12, af 0.04, next SAR 9.1584.
	out, err := SAR([]float64{10, 11, 12, 13}, []float64{9, 10, 11, 12}, 0.02, 0.2)
	if err != nil {
		t.Fatal(err)
	}
	// Bar 3 emits 9.1584.
	assertReal(t, "SAR rising", out, []float64{nan, 9, 9.04, 9.1584}, 1e-12)

	// Reversal: bar 2's low (8) penetrates SAR 9.04, so it switches to short
	// and emits the prior ep (11), clamped to >= yesterday's/today's high.
	out, _ = SAR([]float64{10, 11, 9}, []float64{9, 10, 8}, 0.02, 0.2)
	assertReal(t, "SAR reversal", out, []float64{nan, 9, 11}, 1e-12)

	// Falling first bars: -DM wins, so short; SAR starts at H[0].
	out, _ = SAR([]float64{10, 9}, []float64{9, 8}, 0.02, 0.2)
	assertReal(t, "SAR short start", out, []float64{nan, 10}, 0)

	// An inside bar (+DM and -DM both 0) is long by default: SAR starts at
	// L[0] = 9 and bar 1's low 9.5 does not penetrate it.
	out, _ = SAR([]float64{10, 9.75}, []float64{9, 9.5}, 0.02, 0.2)
	assertReal(t, "SAR tie", out, []float64{nan, 9}, 0)

	// Equal lows also start long, but the low touching the SAR (L[1] <= L[0])
	// reverses immediately to short at the ep H[1].
	out, _ = SAR([]float64{10, 10}, []float64{9, 9}, 0.02, 0.2)
	assertReal(t, "SAR tie reversal", out, []float64{nan, 10}, 0)

	// Acceleration above the maximum is clamped to the maximum.
	h := fixtures(200)[0].H
	l := fixtures(200)[0].L
	a, _ := SAR(h, l, 0.5, 0.1)
	b, _ := SAR(h, l, 0.1, 0.1)
	assertReal(t, "SAR acc>max clamp", a, b, 0)

	// A constant series stays at the constant.
	c := fixtures(40)[5]
	out, _ = SAR(c.H, c.L, 0.02, 0.2)
	for i := 1; i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Bounds: in a strict uptrend the SAR stays long, at or below each low;
	// in a strict downtrend it stays short, at or above each high.
	u := fixtures(300)[7]
	out, _ = SAR(u.H, u.L, 0.02, 0.2)
	for i := 1; i < len(out); i++ {
		if out[i] > u.L[i] {
			t.Fatalf("uptrend: SAR[%d]=%g above low %g", i, out[i], u.L[i])
		}
	}
	d := fixtures(300)[8]
	out, _ = SAR(d.H, d.L, 0.02, 0.2)
	for i := 1; i < len(out); i++ {
		if out[i] < d.H[i] {
			t.Fatalf("downtrend: SAR[%d]=%g below high %g", i, out[i], d.H[i])
		}
	}
}

func TestSARRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := sarRange(-1, 5, s.H, s.L, 0.02, 0.2, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := sarRange(5, 4, s.H, s.L, 0.02, 0.2, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := sarRange(0, -1, s.H, s.L, 0.02, 0.2, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := sarRange(0, 0, s.H, s.L, 0.02, 0.2, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := sarRange(0, 10, s.H[:5], s.L, 0.02, 0.2, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high: %v", err)
	}
	if _, _, err := sarRange(0, 10, s.H, s.L[:5], 0.02, 0.2, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short low: %v", err)
	}
	if _, _, err := sarRange(0, 10, s.H, s.L, 0.02, 0.2, out[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short out: %v", err)
	}
	// A single bar.
	beg, nb, err = sarRange(7, 7, s.H, s.L, 0.02, 0.2, out)
	if err != nil || beg != 7 || nb != 1 {
		t.Errorf("single bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Full range in place over a copy of inHigh matches the public form.
	want, _ := SAR(s.H, s.L, 0.02, 0.2)
	buf := append([]float64(nil), s.H...)
	beg, nb, _ = sarRange(0, len(buf)-1, buf, s.L, 0.02, 0.2, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
