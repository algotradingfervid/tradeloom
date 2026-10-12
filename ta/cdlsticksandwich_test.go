package ta

import (
	"errors"
	"testing"
)

// cdlsticksandwichBase returns n identical background bars: open 100, close
// 102, high 104, low 99 (high-low range 5, so the default Equal tolerance is
// 0.05 * 5 = 0.25 exactly).
func cdlsticksandwichBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 104)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

type cdlsticksandwichBar struct{ o, h, l, c float64 }

// cdlsticksandwichSeries appends the given bars to 10 background bars, so the
// third bar is at index 12. With default settings (Equal: HighLow, period 5,
// factor 0.05) the Equal average for bar 12 is over bars 5..9 (all
// background), i.e. the five bars before the first candle.
func cdlsticksandwichSeries(bars ...cdlsticksandwichBar) (o, h, l, c []float64) {
	o, h, l, c = cdlsticksandwichBase(10)
	for _, b := range bars {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

var (
	cdlsticksandwichB1 = cdlsticksandwichBar{105, 106, 99.5, 100}   // black, close 100
	cdlsticksandwichB2 = cdlsticksandwichBar{101, 104, 100.5, 103}  // white, low 100.5 > 100
	cdlsticksandwichB3 = cdlsticksandwichBar{104, 104.5, 99.5, 100} // black, same close as 1st
)

func TestCDLSTICKSANDWICHParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLSTICKSANDWICH(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLSTICKSANDWICH(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLSTICKSANDWICH(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLSTICKSANDWICH(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLSTICKSANDWICH(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLSTICKSANDWICH([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLSTICKSANDWICHLookback(); lb != 7 {
		t.Errorf("default lookback = %d, want 7", lb)
	}
}

func TestCDLSTICKSANDWICHLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, tc := range []struct{ period, want int }{{0, 2}, {1, 3}, {5, 7}, {10, 12}, {50, 52}} {
		RestoreAllCandleDefaultSettings()
		if err := SetCandleSettings(Equal, RangeHighLow, tc.period, 0.05); err != nil {
			t.Fatal(err)
		}
		if lb := CDLSTICKSANDWICHLookback(); lb != tc.want {
			t.Errorf("Equal period %d: lookback %d, want %d", tc.period, lb, tc.want)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, BodyShort, BodyDoji, ShadowLong, Near, Far} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 40, d.Factor)
	}
	if lb := CDLSTICKSANDWICHLookback(); lb != 7 {
		t.Errorf("other settings changed: lookback %d, want 7", lb)
	}
	// Period 0: the tolerance is factor * the first candle's own range.
	// First candle range 6.5 → tolerance 0.05*6.5 = 0.325.
	RestoreAllCandleDefaultSettings()
	_ = SetCandleSettings(Equal, RangeHighLow, 0, 0.05)
	b1, b2 := cdlsticksandwichB1, cdlsticksandwichB2
	o, h, l, c := cdlsticksandwichSeries(b1, b2, cdlsticksandwichBar{104, 104.5, 99.5, 100.3})
	out, err := CDLSTICKSANDWICH(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	want := make([]int, 13)
	want[12] = 100
	assertInt(t, "period 0 within own range", out, want)
	c[12] = 100.4 // 0.4 > 0.325
	out, _ = CDLSTICKSANDWICH(o, h, l, c)
	assertInt(t, "period 0 outside own range", out, make([]int, 13))
	// n=2 is exactly the lookback with period 0; n=3 is the first output.
	out, err = CDLSTICKSANDWICH(o[:2], h[:2], l[:2], c[:2])
	if err != nil || len(out) != 2 || out[0] != 0 || out[1] != 0 {
		t.Errorf("n=2 with lookback 2: out=%v err=%v", out, err)
	}
	o3, h3, l3, c3 := []float64{b1.o, b2.o, 104}, []float64{b1.h, b2.h, 104.5}, []float64{b1.l, b2.l, 99.5}, []float64{b1.c, b2.c, 100}
	out, err = CDLSTICKSANDWICH(o3, h3, l3, c3)
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "n=3 period 0", out, []int{0, 0, 100})
	// Factor 0 with period 0: closes must be exactly equal.
	_ = SetCandleSettings(Equal, RangeHighLow, 0, 0)
	o, h, l, c = cdlsticksandwichSeries(b1, b2, cdlsticksandwichB3)
	out, _ = CDLSTICKSANDWICH(o, h, l, c)
	assertInt(t, "factor 0 exact", out, want)
	c[12] = 100.0000001
	out, _ = CDLSTICKSANDWICH(o, h, l, c)
	assertInt(t, "factor 0 not exact", out, make([]int, 13))
}

func TestCDLSTICKSANDWICHLengths(t *testing.T) {
	lb := CDLSTICKSANDWICHLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLSTICKSANDWICH(s.O, s.H, s.L, s.C)
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

func TestCDLSTICKSANDWICHKnownValues(t *testing.T) {
	type B = cdlsticksandwichBar
	b1, b2, b3 := cdlsticksandwichB1, cdlsticksandwichB2, cdlsticksandwichB3
	cases := []struct {
		name       string
		b1, b2, b3 B
		want       int
	}{
		{"stick sandwich", b1, b2, b3, 100},
		{"3rd close at +tolerance", b1, b2, B{104, 104.5, 99.5, 100.25}, 100},
		{"3rd close above +tolerance", b1, b2, B{104, 104.5, 99.5, 100.26}, 0},
		{"3rd close at -tolerance", b1, b2, B{104, 104.5, 99.5, 99.75}, 100},
		{"3rd close below -tolerance", b1, b2, B{104, 104.5, 99.5, 99.74}, 0},
		{"1st white", B{99.5, 106, 99, 100}, b2, b3, 0},
		{"1st doji (white)", B{100, 106, 99.5, 100}, b2, b3, 0},
		{"2nd black", b1, B{103, 104, 100.5, 101}, b3, 0},
		{"2nd doji (white)", b1, B{102, 104, 100.5, 102}, b3, 100},
		{"2nd low equals 1st close", b1, B{101, 104, 100, 103}, b3, 0},
		{"2nd low just above 1st close", b1, B{101, 104, 100.01, 103}, b3, 100},
		{"2nd low below 1st close", b1, B{101, 104, 99, 103}, b3, 0},
		{"3rd white", b1, b2, B{99.9, 104.5, 99.5, 100}, 0},
		{"3rd doji (white)", b1, b2, B{100, 104.5, 99.5, 100}, 0},
		{"no size or shadow checks", B{100.5, 100.5, 100, 100}, B{101, 180, 100.5, 101}, B{140, 150, 50, 100.1}, 100},
	}
	for _, tc := range cases {
		o, h, l, c := cdlsticksandwichSeries(tc.b1, tc.b2, tc.b3)
		out, err := CDLSTICKSANDWICH(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 13)
		want[12] = tc.want
		assertInt(t, "CDLSTICKSANDWICH "+tc.name, out, want)
	}

	// The Equal average excludes the first candle itself: a huge first candle
	// range does not widen the tolerance (still 0.25).
	o, h, l, c := cdlsticksandwichSeries(B{150, 200, 0, 100}, b2, B{104, 104.5, 99.5, 100.26})
	out, _ := CDLSTICKSANDWICH(o, h, l, c)
	assertInt(t, "average excludes 1st candle", out, make([]int, 13))

	// The average includes the five bars before the first candle: widening the
	// background range of bars 5..9 to 10 makes the tolerance 0.5.
	o, h, l, c = cdlsticksandwichSeries(b1, b2, B{104, 104.5, 99.5, 100.5})
	for i := 5; i < 10; i++ {
		h[i], l[i] = 109, 99
	}
	out, _ = CDLSTICKSANDWICH(o, h, l, c)
	want := make([]int, 13)
	want[12] = 100
	assertInt(t, "wider average", out, want)
	// ...but bars before the window (0..4) do not matter.
	o, h, l, c = cdlsticksandwichSeries(b1, b2, B{104, 104.5, 99.5, 100.5})
	for i := 0; i < 5; i++ {
		h[i], l[i] = 109, 99
	}
	out, _ = CDLSTICKSANDWICH(o, h, l, c)
	assertInt(t, "outside window", out, make([]int, 13))

	// Chained sandwiches sharing a black candle fire twice.
	o, h, l, c = cdlsticksandwichSeries(b1, b2, b3, b2, b3)
	out, _ = CDLSTICKSANDWICH(o, h, l, c)
	want = make([]int, 15)
	want[12], want[14] = 100, 100
	assertInt(t, "chained", out, want)

	// Flat, rising, falling and alternating series never fire.
	for _, s := range fixtures(100) {
		switch s.Name {
		case "constant", "zeros", "up", "down":
		default:
			continue
		}
		out, _ := CDLSTICKSANDWICH(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLSTICKSANDWICHRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlsticksandwichRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlsticksandwichRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlsticksandwichRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlsticksandwichRange(0, 6, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlsticksandwichRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlsticksandwichRange(0, 29, s.O[:29], s.H, s.L, s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("input one short: %v", err)
	}
	if _, _, err := cdlsticksandwichRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	if _, nb, err := cdlsticksandwichRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	if beg, nb, err := cdlsticksandwichRange(7, 7, s.O, s.H, s.L, s.C, out); err != nil || beg != 7 || nb != 1 {
		t.Errorf("single bar at lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values match the full series, including a firing bar.
	o, h, l, c := cdlsticksandwichSeries(cdlsticksandwichB1, cdlsticksandwichB2, cdlsticksandwichB3)
	o2, h2, l2, c2 := cdlsticksandwichBase(10)
	o, h, l, c = append(o, o2...), append(h, h2...), append(l, l2...), append(c, c2...)
	full, _ := CDLSTICKSANDWICH(o, h, l, c)
	if full[12] != 100 {
		t.Fatalf("constructed pattern did not fire: %v", full)
	}
	buf := make([]int, len(o))
	beg, nb, err = cdlsticksandwichRange(2, 20, o, h, l, c, buf)
	if err != nil || beg != 7 || nb != 14 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", buf[:nb], full[beg:beg+nb])
	beg, nb, err = cdlsticksandwichRange(12, 22, o, h, l, c, buf)
	if err != nil || beg != 12 || nb != 11 {
		t.Fatalf("sub-range 2: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range 2", buf[:nb], full[beg:beg+nb])
	if buf[0] != 100 {
		t.Errorf("sub-range starting at the firing bar: buf[0]=%d", buf[0])
	}
}
