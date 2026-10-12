package ta

import (
	"errors"
	"testing"
)

// cdlladderbottomSeries returns 10 white background bars (open 100, close 102,
// high 103, low 99; high-low 4) followed by a ladder bottom on bars 10..14.
// With default settings (ShadowVeryShort: HighLow, 10, 0.1) the very-short
// threshold at bar 14 averages the ranges of bars 3..12:
// (7*4 + 3 + 3.5 + 4)/10 = 3.85, times 0.1 = 0.385. The 4th candle's upper
// shadow is 106 - 104.5 = 1.5.
func cdlladderbottomSeries() (o, h, l, c []float64) {
	for range 10 {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	o = append(o, 110, 108.5, 107, 104.5, 105)
	h = append(h, 110.5, 109, 107.5, 106, 107.5)
	l = append(l, 107.5, 105.5, 103.5, 102.5, 104.8)
	c = append(c, 108, 106, 104, 103, 107)
	return
}

func TestCDLLADDERBOTTOMParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLLADDERBOTTOM(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLLADDERBOTTOM(s.O[:10], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLLADDERBOTTOM(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLLADDERBOTTOM(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLLADDERBOTTOM(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLLADDERBOTTOM([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLLADDERBOTTOMLookback(); lb != 14 {
		t.Errorf("default lookback = %d, want 14", lb)
	}
}

func TestCDLLADDERBOTTOMLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, p := range []int{0, 1, 5, 30} {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(ShadowVeryShort)
		if err := SetCandleSettings(ShadowVeryShort, d.RangeType, p, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLLADDERBOTTOMLookback(); lb != p+4 {
			t.Errorf("period %d: lookback %d, want %d", p, lb, p+4)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	d := GetCandleSetting(BodyLong)
	_ = SetCandleSettings(BodyLong, d.RangeType, 40, d.Factor)
	if lb := CDLLADDERBOTTOMLookback(); lb != 14 {
		t.Errorf("BodyLong changed: lookback %d, want 14", lb)
	}
	// Period 0: lookback 4; with 4 bars there is no output, with 5 one.
	RestoreAllCandleDefaultSettings()
	d = GetCandleSetting(ShadowVeryShort)
	_ = SetCandleSettings(ShadowVeryShort, d.RangeType, 0, d.Factor)
	o, h, l, c := cdlladderbottomSeries()
	out, err := CDLLADDERBOTTOM(o[10:14], h[10:14], l[10:14], c[10:14])
	if err != nil || len(out) != 4 || out[0] != 0 || out[1] != 0 || out[2] != 0 || out[3] != 0 {
		t.Errorf("n=4 with lookback 4: out=%v err=%v", out, err)
	}
	// Period 0 compares the 4th candle's upper shadow (1.5) with 0.1 times its
	// own high-low range (0.35).
	out, err = CDLLADDERBOTTOM(o[10:], h[10:], l[10:], c[10:])
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "period 0", out, []int{0, 0, 0, 0, 100})
	// Factor 5 with period 0: threshold 1.75 > 1.5, no pattern.
	_ = SetCandleSettings(ShadowVeryShort, d.RangeType, 0, 5)
	out, _ = CDLLADDERBOTTOM(o[10:], h[10:], l[10:], c[10:])
	assertInt(t, "period 0 factor 5", out, []int{0, 0, 0, 0, 0})
}

func TestCDLLADDERBOTTOMLengths(t *testing.T) {
	lb := CDLLADDERBOTTOMLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLLADDERBOTTOM(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			for i, v := range out {
				if i < lb && v != 0 {
					t.Fatalf("%s n=%d: [%d]=%d inside lookback %d", s.Name, n, i, v, lb)
				}
				if v != 0 && v != 100 {
					t.Fatalf("%s n=%d: [%d]=%d, want 0 or 100", s.Name, n, i, v)
				}
			}
		}
	}
}

func TestCDLLADDERBOTTOMKnownValues(t *testing.T) {
	type mod func(o, h, l, c []float64)
	cases := []struct {
		name string
		m    mod
		want int
	}{
		{"ladder bottom", func(o, h, l, c []float64) {}, 100},
		{"1st doji is white", func(o, h, l, c []float64) { c[10] = 110 }, 0},
		{"2nd white", func(o, h, l, c []float64) { c[11] = 109 }, 0},
		{"3rd doji is white", func(o, h, l, c []float64) { c[12] = 107 }, 0},
		{"2nd opens at 1st open", func(o, h, l, c []float64) { o[11] = 110 }, 0},
		{"3rd opens at 2nd open", func(o, h, l, c []float64) { o[12] = 108.5 }, 0},
		{"2nd closes at 1st close", func(o, h, l, c []float64) { c[11] = 108 }, 0},
		{"3rd closes at 2nd close", func(o, h, l, c []float64) { c[12] = 106 }, 0},
		{"4th doji is white", func(o, h, l, c []float64) { c[13] = 104.5 }, 0},
		{"4th upper shadow very short", func(o, h, l, c []float64) { h[13] = 104.8 }, 0},
		{"4th upper shadow just long", func(o, h, l, c []float64) { h[13] = 105 }, 100},
		{"5th black", func(o, h, l, c []float64) { o[14] = 108 }, 0},
		{"5th doji is white", func(o, h, l, c []float64) { o[14] = 107 }, 100},
		{"5th opens at 4th open", func(o, h, l, c []float64) { o[14] = 104.5 }, 0},
		{"5th opens inside 4th body", func(o, h, l, c []float64) { o[14] = 104 }, 0},
		{"5th closes at 4th high", func(o, h, l, c []float64) { c[14] = 106 }, 0},
		{"4th need not open below 3rd", func(o, h, l, c []float64) {
			o[13], h[13], c[13] = 108, 109.5, 107.5
			o[14], h[14], c[14] = 108.2, 110.2, 110
		}, 100},
	}
	for _, tc := range cases {
		o, h, l, c := cdlladderbottomSeries()
		tc.m(o, h, l, c)
		out, err := CDLLADDERBOTTOM(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 15)
		want[14] = tc.want
		assertInt(t, "CDLLADDERBOTTOM "+tc.name, out, want)
	}

	// Constant or zero series have no black candles: never fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLLADDERBOTTOM(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLLADDERBOTTOMRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlladderbottomRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlladderbottomRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlladderbottomRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlladderbottomRange(0, 13, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlladderbottomRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlladderbottomRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	if _, _, err := cdlladderbottomRange(0, 29, s.O, s.H, s.L, s.C, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlladderbottomRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLLADDERBOTTOM(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlladderbottomRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 14 || nb != 12 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
	// Sub-range on the constructed pattern: the single last bar.
	o, h, l, c := cdlladderbottomSeries()
	beg, nb, err = cdlladderbottomRange(14, 14, o, h, l, c, out)
	if err != nil || beg != 14 || nb != 1 || out[0] != 100 {
		t.Errorf("single bar: beg=%d nb=%d out0=%d err=%v", beg, nb, out[0], err)
	}
}
