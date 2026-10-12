package ta

import (
	"errors"
	"testing"
)

// cdlhangingmanBase returns n identical background bars: open 100, close 102,
// high 103, low 99 (real body 2, high-low 4).
func cdlhangingmanBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

func TestCDLHANGINGMANParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLHANGINGMAN(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLHANGINGMAN(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLHANGINGMAN(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLHANGINGMAN(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLHANGINGMAN([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLHANGINGMANLookback(); lb != 11 {
		t.Errorf("default lookback = %d, want 11", lb)
	}
}

func TestCDLHANGINGMANLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyShort, 20, 21},
		{ShadowLong, 30, 31},
		{ShadowVeryShort, 0, 11}, // BodyShort still 10
		{Near, 40, 41},
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLHANGINGMANLookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Every average period 0: only the prior candle is needed.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyShort, ShadowLong, ShadowVeryShort, Near} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLHANGINGMANLookback(); lb != 1 {
		t.Errorf("all periods 0: lookback %d, want 1", lb)
	}
	o, h, l, c := cdlhangingmanBase(1)
	out, err := CDLHANGINGMAN(o, h, l, c)
	if err != nil || len(out) != 1 || out[0] != 0 {
		t.Errorf("n=1 with lookback 1: out=%v err=%v", out, err)
	}
}

func TestCDLHANGINGMANLengths(t *testing.T) {
	lb := CDLHANGINGMANLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLHANGINGMAN(s.O, s.H, s.L, s.C)
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

func TestCDLHANGINGMANKnownValues(t *testing.T) {
	// Background bars: body 2, high-low 4, prior high 103, so with default
	// settings BodyShort avg = 2, ShadowVeryShort avg = 0.1*4 = 0.4,
	// Near avg (prior 5 bars) = 0.2*4 = 0.8 (threshold 103-0.8 = 102.2),
	// ShadowLong = current body.
	type bar struct{ o, h, l, c float64 }
	cases := []struct {
		name string
		b    bar
		want int
	}{
		// body 0.5 < 2; lower 1.5 > 0.5; upper 0.1 < 0.4; min 102.5 >= 102.2.
		{"hanging man", bar{102.5, 103.1, 101, 103}, -100},
		{"hanging man black", bar{103, 103.1, 101, 102.5}, -100},
		{"body too big", bar{101, 103.1, 100.5, 103}, 0},                  // body 2 not < 2
		{"lower shadow short", bar{102.5, 103.1, 102.2, 103}, 0},          // lower 0.3 not > 0.5
		{"lower shadow equal body", bar{102.5, 103.1, 102, 103}, 0},       // lower 0.5 not > 0.5
		{"upper shadow long", bar{102.5, 103.5, 101, 103}, 0},             // upper 0.5 not < 0.4
		{"body too low", bar{102, 102.2, 100.5, 102.1}, 0},                // min 102 < 102.2
		{"body near threshold", bar{102.25, 102.85, 100.5, 102.75}, -100}, // min 102.25 >= 102.2
		{"hammer shape below prior high", bar{99.5, 100.1, 97.5, 100}, 0}, // hammer, but far below 103
	}
	for _, tc := range cases {
		o, h, l, c := cdlhangingmanBase(11)
		o = append(o, tc.b.o)
		h = append(h, tc.b.h)
		l = append(l, tc.b.l)
		c = append(c, tc.b.c)
		out, err := CDLHANGINGMAN(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 12)
		want[11] = tc.want
		assertInt(t, "CDLHANGINGMAN "+tc.name, out, want)
	}

	// A constant series has zero body, zero shadows: never a hanging man
	// (lower shadow 0 is not > body 0).
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLHANGINGMAN(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLHANGINGMANRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlhangingmanRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlhangingmanRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlhangingmanRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlhangingmanRange(0, 10, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlhangingmanRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlhangingmanRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlhangingmanRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLHANGINGMAN(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlhangingmanRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 11 || nb != 15 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
