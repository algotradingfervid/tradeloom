package ta

import (
	"errors"
	"math"
	"testing"
)

// cdlseparatinglinesBar is one OHLC candle.
type cdlseparatinglinesBar struct{ o, h, l, c float64 }

// cdlseparatinglinesBase returns n identical background bars: open 100,
// close 102, high 103, low 99 (real body 2, high-low range 4).
func cdlseparatinglinesBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

// cdlseparatinglinesSeries appends the two pattern bars to 10 background
// bars, so the second bar is index 11 (the first evaluated bar with default
// settings).
func cdlseparatinglinesSeries(b1, b2 cdlseparatinglinesBar) (o, h, l, c []float64) {
	o, h, l, c = cdlseparatinglinesBase(10)
	for _, b := range []cdlseparatinglinesBar{b1, b2} {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

// cdlseparatinglinesEq is the default Equal band for the 1st candle (bar
// 10): 0.05 times the mean high-low range of bars 5..9, computed exactly as
// the implementation does.
func cdlseparatinglinesEq() float64 {
	sum := 0.0
	for range 5 {
		sum += 4.0
	}
	return 0.05 * (sum / 5) / 1.0
}

// cdlseparatinglinesSVS is the default ShadowVeryShort threshold for the
// 2nd candle (bar 11): 0.1 times the mean high-low range of bars 1..10 (nine
// background bars, then b1), accumulated in the implementation's order.
func cdlseparatinglinesSVS(b1 cdlseparatinglinesBar) float64 {
	sum := 0.0
	for range 9 {
		sum += 4.0
	}
	sum += b1.h - b1.l
	return 0.1 * (sum / 10) / 1.0
}

// cdlseparatinglinesExactBelow returns x near ref-d with ref-x == d exactly,
// or ok=false if no nearby float gives that.
func cdlseparatinglinesExactBelow(ref, d float64) (x float64, ok bool) {
	x = ref - d
	for range 8 {
		x = math.Nextafter(x, math.Inf(1))
	}
	for range 17 {
		if ref-x == d {
			return x, true
		}
		x = math.Nextafter(x, math.Inf(-1))
	}
	return 0, false
}

func TestCDLSEPARATINGLINESParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLSEPARATINGLINES(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLSEPARATINGLINES(s.O[:3], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLSEPARATINGLINES(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLSEPARATINGLINES(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLSEPARATINGLINES(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLSEPARATINGLINES([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLSEPARATINGLINESLookback(); lb != 11 {
		t.Errorf("default lookback = %d, want 11", lb)
	}
}

func TestCDLSEPARATINGLINESLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{ShadowVeryShort, 20, 21},
		{BodyLong, 25, 26},
		{Equal, 30, 31},
		{ShadowVeryShort, 0, 11}, // BodyLong still 10
		{BodyLong, 0, 11},        // ShadowVeryShort still 10
		{Equal, 0, 11},
		{BodyShort, 50, 11},
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLSEPARATINGLINESLookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// BodyLong and ShadowVeryShort both 0: Equal (5) decides.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, ShadowVeryShort} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLSEPARATINGLINESLookback(); lb != 6 {
		t.Errorf("BodyLong, ShadowVeryShort 0: lookback %d, want 6", lb)
	}
	// All average periods 0: only the prior candle is needed.
	d := GetCandleSetting(Equal)
	_ = SetCandleSettings(Equal, d.RangeType, 0, d.Factor)
	if lb := CDLSEPARATINGLINESLookback(); lb != 1 {
		t.Errorf("all periods 0: lookback %d, want 1", lb)
	}
	for n := 0; n <= 2; n++ {
		o, h, l, c := cdlseparatinglinesBase(n)
		out, err := CDLSEPARATINGLINES(o, h, l, c)
		if err != nil || len(out) != n {
			t.Errorf("n=%d with lookback 1: out=%v err=%v", n, out, err)
		}
		for i, v := range out {
			if v != 0 {
				t.Errorf("n=%d: [%d]=%d", n, i, v)
			}
		}
	}
	// Lookback 1 with a two-bar pattern: bar 1 is evaluated against its own
	// candle (BodyLong factor 1 on its own body: never strictly longer).
	out, _ := CDLSEPARATINGLINES([]float64{100, 100}, []float64{100.5, 106}, []float64{94, 100}, []float64{94, 106})
	assertInt(t, "lookback 1 own-body", out, []int{0, 0})
	// With BodyLong factor 0.5 on its own body the long-body test passes; the
	// shadow test uses 0.1 * own high-low range (6): 0 < 0.6.
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 0.5)
	out, _ = CDLSEPARATINGLINES([]float64{100, 100}, []float64{100.5, 106}, []float64{94, 100}, []float64{94, 106})
	assertInt(t, "lookback 1 fires", out, []int{0, 100})
}

func TestCDLSEPARATINGLINESLengths(t *testing.T) {
	lb := CDLSEPARATINGLINESLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLSEPARATINGLINES(s.O, s.H, s.L, s.C)
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

// cdlseparatinglinesCases are hand-derived cases. Background bodies are 2
// and ranges 4. For the 2nd candle (bar 11) the BodyLong average covers bars
// 1..10: (9*2 + body1)/10, and the ShadowVeryShort threshold is 0.1 * (9*4 +
// range1)/10. The Equal band around the 1st open is 0.05 * 4 = 0.2.
func cdlseparatinglinesCases() []struct {
	name   string
	b1, b2 cdlseparatinglinesBar
	want   int
} {
	type b = cdlseparatinglinesBar
	eq := cdlseparatinglinesEq()
	up, dn := 100+eq, 100-eq
	// Black 1st: body 6, range 7 -> BodyLong avg 2.4, SVS threshold 0.43
	// (see cdlseparatinglinesSVS).
	black := b{100, 100.5, 93.5, 94}
	// White 1st: body 6, range 7.
	white := b{100, 106.5, 99.5, 106}
	cases := []struct {
		name   string
		b1, b2 cdlseparatinglinesBar
		want   int
	}{
		{"bullish", black, b{100, 106.5, 100, 106}, 100},
		{"bearish", white, b{100, 100, 93.5, 94}, -100},
		{"bullish long upper shadow", black, b{100, 115, 100, 106}, 100},
		{"bearish long lower shadow", white, b{100, 100, 85, 94}, -100},
		{"bullish small lower shadow", black, b{100, 106.5, 99.7, 106}, 100},
		{"bullish lower shadow too long", black, b{100, 106.5, 99.5, 106}, 0},
		{"bearish small upper shadow", white, b{100, 100.3, 93.5, 94}, -100},
		{"bearish upper shadow too long", white, b{100, 100.5, 93.5, 94}, 0},
		{"bullish open at upper band", black, b{up, 106.5, up, 106}, 100},
		{"bullish open at lower band", black, b{dn, 106.5, dn, 106}, 100},
		{"bullish open above band", black, b{math.Nextafter(up, math.Inf(1)), 106.5, 100.3, 106}, 0},
		{"bullish open below band", black, b{math.Nextafter(dn, math.Inf(-1)), 106.5, 99.7, 106}, 0},
		{"bearish open at upper band", white, b{up, up, 93.5, 94}, -100},
		{"bearish open above band", white, b{math.Nextafter(up, math.Inf(1)), 100.3, 93.5, 94}, 0},
		{"same color (white)", white, b{100, 106.5, 100, 106}, 0},
		{"same color (black)", black, b{100, 100, 93.5, 94}, 0},
		// 1st body 2: BodyLong average exactly 2. Body 2 is not long; 2.5 is.
		{"body equals average", b{100, 100.5, 97.5, 98}, b{100, 102.5, 100, 102}, 0},
		{"body above average", b{100, 100.5, 97.5, 98}, b{100, 102.5, 100, 102.5}, 100},
		// A doji 2nd counts as white but has no body: never long.
		{"2nd doji", black, b{100, 100.5, 100, 100}, 0},
		// A doji 1st counts as white, so a black 2nd is opposite.
		{"1st doji, bearish 2nd", b{100, 100.5, 99.5, 100}, b{100, 100, 93.5, 94}, -100},
	}
	// Shadow boundary: 1st black candle with open 1, body 4 and range 4, so
	// the ShadowVeryShort threshold is 0.1*(40/10) = 0.4 and an open of 1
	// lets the lower shadow equal it exactly (1 - 0.6 == 0.4 in float64).
	// BodyLong average is (18+4)/10 = 2.2; the 2nd body is 4.
	small := b{1, 1, -3, -3}
	l, ok := cdlseparatinglinesExactBelow(1, cdlseparatinglinesSVS(small))
	if !ok {
		panic("cdlseparatinglines: no float gives shadow == threshold")
	}
	cases = append(cases, []struct {
		name   string
		b1, b2 cdlseparatinglinesBar
		want   int
	}{
		{"bullish lower shadow equals threshold", small, b{1, 5, l, 5}, 0},
		{"bullish lower shadow just below threshold", small, b{1, 5, math.Nextafter(l, math.Inf(1)), 5}, 100},
		{"bullish small open", small, b{1, 5, 1, 5}, 100},
	}...)
	return cases
}

func TestCDLSEPARATINGLINESKnownValues(t *testing.T) {
	for _, tc := range cdlseparatinglinesCases() {
		o, h, l, c := cdlseparatinglinesSeries(tc.b1, tc.b2)
		out, err := CDLSEPARATINGLINES(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 12)
		want[11] = tc.want
		assertInt(t, "CDLSEPARATINGLINES "+tc.name, out, want)
	}

	// Constant and zero series have zero bodies: never long, never fire.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLSEPARATINGLINES(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLSEPARATINGLINESRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlseparatinglinesRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlseparatinglinesRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlseparatinglinesRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlseparatinglinesRange(0, 10, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlseparatinglinesRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlseparatinglinesRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlseparatinglinesRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLSEPARATINGLINES(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlseparatinglinesRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 11 || nb != 15 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
