package ta

import (
	"errors"
	"math"
	"testing"
)

// cdlmorningstarBar is one OHLC candle for the hand-built cases.
type cdlmorningstarBar struct{ o, h, l, c float64 }

// cdlmorningstarSeries returns 12 identical background bars (open 100,
// close 101, high 101.5, low 99.5: real body 1) followed by the given bars.
func cdlmorningstarSeries(bars ...cdlmorningstarBar) (o, h, l, c []float64) {
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

// Default settings on cdlmorningstarSeries with the pattern at bars 12-14:
//   - 1st candle (bar 12): BodyLong avg over bars 2..11 = 1; body 4 > 1.
//   - star (bar 13): BodyShort avg over bars 3..12 = (9*1 + 4)/10 = 1.3;
//     body 0.5 <= 1.3, body 98.5..99 below 1st body bottom 100.
//   - 3rd candle (bar 14): BodyShort avg over bars 4..13
//     = (8*1 + 4 + 0.5)/10 = 1.25; body 2 > 1.25; 103 > fma(4, p, 100).
var cdlmorningstarPattern = [3]cdlmorningstarBar{
	{104, 104.5, 99.5, 100},  // long black, body 4
	{99, 99.2, 98, 98.5},     // short star, real body gapped down
	{101, 103.5, 100.5, 103}, // white, body 2
}

func TestCDLMORNINGSTARParams(t *testing.T) {
	s := fixtures(50)[0]
	// TA_REAL_DEFAULT (-4e37) is a C sentinel; Go has no sentinel and rejects it.
	for _, p := range []float64{-1, -1e-300, math.NaN(), math.Inf(1), math.Inf(-1), 3.0000001e37, -4e37} {
		if _, err := CDLMORNINGSTAR(s.O, s.H, s.L, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("penetration %g: err = %v, want ErrBadParam", p, err)
		}
		if lb := CDLMORNINGSTARLookback(p); lb != -1 {
			t.Errorf("CDLMORNINGSTARLookback(%g) = %d, want -1", p, lb)
		}
		if _, _, err := cdlmorningstarRange(0, 49, s.O, s.H, s.L, s.C, p, make([]int, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range penetration %g: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []float64{0, 0.3, 1, 3e37} {
		if _, err := CDLMORNINGSTAR(s.O, s.H, s.L, s.C, p); err != nil {
			t.Errorf("penetration %g: unexpected err %v", p, err)
		}
		if lb := CDLMORNINGSTARLookback(p); lb != 12 {
			t.Errorf("CDLMORNINGSTARLookback(%g) = %d, want 12", p, lb)
		}
	}
	if _, err := CDLMORNINGSTAR(s.O, s.H[:49], s.L, s.C, 0.3); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLMORNINGSTAR(s.O, s.H, s.L, s.C[:10], 0.3); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLMORNINGSTAR(s.O, s.H, nil, s.C, 0.3); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLMORNINGSTAR(nil, nil, nil, nil, 0.3); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLMORNINGSTAR([]float64{}, []float64{}, []float64{}, []float64{}, 0.3); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// A bad penetration is reported even on empty input.
	if _, err := CDLMORNINGSTAR(nil, nil, nil, nil, -1); !errors.Is(err, ErrBadParam) {
		t.Errorf("empty input, bad penetration: err = %v", err)
	}
}

func TestCDLMORNINGSTARLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyLong, 30, 32},
		{BodyShort, 15, 17},
		{BodyLong, 0, 12},    // BodyShort still 10
		{BodyDoji, 50, 12},   // unused setting
		{ShadowLong, 50, 12}, // unused setting
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLMORNINGSTARLookback(0.3); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Both average periods 0: only the two prior candles are needed.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, BodyShort} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLMORNINGSTARLookback(0.3); lb != 2 {
		t.Errorf("all periods 0: lookback %d, want 2", lb)
	}
	for n := 0; n <= 3; n++ {
		o, h, l, c := cdlmorningstarSeries()
		out, err := CDLMORNINGSTAR(o[:n], h[:n], l[:n], c[:n], 0.3)
		if err != nil || len(out) != n {
			t.Errorf("n=%d with lookback 2: out=%v err=%v", n, out, err)
		}
	}
	p := cdlmorningstarPattern
	bare := func() []int {
		out, _ := CDLMORNINGSTAR([]float64{p[0].o, p[1].o, p[2].o}, []float64{p[0].h, p[1].h, p[2].h},
			[]float64{p[0].l, p[1].l, p[2].l}, []float64{p[0].c, p[1].c, p[2].c}, 0.3)
		return out
	}
	// With periods 0 each candle is compared with its own range: with
	// BodyLong factor 1 the 1st body is never > itself.
	assertInt(t, "periods 0, body > itself", bare(), []int{0, 0, 0})
	// BodyLong factor 0.5: 4 > 2; star 0.5 <= 0.5 (BodyShort factor 1);
	// but the 3rd body 2 > 2 is false.
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 0.5)
	assertInt(t, "periods 0, 3rd body > itself", bare(), []int{0, 0, 0})
	// BodyShort factor 0.5 too: star 0.5 <= 0.25 is false.
	_ = SetCandleSettings(BodyShort, RangeRealBody, 0, 0.5)
	assertInt(t, "periods 0, star > half itself", bare(), []int{0, 0, 0})
	// BodyShort on high-low with factor 0.6:
	_ = SetCandleSettings(BodyShort, RangeHighLow, 0, 0.6)
	// star: 0.5 <= 0.6*1.2 = 0.72; 3rd: 2 > 0.6*3 = 1.8.
	assertInt(t, "periods 0, BodyShort high-low 0.6", bare(), []int{0, 0, 100})
}

