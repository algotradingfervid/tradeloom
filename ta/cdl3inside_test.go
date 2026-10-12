package ta

import (
	"errors"
	"testing"
)

// cdl3insideBase returns n identical background bars: open 100, close 102,
// high 103, low 99 (real body 2).
func cdl3insideBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

// cdl3insideBar is one open/close pair; high and low are set just outside
// the body (CDL3INSIDE with default settings ignores them).
type cdl3insideBar struct{ o, c float64 }

// cdl3insideSeries appends the three pattern bars to 10 background bars, so
// the third bar is index 12 (the first evaluated bar with default settings).
func cdl3insideSeries(b1, b2, b3 cdl3insideBar) (o, h, l, c []float64) {
	o, h, l, c = cdl3insideBase(10)
	for _, b := range []cdl3insideBar{b1, b2, b3} {
		o = append(o, b.o)
		c = append(c, b.c)
		h = append(h, cMax(b.o, b.c)+0.5)
		l = append(l, cMin(b.o, b.c)-0.5)
	}
	return
}

func TestCDL3INSIDEParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDL3INSIDE(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3INSIDE(s.O[:3], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3INSIDE(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3INSIDE(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDL3INSIDE(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDL3INSIDE([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDL3INSIDELookback(); lb != 12 {
		t.Errorf("default lookback = %d, want 12", lb)
	}
}

func TestCDL3INSIDELookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyLong, 20, 22},
		{BodyShort, 30, 32},
		{BodyLong, 0, 12},  // BodyShort still 10
		{BodyShort, 0, 12}, // BodyLong still 10
		{ShadowLong, 50, 12},
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDL3INSIDELookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Both average periods 0: only the two prior candles are needed.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, BodyShort} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDL3INSIDELookback(); lb != 2 {
		t.Errorf("all periods 0: lookback %d, want 2", lb)
	}
	for n := 0; n <= 2; n++ {
		o, h, l, c := cdl3insideBase(n)
		out, err := CDL3INSIDE(o, h, l, c)
		if err != nil || len(out) != n {
			t.Errorf("n=%d with lookback 2: out=%v err=%v", n, out, err)
		}
		for i, v := range out {
			if v != 0 {
				t.Errorf("n=%d: [%d]=%d", n, i, v)
			}
		}
	}
	// With periods 0 each candle is compared with its own body: the first
	// body is never > itself, so the pattern cannot fire.
	o, h, l, c := cdl3insideSeries(cdl3insideBar{105, 100}, cdl3insideBar{101, 102}, cdl3insideBar{101, 106})
	out, _ := CDL3INSIDE(o, h, l, c)
	for i, v := range out {
		if v != 0 {
			t.Errorf("periods 0: [%d]=%d", i, v)
		}
	}
}

func TestCDL3INSIDELengths(t *testing.T) {
	lb := CDL3INSIDELookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDL3INSIDE(s.O, s.H, s.L, s.C)
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

func TestCDL3INSIDEKnownValues(t *testing.T) {
	// Background bodies are 2. The BodyLong average for the 1st candle
	// (bar 10) covers bars 0..9: 2. The BodyShort average for the 2nd
	// candle (bar 11) covers bars 1..10, so it includes the 1st candle:
	// (9*2 + body1)/10.
	type b = cdl3insideBar
	cases := []struct {
		name       string
		b1, b2, b3 b
		want       int
	}{
		// Black 105->100 (body 5 > 2); 101/102 inside, body 1 <= 2.3;
		// white closes 106 > 105.
		{"three inside up", b{105, 100}, b{101, 102}, b{101, 106}, 100},
		// White 100->105; 103/102 inside; black closes 99 < 100.
		{"three inside down", b{100, 105}, b{103, 102}, b{104, 99}, -100},
		// 2nd candle black or white does not matter.
		{"up, 2nd black", b{105, 100}, b{102, 101}, b{101, 106}, 100},
		{"3rd close equals 1st open (up)", b{105, 100}, b{101, 102}, b{101, 105}, 0},
		{"3rd close equals 1st open (down)", b{100, 105}, b{103, 102}, b{104, 100}, 0},
		{"3rd same color as 1st (up)", b{105, 100}, b{101, 102}, b{107, 106}, 0},
		{"3rd same color as 1st (down)", b{100, 105}, b{103, 102}, b{98, 99}, 0},
		// A doji 3rd counts as white: fires after a black 1st...
		{"3rd doji after black", b{105, 100}, b{101, 102}, b{106, 106}, 100},
		// ...but never after a white 1st.
		{"3rd doji after white", b{100, 105}, b{103, 102}, b{99, 99}, 0},
		// 1st body equal to the average is not long (strict >).
		{"1st body equals average", b{102, 100}, b{100.5, 101.5}, b{101, 103}, 0},
		// 1st body 12: BodyShort average = (18+12)/10 = 3. 2nd body 3 is
		// short (<=); 3.5 is not.
		{"2nd body equals average", b{112, 100}, b{101, 104}, b{101, 113}, 100},
		{"2nd body above average", b{112, 100}, b{101, 104.5}, b{101, 113}, 0},
		// The 2nd body must be strictly inside the 1st.
		{"2nd top touches", b{105, 100}, b{104, 105}, b{101, 106}, 0},
		{"2nd bottom touches", b{105, 100}, b{100, 101}, b{101, 106}, 0},
		{"2nd outside", b{105, 100}, b{104, 106}, b{101, 107}, 0},
	}
	for _, tc := range cases {
		o, h, l, c := cdl3insideSeries(tc.b1, tc.b2, tc.b3)
		out, err := CDL3INSIDE(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 13)
		want[12] = tc.want
		assertInt(t, "CDL3INSIDE "+tc.name, out, want)
	}

	// Constant and zero series have zero bodies: never long, never fire.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDL3INSIDE(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDL3INSIDERangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdl3insideRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdl3insideRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdl3insideRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdl3insideRange(0, 11, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdl3insideRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdl3insideRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdl3insideRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDL3INSIDE(s.O, s.H, s.L, s.C)
	beg, nb, err = cdl3insideRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 12 || nb != 14 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
