package ta

import (
	"errors"
	"testing"
)

// cdllonglineBase returns n identical background bars: open 100, close 102,
// high 103, low 99 (real body 2, upper shadow 1, lower shadow 1).
func cdllonglineBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

func TestCDLLONGLINEParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLLONGLINE(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLLONGLINE(s.O[:10], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLLONGLINE(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLLONGLINE(s.O, s.H, s.L, s.C[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLLONGLINE(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLLONGLINE([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLLONGLINELookback(); lb != 10 {
		t.Errorf("default lookback = %d, want 10", lb)
	}
}

func TestCDLLONGLINELookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct{ body, shadow, want int }{
		{0, 0, 0}, {1, 0, 1}, {0, 7, 7}, {5, 30, 30}, {30, 5, 30}, {100, 100, 100},
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		b := GetCandleSetting(BodyLong)
		s := GetCandleSetting(ShadowShort)
		if err := SetCandleSettings(BodyLong, b.RangeType, tc.body, b.Factor); err != nil {
			t.Fatal(err)
		}
		if err := SetCandleSettings(ShadowShort, s.RangeType, tc.shadow, s.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLLONGLINELookback(); lb != tc.want {
			t.Errorf("BodyLong %d ShadowShort %d: lookback %d, want %d", tc.body, tc.shadow, lb, tc.want)
		}
	}
	// Unrelated settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	d := GetCandleSetting(BodyShort)
	_ = SetCandleSettings(BodyShort, d.RangeType, 40, d.Factor)
	if lb := CDLLONGLINELookback(); lb != 10 {
		t.Errorf("BodyShort 40: lookback %d, want 10", lb)
	}

	// Both periods 0: lookback 0, each bar compared with its own ranges.
	// BodyLong on HighLow factor 0.5: body > half the range. ShadowShort on
	// HighLow factor 0.25: each shadow < a quarter of the range.
	RestoreAllCandleDefaultSettings()
	_ = SetCandleSettings(BodyLong, RangeHighLow, 0, 0.5)
	_ = SetCandleSettings(ShadowShort, RangeHighLow, 0, 0.25)
	o := []float64{100, 104, 100}
	h := []float64{104.5, 104.5, 104}
	l := []float64{99.5, 99.5, 98}
	c := []float64{104, 100, 104} // bar0 white hit, bar1 black hit, bar2 lower 2 >= 1.5
	out, err := CDLLONGLINE(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "period 0", out, []int{100, -100, 0})
	out, err = CDLLONGLINE(o[:1], h[:1], l[:1], c[:1])
	if err != nil || len(out) != 1 || out[0] != 100 {
		t.Errorf("n=1 with lookback 0: out=%v err=%v", out, err)
	}
}

func TestCDLLONGLINELengths(t *testing.T) {
	lb := CDLLONGLINELookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLLONGLINE(s.O, s.H, s.L, s.C)
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

func TestCDLLONGLINEKnownValues(t *testing.T) {
	// Background bars have real body 2 and shadows 1+1, so the default
	// BodyLong threshold is 1.0 * 2 = 2 (body must be > 2) and the default
	// ShadowShort threshold is 1.0 * 2 / 2 = 1 (each shadow must be < 1).
	type bar struct{ o, h, l, c float64 }
	cases := []struct {
		name string
		b    bar
		want int
	}{
		{"white long line", bar{100, 104.5, 99.5, 104}, 100},
		{"black long line", bar{104, 104.5, 99.5, 100}, -100},
		{"white marubozu", bar{100, 104, 100, 104}, 100},
		{"black marubozu", bar{104, 104, 100, 100}, -100},
		{"body at threshold", bar{100, 102.5, 99.5, 102}, 0}, // 2 > 2 false
		{"body just above", bar{100, 102.75, 99.75, 102.5}, 100},
		{"upper shadow at threshold", bar{100, 105, 99.5, 104}, 0}, // 1 < 1 false
		{"lower shadow at threshold", bar{100, 104.5, 99, 104}, 0},
		{"black upper at threshold", bar{104, 105, 99.5, 100}, 0},
		{"black lower at threshold", bar{104, 104.5, 99, 100}, 0},
		{"doji", bar{100, 100.5, 99.5, 100}, 0},
	}
	for _, tc := range cases {
		o, h, l, c := cdllonglineBase(10)
		o = append(o, tc.b.o)
		h = append(h, tc.b.h)
		l = append(l, tc.b.l)
		c = append(c, tc.b.c)
		out, err := CDLLONGLINE(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 11)
		want[10] = tc.want
		assertInt(t, "CDLLONGLINE "+tc.name, out, want)
	}

	// The BodyLong window slides: bar 10 has body 12 and no shadows (a hit),
	// then bars of body 3.5 with shadows 0.25. For bar k in 11..20 the body
	// average over k-10..k-1 is (13.5+1.5k)/10, below 3.5 only for k <= 14;
	// the shadow threshold (34.5-1.5k)/20 stays above 0.25 there.
	o, h, l, c := cdllonglineBase(10)
	o = append(o, 100)
	h = append(h, 112)
	l = append(l, 100)
	c = append(c, 112)
	for range 11 {
		o = append(o, 100)
		h = append(h, 103.75)
		l = append(l, 99.75)
		c = append(c, 103.5)
	}
	out, err := CDLLONGLINE(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	want := make([]int, len(o))
	for k := 10; k <= 14; k++ {
		want[k] = 100
	}
	assertInt(t, "sliding window", out, want)

	// Constant and zero series: body 0 > 0 is false, so nothing fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLLONGLINE(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d want 0", s.Name, i, v)
			}
		}
	}
}

func TestCDLLONGLINERangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdllonglineRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdllonglineRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdllonglineRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdllonglineRange(0, 9, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdllonglineRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdllonglineRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdllonglineRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLLONGLINE(s.O, s.H, s.L, s.C)
	beg, nb, err = cdllonglineRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 10 || nb != 16 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
