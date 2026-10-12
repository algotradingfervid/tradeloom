package ta

import (
	"errors"
	"testing"
)

// cdldojistarBar is one candle.
type cdldojistarBar struct{ o, h, l, c float64 }

// cdldojistarSeries appends the two pattern bars to 10 background bars
// (open 100, close 102, high 104, low 99: body 2, high-low range 5), so the
// doji is bar 11, the first evaluated bar with default settings.
func cdldojistarSeries(b1, b2 cdldojistarBar) (o, h, l, c []float64) {
	for range 10 {
		o = append(o, 100)
		h = append(h, 104)
		l = append(l, 99)
		c = append(c, 102)
	}
	for _, b := range []cdldojistarBar{b1, b2} {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

func TestCDLDOJISTARParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLDOJISTAR(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLDOJISTAR(s.O[:3], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLDOJISTAR(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLDOJISTAR(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLDOJISTAR(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLDOJISTAR([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLDOJISTARLookback(); lb != 11 {
		t.Errorf("default lookback = %d, want 11", lb)
	}
}

func TestCDLDOJISTARLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyLong, 20, 21},
		{BodyDoji, 30, 31},
		{BodyLong, 0, 11}, // BodyDoji still 10
		{BodyDoji, 0, 11}, // BodyLong still 10
		{BodyShort, 50, 11},
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLDOJISTARLookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Both average periods 0: only the previous candle is needed.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, BodyDoji} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLDOJISTARLookback(); lb != 1 {
		t.Errorf("all periods 0: lookback %d, want 1", lb)
	}
	for n := 0; n <= 1; n++ {
		o, h, l, c := cdldojistarSeries(cdldojistarBar{}, cdldojistarBar{})
		out, err := CDLDOJISTAR(o[:n], h[:n], l[:n], c[:n])
		if err != nil || len(out) != n {
			t.Errorf("n=%d with lookback 1: out=%v err=%v", n, out, err)
		}
		for i, v := range out {
			if v != 0 {
				t.Errorf("n=%d: [%d]=%d", n, i, v)
			}
		}
	}
	// With periods 0 each candle is compared with its own body: the first
	// body is never > itself, so the pattern cannot fire.
	o, h, l, c := cdldojistarSeries(cdldojistarBar{100, 105, 100, 105}, cdldojistarBar{106, 107, 105.9, 106.2})
	out, _ := CDLDOJISTAR(o, h, l, c)
	for i, v := range out {
		if v != 0 {
			t.Errorf("periods 0: [%d]=%d", i, v)
		}
	}
}

func TestCDLDOJISTARLengths(t *testing.T) {
	lb := CDLDOJISTARLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLDOJISTAR(s.O, s.H, s.L, s.C)
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

func TestCDLDOJISTARKnownValues(t *testing.T) {
	// Background bodies are 2 and high-low ranges 5. The BodyLong average
	// for the 1st candle (bar 10) covers bars 0..9: 2. The BodyDoji average
	// for the doji (bar 11) covers bars 1..10: 0.1*(9*5 + range1)/10, which
	// is exactly 0.5 when the 1st candle's range is 5.
	type b = cdldojistarBar
	white := b{100, 105, 100, 105} // body 5, range 5
	black := b{105, 105, 100, 100} // body 5, range 5
	cases := []struct {
		name   string
		b1, b2 b
		want   int
	}{
		{"white then doji gapping up", white, b{106, 107, 105.8, 106.2}, -100},
		{"black then doji gapping down", black, b{99, 99.3, 98, 98.8}, 100},
		// Doji body exactly at the average (0.5) is a doji (<=).
		{"doji body equals average", white, b{106, 107, 105.8, 106.5}, -100},
		{"doji body above average", white, b{106, 107, 105.8, 106.5000001}, 0},
		// A black doji counts the same.
		{"black doji gapping up", white, b{106.2, 107, 105.8, 106}, -100},
		// Only the real bodies must gap; overlapping shadows are fine.
		{"shadows overlap", white, b{106, 107, 104, 106.2}, -100},
		// Touching bodies do not gap (strict).
		{"doji touches white body", white, b{105, 106, 104.5, 105.2}, 0},
		{"doji touches black body", black, b{100, 100.5, 99, 99.8}, 0},
		// Gap in the wrong direction for the 1st candle's color.
		{"white then gap down", white, b{99, 99.3, 98, 98.8}, 0},
		{"black then gap up", black, b{106, 107, 105.8, 106.2}, 0},
		// 1st body equal to the average (2) is not long (strict >).
		{"1st body equals average", b{100, 102.5, 97.5, 102}, b{103, 104, 102.8, 103.2}, 0},
		{"1st body just long", b{100, 102.5, 97.5, 102.0000001}, b{103, 104, 102.8, 103.2}, -100},
	}
	for _, tc := range cases {
		o, h, l, c := cdldojistarSeries(tc.b1, tc.b2)
		out, err := CDLDOJISTAR(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 12)
		want[11] = tc.want
		assertInt(t, "CDLDOJISTAR "+tc.name, out, want)
	}

	// Constant and zero series have zero bodies: never long, never fire.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLDOJISTAR(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLDOJISTARRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdldojistarRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdldojistarRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdldojistarRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdldojistarRange(0, 10, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdldojistarRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdldojistarRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdldojistarRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLDOJISTAR(s.O, s.H, s.L, s.C)
	beg, nb, err = cdldojistarRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 11 || nb != 15 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
