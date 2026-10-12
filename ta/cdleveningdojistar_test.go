package ta

import (
	"errors"
	"math"
	"testing"
)

// cdleveningdojistarBar is one OHLC candle for the hand-built cases.
type cdleveningdojistarBar struct{ o, h, l, c float64 }

// cdleveningdojistarSeries returns 12 identical background bars (open 100,
// close 101, high 101.5, low 99.5: real body 1, high-low 2) followed by the
// given bars.
func cdleveningdojistarSeries(bars ...cdleveningdojistarBar) (o, h, l, c []float64) {
	for range 12 {
		o = append(o, 100)
		h = append(h, 101.5)
		l = append(l, 99.5)
		c = append(c, 101)
	}
	for _, b := range bars {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

// Default settings on cdleveningdojistarSeries with the pattern at bars 12-14:
//   - 1st candle (bar 12): BodyLong avg (real body, factor 1) over bars 2..11 = 1.
//   - doji (bar 13): BodyDoji avg = 0.1 * mean high-low of bars 3..12
//     = 0.1 * (9*2 + 5)/10 = 0.23 when bar 12 has high-low 5.
//   - 3rd candle (bar 14): BodyShort avg over bars 4..13
//     = (8*1 + 4 + 0.05)/10 = 1.205 when bar 12 has body 4, bar 13 body 0.05.
//   - penetration: 3rd close 102 < 104 - 4*p, i.e. p < 0.5.
var cdleveningdojistarStar = [3]cdleveningdojistarBar{
	{100, 104.5, 99.5, 104},     // long white, body 4, high-low 5
	{105, 105.5, 104.8, 105.05}, // doji, body 0.05, body gapped above 104
	{104, 104.2, 101.5, 102},    // black, body 2, closes at 102
}

func TestCDLEVENINGDOJISTARParams(t *testing.T) {
	s := fixtures(50)[0]
	// TA_REAL_DEFAULT (-4e37) is a C sentinel; Go has no sentinel and rejects it.
	for _, p := range []float64{-1, -1e-300, math.NaN(), math.Inf(1), math.Inf(-1), 3.0000001e37, -4e37} {
		if _, err := CDLEVENINGDOJISTAR(s.O, s.H, s.L, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("penetration %g: err = %v, want ErrBadParam", p, err)
		}
		if lb := CDLEVENINGDOJISTARLookback(p); lb != -1 {
			t.Errorf("CDLEVENINGDOJISTARLookback(%g) = %d, want -1", p, lb)
		}
		if _, _, err := cdleveningdojistarRange(0, 49, s.O, s.H, s.L, s.C, p, make([]int, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range penetration %g: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []float64{0, 0.3, 1, 3e37} {
		if _, err := CDLEVENINGDOJISTAR(s.O, s.H, s.L, s.C, p); err != nil {
			t.Errorf("penetration %g: unexpected err %v", p, err)
		}
		if lb := CDLEVENINGDOJISTARLookback(p); lb != 12 {
			t.Errorf("CDLEVENINGDOJISTARLookback(%g) = %d, want 12", p, lb)
		}
	}
	if _, err := CDLEVENINGDOJISTAR(s.O, s.H[:49], s.L, s.C, 0.3); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLEVENINGDOJISTAR(s.O, s.H, s.L, s.C[:10], 0.3); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLEVENINGDOJISTAR(s.O, s.H, nil, s.C, 0.3); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLEVENINGDOJISTAR(nil, nil, nil, nil, 0.3); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLEVENINGDOJISTAR([]float64{}, []float64{}, []float64{}, []float64{}, 0.3); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// A bad penetration is reported even on empty input.
	if _, err := CDLEVENINGDOJISTAR(nil, nil, nil, nil, -1); !errors.Is(err, ErrBadParam) {
		t.Errorf("empty input, bad penetration: err = %v", err)
	}
}

func TestCDLEVENINGDOJISTARLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyDoji, 20, 22},
		{BodyLong, 30, 32},
		{BodyShort, 15, 17},
		{BodyDoji, 0, 12},    // the other two still 10
		{ShadowLong, 50, 12}, // unused setting
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLEVENINGDOJISTARLookback(0.3); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Every average period 0: only the two prior candles are needed.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyDoji, BodyLong, BodyShort} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLEVENINGDOJISTARLookback(0.3); lb != 2 {
		t.Errorf("all periods 0: lookback %d, want 2", lb)
	}
	for n := 0; n <= 3; n++ {
		o, h, l, c := cdleveningdojistarSeries()
		out, err := CDLEVENINGDOJISTAR(o[:n], h[:n], l[:n], c[:n], 0.3)
		if err != nil || len(out) != n {
			t.Errorf("n=%d with lookback 2: out=%v err=%v", n, out, err)
		}
	}
	// With periods 0, each candle is compared with its own range: with
	// BodyLong factor 1 the 1st candle's body is never > itself, so the bare
	// three-candle star does not fire.
	st := cdleveningdojistarStar
	bare := func() ([]float64, []float64, []float64, []float64) {
		return []float64{st[0].o, st[1].o, st[2].o}, []float64{st[0].h, st[1].h, st[2].h},
			[]float64{st[0].l, st[1].l, st[2].l}, []float64{st[0].c, st[1].c, st[2].c}
	}
	o, h, l, c := bare()
	out, _ := CDLEVENINGDOJISTAR(o, h, l, c, 0.3)
	assertInt(t, "periods 0, body > itself", out, []int{0, 0, 0})
	// Halve the BodyLong and BodyShort factors: now it fires (doji body 0.05
	// <= 0.1 * its own high-low 0.7).
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 0.5)
	_ = SetCandleSettings(BodyShort, RangeRealBody, 0, 0.5)
	out, _ = CDLEVENINGDOJISTAR(o, h, l, c, 0.3)
	assertInt(t, "periods 0, factor 0.5", out, []int{0, 0, -100})
}

func TestCDLEVENINGDOJISTARLengths(t *testing.T) {
	for _, p := range []float64{0, 0.3, 1, 3e37} {
		lb := CDLEVENINGDOJISTARLookback(p)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := CDLEVENINGDOJISTAR(s.O, s.H, s.L, s.C, p)
				if err != nil {
					t.Fatalf("%s n=%d p=%g: %v", s.Name, n, p, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%g: len(out)=%d", s.Name, n, p, len(out))
				}
				for i, v := range out {
					if i < lb && v != 0 {
						t.Fatalf("%s n=%d p=%g: [%d]=%d inside lookback %d", s.Name, n, p, i, v, lb)
					}
					if v != 0 && v != -100 {
						t.Fatalf("%s n=%d p=%g: [%d]=%d, want -100 or 0", s.Name, n, p, i, v)
					}
				}
			}
		}
	}
}

