package ta

import (
	"errors"
	"testing"
)

// cdlshortlineBase returns n identical background bars: open 100, close 102,
// high 103, low 99 (real body 2, upper shadow 1, lower shadow 1).
func cdlshortlineBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

func TestCDLSHORTLINEParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLSHORTLINE(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLSHORTLINE(s.O[:10], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLSHORTLINE(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLSHORTLINE(s.O, s.H, s.L, s.C[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLSHORTLINE(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLSHORTLINE([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLSHORTLINELookback(); lb != 10 {
		t.Errorf("default lookback = %d, want 10", lb)
	}
}

func TestCDLSHORTLINELookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct{ body, shadow, want int }{
		{0, 0, 0}, {1, 0, 1}, {0, 7, 7}, {5, 30, 30}, {30, 5, 30}, {100, 100, 100},
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		b := GetCandleSetting(BodyShort)
		s := GetCandleSetting(ShadowShort)
		if err := SetCandleSettings(BodyShort, b.RangeType, tc.body, b.Factor); err != nil {
			t.Fatal(err)
		}
		if err := SetCandleSettings(ShadowShort, s.RangeType, tc.shadow, s.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLSHORTLINELookback(); lb != tc.want {
			t.Errorf("BodyShort %d ShadowShort %d: lookback %d, want %d", tc.body, tc.shadow, lb, tc.want)
		}
	}
	// Unrelated settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	d := GetCandleSetting(BodyLong)
	_ = SetCandleSettings(BodyLong, d.RangeType, 40, d.Factor)
	if lb := CDLSHORTLINELookback(); lb != 10 {
		t.Errorf("BodyLong 40: lookback %d, want 10", lb)
	}

	// Both periods 0: lookback 0, each bar compared with its own ranges.
	// BodyShort on HighLow factor 0.5: body < half the range. ShadowShort on
	// HighLow factor 0.4: each shadow < 0.4 of the range. All bars have range
	// 10, so body < 5 and each shadow < 4.
	RestoreAllCandleDefaultSettings()
	_ = SetCandleSettings(BodyShort, RangeHighLow, 0, 0.5)
	_ = SetCandleSettings(ShadowShort, RangeHighLow, 0, 0.4)
	o := []float64{100, 104, 100, 100}
	h := []float64{107, 107, 108, 105}
	l := []float64{97, 97, 98, 95}
	c := []float64{104, 100, 104, 105} // bar0 white hit, bar1 black hit, bar2 upper 4 >= 4, bar3 body 5 >= 5
	out, err := CDLSHORTLINE(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "period 0", out, []int{100, -100, 0, 0})
	out, err = CDLSHORTLINE(o[:1], h[:1], l[:1], c[:1])
	if err != nil || len(out) != 1 || out[0] != 100 {
		t.Errorf("n=1 with lookback 0: out=%v err=%v", out, err)
	}
}

func TestCDLSHORTLINELengths(t *testing.T) {
	lb := CDLSHORTLINELookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLSHORTLINE(s.O, s.H, s.L, s.C)
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

func TestCDLSHORTLINEKnownValues(t *testing.T) {
	// Background bars have real body 2 and shadows 1+1, so the default
	// BodyShort threshold is 1.0 * 2 = 2 (body must be < 2) and the default
	// ShadowShort threshold is 1.0 * 2 / 2 = 1 (each shadow must be < 1).
	type bar struct{ o, h, l, c float64 }
	cases := []struct {
		name string
		b    bar
		want int
	}{
		{"white short line", bar{100, 101.5, 99.5, 101}, 100},
		{"black short line", bar{101, 101.5, 99.5, 100}, -100},
		{"doji is white", bar{100, 100.5, 99.5, 100}, 100},
		{"flat bar", bar{100, 100, 100, 100}, 100},           // 0 < 2, 0 < 1, 0 < 1
		{"body at threshold", bar{100, 102.5, 99.5, 102}, 0}, // 2 < 2 false
		{"body just below", bar{100, 102.25, 99.75, 101.75}, 100},
		{"upper shadow at threshold", bar{100, 102, 99.5, 101}, 0}, // 1 < 1 false
		{"lower shadow at threshold", bar{100, 101.5, 99, 101}, 0},
		{"black upper at threshold", bar{101, 102, 99.5, 100}, 0},
		{"black lower at threshold", bar{101, 101.5, 99, 100}, 0},
		{"long body", bar{100, 104, 100, 104}, 0},
	}
	for _, tc := range cases {
		o, h, l, c := cdlshortlineBase(10)
		o = append(o, tc.b.o)
		h = append(h, tc.b.h)
		l = append(l, tc.b.l)
		c = append(c, tc.b.c)
		out, err := CDLSHORTLINE(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 11)
		want[10] = tc.want
		assertInt(t, "CDLSHORTLINE "+tc.name, out, want)
	}

	// The windows slide: bar 10 has body 12 and no shadows (not short), then
	// bars of body 3 with shadows 0.125 each. For bar k in 11..21 the body
	// average over k-10..k-1 is (19+k)/10 for k <= 20 (3 at k = 21), so the
	// body is short only for 12 <= k <= 20; the shadow threshold is
	// (37.25-1.75k)/20 for k <= 20, above 0.125 only for k <= 19.
	o, h, l, c := cdlshortlineBase(10)
	o = append(o, 100)
	h = append(h, 112)
	l = append(l, 100)
	c = append(c, 112)
	for range 11 {
		o = append(o, 100)
		h = append(h, 103.125)
		l = append(l, 99.875)
		c = append(c, 103)
	}
	out, err := CDLSHORTLINE(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	want := make([]int, len(o))
	for k := 12; k <= 19; k++ {
		want[k] = 100
	}
	assertInt(t, "sliding window", out, want)

	// Constant and zero series: averages are 0 and 0 < 0 is false, so
	// nothing fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLSHORTLINE(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d want 0", s.Name, i, v)
			}
		}
	}
}

func TestCDLSHORTLINERangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlshortlineRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlshortlineRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlshortlineRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlshortlineRange(0, 9, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlshortlineRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlshortlineRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlshortlineRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLSHORTLINE(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlshortlineRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 10 || nb != 16 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
