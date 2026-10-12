package ta

import (
	"errors"
	"testing"
)

// cdlgravestonedojiBase returns n identical background bars: open 100,
// close 104, high 108, low 98 (high-low 10). With default settings both the
// BodyDoji and the ShadowVeryShort thresholds are 0.1*10 = 1 (exactly).
func cdlgravestonedojiBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 108)
		l = append(l, 98)
		c = append(c, 104)
	}
	return
}

func TestCDLGRAVESTONEDOJIParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLGRAVESTONEDOJI(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLGRAVESTONEDOJI(s.O[:10], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLGRAVESTONEDOJI(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLGRAVESTONEDOJI(s.O, s.H, s.L, s.C[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLGRAVESTONEDOJI(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLGRAVESTONEDOJI([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLGRAVESTONEDOJILookback(); lb != 10 {
		t.Errorf("default lookback = %d, want 10", lb)
	}
}

func TestCDLGRAVESTONEDOJILookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyDoji, 20, 20},
		{ShadowVeryShort, 30, 30},
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
		if lb := CDLGRAVESTONEDOJILookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Both average periods 0: lookback 0, every bar is judged on itself
	// (thresholds 0.1 * its own high-low).
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyDoji, ShadowVeryShort} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLGRAVESTONEDOJILookback(); lb != 0 {
		t.Errorf("all periods 0: lookback %d, want 0", lb)
	}
	// Range 10: body 0 <= 1, lower 0 < 1, upper 10 > 1.
	out, err := CDLGRAVESTONEDOJI([]float64{100}, []float64{110}, []float64{100}, []float64{100})
	if err != nil || len(out) != 1 || out[0] != 100 {
		t.Errorf("n=1 gravestone with lookback 0: out=%v err=%v", out, err)
	}
	// Flat bar: upper 0 not > 0.
	out, err = CDLGRAVESTONEDOJI([]float64{100}, []float64{100}, []float64{100}, []float64{100})
	if err != nil || len(out) != 1 || out[0] != 0 {
		t.Errorf("n=1 flat with lookback 0: out=%v err=%v", out, err)
	}
}

func TestCDLGRAVESTONEDOJILengths(t *testing.T) {
	lb := CDLGRAVESTONEDOJILookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLGRAVESTONEDOJI(s.O, s.H, s.L, s.C)
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

func TestCDLGRAVESTONEDOJIKnownValues(t *testing.T) {
	// Background thresholds: BodyDoji 1, ShadowVeryShort 1.
	type bar struct{ o, h, l, c float64 }
	cases := []struct {
		name string
		b    bar
		want int
	}{
		{"gravestone", bar{100, 105, 100, 100}, 100},
		{"body equal avg", bar{100, 105, 100, 101}, 100},       // 1 <= 1
		{"body above avg", bar{100, 105, 100, 101.5}, 0},       // 1.5 > 1
		{"black doji", bar{100.5, 105, 100, 100}, 100},         // lower uses min(o,c)
		{"short lower shadow", bar{100, 105, 99.5, 100}, 100},  // 0.5 < 1
		{"lower shadow equal avg", bar{100, 105, 99, 100}, 0},  // 1 not < 1
		{"upper shadow equal avg", bar{100, 101, 100, 100}, 0}, // 1 not > 1
		{"upper shadow just above", bar{100, 101.5, 100, 100}, 100},
		{"dragonfly", bar{105, 105, 100, 105}, 0},
		{"flat", bar{100, 100, 100, 100}, 0},
	}
	for _, tc := range cases {
		o, h, l, c := cdlgravestonedojiBase(10)
		o = append(o, tc.b.o)
		h = append(h, tc.b.h)
		l = append(l, tc.b.l)
		c = append(c, tc.b.c)
		out, err := CDLGRAVESTONEDOJI(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 11)
		want[10] = tc.want
		assertInt(t, "CDLGRAVESTONEDOJI "+tc.name, out, want)
	}

	// The averages exclude the current candle and slide: after a bar with
	// range 20 the thresholds of the next bar become 0.1*(9*10+20)/10 = 1.1,
	// so an upper shadow of 1.05 no longer qualifies, though it would against
	// the plain background.
	o, h, l, c := cdlgravestonedojiBase(10)
	o = append(o, 100, 100)
	h = append(h, 120, 101.05)
	l = append(l, 100, 100)
	c = append(c, 100, 100)
	out, _ := CDLGRAVESTONEDOJI(o, h, l, c)
	assertInt(t, "sliding average", out, []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100, 0})

	// A constant series has zero shadows: never a gravestone doji.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLGRAVESTONEDOJI(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLGRAVESTONEDOJIRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlgravestonedojiRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlgravestonedojiRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlgravestonedojiRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	beg, nb, err := cdlgravestonedojiRange(0, 9, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	beg, nb, err = cdlgravestonedojiRange(0, 10, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 10 || nb != 1 {
		t.Errorf("first output: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if _, _, err := cdlgravestonedojiRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlgravestonedojiRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	if _, nb, err := cdlgravestonedojiRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	full, _ := CDLGRAVESTONEDOJI(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlgravestonedojiRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 10 || nb != 16 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
