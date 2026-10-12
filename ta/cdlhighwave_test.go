package ta

import (
	"errors"
	"testing"
)

// cdlhighwaveBase returns n identical background bars: open 100, close 102,
// high 103, low 99 (real body 2, upper shadow 1, lower shadow 1).
func cdlhighwaveBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

func TestCDLHIGHWAVEParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLHIGHWAVE(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLHIGHWAVE(s.O[:10], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLHIGHWAVE(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLHIGHWAVE(s.O, s.H, s.L, s.C[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLHIGHWAVE(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLHIGHWAVE([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLHIGHWAVELookback(); lb != 10 {
		t.Errorf("default lookback = %d, want 10", lb)
	}
}

func TestCDLHIGHWAVELookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct{ body, shadow, want int }{
		{0, 0, 0}, {1, 0, 1}, {0, 7, 7}, {5, 30, 30}, {30, 5, 30}, {100, 100, 100},
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		b := GetCandleSetting(BodyShort)
		s := GetCandleSetting(ShadowVeryLong)
		if err := SetCandleSettings(BodyShort, b.RangeType, tc.body, b.Factor); err != nil {
			t.Fatal(err)
		}
		if err := SetCandleSettings(ShadowVeryLong, s.RangeType, tc.shadow, s.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLHIGHWAVELookback(); lb != tc.want {
			t.Errorf("BodyShort %d ShadowVeryLong %d: lookback %d, want %d", tc.body, tc.shadow, lb, tc.want)
		}
	}
	// Unrelated settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	d := GetCandleSetting(BodyLong)
	_ = SetCandleSettings(BodyLong, d.RangeType, 40, d.Factor)
	if lb := CDLHIGHWAVELookback(); lb != 10 {
		t.Errorf("BodyLong 40: lookback %d, want 10", lb)
	}

	// Both periods 0: lookback 0, each bar compared with its own ranges.
	// BodyShort (RealBody, factor 1): body < body is never true, so use
	// HighLow for the body average instead.
	RestoreAllCandleDefaultSettings()
	_ = SetCandleSettings(BodyShort, RangeHighLow, 0, 0.5)
	o := []float64{100, 100}
	h := []float64{104, 101}
	l := []float64{96, 99}
	c := []float64{101, 101} // bar0: body 1 < 4, shadows 3 > 2; bar1: upper 0
	out, err := CDLHIGHWAVE(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "period 0", out, []int{100, 0})
	out, err = CDLHIGHWAVE(o[:1], h[:1], l[:1], c[:1])
	if err != nil || len(out) != 1 || out[0] != 100 {
		t.Errorf("n=1 with lookback 0: out=%v err=%v", out, err)
	}
}

func TestCDLHIGHWAVELengths(t *testing.T) {
	lb := CDLHIGHWAVELookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLHIGHWAVE(s.O, s.H, s.L, s.C)
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

func TestCDLHIGHWAVEKnownValues(t *testing.T) {
	// Background bars have real body 2, so the default BodyShort threshold
	// is 1.0 * 2 = 2 (body must be < 2). ShadowVeryLong has period 0 and
	// factor 2 on RealBody: each shadow must exceed 2 * the bar's own body.
	type bar struct{ o, h, l, c float64 }
	cases := []struct {
		name string
		b    bar
		want int
	}{
		{"white high wave", bar{100, 105, 96, 101}, 100},
		{"black high wave", bar{101, 105, 96, 100}, -100},
		{"doji with shadows", bar{100, 101, 99, 100}, 100},
		{"body at threshold", bar{100, 108, 92, 102}, 0},         // 2 < 2 false
		{"upper shadow at threshold", bar{100, 103, 96, 101}, 0}, // 2 > 2 false
		{"lower shadow at threshold", bar{100, 105, 98, 101}, 0},
		{"no upper shadow", bar{100, 101, 96, 101}, 0},
		{"no lower shadow", bar{100, 105, 100, 101}, 0},
		{"just above shadow threshold", bar{100, 103.25, 97.75, 101}, 100},
	}
	for _, tc := range cases {
		o, h, l, c := cdlhighwaveBase(10)
		o = append(o, tc.b.o)
		h = append(h, tc.b.h)
		l = append(l, tc.b.l)
		c = append(c, tc.b.c)
		out, err := CDLHIGHWAVE(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 11)
		want[10] = tc.want
		assertInt(t, "CDLHIGHWAVE "+tc.name, out, want)
	}

	// The BodyShort window slides: one bar with body 12 (index 10) raises
	// the threshold to (9*2+12)/10 = 3 while it is inside the previous-10
	// window. Following bars have body 2.5 with shadows 6.
	o, h, l, c := cdlhighwaveBase(10)
	o = append(o, 100)
	h = append(h, 113)
	l = append(l, 99)
	c = append(c, 112) // shadows 1, not a high wave
	for range 11 {
		o = append(o, 100)
		h = append(h, 108.5)
		l = append(l, 94)
		c = append(c, 102.5)
	}
	out, err := CDLHIGHWAVE(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	want := make([]int, len(o))
	// Bar k (11..20): window k-10..k-1 contains bar 10 (body 12), other
	// bodies 2 or 2.5, so the average is >= 3 > 2.5. Bar 21: window 11..20
	// is all 2.5, so 2.5 < 2.5 is false.
	for k := 11; k <= 20; k++ {
		want[k] = 100
	}
	assertInt(t, "sliding window", out, want)

	// Constant and zero series: body 0 < 0 is false, so nothing fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLHIGHWAVE(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d want 0", s.Name, i, v)
			}
		}
	}
}

func TestCDLHIGHWAVERangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlhighwaveRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlhighwaveRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlhighwaveRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlhighwaveRange(0, 9, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlhighwaveRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlhighwaveRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlhighwaveRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLHIGHWAVE(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlhighwaveRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 10 || nb != 16 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