func TestCDLEVENINGDOJISTARKnownValues(t *testing.T) {
	st := cdleveningdojistarStar
	with := func(k int, nb cdleveningdojistarBar) [3]cdleveningdojistarBar {
		b := st
		b[k] = nb
		return b
	}
	cases := []struct {
		name string
		bars [3]cdleveningdojistarBar
		pen  float64
		want int
	}{
		// Penetration: 102 < 104 - 4*p.
		{"star", st, 0.3, -100},
		{"pen 0", st, 0, -100},
		{"pen 0.45", st, 0.45, -100}, // 102 < 102.2
		{"pen 0.5", st, 0.5, 0},      // 102 < 102 is false
		{"pen 1", st, 1, 0},
		{"pen huge", st, 3e37, 0},
		{"1st not long", with(0, cdleveningdojistarBar{103, 104.5, 99.5, 104}), 0.3, 0},          // body 1 not > 1
		{"1st black", with(0, cdleveningdojistarBar{104, 104.5, 99.5, 100}), 0.3, 0},             // no white
		{"doji too big", with(1, cdleveningdojistarBar{105, 105.5, 104.8, 105.3}), 0.3, 0},       // 0.3 > 0.23
		{"doji at limit", with(1, cdleveningdojistarBar{105, 105.5, 104.8, 105.125}), 0.3, -100}, // 0.125 <= 0.23
		{"doji no gap", with(1, cdleveningdojistarBar{104, 104.5, 103.8, 104.05}), 0.3, 0},       // min 104 not > 104
		{"doji black gap", with(1, cdleveningdojistarBar{105.05, 105.5, 104.8, 105}), 0.3, -100},
		{"3rd short", with(2, cdleveningdojistarBar{103, 104.2, 101.5, 102}), 0.3, 0}, // body 1 not > 1.205
		{"3rd white", with(2, cdleveningdojistarBar{102, 104.2, 101.5, 104}), 0.3, 0},
		{"3rd zero body", with(2, cdleveningdojistarBar{102, 104.2, 101.5, 102}), 0, 0},         // colour white when equal
		{"3rd too high", with(2, cdleveningdojistarBar{105, 105.2, 102.5, 103}), 0.3, 0},        // 103 not < 102.8
		{"3rd pen 0 short", with(2, cdleveningdojistarBar{105, 105.2, 103.5, 103.9}), 0, 0},     // 103.9 < 104 but body 1.1 not > 1.205
		{"3rd pen 0 long", with(2, cdleveningdojistarBar{105.2, 105.3, 103.5, 103.9}), 0, -100}, // body 1.3 > 1.205
	}
	for _, tc := range cases {
		o, h, l, c := cdleveningdojistarSeries(tc.bars[:]...)
		out, err := CDLEVENINGDOJISTAR(o, h, l, c, tc.pen)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 15)
		want[14] = tc.want
		assertInt(t, "CDLEVENINGDOJISTAR "+tc.name, out, want)
	}

	// Flat series have zero bodies: never long, never fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLEVENINGDOJISTAR(s.O, s.H, s.L, s.C, 0.3)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLEVENINGDOJISTARRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdleveningdojistarRange(-1, 5, s.O, s.H, s.L, s.C, 0.3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdleveningdojistarRange(5, 4, s.O, s.H, s.L, s.C, 0.3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdleveningdojistarRange(0, -1, s.O, s.H, s.L, s.C, 0.3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdleveningdojistarRange(0, 11, s.O, s.H, s.L, s.C, 0.3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdleveningdojistarRange(0, 29, s.O, s.H, s.L[:20], s.C, 0.3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdleveningdojistarRange(0, 29, s.O[:29], s.H, s.L, s.C, 0.3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: %v", err)
	}
	if _, _, err := cdleveningdojistarRange(0, 29, s.O, s.H, s.L, s.C, 0.3, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdleveningdojistarRange(20, 29, s.O, s.H, s.L, s.C, 0.3, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series, including a firing bar.
	o, h, l, c := cdleveningdojistarSeries(cdleveningdojistarStar[:]...)
	full, _ := CDLEVENINGDOJISTAR(o, h, l, c, 0.3)
	beg, nb, err = cdleveningdojistarRange(5, 14, o, h, l, c, 0.3, out)
	if err != nil || beg != 12 || nb != 3 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
	if out[2] != -100 {
		t.Errorf("sub-range did not fire: %v", out[:nb])
	}
	// A single-bar range on the firing bar.
	beg, nb, err = cdleveningdojistarRange(14, 14, o, h, l, c, 0.3, out)
	if err != nil || beg != 14 || nb != 1 || out[0] != -100 {
		t.Errorf("single bar: beg=%d nb=%d out=%d err=%v", beg, nb, out[0], err)
	}
}
