package ta

import (
	"errors"
	"testing"
)

// cdlconcealbabyswallSeries returns 10 white background bars (open 100, close
// 102, high 103, low 99; high-low 4) followed by a Concealing Baby Swallow at
// bars 10..13. With default settings (ShadowVeryShort: HighLow, 10, 0.1) the
// very-short-shadow threshold is about 0.4: the two marubozus have 0.05
// shadows, the third candle's upper shadow is 2 and reaches 103 > 102 (the
// second close), and the fourth candle spans 98..104, engulfing 99.5..103.
func cdlconcealbabyswallSeries() (o, h, l, c []float64) {
	for range 10 {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	o = append(o, 110, 106, 101, 103.5)
	h = append(h, 110.05, 106.05, 103, 104)
	l = append(l, 105.95, 101.95, 99.5, 98)
	c = append(c, 106, 102, 100, 99)
	return
}

func TestCDLCONCEALBABYSWALLParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLCONCEALBABYSWALL(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLCONCEALBABYSWALL(s.O[:10], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLCONCEALBABYSWALL(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLCONCEALBABYSWALL(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLCONCEALBABYSWALL(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLCONCEALBABYSWALL([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLCONCEALBABYSWALLLookback(); lb != 13 {
		t.Errorf("default lookback = %d, want 13", lb)
	}
}

func TestCDLCONCEALBABYSWALLLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, p := range []int{0, 1, 5, 30} {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(ShadowVeryShort)
		if err := SetCandleSettings(ShadowVeryShort, d.RangeType, p, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLCONCEALBABYSWALLLookback(); lb != p+3 {
			t.Errorf("period %d: lookback %d, want %d", p, lb, p+3)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	d := GetCandleSetting(BodyLong)
	_ = SetCandleSettings(BodyLong, d.RangeType, 40, d.Factor)
	if lb := CDLCONCEALBABYSWALLLookback(); lb != 13 {
		t.Errorf("BodyLong changed: lookback %d, want 13", lb)
	}
	// Period 0: lookback 3; with 3 bars there is no output, with 4 one.
	RestoreAllCandleDefaultSettings()
	d = GetCandleSetting(ShadowVeryShort)
	_ = SetCandleSettings(ShadowVeryShort, d.RangeType, 0, d.Factor)
	o, h, l, c := cdlconcealbabyswallSeries()
	out, err := CDLCONCEALBABYSWALL(o[10:13], h[10:13], l[10:13], c[10:13])
	if err != nil || len(out) != 3 || out[0] != 0 || out[1] != 0 || out[2] != 0 {
		t.Errorf("n=3 with lookback 3: out=%v err=%v", out, err)
	}
	// Period 0 compares each shadow with 0.1*its own high-low range:
	// 0.05 < 0.41 (1st, 2nd) and 2 > 0.35 (3rd upper shadow).
	out, err = CDLCONCEALBABYSWALL(o[10:], h[10:], l[10:], c[10:])
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "period 0", out, []int{0, 0, 0, 100})
}

func TestCDLCONCEALBABYSWALLLengths(t *testing.T) {
	lb := CDLCONCEALBABYSWALLLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLCONCEALBABYSWALL(s.O, s.H, s.L, s.C)
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

func TestCDLCONCEALBABYSWALLKnownValues(t *testing.T) {
	type mod func(o, h, l, c []float64)
	cases := []struct {
		name string
		m    mod
		want int
	}{
		{"swallow", func(o, h, l, c []float64) {}, 100},
		{"1st doji counts as white", func(o, h, l, c []float64) { c[10] = 110 }, 0},
		{"2nd doji counts as white", func(o, h, l, c []float64) { c[11] = 106 }, 0},
		{"3rd white", func(o, h, l, c []float64) { o[12], c[12] = 100, 101 }, 0},
		{"4th doji counts as white", func(o, h, l, c []float64) { c[13] = 103.5 }, 0},
		{"1st lower shadow long", func(o, h, l, c []float64) { l[10] = 105.5 }, 0},
		{"1st upper shadow long", func(o, h, l, c []float64) { h[10] = 110.5 }, 0},
		{"2nd lower shadow long", func(o, h, l, c []float64) { l[11] = 101.5 }, 0},
		{"2nd upper shadow long", func(o, h, l, c []float64) { h[11] = 106.5 }, 0},
		{"3rd body touches 2nd close", func(o, h, l, c []float64) { o[12] = 102 }, 0},
		{"3rd upper shadow too short", func(o, h, l, c []float64) { o[12], h[12] = 101.9, 102.1 }, 0},
		{"3rd upper shadow just long enough", func(o, h, l, c []float64) { o[12], h[12] = 101.9, 102.5 }, 100},
		{"3rd high equals 2nd close", func(o, h, l, c []float64) { h[12] = 102 }, 0},
		{"4th high equals 3rd high", func(o, h, l, c []float64) { h[13] = 103 }, 0},
		{"4th low equals 3rd low", func(o, h, l, c []float64) { l[13] = 99.5 }, 0},
	}
	for _, tc := range cases {
		o, h, l, c := cdlconcealbabyswallSeries()
		tc.m(o, h, l, c)
		out, err := CDLCONCEALBABYSWALL(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 14)
		want[13] = tc.want
		assertInt(t, "CDLCONCEALBABYSWALL "+tc.name, out, want)
	}

	// Constant or zero series have no black candles: never fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLCONCEALBABYSWALL(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLCONCEALBABYSWALLRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlconcealbabyswallRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlconcealbabyswallRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlconcealbabyswallRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlconcealbabyswallRange(0, 12, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlconcealbabyswallRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlconcealbabyswallRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	if _, _, err := cdlconcealbabyswallRange(0, 29, s.O, s.H, s.L, s.C, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlconcealbabyswallRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLCONCEALBABYSWALL(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlconcealbabyswallRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 13 || nb != 13 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
	// Sub-range on the constructed pattern: the single last bar.
	o, h, l, c := cdlconcealbabyswallSeries()
	beg, nb, err = cdlconcealbabyswallRange(13, 13, o, h, l, c, out)
	if err != nil || beg != 13 || nb != 1 || out[0] != 100 {
		t.Errorf("single bar: beg=%d nb=%d out0=%d err=%v", beg, nb, out[0], err)
	}
}
