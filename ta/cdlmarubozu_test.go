package ta

import (
	"errors"
	"testing"
)

// cdlmarubozuBase returns n identical background bars: open 100,
// close 102, high 103, low 99 (real body 2, high-low 4).
func cdlmarubozuBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

func TestCDLMARUBOZUParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLMARUBOZU(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLMARUBOZU(s.O[:10], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLMARUBOZU(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLMARUBOZU(s.O, s.H, s.L, s.C[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLMARUBOZU(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLMARUBOZU([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLMARUBOZULookback(); lb != 10 {
		t.Errorf("default lookback = %d, want 10", lb)
	}
}

func TestCDLMARUBOZULookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyLong, 20, 20},
		{ShadowVeryShort, 30, 30},
		{BodyLong, 0, 10},        // ShadowVeryShort still 10
		{ShadowVeryShort, 0, 10}, // BodyLong still 10
		{BodyShort, 50, 10},      // unrelated setting
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLMARUBOZULookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Both average periods 0: lookback 0, every bar is judged on itself.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, ShadowVeryShort} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLMARUBOZULookback(); lb != 0 {
		t.Errorf("all periods 0: lookback %d, want 0", lb)
	}
	// Body 3 > factor 1.0 * own body 3? No: equal, so no pattern.
	out, err := CDLMARUBOZU([]float64{100}, []float64{103}, []float64{99}, []float64{103})
	if err != nil || len(out) != 1 || out[0] != 0 {
		t.Errorf("n=1 with lookback 0: out=%v err=%v", out, err)
	}
	// Lower the BodyLong factor so the bar's own body qualifies:
	// body 3 > 0.5*3; both shadows 0 < 0.1*3.
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 0.5)
	out, err = CDLMARUBOZU([]float64{100}, []float64{103}, []float64{100}, []float64{103})
	if err != nil || len(out) != 1 || out[0] != 100 {
		t.Errorf("n=1 white with lookback 0: out=%v err=%v", out, err)
	}
	out, err = CDLMARUBOZU([]float64{103}, []float64{103}, []float64{100}, []float64{100})
	if err != nil || len(out) != 1 || out[0] != -100 {
		t.Errorf("n=1 black with lookback 0: out=%v err=%v", out, err)
	}
	// Lower shadow 1 is not < 0.1*4: no pattern.
	out, err = CDLMARUBOZU([]float64{100}, []float64{103}, []float64{99}, []float64{103})
	if err != nil || len(out) != 1 || out[0] != 0 {
		t.Errorf("n=1 lower shadow with lookback 0: out=%v err=%v", out, err)
	}
}

func TestCDLMARUBOZULengths(t *testing.T) {
	lb := CDLMARUBOZULookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLMARUBOZU(s.O, s.H, s.L, s.C)
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

func TestCDLMARUBOZUKnownValues(t *testing.T) {
	// Background bars: body 2, high-low 4, so with default settings
	// BodyLong avg = 2 and ShadowVeryShort avg = 0.1*4 = 0.4.
	type bar struct{ o, h, l, c float64 }
	cases := []struct {
		name string
		b    bar
		want int
	}{
		// body 3 > 2; both shadows 0.25 < 0.4.
		{"white", bar{100, 103.25, 99.75, 103}, 100},
		{"white no shadows", bar{100, 103, 100, 103}, 100},
		{"white long lower shadow", bar{100, 103, 90, 103}, 0}, // both ends matter
		{"white long upper shadow", bar{100, 110, 100, 103}, 0},
		{"black", bar{103, 103.25, 99.75, 100}, -100},
		{"black no shadows", bar{103, 103, 100, 100}, -100},
		{"black long upper shadow", bar{103, 110, 100, 100}, 0},
		{"black long lower shadow", bar{103, 103, 90, 100}, 0},
		{"white body equal avg", bar{100, 102, 100, 102}, 0},     // body 2 not > 2
		{"black body equal avg", bar{102, 102, 100, 100}, 0},     // body 2 not > 2
		{"white upper shadow 0.5", bar{100, 103.5, 100, 103}, 0}, // 0.5 not < 0.4
		{"white lower shadow 0.5", bar{100, 103, 99.5, 103}, 0},
		{"black upper shadow 0.5", bar{103, 103.5, 100, 100}, 0},
		{"black lower shadow 0.5", bar{103, 103, 99.5, 100}, 0},
		{"doji", bar{100, 100, 100, 100}, 0},
	}
	for _, tc := range cases {
		o, h, l, c := cdlmarubozuBase(10)
		o = append(o, tc.b.o)
		h = append(h, tc.b.h)
		l = append(l, tc.b.l)
		c = append(c, tc.b.c)
		out, err := CDLMARUBOZU(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 11)
		want[10] = tc.want
		assertInt(t, "CDLMARUBOZU "+tc.name, out, want)
	}

	// The averages exclude the current candle and slide: after a big bar the
	// next bar's BodyLong average rises. Background body 2; bar 10 has body
	// 12 (white marubozu), so bar 11's average is (9*2+12)/10 = 3 and a body
	// of 2.5 no longer qualifies, though it would against the plain background.
	o, h, l, c := cdlmarubozuBase(10)
	o = append(o, 100, 100)
	h = append(h, 112, 102.5)
	l = append(l, 100, 100)
	c = append(c, 112, 102.5)
	out, _ := CDLMARUBOZU(o, h, l, c)
	assertInt(t, "sliding average", out, []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100, 0})

	// A constant series has zero bodies: never a marubozu.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLMARUBOZU(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLMARUBOZURangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlmarubozuRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlmarubozuRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlmarubozuRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlmarubozuRange(0, 9, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// First bar after the lookback.
	beg, nb, err = cdlmarubozuRange(0, 10, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 10 || nb != 1 {
		t.Errorf("first output: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlmarubozuRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlmarubozuRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlmarubozuRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLMARUBOZU(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlmarubozuRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 10 || nb != 16 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
