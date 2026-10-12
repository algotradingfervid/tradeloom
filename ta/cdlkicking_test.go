package ta

import (
	"errors"
	"testing"
)

// cdlkickingBar is one OHLC bar.
type cdlkickingBar struct{ o, h, l, c float64 }

// cdlkickingSeries returns 10 identical background bars (open 100, high 103,
// low 99, close 102: body 2, high-low range 4) followed by the two pattern
// bars, so the second pattern bar is index 11, the first bar evaluated with
// default settings (lookback 11).
func cdlkickingSeries(b1, b2 cdlkickingBar) (o, h, l, c []float64) {
	for range 10 {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	for _, b := range []cdlkickingBar{b1, b2} {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

// cdlkickingCases are hand-derived with default settings. For the 1st
// pattern bar (10) the BodyLong average is 2 and the ShadowVeryShort average
// is 0.1*4 = 0.4 (bars 0..9). For the 2nd bar (11) both averages include
// bar 10: body (18+body10)/10, shadow 0.1*(36+range10)/10.
var cdlkickingCases = []struct {
	name   string
	b1, b2 cdlkickingBar
	want   int
}{
	// Black marubozu 110->100, then white marubozu gapping above its high.
	// Bar 11: body 10 > 2.8, shadows 0.1 < 0.462.
	{"bullish", cdlkickingBar{110, 110.1, 99.9, 100}, cdlkickingBar{111, 121.1, 110.9, 121}, 100},
	// White marubozu 100->110, then black marubozu gapping below its low.
	{"bearish", cdlkickingBar{100, 110.1, 99.9, 110}, cdlkickingBar{99, 99.1, 88.9, 89}, -100},
	// Exact marubozu (no shadows at all).
	{"bullish no shadows", cdlkickingBar{110, 110, 100, 100}, cdlkickingBar{111, 121, 111, 121}, 100},
	// Gap boundary: low2 equal to high1 is not a gap (strict >).
	{"bullish touching", cdlkickingBar{110, 110.1, 99.9, 100}, cdlkickingBar{111, 121.1, 110.1, 121}, 0},
	// Gap boundary: high2 equal to low1 is not a gap (strict <).
	{"bearish touching", cdlkickingBar{100, 110.1, 99.9, 110}, cdlkickingBar{99, 99.9, 88.9, 89}, 0},
	// Overlapping bars: no gap.
	{"bullish no gap", cdlkickingBar{110, 110.1, 99.9, 100}, cdlkickingBar{105, 115.1, 104.9, 115}, 0},
	// Gap in the wrong direction: black then white gapping down.
	{"black-white gap down", cdlkickingBar{110, 110.1, 99.9, 100}, cdlkickingBar{80, 90.1, 79.9, 90}, 0},
	// Gap in the wrong direction: white then black gapping up.
	{"white-black gap up", cdlkickingBar{100, 110.1, 99.9, 110}, cdlkickingBar{121, 121.1, 110.9, 111}, 0},
	// Same color.
	{"both white", cdlkickingBar{100, 110.1, 99.9, 110}, cdlkickingBar{111, 121.1, 110.9, 121}, 0},
	{"both black", cdlkickingBar{110, 110.1, 99.9, 100}, cdlkickingBar{99, 99.1, 88.9, 89}, 0},
	// 1st upper shadow 0.5 >= 0.4: not a marubozu.
	{"1st upper shadow", cdlkickingBar{110, 110.5, 99.9, 100}, cdlkickingBar{111, 121.1, 110.9, 121}, 0},
	// 1st lower shadow 0.5 >= 0.4.
	{"1st lower shadow", cdlkickingBar{110, 110.1, 99.5, 100}, cdlkickingBar{111, 121.1, 110.9, 121}, 0},
	// 2nd upper shadow 0.5 >= 0.462.
	{"2nd upper shadow", cdlkickingBar{110, 110.1, 99.9, 100}, cdlkickingBar{111, 121.5, 110.9, 121}, 0},
	// 2nd lower shadow 0.5 >= 0.462 (low 110.5 still gaps above 110.1).
	{"2nd lower shadow", cdlkickingBar{110, 110.1, 99.9, 100}, cdlkickingBar{111, 121.1, 110.5, 121}, 0},
	// 1st body 2 equals the average 2: not long (strict >).
	{"1st body equals average", cdlkickingBar{102, 102.1, 99.9, 100}, cdlkickingBar{103, 113.1, 102.9, 113}, 0},
	// 1st body 12: bar-11 average (18+12)/10 = 3. Body 3 is not long.
	{"2nd body equals average", cdlkickingBar{112, 112, 100, 100}, cdlkickingBar{113, 116, 113, 116}, 0},
	{"2nd body above average", cdlkickingBar{112, 112, 100, 100}, cdlkickingBar{113, 116.5, 113, 116.5}, 100},
	// A doji 2nd counts as white (close >= open) but has no body.
	{"2nd doji", cdlkickingBar{110, 110.1, 99.9, 100}, cdlkickingBar{111, 111, 111, 111}, 0},
}

func TestCDLKICKINGParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLKICKING(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLKICKING(s.O[:3], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLKICKING(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLKICKING(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLKICKING(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLKICKING([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLKICKINGLookback(); lb != 11 {
		t.Errorf("default lookback = %d, want 11", lb)
	}
}

func TestCDLKICKINGLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyLong, 20, 21},
		{ShadowVeryShort, 30, 31},
		{BodyLong, 0, 11},        // ShadowVeryShort still 10
		{ShadowVeryShort, 0, 11}, // BodyLong still 10
		{BodyLong, 3, 11},
		{BodyShort, 50, 11},
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLKICKINGLookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Both average periods 0: only the prior candle is needed.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, ShadowVeryShort} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLKICKINGLookback(); lb != 1 {
		t.Errorf("all periods 0: lookback %d, want 1", lb)
	}
	for n := 0; n <= 2; n++ {
		s := fixtures(n)[0]
		out, err := CDLKICKING(s.O, s.H, s.L, s.C)
		if err != nil || len(out) != n {
			t.Errorf("n=%d with lookback 1: out=%v err=%v", n, out, err)
		}
		if n >= 1 && out[0] != 0 {
			t.Errorf("n=%d: [0]=%d inside lookback", n, out[0])
		}
	}
	// With BodyLong period 0 and factor 1 each body is compared with itself:
	// never strictly greater, so the pattern cannot fire.
	o, h, l, c := cdlkickingSeries(cdlkickingCases[0].b1, cdlkickingCases[0].b2)
	out, _ := CDLKICKING(o, h, l, c)
	for i, v := range out {
		if v != 0 {
			t.Errorf("periods 0: [%d]=%d", i, v)
		}
	}
}

func TestCDLKICKINGLengths(t *testing.T) {
	lb := CDLKICKINGLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLKICKING(s.O, s.H, s.L, s.C)
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
				if v != 0 && v != 100 && v != -100 {
					t.Fatalf("%s n=%d: [%d]=%d, want -100, 0 or 100", s.Name, n, i, v)
				}
			}
		}
	}
}

func TestCDLKICKINGKnownValues(t *testing.T) {
	for _, tc := range cdlkickingCases {
		o, h, l, c := cdlkickingSeries(tc.b1, tc.b2)
		out, err := CDLKICKING(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 12)
		want[11] = tc.want
		assertInt(t, "CDLKICKING "+tc.name, out, want)
	}

	// Constant and zero series have zero bodies: never long, never fire.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLKICKING(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLKICKINGRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlkickingRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlkickingRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlkickingRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlkickingRange(0, 10, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlkickingRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlkickingRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlkickingRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLKICKING(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlkickingRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 11 || nb != 15 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
