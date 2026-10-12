package ta

import (
	"errors"
	"testing"
)

// cdlkickingbylengthBar is one full candle.
type cdlkickingbylengthBar struct{ o, h, l, c float64 }

// cdlkickingbylengthSeries appends the two pattern bars to 10 background bars
// (open 100, close 102, high 103, low 99: body 2, range 4), so the second
// pattern bar is index 11, the first evaluated bar with default settings.
// Default averages: BodyLong for bar 10 is 2, for bar 11 (18+body10)/10;
// ShadowVeryShort for bar 10 is 0.1*4 = 0.4, for bar 11 0.1*(36+range10)/10.
func cdlkickingbylengthSeries(b1, b2 cdlkickingbylengthBar) (o, h, l, c []float64) {
	for range 10 {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	for _, b := range []cdlkickingbylengthBar{b1, b2} {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

type cdlkickingbylengthCase struct {
	name   string
	b1, b2 cdlkickingbylengthBar
	want   int
}

// cdlkickingbylengthCases are hand-checked against the formula in
// cdlkickingbylength.md and the C source.
func cdlkickingbylengthCases() []cdlkickingbylengthCase {
	type b = cdlkickingbylengthBar
	black1 := b{110, 110.1, 103.9, 104} // body 6, shadows 0.1
	white1 := b{104, 110.1, 103.9, 110}
	return []cdlkickingbylengthCase{
		// Black marubozu, then white marubozu gapping up; white longer.
		{"bull longer 2nd", black1, b{111, 118.1, 110.9, 118}, 100},
		// Bodies equal (6): the first (black) decides.
		{"tie first decides black", black1, b{111, 117.1, 110.9, 117}, -100},
		// White shorter (3): black decides.
		{"black longer 1st", black1, b{111, 114.1, 110.9, 114}, -100},
		// White marubozu, then black marubozu gapping down; black longer.
		{"bear longer 2nd", white1, b{103, 103.1, 95.9, 96}, -100},
		{"tie first decides white", white1, b{103, 103.1, 96.9, 97}, 100},
		{"white longer 1st", white1, b{103, 103.1, 97.9, 98}, 100},
		// Gap missing: low equals previous high (not strictly above).
		{"no gap up", black1, b{111, 118.1, 110.1, 118}, 0},
		{"no gap down", white1, b{103, 103.9, 95.9, 96}, 0},
		// Gap the wrong way for the colors.
		{"gap down after black", black1, b{96, 103.1, 95.9, 103}, 0},
		{"gap up after white", white1, b{111, 117.1, 110.9, 111.5}, 0},
		// Same colors.
		{"both white", white1, b{111, 118.1, 110.9, 118}, 0},
		{"both black", black1, b{103, 103.1, 95.9, 96}, 0},
		// 1st shadows too long (0.5 > 0.4).
		{"1st upper shadow long", b{110, 110.5, 103.9, 104}, b{111, 118.1, 110.9, 118}, 0},
		{"1st lower shadow long", b{110, 110.1, 103.5, 104}, b{111, 118.1, 110.9, 118}, 0},
		// 2nd shadows too long (> 0.422).
		{"2nd upper shadow long", black1, b{111, 118.5, 110.9, 118}, 0},
		{"2nd lower shadow long", black1, b{111, 118.1, 110.5, 118}, 0},
		// 1st body equal to the average (2): not long.
		{"1st body not long", b{106, 106.1, 103.9, 104}, b{107, 114.1, 106.9, 114}, 0},
		// 1st body 6 -> 2nd average (18+6)/10 = 2.4; body 2 is not long.
		{"2nd body not long", black1, b{111, 113.1, 110.9, 113}, 0},
		// 2nd doji counts as white but has no body.
		{"2nd doji", black1, b{111, 111.1, 110.9, 111}, 0},
	}
}

func TestCDLKICKINGBYLENGTHParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLKICKINGBYLENGTH(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLKICKINGBYLENGTH(s.O[:3], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLKICKINGBYLENGTH(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLKICKINGBYLENGTH(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLKICKINGBYLENGTH(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLKICKINGBYLENGTH([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLKICKINGBYLENGTHLookback(); lb != 11 {
		t.Errorf("default lookback = %d, want 11", lb)
	}
}

func TestCDLKICKINGBYLENGTHLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyLong, 20, 21},
		{ShadowVeryShort, 30, 31},
		{BodyLong, 0, 11},        // ShadowVeryShort still 10
		{ShadowVeryShort, 3, 11}, // BodyLong still 10
		{BodyShort, 50, 11},
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLKICKINGBYLENGTHLookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Both average periods 0: only the prior candle is needed.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, ShadowVeryShort} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLKICKINGBYLENGTHLookback(); lb != 1 {
		t.Errorf("all periods 0: lookback %d, want 1", lb)
	}
	for n := 0; n <= 2; n++ {
		s := fixtures(n)[0]
		out, err := CDLKICKINGBYLENGTH(s.O, s.H, s.L, s.C)
		if err != nil || len(out) != n {
			t.Errorf("n=%d with lookback 1: out=%v err=%v", n, out, err)
		}
		if n > 0 && out[0] != 0 {
			t.Errorf("n=%d: [0]=%d inside lookback", n, out[0])
		}
	}
	// With BodyLong period 0 each body is compared with itself (factor 1):
	// never strictly greater, so the pattern cannot fire.
	c := cdlkickingbylengthCases()[0]
	o, h, l, cl := cdlkickingbylengthSeries(c.b1, c.b2)
	out, _ := CDLKICKINGBYLENGTH(o, h, l, cl)
	for i, v := range out {
		if v != 0 {
			t.Errorf("periods 0: [%d]=%d", i, v)
		}
	}
}

func TestCDLKICKINGBYLENGTHLengths(t *testing.T) {
	lb := CDLKICKINGBYLENGTHLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLKICKINGBYLENGTH(s.O, s.H, s.L, s.C)
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

func TestCDLKICKINGBYLENGTHKnownValues(t *testing.T) {
	for _, tc := range cdlkickingbylengthCases() {
		o, h, l, c := cdlkickingbylengthSeries(tc.b1, tc.b2)
		out, err := CDLKICKINGBYLENGTH(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 12)
		want[11] = tc.want
		assertInt(t, "CDLKICKINGBYLENGTH "+tc.name, out, want)
	}

	// Constant and zero series have zero bodies: never long, never fire.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLKICKINGBYLENGTH(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLKICKINGBYLENGTHRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlkickingbylengthRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlkickingbylengthRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlkickingbylengthRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlkickingbylengthRange(0, 10, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlkickingbylengthRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlkickingbylengthRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlkickingbylengthRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLKICKINGBYLENGTH(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlkickingbylengthRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 11 || nb != 15 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
