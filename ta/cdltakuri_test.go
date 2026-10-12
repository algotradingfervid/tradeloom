package ta

import (
	"errors"
	"testing"
)

// cdltakuriBase returns n identical background bars: open 100, close 104,
// high 106, low 96 (high-low 10). With default settings (BodyDoji and
// ShadowVeryShort both HighLow, period 10, factor 0.1) both averages are
// exactly 0.1*10 = 1. ShadowVeryLong defaults to RealBody, period 0, factor
// 2: twice the current candle's own body.
func cdltakuriBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 106)
		l = append(l, 96)
		c = append(c, 104)
	}
	return
}

func TestCDLTAKURIParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLTAKURI(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLTAKURI(s.O[:10], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLTAKURI(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLTAKURI(s.O, s.H, s.L, s.C[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLTAKURI(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLTAKURI([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLTAKURILookback(); lb != 10 {
		t.Errorf("default lookback = %d, want 10", lb)
	}
}

func TestCDLTAKURILookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyDoji, 20, 20},
		{ShadowVeryShort, 30, 30},
		{ShadowVeryLong, 40, 40},
		{ShadowVeryLong, 5, 10},
		{BodyDoji, 0, 10},        // ShadowVeryShort still 10
		{ShadowVeryShort, 0, 10}, // BodyDoji still 10
		{BodyLong, 50, 10},       // unrelated setting
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLTAKURILookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// All three average periods 0: lookback 0, every bar judged on itself.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyDoji, ShadowVeryShort, ShadowVeryLong} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLTAKURILookback(); lb != 0 {
		t.Errorf("all periods 0: lookback %d, want 0", lb)
	}
	// Own range 10: doji/very-short averages 1, very-long 2*0 = 0.
	// body 0 <= 1, upper 0 < 1, lower 10 > 0.
	out, err := CDLTAKURI([]float64{100}, []float64{100}, []float64{90}, []float64{100})
	if err != nil || len(out) != 1 || out[0] != 100 {
		t.Errorf("n=1 takuri with lookback 0: out=%v err=%v", out, err)
	}
	// Gravestone shape: upper 10 not < 1.
	out, err = CDLTAKURI([]float64{100}, []float64{110}, []float64{100}, []float64{100})
	if err != nil || len(out) != 1 || out[0] != 0 {
		t.Errorf("n=1 gravestone with lookback 0: out=%v err=%v", out, err)
	}
}

func TestCDLTAKURILengths(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for k, svlPeriod := range []int{0, 25} {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(ShadowVeryLong)
		_ = SetCandleSettings(ShadowVeryLong, d.RangeType, svlPeriod, d.Factor)
		lb := CDLTAKURILookback()
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := CDLTAKURI(s.O, s.H, s.L, s.C)
				if err != nil {
					t.Fatalf("#%d %s n=%d: %v", k, s.Name, n, err)
				}
				if len(out) != n {
					t.Fatalf("#%d %s n=%d: len(out)=%d", k, s.Name, n, len(out))
				}
				for i, v := range out {
					if i < lb && v != 0 {
						t.Fatalf("#%d %s n=%d: [%d]=%d inside lookback %d", k, s.Name, n, i, v, lb)
					}
					if v != 0 && v != 100 {
						t.Fatalf("#%d %s n=%d: [%d]=%d, want 0 or 100", k, s.Name, n, i, v)
					}
				}
			}
		}
	}
}

func TestCDLTAKURIKnownValues(t *testing.T) {
	// Doji and very-short averages are exactly 1 against the background (see
	// cdltakuriBase); the very-long threshold is 2*body of the bar itself.
	// All values are binary fractions, so every boundary is exact.
	type bar struct{ o, h, l, c float64 }
	cases := []struct {
		name string
		b    bar
		want int
	}{
		{"classic", bar{100, 100, 95, 100}, 100},                  // body 0, upper 0, lower 5 > 0
		{"tiny lower shadow", bar{100, 100, 99.75, 100}, 100},     // lower 0.25 > 2*0
		{"white body equal avg", bar{100, 101, 95, 101}, 100},     // body 1 <= 1, lower 5 > 2
		{"black body equal avg", bar{101, 101, 95, 100}, 100},     // lower from close 5 > 2
		{"lower equal 2*body", bar{100, 101, 98, 101}, 0},         // lower 2 not > 2
		{"lower just over 2*body", bar{100, 101, 97.5, 101}, 100}, // lower 2.5 > 2
		{"black lower equal 2*body", bar{101, 101, 98, 100}, 0},   // lower 100-98 = 2 not > 2
		{"body too big", bar{100, 101.5, 95, 101.5}, 0},           // body 1.5 > 1
		{"short upper shadow", bar{100, 100.5, 95, 100}, 100},     // upper 0.5 < 1
		{"upper shadow equal avg", bar{100, 101, 95, 100}, 0},     // upper 1 not < 1
		{"black upper from open", bar{100.5, 101, 95, 100}, 100},  // upper 0.5, body 0.5, lower 5 > 1
		{"gravestone", bar{100, 105, 100, 100}, 0},
		{"long-legged", bar{100, 105, 95, 100}, 0},
		{"four price", bar{100, 100, 100, 100}, 0}, // lower 0 not > 0
		{"long white body", bar{100, 104, 96, 104}, 0},
	}
	for _, tc := range cases {
		o, h, l, c := cdltakuriBase(10)
		o = append(o, tc.b.o)
		h = append(h, tc.b.h)
		l = append(l, tc.b.l)
		c = append(c, tc.b.c)
		out, err := CDLTAKURI(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 11)
		want[10] = tc.want
		assertInt(t, "CDLTAKURI "+tc.name, out, want)
	}

	// The averages exclude the current candle and slide: bar 10 has range 30,
	// so bar 11's doji average is (9*10+30)/10*0.1 = 1.2 and a body of 1.125
	// qualifies, though it would not against the plain background.
	o, h, l, c := cdltakuriBase(10)
	o = append(o, 100, 100)
	h = append(h, 100, 101.125)
	l = append(l, 70, 95)
	c = append(c, 100, 101.125)
	out, _ := CDLTAKURI(o, h, l, c)
	assertInt(t, "sliding average", out, []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100, 100})
	o[10], h[10], l[10], c[10] = 100, 106, 96, 104
	out, _ = CDLTAKURI(o, h, l, c)
	assertInt(t, "plain background", out, []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})

	// A constant series has zero lower shadows: never a takuri.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLTAKURI(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLTAKURIShadowVeryLongSetting(t *testing.T) {
	// With ShadowVeryLong = HighLow, period 10, factor 0.5 the very-long
	// threshold against the background is 0.5*10 = 5 (and the lookback stays 10).
	defer RestoreAllCandleDefaultSettings()
	if err := SetCandleSettings(ShadowVeryLong, RangeHighLow, 10, 0.5); err != nil {
		t.Fatal(err)
	}
	type bar struct{ o, h, l, c float64 }
	cases := []struct {
		b    bar
		want int
	}{
		{bar{100, 100, 95, 100}, 0},     // lower 5 not > 5
		{bar{100, 100, 94.5, 100}, 100}, // lower 5.5 > 5
	}
	for k, tc := range cases {
		o, h, l, c := cdltakuriBase(10)
		o = append(o, tc.b.o)
		h = append(h, tc.b.h)
		l = append(l, tc.b.l)
		c = append(c, tc.b.c)
		out, err := CDLTAKURI(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 11)
		want[10] = tc.want
		assertInt(t, lbl("svl#", k), out, want)
	}
}

func TestCDLTAKURIRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdltakuriRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdltakuriRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdltakuriRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdltakuriRange(0, 9, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// First bar after the lookback.
	beg, nb, err = cdltakuriRange(0, 10, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 10 || nb != 1 {
		t.Errorf("first output: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdltakuriRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdltakuriRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdltakuriRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLTAKURI(s.O, s.H, s.L, s.C)
	beg, nb, err = cdltakuriRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 10 || nb != 16 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
