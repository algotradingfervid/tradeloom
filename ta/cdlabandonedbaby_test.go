package ta

import (
	"errors"
	"math"
	"testing"
)

// cdlabandonedbabyBar is one OHLC candle for the hand-built cases.
type cdlabandonedbabyBar struct{ o, h, l, c float64 }

// cdlabandonedbabySeries returns 12 identical background bars (open 100,
// close 101, high 101.5, low 99.5: real body 1, high-low 2) followed by the
// given bars.
func cdlabandonedbabySeries(bars ...cdlabandonedbabyBar) (o, h, l, c []float64) {
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

// Default settings on cdlabandonedbabySeries with the pattern at bars 12-14:
//   - 1st candle (bar 12): BodyLong avg over bars 2..11 = 1.
//   - doji (bar 13): BodyDoji avg = 0.1 * mean high-low of bars 3..12
//     = 0.1 * (9*2 + 5)/10 = 0.23 when bar 12 has high-low 5.
//   - 3rd candle (bar 14): BodyShort avg over bars 4..13
//     = (8*1 + 4 + 0)/10 = 1.2 when bar 12 has body 4 and bar 13 is a doji.
var (
	cdlabandonedbabyTop = [3]cdlabandonedbabyBar{
		{100, 104.5, 99.5, 104},  // long white, body 4, high-low 5
		{106, 106.2, 105.8, 106}, // doji gapped up
		{103, 103.5, 100.5, 101}, // black, body 2, gapped down
	}
	cdlabandonedbabyBottom = [3]cdlabandonedbabyBar{
		{104, 104.5, 99.5, 100},  // long black, body 4, high-low 5
		{98, 98.2, 97.8, 98},     // doji gapped down
		{101, 103.5, 100.5, 103}, // white, body 2, gapped up
	}
)

func TestCDLABANDONEDBABYParams(t *testing.T) {
	s := fixtures(50)[0]
	// TA_REAL_DEFAULT (-4e37) is a C sentinel; Go has no sentinel and rejects it.
	for _, p := range []float64{-1, -1e-300, math.NaN(), math.Inf(1), math.Inf(-1), 3.0000001e37, -4e37} {
		if _, err := CDLABANDONEDBABY(s.O, s.H, s.L, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("penetration %g: err = %v, want ErrBadParam", p, err)
		}
		if lb := CDLABANDONEDBABYLookback(p); lb != -1 {
			t.Errorf("CDLABANDONEDBABYLookback(%g) = %d, want -1", p, lb)
		}
		if _, _, err := cdlabandonedbabyRange(0, 49, s.O, s.H, s.L, s.C, p, make([]int, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range penetration %g: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []float64{0, 0.3, 1, 3e37} {
		if _, err := CDLABANDONEDBABY(s.O, s.H, s.L, s.C, p); err != nil {
			t.Errorf("penetration %g: unexpected err %v", p, err)
		}
		if lb := CDLABANDONEDBABYLookback(p); lb != 12 {
			t.Errorf("CDLABANDONEDBABYLookback(%g) = %d, want 12", p, lb)
		}
	}
	if _, err := CDLABANDONEDBABY(s.O, s.H[:49], s.L, s.C, 0.3); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLABANDONEDBABY(s.O, s.H, s.L, s.C[:10], 0.3); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLABANDONEDBABY(s.O, s.H, nil, s.C, 0.3); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLABANDONEDBABY(nil, nil, nil, nil, 0.3); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLABANDONEDBABY([]float64{}, []float64{}, []float64{}, []float64{}, 0.3); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// A bad penetration is reported even on empty input.
	if _, err := CDLABANDONEDBABY(nil, nil, nil, nil, -1); !errors.Is(err, ErrBadParam) {
		t.Errorf("empty input, bad penetration: err = %v", err)
	}
}

func TestCDLABANDONEDBABYLookbackSettings(t *testing.T) {
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
		if lb := CDLABANDONEDBABYLookback(0.3); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Every average period 0: only the two prior candles are needed.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyDoji, BodyLong, BodyShort} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLABANDONEDBABYLookback(0.3); lb != 2 {
		t.Errorf("all periods 0: lookback %d, want 2", lb)
	}
	for n := 0; n <= 3; n++ {
		o, h, l, c := cdlabandonedbabySeries()
		out, err := CDLABANDONEDBABY(o[:n], h[:n], l[:n], c[:n], 0.3)
		if err != nil || len(out) != n {
			t.Errorf("n=%d with lookback 2: out=%v err=%v", n, out, err)
		}
	}
	// With periods 0, each candle is compared with its own range: with
	// BodyLong factor 1 the 1st candle's body is never > itself, so the bare
	// three-candle top does not fire.
	tp := cdlabandonedbabyTop
	out, _ := CDLABANDONEDBABY([]float64{tp[0].o, tp[1].o, tp[2].o}, []float64{tp[0].h, tp[1].h, tp[2].h},
		[]float64{tp[0].l, tp[1].l, tp[2].l}, []float64{tp[0].c, tp[1].c, tp[2].c}, 0.3)
	assertInt(t, "periods 0, body > itself", out, []int{0, 0, 0})
	// Halve the BodyLong and BodyShort factors: now it fires.
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 0.5)
	_ = SetCandleSettings(BodyShort, RangeRealBody, 0, 0.5)
	out, _ = CDLABANDONEDBABY([]float64{tp[0].o, tp[1].o, tp[2].o}, []float64{tp[0].h, tp[1].h, tp[2].h},
		[]float64{tp[0].l, tp[1].l, tp[2].l}, []float64{tp[0].c, tp[1].c, tp[2].c}, 0.3)
	assertInt(t, "periods 0, factor 0.5", out, []int{0, 0, -100})
}

func TestCDLABANDONEDBABYLengths(t *testing.T) {
	for _, p := range []float64{0, 0.3, 1, 3e37} {
		lb := CDLABANDONEDBABYLookback(p)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := CDLABANDONEDBABY(s.O, s.H, s.L, s.C, p)
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
					if v != 0 && v != 100 && v != -100 {
						t.Fatalf("%s n=%d p=%g: [%d]=%d, want -100, 0 or 100", s.Name, n, p, i, v)
					}
				}
			}
		}
	}
}

