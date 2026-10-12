package ta

import (
	"errors"
	"testing"
)

type cdlgapsidesidewhiteBar struct{ o, h, l, c float64 }

// cdlgapsidesidewhiteSeries appends three bars to 10 background bars (open
// 100, close 102, high 103, low 99: high-low range 4, body 2), so the third
// bar is at index 12.
func cdlgapsidesidewhiteSeries(bars [3]cdlgapsidesidewhiteBar) (o, h, l, c []float64) {
	for range 10 {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	for _, b := range bars {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

// Canonical up-gap and down-gap patterns. The first candle has range 4 like
// the background, so every Near/Equal average sees a high-low range of 4.
var (
	cdlgapsidesidewhiteUp = [3]cdlgapsidesidewhiteBar{
		{100, 103, 99, 102},      // 1st: any candle, body [100,102]
		{105, 108.5, 104.5, 108}, // white, body gaps above 102
		{105, 108.5, 104.5, 108}, // white, same open, same size
	}
	cdlgapsidesidewhiteDown = [3]cdlgapsidesidewhiteBar{
		{110, 111, 107, 108},     // 1st: black, body [108,110]
		{102, 105.5, 101.5, 105}, // white, body gaps below 108
		{102, 105.5, 101.5, 105}, // white, same open, same size
	}
)

func cdlgapsidesidewhiteWith(base [3]cdlgapsidesidewhiteBar, k int, b cdlgapsidesidewhiteBar) [3]cdlgapsidesidewhiteBar {
	base[k] = b
	return base
}

// cdlgapsidesidewhiteExactSettings sets dyadic factors so the tolerance bands
// are exact: Near average 0.25*4 = 1, Equal average 0.125*4 = 0.5.
func cdlgapsidesidewhiteExactSettings(t *testing.T) {
	t.Helper()
	if err := SetCandleSettings(Near, RangeHighLow, 5, 0.25); err != nil {
		t.Fatal(err)
	}
	if err := SetCandleSettings(Equal, RangeHighLow, 5, 0.125); err != nil {
		t.Fatal(err)
	}
}

func TestCDLGAPSIDESIDEWHITEParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLGAPSIDESIDEWHITE(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLGAPSIDESIDEWHITE(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLGAPSIDESIDEWHITE(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLGAPSIDESIDEWHITE(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLGAPSIDESIDEWHITE(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLGAPSIDESIDEWHITE([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLGAPSIDESIDEWHITELookback(); lb != 7 {
		t.Errorf("default lookback = %d, want 7", lb)
	}
}

func TestCDLGAPSIDESIDEWHITELookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, tc := range []struct{ near, equal, want int }{
		{0, 0, 2}, {1, 0, 3}, {0, 1, 3}, {5, 5, 7}, {10, 3, 12}, {3, 10, 12}, {50, 20, 52},
	} {
		RestoreAllCandleDefaultSettings()
		if err := SetCandleSettings(Near, RangeHighLow, tc.near, 0.2); err != nil {
			t.Fatal(err)
		}
		if err := SetCandleSettings(Equal, RangeHighLow, tc.equal, 0.05); err != nil {
			t.Fatal(err)
		}
		if lb := CDLGAPSIDESIDEWHITELookback(); lb != tc.want {
			t.Errorf("near %d equal %d: lookback %d, want %d", tc.near, tc.equal, lb, tc.want)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, BodyShort, BodyDoji, ShadowLong, Far} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 40, d.Factor)
	}
	if lb := CDLGAPSIDESIDEWHITELookback(); lb != 7 {
		t.Errorf("other settings changed: lookback %d, want 7", lb)
	}

	// Periods 0: the averages use bar i-1's own range; lookback 2, so n=3
	// is the smallest input with an output.
	RestoreAllCandleDefaultSettings()
	_ = SetCandleSettings(Near, RangeHighLow, 0, 0.25)
	_ = SetCandleSettings(Equal, RangeHighLow, 0, 0.125)
	o, h, l, c := cdlgapsidesidewhiteSeries(cdlgapsidesidewhiteUp)
	out, err := CDLGAPSIDESIDEWHITE(o[10:], h[10:], l[10:], c[10:])
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "periods 0", out, []int{0, 0, 100})
	out, err = CDLGAPSIDESIDEWHITE(o[10:12], h[10:12], l[10:12], c[10:12])
	if err != nil || len(out) != 2 {
		t.Fatalf("n=2 with lookback 2: out=%v err=%v", out, err)
	}
	assertInt(t, "n=2", out, []int{0, 0})
	// Periods 0: bar 2's range is 4 here (108.5-104.5), so Equal band is 0.5.
	bars := cdlgapsidesidewhiteWith(cdlgapsidesidewhiteUp, 2, cdlgapsidesidewhiteBar{105.5, 108.5, 104.5, 108.5})
	o, h, l, c = cdlgapsidesidewhiteSeries(bars)
	out, _ = CDLGAPSIDESIDEWHITE(o[10:], h[10:], l[10:], c[10:])
	assertInt(t, "periods 0 open +0.5", out, []int{0, 0, 100})
	bars[2] = cdlgapsidesidewhiteBar{105.75, 108.75, 104.5, 108.75}
	o, h, l, c = cdlgapsidesidewhiteSeries(bars)
	out, _ = CDLGAPSIDESIDEWHITE(o[10:], h[10:], l[10:], c[10:])
	assertInt(t, "periods 0 open +0.75", out, []int{0, 0, 0})
}

func TestCDLGAPSIDESIDEWHITELengths(t *testing.T) {
	lb := CDLGAPSIDESIDEWHITELookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLGAPSIDESIDEWHITE(s.O, s.H, s.L, s.C)
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
				if v != 0 && v != -100 && v != 100 {
					t.Fatalf("%s n=%d: [%d]=%d, want 0 or +-100", s.Name, n, i, v)
				}
			}
		}
	}
}

func TestCDLGAPSIDESIDEWHITEKnownValues(t *testing.T) {
	type B = cdlgapsidesidewhiteBar
	up, dn := cdlgapsidesidewhiteUp, cdlgapsidesidewhiteDown
	w := cdlgapsidesidewhiteWith
	run := func(label string, bars [3]B, want int) {
		t.Helper()
		o, h, l, c := cdlgapsidesidewhiteSeries(bars)
		out, err := CDLGAPSIDESIDEWHITE(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		exp := make([]int, 13)
		exp[12] = want
		assertInt(t, "CDLGAPSIDESIDEWHITE "+label, out, exp)
	}

	// Default settings: Near band 0.2*4 = 0.8, Equal band 0.05*4 = 0.2.
	run("default up", up, 100)
	run("default down", dn, -100)
	run("default up open +0.1", w(up, 2, B{105.1, 108.6, 104.6, 108.1}), 100)
	run("default up open +0.3", w(up, 2, B{105.3, 108.8, 104.8, 108.3}), 0)
	run("default up body +0.5", w(up, 2, B{105, 109, 104.5, 108.5}), 100)
	run("default up body +1", w(up, 2, B{105, 109.5, 104.5, 109}), 0)
	run("default up body -1", w(up, 2, B{105, 107.5, 104.5, 107}), 0)

	defer RestoreAllCandleDefaultSettings()
	cdlgapsidesidewhiteExactSettings(t)
	// Near band exactly 1, Equal band exactly 0.5.
	cases := []struct {
		name string
		bars [3]B
		want int
	}{
		{"up", up, 100},
		{"down", dn, -100},
		{"up, 1st white or black irrelevant", w(up, 0, B{102, 103, 99, 100}), 100},
		{"down, 1st white", w(dn, 0, B{108, 111, 107, 110}), -100},
		// Gap conditions.
		{"up 2nd body touches 1st", w(w(up, 1, B{102, 105.5, 101.5, 105}), 2, B{102.5, 105.5, 102, 105.5}), 0},
		{"up 2nd body just above 1st", w(w(up, 1, B{102.25, 105.5, 101.5, 105.25}), 2, B{102.5, 105.5, 102, 105.5}), 100},
		{"up 3rd body touches 1st", w(w(up, 1, B{102.5, 106, 102, 105.5}), 2, B{102, 105.5, 101.5, 105}), 0},
		{"up 3rd body just above 1st", w(w(up, 1, B{102.5, 106, 102, 105.5}), 2, B{102.25, 105.5, 101.5, 105.25}), 100},
		{"down 2nd body touches 1st", w(w(dn, 1, B{105, 108.5, 104.5, 108}), 2, B{104.5, 107.5, 104, 107.5}), 0},
		{"down 2nd body just below 1st", w(w(dn, 1, B{105, 108.5, 104.5, 107.75}), 2, B{104.5, 107.5, 104, 107.5}), -100},
		{"down 3rd body touches 1st", w(w(dn, 1, B{104, 107.5, 103.5, 107}), 2, B{104.5, 108.5, 104, 108}), 0},
		{"down 3rd body just below 1st", w(w(dn, 1, B{104, 107.5, 103.5, 107}), 2, B{104.5, 108.5, 104, 107.75}), -100},
		{"2nd up 3rd down", w(w(up, 1, B{103, 106, 102.5, 105.5}), 2, B{98, 101, 97.5, 98}), 0},
		// Colours: a doji (close == open) counts as white.
		{"up 2nd black", w(up, 1, B{108, 108.5, 104.5, 105}), 0},
		{"up 3rd black", w(up, 2, B{108, 108.5, 104.5, 105}), 0},
		{"down 3rd black", w(dn, 2, B{105, 105.5, 101.5, 102}), 0},
		{"up two dojis", w(w(up, 1, B{105, 107, 103, 105}), 2, B{105, 107, 103, 105}), 100},
		// Near: |body3 - body2| <= 1 (body2 = 3).
		{"up body3 = body2+1", w(up, 2, B{105, 109.5, 104.5, 109}), 100},
		{"up body3 = body2+1.25", w(up, 2, B{105, 109.5, 104.5, 109.25}), 0},
		{"up body3 = body2-1", w(up, 2, B{105, 108.5, 104.5, 107}), 100},
		{"up body3 = body2-1.25", w(up, 2, B{105, 108.5, 104.5, 106.75}), 0},
		{"down body3 = body2+1", w(dn, 2, B{102, 106.5, 101.5, 106}), -100},
		{"down body3 = body2-1.25", w(dn, 2, B{102, 105.5, 101.5, 103.75}), 0},
		// Equal: |open3 - open2| <= 0.5.
		{"up open3 = open2+0.5", w(up, 2, B{105.5, 109, 105, 108.5}), 100},
		{"up open3 = open2+0.75", w(up, 2, B{105.75, 109.25, 105, 108.75}), 0},
		{"up open3 = open2-0.5", w(up, 2, B{104.5, 108, 104, 107.5}), 100},
		{"up open3 = open2-0.75", w(up, 2, B{104.25, 108, 104, 107.25}), 0},
		{"down open3 = open2-0.5", w(dn, 2, B{101.5, 105, 101, 104.5}), -100},
		{"down open3 = open2+0.75", w(dn, 2, B{102.75, 106, 102, 105.75}), 0},
	}
	for _, tc := range cases {
		run(tc.name, tc.bars, tc.want)
	}

	// The averages exclude bar i-1 itself: a huge range on bar 2 does not
	// widen the bands (open +0.75 still rejected).
	run("2nd range huge", w(w(up, 1, B{105, 200, 104.5, 108}), 2, B{105.75, 109.25, 105, 108.75}), 0)
	// But the bars before it do: widening the background ranges to 8 doubles
	// both bands, so open +0.75 and body +1.25 now pass.
	o, h, l, c := cdlgapsidesidewhiteSeries(w(up, 2, B{105.75, 109.5, 105, 109}))
	for i := 7; i < 11; i++ {
		h[i], l[i] = h[i]+2, l[i]-2
	}
	h[10], l[10] = 105, 97 // keep the first candle's body below bar 2's
	out, _ := CDLGAPSIDESIDEWHITE(o, h, l, c)
	if out[12] != 100 {
		t.Errorf("wider background: got %d, want 100", out[12])
	}

	// Flat and monotone series never fire (no gaps).
	RestoreAllCandleDefaultSettings()
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" && s.Name != "zigzag" {
			continue
		}
		out, _ := CDLGAPSIDESIDEWHITE(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLGAPSIDESIDEWHITERangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlgapsidesidewhiteRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlgapsidesidewhiteRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlgapsidesidewhiteRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlgapsidesidewhiteRange(0, 6, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlgapsidesidewhiteRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlgapsidesidewhiteRange(0, 29, s.O[:29], s.H, s.L, s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("input one short: %v", err)
	}
	if _, _, err := cdlgapsidesidewhiteRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	if _, nb, err := cdlgapsidesidewhiteRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	if beg, nb, err := cdlgapsidesidewhiteRange(7, 7, s.O, s.H, s.L, s.C, out); err != nil || beg != 7 || nb != 1 {
		t.Errorf("single bar at lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values match the full series, including firing bars.
	o, h, l, c := cdlgapsidesidewhiteSeries(cdlgapsidesidewhiteUp)
	o2, h2, l2, c2 := cdlgapsidesidewhiteSeries(cdlgapsidesidewhiteDown)
	o, h, l, c = append(o, o2...), append(h, h2...), append(l, l2...), append(c, c2...)
	full, _ := CDLGAPSIDESIDEWHITE(o, h, l, c)
	if full[12] != 100 || full[25] != -100 {
		t.Fatalf("constructed patterns did not fire: %v", full)
	}
	buf := make([]int, len(o))
	beg, nb, err = cdlgapsidesidewhiteRange(3, 20, o, h, l, c, buf)
	if err != nil || beg != 7 || nb != 14 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", buf[:nb], full[beg:beg+nb])
	beg, nb, err = cdlgapsidesidewhiteRange(15, 25, o, h, l, c, buf)
	if err != nil || beg != 15 || nb != 11 {
		t.Fatalf("sub-range 2: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range 2", buf[:nb], full[beg:beg+nb])
}
