package ta

import (
	"errors"
	"math"
	"testing"
)

// cdlcounterattackBase returns n identical background bars: open 100, close
// 102, high 103, low 99 (real body 2, high-low range 4).
func cdlcounterattackBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

// cdlcounterattackBar is one open/close pair; high and low are set just
// outside the body.
type cdlcounterattackBar struct{ o, c float64 }

// cdlcounterattackSeries appends the two pattern bars to 10 background bars,
// so the second bar is index 11 (the first evaluated bar with default
// settings).
func cdlcounterattackSeries(b1, b2 cdlcounterattackBar) (o, h, l, c []float64) {
	o, h, l, c = cdlcounterattackBase(10)
	for _, b := range []cdlcounterattackBar{b1, b2} {
		o = append(o, b.o)
		c = append(c, b.c)
		h = append(h, cMax(b.o, b.c)+0.5)
		l = append(l, cMin(b.o, b.c)-0.5)
	}
	return
}

// cdlcounterattackEq is the default Equal band for the 1st candle (bar 10):
// factor 0.05 times the average high-low range of bars 5..9, computed
// exactly as the implementation does.
func cdlcounterattackEq() float64 {
	sum := 0.0
	for range 5 {
		sum += 4.0
	}
	return 0.05 * (sum / 5) / 1.0
}

func TestCDLCOUNTERATTACKParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLCOUNTERATTACK(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLCOUNTERATTACK(s.O[:3], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLCOUNTERATTACK(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLCOUNTERATTACK(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLCOUNTERATTACK(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLCOUNTERATTACK([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLCOUNTERATTACKLookback(); lb != 11 {
		t.Errorf("default lookback = %d, want 11", lb)
	}
}

func TestCDLCOUNTERATTACKLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyLong, 20, 21},
		{Equal, 30, 31},
		{BodyLong, 0, 6}, // Equal still 5
		{Equal, 0, 11},   // BodyLong still 10
		{BodyShort, 50, 11},
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLCOUNTERATTACKLookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Both average periods 0: only the prior candle is needed.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, Equal} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLCOUNTERATTACKLookback(); lb != 1 {
		t.Errorf("all periods 0: lookback %d, want 1", lb)
	}
	for n := 0; n <= 2; n++ {
		o, h, l, c := cdlcounterattackBase(n)
		out, err := CDLCOUNTERATTACK(o, h, l, c)
		if err != nil || len(out) != n {
			t.Errorf("n=%d with lookback 1: out=%v err=%v", n, out, err)
		}
		for i, v := range out {
			if v != 0 {
				t.Errorf("n=%d: [%d]=%d", n, i, v)
			}
		}
	}
	// With BodyLong period 0 each body is compared with itself (factor 1):
	// never strictly greater, so the pattern cannot fire.
	o, h, l, c := cdlcounterattackSeries(cdlcounterattackBar{106, 100}, cdlcounterattackBar{95, 100})
	out, _ := CDLCOUNTERATTACK(o, h, l, c)
	for i, v := range out {
		if v != 0 {
			t.Errorf("periods 0: [%d]=%d", i, v)
		}
	}
}

func TestCDLCOUNTERATTACKLengths(t *testing.T) {
	lb := CDLCOUNTERATTACKLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLCOUNTERATTACK(s.O, s.H, s.L, s.C)
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

func TestCDLCOUNTERATTACKKnownValues(t *testing.T) {
	// Background bodies are 2. The BodyLong average for the 1st candle
	// (bar 10) covers bars 0..9: 2. For the 2nd candle (bar 11) it covers
	// bars 1..10, so it includes the 1st: (9*2 + body1)/10. The Equal band
	// for the 1st candle is 0.05 * mean range of bars 5..9 = 0.05*4 = 0.2.
	eq := cdlcounterattackEq()
	up := 100 + eq
	dn := 100 - eq
	type b = cdlcounterattackBar
	cases := []struct {
		name   string
		b1, b2 b
		want   int
	}{
		// Black 106->100 (body 6 > 2); white 95->100 (body 5 > 2.4).
		{"bullish", b{106, 100}, b{95, 100}, 100},
		// White 100->106; black 111->106.
		{"bearish", b{100, 106}, b{111, 106}, -100},
		{"bullish close at upper band", b{106, 100}, b{95, up}, 100},
		{"bullish close at lower band", b{106, 100}, b{95, dn}, 100},
		{"bullish close above band", b{106, 100}, b{95, math.Nextafter(up, math.Inf(1))}, 0},
		{"bullish close below band", b{106, 100}, b{95, math.Nextafter(dn, math.Inf(-1))}, 0},
		{"bearish close at upper band", b{94, 100}, b{105, up}, -100},
		{"bearish close above band", b{94, 100}, b{105, math.Nextafter(up, math.Inf(1))}, 0},
		{"same color (black)", b{106, 100}, b{105, 100}, 0},
		{"same color (white)", b{94, 100}, b{95, 100}, 0},
		// 1st body equal to the average (2) is not long.
		{"1st body equals average", b{102, 100}, b{95, 100}, 0},
		// 1st body 12: 2nd average = (18+12)/10 = 3. Body 3 is not long;
		// 3.5 is.
		{"2nd body equals average", b{112, 100}, b{97, 100}, 0},
		{"2nd body above average", b{112, 100}, b{96.5, 100}, 100},
		// A doji 2nd counts as white but has no body: never long.
		{"2nd doji", b{106, 100}, b{100, 100}, 0},
	}
	for _, tc := range cases {
		o, h, l, c := cdlcounterattackSeries(tc.b1, tc.b2)
		out, err := CDLCOUNTERATTACK(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 12)
		want[11] = tc.want
		assertInt(t, "CDLCOUNTERATTACK "+tc.name, out, want)
	}

	// Constant and zero series have zero bodies: never long, never fire.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLCOUNTERATTACK(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLCOUNTERATTACKRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlcounterattackRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlcounterattackRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlcounterattackRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlcounterattackRange(0, 10, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlcounterattackRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlcounterattackRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlcounterattackRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLCOUNTERATTACK(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlcounterattackRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 11 || nb != 15 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
