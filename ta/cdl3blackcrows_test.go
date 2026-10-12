package ta

import (
	"errors"
	"testing"
)

// cdl3blackcrowsSeries returns 10 white background bars (open 100, close 102,
// high 103, low 99; high-low 4) followed by a white candle and three black
// crows. With default settings (ShadowVeryShort: HighLow, 10, 0.1) the
// very-short-shadow threshold is about 0.4 and every crow's lower shadow is 0.1.
func cdl3blackcrowsSeries() (o, h, l, c []float64) {
	for range 10 {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	o = append(o, 100, 104, 103, 102)
	h = append(h, 105, 104.2, 103.1, 102.1)
	l = append(l, 99.5, 101.9, 100.9, 99.9)
	c = append(c, 104, 102, 101, 100)
	return
}

func TestCDL3BLACKCROWSParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDL3BLACKCROWS(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3BLACKCROWS(s.O[:10], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3BLACKCROWS(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3BLACKCROWS(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDL3BLACKCROWS(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDL3BLACKCROWS([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDL3BLACKCROWSLookback(); lb != 13 {
		t.Errorf("default lookback = %d, want 13", lb)
	}
}

func TestCDL3BLACKCROWSLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, p := range []int{0, 1, 5, 30} {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(ShadowVeryShort)
		if err := SetCandleSettings(ShadowVeryShort, d.RangeType, p, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDL3BLACKCROWSLookback(); lb != p+3 {
			t.Errorf("period %d: lookback %d, want %d", p, lb, p+3)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	d := GetCandleSetting(BodyLong)
	_ = SetCandleSettings(BodyLong, d.RangeType, 40, d.Factor)
	if lb := CDL3BLACKCROWSLookback(); lb != 13 {
		t.Errorf("BodyLong changed: lookback %d, want 13", lb)
	}
	// Period 0: lookback 3; with 3 bars there is no output, with 4 one.
	RestoreAllCandleDefaultSettings()
	d = GetCandleSetting(ShadowVeryShort)
	_ = SetCandleSettings(ShadowVeryShort, d.RangeType, 0, d.Factor)
	o, h, l, c := cdl3blackcrowsSeries()
	out, err := CDL3BLACKCROWS(o[10:13], h[10:13], l[10:13], c[10:13])
	if err != nil || len(out) != 3 || out[0] != 0 || out[1] != 0 || out[2] != 0 {
		t.Errorf("n=3 with lookback 3: out=%v err=%v", out, err)
	}
	// Period 0 compares each crow's lower shadow with 0.1*its own high-low
	// range: 0.1 < 0.23, 0.1 < 0.22, 0.1 < 0.22.
	out, err = CDL3BLACKCROWS(o[10:], h[10:], l[10:], c[10:])
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "period 0", out, []int{0, 0, 0, -100})
}

func TestCDL3BLACKCROWSLengths(t *testing.T) {
	lb := CDL3BLACKCROWSLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDL3BLACKCROWS(s.O, s.H, s.L, s.C)
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
				if v != 0 && v != -100 {
					t.Fatalf("%s n=%d: [%d]=%d, want 0 or -100", s.Name, n, i, v)
				}
			}
		}
	}
}

func TestCDL3BLACKCROWSKnownValues(t *testing.T) {
	type mod func(o, h, l, c []float64)
	cases := []struct {
		name string
		m    mod
		want int
	}{
		{"crows", func(o, h, l, c []float64) {}, -100},
		{"first candle black", func(o, h, l, c []float64) { o[10], c[10] = 104, 100 }, 0},
		{"first candle doji counts as white", func(o, h, l, c []float64) { c[10] = 100 }, -100},
		{"1st crow white", func(o, h, l, c []float64) { o[11], c[11] = 102, 104 }, 0},
		{"3rd crow doji is white", func(o, h, l, c []float64) { c[13] = 102 }, 0},
		{"2nd opens at 1st open", func(o, h, l, c []float64) { o[12] = 104 }, 0},
		{"2nd opens at 1st close", func(o, h, l, c []float64) { o[12] = 102 }, 0},
		{"3rd opens at 2nd open", func(o, h, l, c []float64) { o[13] = 103 }, 0},
		{"3rd opens at 2nd close", func(o, h, l, c []float64) { o[13] = 101 }, 0},
		{"white high equals 1st close", func(o, h, l, c []float64) { h[10] = 102 }, 0},
		{"2nd close not below 1st", func(o, h, l, c []float64) { c[12], l[12] = 102, 101.9 }, 0},
		{"3rd close not below 2nd", func(o, h, l, c []float64) { c[13], l[13] = 101, 100.9 }, 0},
		{"1st lower shadow long", func(o, h, l, c []float64) { l[11] = 101.4 }, 0},
		{"2nd lower shadow long", func(o, h, l, c []float64) { l[12] = 100.4 }, 0},
		{"3rd lower shadow long", func(o, h, l, c []float64) { l[13] = 99.4 }, 0},
	}
	for _, tc := range cases {
		o, h, l, c := cdl3blackcrowsSeries()
		tc.m(o, h, l, c)
		out, err := CDL3BLACKCROWS(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 14)
		want[13] = tc.want
		assertInt(t, "CDL3BLACKCROWS "+tc.name, out, want)
	}

	// Constant or zero series have no black candles: never fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDL3BLACKCROWS(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDL3BLACKCROWSRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdl3blackcrowsRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdl3blackcrowsRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdl3blackcrowsRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdl3blackcrowsRange(0, 12, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdl3blackcrowsRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdl3blackcrowsRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	if _, _, err := cdl3blackcrowsRange(0, 29, s.O, s.H, s.L, s.C, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdl3blackcrowsRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDL3BLACKCROWS(s.O, s.H, s.L, s.C)
	beg, nb, err = cdl3blackcrowsRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 13 || nb != 13 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
	// Sub-range on the constructed pattern: the single last bar.
	o, h, l, c := cdl3blackcrowsSeries()
	beg, nb, err = cdl3blackcrowsRange(13, 13, o, h, l, c, out)
	if err != nil || beg != 13 || nb != 1 || out[0] != -100 {
		t.Errorf("single bar: beg=%d nb=%d out0=%d err=%v", beg, nb, out[0], err)
	}
}
