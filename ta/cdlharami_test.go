package ta

import (
	"errors"
	"testing"
)

// cdlharamiBase returns n identical background bars: open 100, close 102,
// high 103, low 99 (real body 2).
func cdlharamiBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

// cdlharamiBar is one open/close pair; high and low are set just outside the
// body (CDLHARAMI with default settings ignores them).
type cdlharamiBar struct{ o, c float64 }

// cdlharamiSeries appends the two pattern bars to 10 background bars, so the
// second bar is index 11 (the first evaluated bar with default settings).
func cdlharamiSeries(b1, b2 cdlharamiBar) (o, h, l, c []float64) {
	o, h, l, c = cdlharamiBase(10)
	for _, b := range []cdlharamiBar{b1, b2} {
		o = append(o, b.o)
		c = append(c, b.c)
		h = append(h, cMax(b.o, b.c)+0.5)
		l = append(l, cMin(b.o, b.c)-0.5)
	}
	return
}

func TestCDLHARAMIParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLHARAMI(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLHARAMI(s.O[:3], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLHARAMI(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLHARAMI(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLHARAMI(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLHARAMI([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLHARAMILookback(); lb != 11 {
		t.Errorf("default lookback = %d, want 11", lb)
	}
}

func TestCDLHARAMILookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyLong, 20, 21},
		{BodyShort, 30, 31},
		{BodyLong, 0, 11},  // BodyShort still 10
		{BodyShort, 0, 11}, // BodyLong still 10
		{ShadowLong, 50, 11},
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLHARAMILookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Both average periods 0: only the prior candle is needed.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, BodyShort} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLHARAMILookback(); lb != 1 {
		t.Errorf("all periods 0: lookback %d, want 1", lb)
	}
	for n := 0; n <= 1; n++ {
		o, h, l, c := cdlharamiBase(n)
		out, err := CDLHARAMI(o, h, l, c)
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
	o, h, l, c := cdlharamiSeries(cdlharamiBar{105, 100}, cdlharamiBar{101, 102})
	out, _ := CDLHARAMI(o, h, l, c)
	for i, v := range out {
		if v != 0 {
			t.Errorf("periods 0: [%d]=%d", i, v)
		}
	}
}

func TestCDLHARAMILengths(t *testing.T) {
	lb := CDLHARAMILookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLHARAMI(s.O, s.H, s.L, s.C)
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
				switch v {
				case -100, -80, 0, 80, 100:
				default:
					t.Fatalf("%s n=%d: [%d]=%d, want one of -100,-80,0,80,100", s.Name, n, i, v)
				}
			}
		}
	}
}

func TestCDLHARAMIKnownValues(t *testing.T) {
	// Background bodies are 2. The BodyLong average for the 1st candle
	// (bar 10) covers bars 0..9: 2. The BodyShort average for the 2nd
	// candle (bar 11) covers bars 1..10, so it includes the 1st candle:
	// (9*2 + body1)/10.
	type b = cdlharamiBar
	cases := []struct {
		name   string
		b1, b2 b
		want   int
	}{
		// Black 105->100 (body 5 > 2); 101/102 strictly inside, body 1 <= 2.3.
		{"bullish", b{105, 100}, b{101, 102}, 100},
		{"bullish, 2nd black", b{105, 100}, b{102, 101}, 100},
		{"bearish", b{100, 105}, b{103, 102}, -100},
		{"bearish, 2nd white", b{100, 105}, b{102, 103}, -100},
		// Bodies share one end: 80.
		{"bullish top matches", b{105, 100}, b{104, 105}, 80},
		{"bullish bottom matches", b{105, 100}, b{100, 101}, 80},
		{"bearish top matches", b{100, 105}, b{105, 104}, -80},
		{"bearish bottom matches", b{100, 105}, b{100, 101}, -80},
		// A doji 2nd inside.
		{"2nd doji", b{105, 100}, b{102, 102}, 100},
		{"2nd doji on edge", b{100, 105}, b{105, 105}, -80},
		// Not contained.
		{"2nd above", b{105, 100}, b{104, 106}, 0},
		{"2nd below", b{105, 100}, b{99, 101}, 0},
		// 1st body equal to the average is not long (strict >).
		{"1st body equals average", b{102, 100}, b{100.5, 101.5}, 0},
		// 1st body 12: BodyShort average = (18+12)/10 = 3. 2nd body 3 is
		// short (<=); 3.5 is not.
		{"2nd body equals average", b{112, 100}, b{101, 104}, 100},
		{"2nd body above average", b{112, 100}, b{101, 104.5}, 0},
	}
	for _, tc := range cases {
		o, h, l, c := cdlharamiSeries(tc.b1, tc.b2)
		out, err := CDLHARAMI(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 12)
		want[11] = tc.want
		assertInt(t, "CDLHARAMI "+tc.name, out, want)
	}

	// Constant and zero series have zero bodies: never long, never fire.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLHARAMI(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLHARAMIRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlharamiRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlharamiRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlharamiRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlharamiRange(0, 10, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlharamiRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlharamiRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlharamiRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLHARAMI(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlharamiRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 11 || nb != 15 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
