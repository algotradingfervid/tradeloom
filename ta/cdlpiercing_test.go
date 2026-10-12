package ta

import (
	"errors"
	"testing"
)

// cdlpiercingSeries returns 10 white background bars (open 100, close 102,
// high 103, low 99: real body 2) followed by a long black candle (open 104,
// close 100, high 104.5, low 99.5: body 4 > the default BodyLong average 2,
// midpoint 102) and a long white candle opening at 99 (< 99.5) and closing at
// 103, inside the black body and above its midpoint (body 4 > average 2.2).
func cdlpiercingSeries() (o, h, l, c []float64) {
	for range 10 {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	o = append(o, 104, 99)
	h = append(h, 104.5, 103.5)
	l = append(l, 99.5, 98.5)
	c = append(c, 100, 103)
	return
}

func TestCDLPIERCINGParams(t *testing.T) {
	s := fixtures(50)[0]
	if lb := CDLPIERCINGLookback(); lb != 11 {
		t.Errorf("lookback %d, want 11", lb)
	}
	if lb := cdlpiercingLookback(); lb != 11 {
		t.Errorf("internal lookback %d, want 11", lb)
	}
	if _, err := CDLPIERCING(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLPIERCING(s.O[:10], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLPIERCING(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLPIERCING(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLPIERCING(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLPIERCING([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestCDLPIERCINGLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, p := range []int{0, 1, 5, 30} {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(BodyLong)
		if err := SetCandleSettings(BodyLong, d.RangeType, p, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLPIERCINGLookback(); lb != p+1 {
			t.Errorf("period %d: lookback %d, want %d", p, lb, p+1)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	d := GetCandleSetting(ShadowVeryShort)
	_ = SetCandleSettings(ShadowVeryShort, d.RangeType, 40, d.Factor)
	if lb := CDLPIERCINGLookback(); lb != 11 {
		t.Errorf("ShadowVeryShort changed: lookback %d, want 11", lb)
	}
	// Period 0: lookback 1; one bar gives no output, two give one. Each body
	// is compared with factor*its own body, so body > body is false: it never
	// fires with factor 1, and fires with factor 0.5.
	RestoreAllCandleDefaultSettings()
	d = GetCandleSetting(BodyLong)
	_ = SetCandleSettings(BodyLong, d.RangeType, 0, d.Factor)
	o, h, l, c := cdlpiercingSeries()
	out, err := CDLPIERCING(o[10:11], h[10:11], l[10:11], c[10:11])
	if err != nil || len(out) != 1 || out[0] != 0 {
		t.Errorf("n=1 with lookback 1: out=%v err=%v", out, err)
	}
	out, _ = CDLPIERCING(o[10:], h[10:], l[10:], c[10:])
	assertInt(t, "period 0 factor 1", out, []int{0, 0})
	_ = SetCandleSettings(BodyLong, d.RangeType, 0, 0.5)
	out, _ = CDLPIERCING(o[10:], h[10:], l[10:], c[10:])
	assertInt(t, "period 0 factor 0.5", out, []int{0, 100})
}

func TestCDLPIERCINGLengths(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, p := range []int{10, 0, 1, 3} {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(BodyLong)
		_ = SetCandleSettings(BodyLong, d.RangeType, p, d.Factor)
		lb := CDLPIERCINGLookback()
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := CDLPIERCING(s.O, s.H, s.L, s.C)
				if err != nil {
					t.Fatalf("%s n=%d: %v", s.Name, n, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
				}
				for i, v := range out {
					if i < lb && v != 0 {
						t.Fatalf("%s n=%d p=%d: [%d]=%d inside lookback %d", s.Name, n, p, i, v, lb)
					}
					if v != 0 && v != 100 {
						t.Fatalf("%s n=%d p=%d: [%d]=%d, want 0 or 100", s.Name, n, p, i, v)
					}
				}
			}
		}
	}
}

func TestCDLPIERCINGKnownValues(t *testing.T) {
	type mod func(o, h, l, c []float64)
	cases := []struct {
		name string
		m    mod
		want int
	}{
		{"piercing", func(o, h, l, c []float64) {}, 100},
		{"first candle white", func(o, h, l, c []float64) { o[10], c[10] = 100, 104 }, 0},
		{"first body equals average", func(o, h, l, c []float64) { o[10] = 102; c[11] = 101.5 }, 0},
		{"first body just above average", func(o, h, l, c []float64) { o[10] = 102.5; c[11] = 101.5 }, 100},
		// Bar 10: open 102.5, close 100, low 100 (body 2.5, midpoint 101.25);
		// average for bar 11 = (9*2 + 2.5)/10 = 2.05.
		{"second body short", func(o, h, l, c []float64) { o[10], l[10], o[11], c[11] = 102.5, 100, 99.9, 101.3 }, 0},
		{"second body long", func(o, h, l, c []float64) { o[10], l[10], o[11], c[11] = 102.5, 100, 99.9, 102.2 }, 100},
		{"second candle black", func(o, h, l, c []float64) { o[11], c[11] = 103, 99 }, 0},
		{"second candle doji", func(o, h, l, c []float64) { c[11] = 99 }, 0},
		{"open equals prior low", func(o, h, l, c []float64) { o[11] = 99.5 }, 0},
		{"open above prior low", func(o, h, l, c []float64) { o[11] = 99.6 }, 0},
		{"close equals prior open", func(o, h, l, c []float64) { c[11] = 104 }, 0},
		{"close above prior open", func(o, h, l, c []float64) { c[11] = 104.1 }, 0},
		{"close just under prior open", func(o, h, l, c []float64) { c[11] = 103.99 }, 100},
		{"close at midpoint", func(o, h, l, c []float64) { c[11] = 102 }, 0},
		{"close just above midpoint", func(o, h, l, c []float64) { c[11] = 102.01 }, 100},
		{"close below midpoint", func(o, h, l, c []float64) { c[11] = 101.5 }, 0},
	}
	for _, tc := range cases {
		o, h, l, c := cdlpiercingSeries()
		tc.m(o, h, l, c)
		out, err := CDLPIERCING(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 12)
		want[11] = tc.want
		assertInt(t, "CDLPIERCING "+tc.name, out, want)
	}

	// Constant or zero series have no black candles: never fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLPIERCING(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLPIERCINGRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlpiercingRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlpiercingRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlpiercingRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlpiercingRange(0, 10, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlpiercingRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlpiercingRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	if _, _, err := cdlpiercingRange(0, 29, s.O, s.H, s.L, s.C, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlpiercingRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLPIERCING(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlpiercingRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 11 || nb != 15 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
	// The single pattern bar of the constructed series.
	o, h, l, c := cdlpiercingSeries()
	beg, nb, err = cdlpiercingRange(11, 11, o, h, l, c, out)
	if err != nil || beg != 11 || nb != 1 || out[0] != 100 {
		t.Errorf("single bar: beg=%d nb=%d out0=%d err=%v", beg, nb, out[0], err)
	}
}