func TestCDLABANDONEDBABYKnownValues(t *testing.T) {
	top, bot := cdlabandonedbabyTop, cdlabandonedbabyBottom
	with := func(b [3]cdlabandonedbabyBar, k int, nb cdlabandonedbabyBar) [3]cdlabandonedbabyBar {
		b[k] = nb
		return b
	}
	cases := []struct {
		name string
		bars [3]cdlabandonedbabyBar
		pen  float64
		want int
	}{
		// Top: 101 < 104 - 4*p.
		{"top", top, 0.3, -100},
		{"top pen 0", top, 0, -100},
		{"top pen 0.7", top, 0.7, -100}, // 101 < 101.2
		{"top pen 0.75", top, 0.75, 0},  // 101 < 101 is false
		{"top pen 0.8", top, 0.8, 0},    // 101 < 100.8 is false
		{"top pen huge", top, 3e37, 0},
		{"top 1st not long", with(top, 0, cdlabandonedbabyBar{103, 104.5, 99.5, 104}), 0.3, 0},    // body 1 not > 1
		{"top doji too big", with(top, 1, cdlabandonedbabyBar{106, 106.4, 105.8, 106.3}), 0.3, 0}, // 0.3 > 0.23
		{"top 3rd short", with(top, 2, cdlabandonedbabyBar{102, 103.5, 100.5, 101}), 0.3, 0},      // body 1 not > 1.2
		{"top 3rd white", with(top, 2, cdlabandonedbabyBar{101, 103.5, 100.5, 103}), 0.3, 0},
		{"top 1st black", with(top, 0, cdlabandonedbabyBar{104, 104.5, 99.5, 100}), 0.3, 0},
		{"top no 1st gap", with(top, 0, cdlabandonedbabyBar{100, 105.8, 99.5, 104}), 0.3, 0},  // low2 105.8 not > high1
		{"top no 2nd gap", with(top, 2, cdlabandonedbabyBar{103, 105.8, 100.5, 101}), 0.3, 0}, // high3 105.8 not < low2
		// Bottom: 103 > 100 + 4*p.
		{"bottom", bot, 0.3, 100},
		{"bottom pen 0", bot, 0, 100},
		{"bottom pen 0.7", bot, 0.7, 100}, // 103 > 102.8
		{"bottom pen 0.75", bot, 0.75, 0}, // 103 > 103 is false
		{"bottom pen 1", bot, 1, 0},
		{"bottom 1st not long", with(bot, 0, cdlabandonedbabyBar{101, 104.5, 99.5, 100}), 0.3, 0},
		{"bottom doji too big", with(bot, 1, cdlabandonedbabyBar{98, 98.2, 97.6, 97.7}), 0.3, 0},
		{"bottom 3rd short", with(bot, 2, cdlabandonedbabyBar{102, 103.5, 100.5, 103}), 0.3, 0},
		{"bottom 3rd black", with(bot, 2, cdlabandonedbabyBar{103, 103.5, 100.5, 101}), 0.3, 0},
		{"bottom no 1st gap", with(bot, 0, cdlabandonedbabyBar{104, 104.5, 98.2, 100}), 0.3, 0}, // high2 98.2 not < low1
		{"bottom no 2nd gap", with(bot, 2, cdlabandonedbabyBar{101, 103.5, 98.2, 103}), 0.3, 0}, // low3 98.2 not > high2
	}
	for _, tc := range cases {
		o, h, l, c := cdlabandonedbabySeries(tc.bars[:]...)
		out, err := CDLABANDONEDBABY(o, h, l, c, tc.pen)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 15)
		want[14] = tc.want
		assertInt(t, "CDLABANDONEDBABY "+tc.name, out, want)
	}

	// Flat series have zero bodies: never long, never fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLABANDONEDBABY(s.O, s.H, s.L, s.C, 0.3)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLABANDONEDBABYRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlabandonedbabyRange(-1, 5, s.O, s.H, s.L, s.C, 0.3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlabandonedbabyRange(5, 4, s.O, s.H, s.L, s.C, 0.3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlabandonedbabyRange(0, -1, s.O, s.H, s.L, s.C, 0.3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlabandonedbabyRange(0, 11, s.O, s.H, s.L, s.C, 0.3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlabandonedbabyRange(0, 29, s.O, s.H, s.L[:20], s.C, 0.3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlabandonedbabyRange(0, 29, s.O[:29], s.H, s.L, s.C, 0.3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: %v", err)
	}
	if _, _, err := cdlabandonedbabyRange(0, 29, s.O, s.H, s.L, s.C, 0.3, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlabandonedbabyRange(20, 29, s.O, s.H, s.L, s.C, 0.3, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series, including a firing bar.
	o, h, l, c := cdlabandonedbabySeries(cdlabandonedbabyTop[:]...)
	full, _ := CDLABANDONEDBABY(o, h, l, c, 0.3)
	beg, nb, err = cdlabandonedbabyRange(5, 14, o, h, l, c, 0.3, out)
	if err != nil || beg != 12 || nb != 3 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
	if out[2] != -100 {
		t.Errorf("sub-range did not fire: %v", out[:nb])
	}
}
