package ta

import (
	"errors"
	"testing"
)

// cdllongleggeddojiBase returns n identical background bars: open 100,
// close 104, high 106, low 96 (high-low 10). With default settings BodyDoji
// (HighLow, period 10, factor 0.1) averages exactly 0.1*10 = 1 against it.
// ShadowLong (RealBody, period 0, factor 1) is the current bar's own body.
func cdllongleggeddojiBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 106)
		l = append(l, 96)
		c = append(c, 104)
	}
	return
}

func TestCDLLONGLEGGEDDOJIParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLLONGLEGGEDDOJI(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLLONGLEGGEDDOJI(s.O[:10], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLLONGLEGGEDDOJI(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLLONGLEGGEDDOJI(s.O, s.H, s.L, s.C[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLLONGLEGGEDDOJI(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLLONGLEGGEDDOJI([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLLONGLEGGEDDOJILookback(); lb != 10 {
		t.Errorf("default lookback = %d, want 10", lb)
	}
}

func TestCDLLONGLEGGEDDOJILookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyDoji, 20, 20},
		{ShadowLong, 30, 30},
		{ShadowLong, 5, 10}, // BodyDoji still 10
		{BodyDoji, 0, 0},    // ShadowLong default period is 0
		{BodyLong, 50, 10},  // unrelated setting
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLLONGLEGGEDDOJILookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// BodyDoji period 0 (ShadowLong already 0): lookback 0, every bar is
	// judged on itself. BodyDoji avg = 0.1 * own range.
	RestoreAllCandleDefaultSettings()
	d := GetCandleSetting(BodyDoji)
	_ = SetCandleSettings(BodyDoji, d.RangeType, 0, d.Factor)
	if lb := CDLLONGLEGGEDDOJILookback(); lb != 0 {
		t.Errorf("all periods 0: lookback %d, want 0", lb)
	}
	// Range 10, body 0 <= 1, shadows 5 > 0.
	out, err := CDLLONGLEGGEDDOJI([]float64{100}, []float64{105}, []float64{95}, []float64{100})
	if err != nil || len(out) != 1 || out[0] != 100 {
		t.Errorf("n=1 long-legged with lookback 0: out=%v err=%v", out, err)
	}
	// Range 10, body 2 > 1.
	out, err = CDLLONGLEGGEDDOJI([]float64{100}, []float64{106}, []float64{96}, []float64{102})
	if err != nil || len(out) != 1 || out[0] != 0 {
		t.Errorf("n=1 big body with lookback 0: out=%v err=%v", out, err)
	}
}

func TestCDLLONGLEGGEDDOJILengths(t *testing.T) {
	lb := CDLLONGLEGGEDDOJILookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLLONGLEGGEDDOJI(s.O, s.H, s.L, s.C)
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

func TestCDLLONGLEGGEDDOJIKnownValues(t *testing.T) {
	// BodyDoji avg is exactly 1 against the background; ShadowLong avg is the
	// tested bar's own body. Every boundary below is exact.
	type bar struct{ o, h, l, c float64 }
	cases := []struct {
		name string
		b    bar
		want int
	}{
		{"classic", bar{100, 105, 95, 100}, 100},                 // body 0, shadows 5 > 0
		{"dragonfly shape", bar{100, 100, 95, 100}, 100},         // lower 5 > 0
		{"gravestone shape", bar{100, 105, 100, 100}, 100},       // upper 5 > 0
		{"four price", bar{100, 100, 100, 100}, 0},               // shadows 0 not > 0
		{"white body equal avg", bar{100, 103, 97, 101}, 100},    // body 1 <= 1, shadows 2 > 1
		{"black body equal avg", bar{101, 103, 97, 100}, 100},    // body 1 <= 1
		{"body too big", bar{100, 105, 95, 101.5}, 0},            // body 1.5 > 1
		{"upper equal body", bar{100, 102, 100, 101}, 0},         // upper 1 not > 1
		{"upper just long", bar{100, 102.5, 100, 101}, 100},      // upper 1.5 > 1
		{"lower equal body", bar{100, 101, 99, 101}, 0},          // lower 1 not > 1
		{"black lower just long", bar{101, 101, 98.5, 100}, 100}, // lower from close 1.5 > 1
		{"black upper from open", bar{101, 102, 99, 100}, 0},     // upper 1, lower 1
		{"long white body", bar{100, 106, 96, 104}, 0},
	}
	for _, tc := range cases {
		o, h, l, c := cdllongleggeddojiBase(10)
		o = append(o, tc.b.o)
		h = append(h, tc.b.h)
		l = append(l, tc.b.l)
		c = append(c, tc.b.c)
		out, err := CDLLONGLEGGEDDOJI(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 11)
		want[10] = tc.want
		assertInt(t, "CDLLONGLEGGEDDOJI "+tc.name, out, want)
	}

	// The BodyDoji average excludes the current candle and slides: bar 10 has
	// range 30, so bar 11's average is (9*10+30)/10*0.1 = 1.2 and a body of
	// 1.125 qualifies, though it would not against the plain background.
	o, h, l, c := cdllongleggeddojiBase(10)
	o = append(o, 100, 100)
	h = append(h, 115, 105)
	l = append(l, 85, 95)
	c = append(c, 100, 101.125)
	out, _ := CDLLONGLEGGEDDOJI(o, h, l, c)
	assertInt(t, "sliding average", out, []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100, 100})
	// Same bar 11 after a plain background bar: no pattern.
	o[10], h[10], l[10], c[10] = 100, 106, 96, 104
	out, _ = CDLLONGLEGGEDDOJI(o, h, l, c)
	assertInt(t, "plain background", out, []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})

	// All-zero bars have no shadows: never a long legged doji.
	for _, s := range fixtures(100) {
		if s.Name != "zeros" {
			continue
		}
		out, _ := CDLLONGLEGGEDDOJI(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLLONGLEGGEDDOJIRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdllongleggeddojiRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdllongleggeddojiRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdllongleggeddojiRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdllongleggeddojiRange(0, 9, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// First bar after the lookback.
	beg, nb, err = cdllongleggeddojiRange(0, 10, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 10 || nb != 1 {
		t.Errorf("first output: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdllongleggeddojiRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdllongleggeddojiRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdllongleggeddojiRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLLONGLEGGEDDOJI(s.O, s.H, s.L, s.C)
	beg, nb, err = cdllongleggeddojiRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 10 || nb != 16 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
