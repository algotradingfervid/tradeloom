package ta

import (
	"errors"
	"testing"
)

// cdlmatchinglowBase returns n identical background bars: open 100, close
// 102, high 104, low 99 (high-low range 5, so the default Equal tolerance is
// 0.05 * 5 = 0.25 exactly).
func cdlmatchinglowBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 104)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

type cdlmatchinglowBar struct{ o, h, l, c float64 }

// cdlmatchinglowSeries appends two bars to 10 background bars, so the second
// bar is at index 11. With default settings (Equal: HighLow, period 5, factor
// 0.05) the Equal average for bar 11 is over bars 5..9 (all background).
func cdlmatchinglowSeries(b1, b2 cdlmatchinglowBar) (o, h, l, c []float64) {
	o, h, l, c = cdlmatchinglowBase(10)
	for _, b := range []cdlmatchinglowBar{b1, b2} {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

var (
	cdlmatchinglowB1 = cdlmatchinglowBar{105, 106, 99.5, 100}   // black, close 100
	cdlmatchinglowB2 = cdlmatchinglowBar{104, 104.5, 99.5, 100} // black, same close
)

func TestCDLMATCHINGLOWParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLMATCHINGLOW(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLMATCHINGLOW(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLMATCHINGLOW(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLMATCHINGLOW(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLMATCHINGLOW(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLMATCHINGLOW([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLMATCHINGLOWLookback(); lb != 6 {
		t.Errorf("default lookback = %d, want 6", lb)
	}
}

func TestCDLMATCHINGLOWLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, tc := range []struct{ period, want int }{{0, 1}, {1, 2}, {5, 6}, {10, 11}, {50, 51}} {
		RestoreAllCandleDefaultSettings()
		if err := SetCandleSettings(Equal, RangeHighLow, tc.period, 0.05); err != nil {
			t.Fatal(err)
		}
		if lb := CDLMATCHINGLOWLookback(); lb != tc.want {
			t.Errorf("Equal period %d: lookback %d, want %d", tc.period, lb, tc.want)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, BodyShort, BodyDoji, ShadowLong, Near, Far} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 40, d.Factor)
	}
	if lb := CDLMATCHINGLOWLookback(); lb != 6 {
		t.Errorf("other settings changed: lookback %d, want 6", lb)
	}
	// Period 0: the tolerance is factor * the first candle's own range.
	// First candle range 6.5 → tolerance 0.05*6.5 = 0.325.
	RestoreAllCandleDefaultSettings()
	_ = SetCandleSettings(Equal, RangeHighLow, 0, 0.05)
	o, h, l, c := cdlmatchinglowSeries(cdlmatchinglowB1, cdlmatchinglowBar{104, 104.5, 99.5, 100.3})
	out, err := CDLMATCHINGLOW(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	want := make([]int, 12)
	want[11] = 100
	assertInt(t, "period 0 within own range", out, want)
	c[11] = 100.4 // 0.4 > 0.325
	out, _ = CDLMATCHINGLOW(o, h, l, c)
	assertInt(t, "period 0 outside own range", out, make([]int, 12))
	// n=1 is exactly the lookback with period 0.
	out, err = CDLMATCHINGLOW(o[:1], h[:1], l[:1], c[:1])
	if err != nil || len(out) != 1 || out[0] != 0 {
		t.Errorf("n=1 with lookback 1: out=%v err=%v", out, err)
	}
	// Factor 0 with period 0: closes must be exactly equal.
	_ = SetCandleSettings(Equal, RangeHighLow, 0, 0)
	o, h, l, c = cdlmatchinglowSeries(cdlmatchinglowB1, cdlmatchinglowB2)
	out, _ = CDLMATCHINGLOW(o, h, l, c)
	assertInt(t, "factor 0 exact", out, want)
}

func TestCDLMATCHINGLOWLengths(t *testing.T) {
	lb := CDLMATCHINGLOWLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLMATCHINGLOW(s.O, s.H, s.L, s.C)
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

func TestCDLMATCHINGLOWKnownValues(t *testing.T) {
	type B = cdlmatchinglowBar
	b1, b2 := cdlmatchinglowB1, cdlmatchinglowB2
	cases := []struct {
		name   string
		b1, b2 B
		want   int
	}{
		{"matching low", b1, b2, 100},
		{"2nd close at +tolerance", b1, B{104, 104.5, 99.5, 100.25}, 100},
		{"2nd close above +tolerance", b1, B{104, 104.5, 99.5, 100.26}, 0},
		{"2nd close at -tolerance", b1, B{104, 104.5, 99.5, 99.75}, 100},
		{"2nd close below -tolerance", b1, B{104, 104.5, 99.5, 99.74}, 0},
		{"1st white", B{100, 106, 99.5, 105}, b2, 0},
		{"1st doji (white)", B{100, 106, 99.5, 100}, B{104, 104.5, 99.5, 100}, 0},
		{"2nd white", b1, B{99.9, 104.5, 99.5, 100}, 0},
		{"2nd doji (white)", b1, B{100, 104.5, 99.5, 100}, 0},
		{"no shadow or body checks", B{100.5, 100.5, 100, 100}, B{140, 150, 50, 100.1}, 100},
	}
	for _, tc := range cases {
		o, h, l, c := cdlmatchinglowSeries(tc.b1, tc.b2)
		out, err := CDLMATCHINGLOW(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 12)
		want[11] = tc.want
		assertInt(t, "CDLMATCHINGLOW "+tc.name, out, want)
	}

	// The Equal average excludes the first candle itself: a huge first candle
	// range does not widen the tolerance (still 0.25).
	o, h, l, c := cdlmatchinglowSeries(B{150, 200, 0, 100}, B{104, 104.5, 99.5, 100.26})
	out, _ := CDLMATCHINGLOW(o, h, l, c)
	assertInt(t, "average excludes 1st candle", out, make([]int, 12))

	// The average does include the bar before the first candle: widening the
	// background range of bars 5..9 to 10 makes the tolerance 0.5.
	o, h, l, c = cdlmatchinglowSeries(b1, B{104, 104.5, 99.5, 100.5})
	for i := 5; i < 10; i++ {
		h[i], l[i] = 109, 99
	}
	out, _ = CDLMATCHINGLOW(o, h, l, c)
	want := make([]int, 12)
	want[11] = 100
	assertInt(t, "wider average", out, want)
	// ...but bars before the window (0..4) do not matter.
	o, h, l, c = cdlmatchinglowSeries(b1, B{104, 104.5, 99.5, 100.5})
	for i := 0; i < 5; i++ {
		h[i], l[i] = 109, 99
	}
	out, _ = CDLMATCHINGLOW(o, h, l, c)
	assertInt(t, "outside window", out, make([]int, 12))

	// Three equal black closes in a row fire twice.
	o, h, l, c = cdlmatchinglowBase(10)
	for _, b := range []B{b1, b2, b2} {
		o, h, l, c = append(o, b.o), append(h, b.h), append(l, b.l), append(c, b.c)
	}
	out, _ = CDLMATCHINGLOW(o, h, l, c)
	want = make([]int, 13)
	want[11], want[12] = 100, 100
	assertInt(t, "three in a row", out, want)

	// Flat, rising, falling and alternating series never fire.
	for _, s := range fixtures(100) {
		switch s.Name {
		case "constant", "zeros", "up", "down", "zigzag":
		default:
			continue
		}
		out, _ := CDLMATCHINGLOW(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLMATCHINGLOWRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlmatchinglowRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlmatchinglowRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlmatchinglowRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlmatchinglowRange(0, 5, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlmatchinglowRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlmatchinglowRange(0, 29, s.O[:29], s.H, s.L, s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("input one short: %v", err)
	}
	if _, _, err := cdlmatchinglowRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	if _, nb, err := cdlmatchinglowRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	if beg, nb, err := cdlmatchinglowRange(6, 6, s.O, s.H, s.L, s.C, out); err != nil || beg != 6 || nb != 1 {
		t.Errorf("single bar at lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values match the full series, including a firing bar.
	o, h, l, c := cdlmatchinglowSeries(cdlmatchinglowB1, cdlmatchinglowB2)
	o2, h2, l2, c2 := cdlmatchinglowBase(10)
	o, h, l, c = append(o, o2...), append(h, h2...), append(l, l2...), append(c, c2...)
	full, _ := CDLMATCHINGLOW(o, h, l, c)
	if full[11] != 100 {
		t.Fatalf("constructed pattern did not fire: %v", full)
	}
	buf := make([]int, len(o))
	beg, nb, err = cdlmatchinglowRange(2, 20, o, h, l, c, buf)
	if err != nil || beg != 6 || nb != 15 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", buf[:nb], full[beg:beg+nb])
	beg, nb, err = cdlmatchinglowRange(11, 21, o, h, l, c, buf)
	if err != nil || beg != 11 || nb != 11 {
		t.Fatalf("sub-range 2: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range 2", buf[:nb], full[beg:beg+nb])
}
