package ta

import (
	"errors"
	"testing"
)

// cdlinvertedhammerBase returns n identical background bars: open 100,
// close 102, high 103, low 99 (real body 2, high-low 4).
func cdlinvertedhammerBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

func TestCDLINVERTEDHAMMERParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLINVERTEDHAMMER(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLINVERTEDHAMMER(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLINVERTEDHAMMER(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLINVERTEDHAMMER(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLINVERTEDHAMMER(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLINVERTEDHAMMER([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLINVERTEDHAMMERLookback(); lb != 11 {
		t.Errorf("default lookback = %d, want 11", lb)
	}
}

func TestCDLINVERTEDHAMMERLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyShort, 20, 21},
		{ShadowLong, 30, 31},
		{ShadowVeryShort, 0, 11}, // BodyShort still 10
		{ShadowVeryShort, 25, 26},
		{Near, 40, 11}, // Near is not used
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLINVERTEDHAMMERLookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Every average period 0: only the prior candle is needed.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyShort, ShadowLong, ShadowVeryShort} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLINVERTEDHAMMERLookback(); lb != 1 {
		t.Errorf("all periods 0: lookback %d, want 1", lb)
	}
	o, h, l, c := cdlinvertedhammerBase(1)
	out, err := CDLINVERTEDHAMMER(o, h, l, c)
	if err != nil || len(out) != 1 || out[0] != 0 {
		t.Errorf("n=1 with lookback 1: out=%v err=%v", out, err)
	}
	// n=2 with lookback 1: bar 1 is compared with its own ranges. With
	// BodyShort factor 2: body 0.5 < 2*0.5; upper 1.5 > body 0.5;
	// lower 0.1 < 0.1*(101-98.9); max 99.5 < prior min 100.
	_ = SetCandleSettings(BodyShort, RangeRealBody, 0, 2.0)
	o = append(o, 99)
	h = append(h, 101)
	l = append(l, 98.9)
	c = append(c, 99.5)
	out, err = CDLINVERTEDHAMMER(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "lookback 1", out, []int{0, 100})
	// With factor 1 the body 0.5 is not < 0.5: no pattern.
	_ = SetCandleSettings(BodyShort, RangeRealBody, 0, 1.0)
	out, _ = CDLINVERTEDHAMMER(o, h, l, c)
	assertInt(t, "lookback 1 factor 1", out, []int{0, 0})
}

func TestCDLINVERTEDHAMMERLengths(t *testing.T) {
	lb := CDLINVERTEDHAMMERLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLINVERTEDHAMMER(s.O, s.H, s.L, s.C)
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

func TestCDLINVERTEDHAMMERKnownValues(t *testing.T) {
	// Background bars: body 2, high-low 4, so with default settings
	// BodyShort avg = 2, ShadowVeryShort avg = 0.1*4 = 0.4,
	// ShadowLong = current body; the prior body spans [100, 102].
	type bar struct{ o, h, l, c float64 }
	cases := []struct {
		name string
		b    bar
		want int
	}{
		// gap: 99.5 < 100; body 0.5 < 2; upper 1.5 > 0.5; lower 0.1 < 0.4.
		{"inverted hammer", bar{99, 101, 98.9, 99.5}, 100},
		{"inverted hammer black", bar{99.5, 101, 98.9, 99}, 100},
		{"doji body", bar{99, 101, 98.9, 99}, 100},                   // body 0, upper 2 > 0
		{"no gap: touches prior body", bar{99.5, 101, 99.4, 100}, 0}, // max 100 not < 100
		{"no gap: overlaps", bar{99.5, 102, 99.4, 100.5}, 0},
		{"body too big", bar{97, 101, 96.9, 99}, 0},              // body 2 not < 2
		{"upper shadow short", bar{99, 99.9, 98.9, 99.5}, 0},     // upper 0.4 not > 0.5
		{"upper shadow equal body", bar{99, 100, 98.9, 99.5}, 0}, // upper 0.5 not > 0.5
		{"lower shadow long", bar{99, 101, 98.5, 99.5}, 0},       // lower 0.5 not < 0.4
		{"hammer shape", bar{99, 99.6, 97, 99.5}, 0},             // long lower, short upper
	}
	for _, tc := range cases {
		o, h, l, c := cdlinvertedhammerBase(11)
		o = append(o, tc.b.o)
		h = append(h, tc.b.h)
		l = append(l, tc.b.l)
		c = append(c, tc.b.c)
		out, err := CDLINVERTEDHAMMER(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 12)
		want[11] = tc.want
		assertInt(t, "CDLINVERTEDHAMMER "+tc.name, out, want)
	}

	// A constant series has no gap and zero shadows: never fires.
	// A strictly rising series never gaps down.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" && s.Name != "up" {
			continue
		}
		out, _ := CDLINVERTEDHAMMER(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLINVERTEDHAMMERRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlinvertedhammerRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlinvertedhammerRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlinvertedhammerRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlinvertedhammerRange(0, 10, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlinvertedhammerRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlinvertedhammerRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlinvertedhammerRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLINVERTEDHAMMER(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlinvertedhammerRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 11 || nb != 15 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
