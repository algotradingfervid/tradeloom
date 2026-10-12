package ta

import (
	"errors"
	"math"
	"testing"
)

// cdlmatholdBar is one OHLC candle for the hand-built cases.
type cdlmatholdBar struct{ o, h, l, c float64 }

// cdlmatholdSeries returns 14 identical background bars (open 100, close
// 101, high 101.5, low 99.5: real body 1) followed by the given bars.
func cdlmatholdSeries(bars ...cdlmatholdBar) (o, h, l, c []float64) {
	for range 14 {
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

// cdlmatholdPattern sits at bars 14-18 of cdlmatholdSeries. With default
// settings (BodyLong and BodyShort: real body, period 10, factor 1):
//   - 1st (bar 14): body 4 > BodyLong avg over bars 4..13 = 1.
//   - 2nd (bar 15): body 0.5 < BodyShort avg over bars 5..14 = (9+4)/10 = 1.3.
//   - 3rd (bar 16): body 0.7 < avg over bars 6..15 = (8+4+0.5)/10 = 1.25.
//   - 4th (bar 17): body 0.6 < avg over bars 7..16 = (7+4+0.5+0.7)/10 = 1.22.
//   - reaction lows 103.8 and 103.4 are below the 1st close 104 and above
//     104 - 4*p, i.e. the pattern needs p > 0.15.
var cdlmatholdPattern = [5]cdlmatholdBar{
	{100, 104.5, 99.5, 104},      // long white
	{105.5, 106, 104.8, 105},     // small black, body gapped up
	{104.5, 104.7, 103.6, 103.8}, // small, holds within the 1st body, falling
	{104, 104.2, 103.2, 103.4},   // small, falling
	{103.6, 106.8, 103.5, 106.5}, // white, opens above 103.4, closes above 106
}

func TestCDLMATHOLDParams(t *testing.T) {
	s := fixtures(50)[0]
	// TA_REAL_DEFAULT (-4e37) is a C sentinel; Go has no sentinel and rejects it.
	for _, p := range []float64{-1, -1e-300, math.NaN(), math.Inf(1), math.Inf(-1), 3.0000001e37, -4e37} {
		if _, err := CDLMATHOLD(s.O, s.H, s.L, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("penetration %g: err = %v, want ErrBadParam", p, err)
		}
		if lb := CDLMATHOLDLookback(p); lb != -1 {
			t.Errorf("CDLMATHOLDLookback(%g) = %d, want -1", p, lb)
		}
		if _, _, err := cdlmatholdRange(0, 49, s.O, s.H, s.L, s.C, p, make([]int, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range penetration %g: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []float64{0, 0.5, 1, 3e37} {
		if _, err := CDLMATHOLD(s.O, s.H, s.L, s.C, p); err != nil {
			t.Errorf("penetration %g: unexpected err %v", p, err)
		}
		if lb := CDLMATHOLDLookback(p); lb != 14 {
			t.Errorf("CDLMATHOLDLookback(%g) = %d, want 14", p, lb)
		}
	}
	if _, err := CDLMATHOLD(s.O, s.H[:49], s.L, s.C, 0.5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLMATHOLD(s.O, s.H, s.L, s.C[:10], 0.5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLMATHOLD(s.O, s.H, nil, s.C, 0.5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLMATHOLD(nil, nil, nil, nil, 0.5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLMATHOLD([]float64{}, []float64{}, []float64{}, []float64{}, 0.5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// A bad penetration is reported even on empty input.
	if _, err := CDLMATHOLD(nil, nil, nil, nil, -1); !errors.Is(err, ErrBadParam) {
		t.Errorf("empty input, bad penetration: err = %v", err)
	}
}

func TestCDLMATHOLDLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyLong, 20, 24},
		{BodyShort, 15, 19},
		{BodyLong, 0, 14},  // BodyShort still 10
		{BodyShort, 0, 14}, // BodyLong still 10
		{BodyDoji, 50, 14}, // unused setting
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLMATHOLDLookback(0.5); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Both average periods 0: only the four prior candles are needed.
	RestoreAllCandleDefaultSettings()
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 1)
	_ = SetCandleSettings(BodyShort, RangeRealBody, 0, 1)
	if lb := CDLMATHOLDLookback(0.5); lb != 4 {
		t.Errorf("all periods 0: lookback %d, want 4", lb)
	}
	p := cdlmatholdPattern
	bare := func() (o, h, l, c []float64) {
		for _, b := range p {
			o, h, l, c = append(o, b.o), append(h, b.h), append(l, b.l), append(c, b.c)
		}
		return
	}
	for n := 0; n <= 5; n++ {
		o, h, l, c := bare()
		out, err := CDLMATHOLD(o[:n], h[:n], l[:n], c[:n], 0.5)
		if err != nil || len(out) != n {
			t.Errorf("n=%d with lookback 4: out=%v err=%v", n, out, err)
		}
	}
	// With periods 0 and factor 1 each candle is compared with its own body:
	// the 1st body is never > itself, so the bare pattern does not fire.
	o, h, l, c := bare()
	out, _ := CDLMATHOLD(o, h, l, c, 0.5)
	assertInt(t, "periods 0, body > itself", out, []int{0, 0, 0, 0, 0})
	// BodyLong factor 0.5 and BodyShort factor 2: now it fires.
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 0.5)
	_ = SetCandleSettings(BodyShort, RangeRealBody, 0, 2)
	out, _ = CDLMATHOLD(o, h, l, c, 0.5)
	assertInt(t, "periods 0, loose factors", out, []int{0, 0, 0, 0, 100})
}

func TestCDLMATHOLDLengths(t *testing.T) {
	for _, p := range []float64{0, 0.5, 1, 3e37} {
		lb := CDLMATHOLDLookback(p)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := CDLMATHOLD(s.O, s.H, s.L, s.C, p)
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
						t.Fatalf("%s n=%d p=%g: [%d]=%d, want 0 or 100", s.Name, n, p, i, v)
					}
				}
			}
		}
	}
}

func TestCDLMATHOLDKnownValues(t *testing.T) {
	pat := cdlmatholdPattern
	with := func(k int, nb cdlmatholdBar) [5]cdlmatholdBar {
		b := pat
		b[k] = nb
		return b
	}
	cases := []struct {
		name string
		bars [5]cdlmatholdBar
		pen  float64
		want int
	}{
		{"pattern", pat, 0.5, 100},
		{"pen 0.2", pat, 0.2, 100}, // 103.4 > 103.2
		{"pen 0.1", pat, 0.1, 0},   // 103.4 > 103.6 is false
		{"pen 0", pat, 0, 0},       // reaction lows must be > 104 and < 104
		{"pen 1", pat, 1, 100},     // 103.4 > 100
		{"pen huge", pat, 3e37, 100},
		// 3rd and 4th colours are not checked.
		{"3rd white", with(2, cdlmatholdBar{103.8, 104.7, 103.6, 104.5}), 0.5, 100},
		{"4th white", with(3, cdlmatholdBar{103.4, 104.2, 103.2, 103.5}), 0.5, 100},
		{"1st black", with(0, cdlmatholdBar{104, 104.5, 99.5, 100}), 0.5, 0},
		{"1st not long", with(0, cdlmatholdBar{103, 104.5, 99.5, 104}), 0.5, 0}, // body 1 not > 1
		{"2nd white", with(1, cdlmatholdBar{105, 106, 104.8, 105.5}), 0.5, 0},
		{"2nd not small", with(1, cdlmatholdBar{106.5, 106.6, 104.8, 105}), 0.5, 0},     // body 1.5 not < 1.3
		{"no body gap", with(1, cdlmatholdBar{104.5, 104.6, 103.8, 104}), 0.5, 0},       // min 104 not > 104
		{"3rd not small", with(2, cdlmatholdBar{105.2, 105.3, 103.6, 103.8}), 0.5, 0},   // body 1.4
		{"3rd above 1st", with(2, cdlmatholdBar{105.2, 105.3, 104.1, 104.2}), 0.5, 0},   // min 104.2 not < 104
		{"3rd not falling", with(2, cdlmatholdBar{105.6, 105.7, 103.6, 105}), 0.5, 0},   // max 105.6 not < 105.5
		{"3rd too deep", with(2, cdlmatholdBar{102.5, 104.7, 101.6, 101.8}), 0.5, 0},    // 101.8 not > 102
		{"4th not falling", with(3, cdlmatholdBar{104.5, 104.6, 103.2, 103.9}), 0.5, 0}, // max 104.5 not < 104.5
		{"4th too deep", with(3, cdlmatholdBar{102.4, 104.2, 101.8, 101.9}), 0.5, 0},
		{"4th above 1st", with(3, cdlmatholdBar{104.3, 104.4, 104.05, 104.1}), 0.5, 0},
		{"4th not small", with(3, cdlmatholdBar{104.4, 104.45, 103.0, 103.1}), 0.5, 0}, // body 1.3 > 1.22
		{"5th black", with(4, cdlmatholdBar{106.6, 106.8, 103.5, 106.5}), 0.5, 0},
		{"5th opens low", with(4, cdlmatholdBar{103.4, 106.8, 103.3, 106.5}), 0.5, 0}, // 103.4 not > 103.4
		{"5th closes low", with(4, cdlmatholdBar{103.6, 106.1, 103.5, 106}), 0.5, 0},  // 106 not > 106
	}
	for _, tc := range cases {
		o, h, l, c := cdlmatholdSeries(tc.bars[:]...)
		out, err := CDLMATHOLD(o, h, l, c, tc.pen)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 19)
		want[18] = tc.want
		assertInt(t, "CDLMATHOLD "+tc.name, out, want)
	}

	// Flat series have zero bodies: never long, never fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLMATHOLD(s.O, s.H, s.L, s.C, 0.5)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLMATHOLDRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlmatholdRange(-1, 5, s.O, s.H, s.L, s.C, 0.5, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlmatholdRange(5, 4, s.O, s.H, s.L, s.C, 0.5, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlmatholdRange(0, -1, s.O, s.H, s.L, s.C, 0.5, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlmatholdRange(0, 13, s.O, s.H, s.L, s.C, 0.5, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlmatholdRange(0, 29, s.O, s.H, s.L[:20], s.C, 0.5, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlmatholdRange(0, 29, s.O[:29], s.H, s.L, s.C, 0.5, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: %v", err)
	}
	if _, _, err := cdlmatholdRange(0, 29, s.O, s.H, s.L, s.C, 0.5, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlmatholdRange(20, 29, s.O, s.H, s.L, s.C, 0.5, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series, including the firing bar.
	o, h, l, c := cdlmatholdSeries(cdlmatholdPattern[:]...)
	full, _ := CDLMATHOLD(o, h, l, c, 0.5)
	beg, nb, err = cdlmatholdRange(5, 18, o, h, l, c, 0.5, out)
	if err != nil || beg != 14 || nb != 5 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
	if out[4] != 100 {
		t.Errorf("sub-range did not fire: %v", out[:nb])
	}
	beg, nb, err = cdlmatholdRange(18, 18, o, h, l, c, 0.5, out)
	if err != nil || beg != 18 || nb != 1 || out[0] != 100 {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%d", beg, nb, err, out[0])
	}
}