func TestCDLMORNINGSTARLengths(t *testing.T) {
	for _, p := range []float64{0, 0.3, 1, 3e37} {
		lb := CDLMORNINGSTARLookback(p)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := CDLMORNINGSTAR(s.O, s.H, s.L, s.C, p)
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
					if v != 0 && v != 100 {
						t.Fatalf("%s n=%d p=%g: [%d]=%d, want 100 or 0", s.Name, n, p, i, v)
					}
				}
			}
		}
	}
}

func TestCDLMORNINGSTARKnownValues(t *testing.T) {
	pat := cdlmorningstarPattern
	with := func(k int, nb cdlmorningstarBar) [3]cdlmorningstarBar {
		b := pat
		b[k] = nb
		return b
	}
	cases := []struct {
		name string
		bars [3]cdlmorningstarBar
		pen  float64
		want int
	}{
		// 103 > fma(4, p, 100).
		{"default", pat, 0.3, 100},
		{"pen 0", pat, 0, 100},
		{"pen 0.7", pat, 0.7, 100}, // 103 > 102.8
		{"pen 0.75", pat, 0.75, 0}, // 103 > 103 is false
		{"pen 0.8", pat, 0.8, 0},   // 103 > 103.2 is false
		{"pen huge", pat, 3e37, 0},
		{"star white", with(1, cdlmorningstarBar{98.5, 99.2, 98, 99}), 0.3, 100},
		{"star doji", with(1, cdlmorningstarBar{99, 99.2, 98, 99}), 0.3, 100},       // avg2 = 1.2, still < 2
		{"1st not long", with(0, cdlmorningstarBar{101, 104.5, 99.5, 100}), 0.3, 0}, // body 1 not > 1
		{"1st white", with(0, cdlmorningstarBar{100, 104.5, 99.5, 104}), 0.3, 0},
		{"1st flat is white", with(0, cdlmorningstarBar{100, 104.5, 99.5, 100}), 0.3, 0},
		{"star too big", with(1, cdlmorningstarBar{99, 99.2, 97.4, 97.5}), 0.3, 0},  // 1.5 > 1.3
		{"star no gap", with(1, cdlmorningstarBar{100.1, 100.5, 98, 99.6}), 0.3, 0}, // 100.1 not < 100
		{"star touching", with(1, cdlmorningstarBar{100, 100.5, 98, 99.5}), 0.3, 0}, // 100 < 100 is false
		{"star shadow overlap ok", with(1, cdlmorningstarBar{99, 101, 98, 98.5}), 0.3, 100},
		{"3rd black", with(2, cdlmorningstarBar{103, 103.5, 100.5, 101}), 0.3, 0},
		{"3rd flat", with(2, cdlmorningstarBar{103, 103.5, 100.5, 103}), 0.3, 0},   // white but body 0
		{"3rd short", with(2, cdlmorningstarBar{102, 103.5, 100.5, 103}), 0.3, 0},  // body 1 not > 1.25
		{"3rd not deep", with(2, cdlmorningstarBar{99, 101.5, 98.8, 101}), 0.3, 0}, // 101 not > 101.2
		{"3rd just deep", with(2, cdlmorningstarBar{99, 101.5, 98.8, 101.3}), 0.3, 100},
	}
	for _, tc := range cases {
		o, h, l, c := cdlmorningstarSeries(tc.bars[:]...)
		out, err := CDLMORNINGSTAR(o, h, l, c, tc.pen)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 15)
		want[14] = tc.want
		assertInt(t, "CDLMORNINGSTAR "+tc.name, out, want)
	}

	// Flat series have zero bodies: never long, never fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLMORNINGSTAR(s.O, s.H, s.L, s.C, 0.3)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLMORNINGSTARRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlmorningstarRange(-1, 5, s.O, s.H, s.L, s.C, 0.3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlmorningstarRange(5, 4, s.O, s.H, s.L, s.C, 0.3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlmorningstarRange(0, -1, s.O, s.H, s.L, s.C, 0.3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlmorningstarRange(0, 11, s.O, s.H, s.L, s.C, 0.3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlmorningstarRange(0, 29, s.O, s.H, s.L[:20], s.C, 0.3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlmorningstarRange(0, 29, s.O[:29], s.H, s.L, s.C, 0.3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: %v", err)
	}
	if _, _, err := cdlmorningstarRange(0, 29, s.O, s.H, s.L, s.C, 0.3, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlmorningstarRange(20, 29, s.O, s.H, s.L, s.C, 0.3, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series, including a firing bar.
	o, h, l, c := cdlmorningstarSeries(cdlmorningstarPattern[:]...)
	full, _ := CDLMORNINGSTAR(o, h, l, c, 0.3)
	beg, nb, err = cdlmorningstarRange(5, 14, o, h, l, c, 0.3, out)
	if err != nil || beg != 12 || nb != 3 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
	if out[2] != 100 {
		t.Errorf("sub-range did not fire: %v", out[:nb])
	}
}
