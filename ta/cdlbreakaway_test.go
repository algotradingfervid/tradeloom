package ta

import (
	"errors"
	"testing"
)

// cdlbreakawayBase returns n identical background bars: open 100, close 102,
// high 103, low 99 (real body 2).
func cdlbreakawayBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

type cdlbreakawayBar struct{ o, h, l, c float64 }

// cdlbreakawaySeries appends five bars to 10 background bars, so the fifth
// bar is at index 14 (the default lookback).
func cdlbreakawaySeries(bars [5]cdlbreakawayBar) (o, h, l, c []float64) {
	o, h, l, c = cdlbreakawayBase(10)
	for _, b := range bars {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

// Canonical bullish and bearish Breakaways used by several tests.
var (
	cdlbreakawayBull = [5]cdlbreakawayBar{
		{110, 110.5, 99.5, 100}, // long black, body 10
		{98, 98.5, 95.5, 96},    // black, body gaps below 100
		{96, 97, 94, 95},        // lower high and low
		{95, 96, 92.5, 93},      // black, lower high and low
		{93, 99.5, 92.8, 99},    // white, closes in the gap (98, 100)
	}
	cdlbreakawayBear = [5]cdlbreakawayBar{
		{100, 110.5, 99.5, 110},  // long white, body 10
		{112, 114.5, 111.5, 114}, // white, body gaps above 110
		{114, 116, 113, 115},     // higher high and low
		{115, 117.5, 114, 117},   // white, higher high and low
		{117, 117.2, 110.8, 111}, // black, closes in the gap (110, 112)
	}
)

func cdlbreakawayWith(base [5]cdlbreakawayBar, k int, b cdlbreakawayBar) [5]cdlbreakawayBar {
	base[k] = b
	return base
}

func TestCDLBREAKAWAYParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLBREAKAWAY(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLBREAKAWAY(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLBREAKAWAY(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLBREAKAWAY(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLBREAKAWAY(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLBREAKAWAY([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLBREAKAWAYLookback(); lb != 14 {
		t.Errorf("default lookback = %d, want 14", lb)
	}
}

func TestCDLBREAKAWAYLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, tc := range []struct{ period, want int }{{0, 4}, {1, 5}, {5, 9}, {10, 14}, {50, 54}} {
		RestoreAllCandleDefaultSettings()
		if err := SetCandleSettings(BodyLong, RangeRealBody, tc.period, 1.0); err != nil {
			t.Fatal(err)
		}
		if lb := CDLBREAKAWAYLookback(); lb != tc.want {
			t.Errorf("BodyLong period %d: lookback %d, want %d", tc.period, lb, tc.want)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyShort, ShadowLong, Near, Far, Equal} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 40, d.Factor)
	}
	if lb := CDLBREAKAWAYLookback(); lb != 14 {
		t.Errorf("other settings changed: lookback %d, want 14", lb)
	}
	// Period 0 with factor 1: "long" compares the body with itself, so the
	// pattern can never fire; n=4 is exactly the lookback.
	RestoreAllCandleDefaultSettings()
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 1.0)
	o, h, l, c := cdlbreakawaySeries(cdlbreakawayBull)
	out, err := CDLBREAKAWAY(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "period 0", out, make([]int, len(o)))
	out, err = CDLBREAKAWAY(o[:4], h[:4], l[:4], c[:4])
	if err != nil || len(out) != 4 {
		t.Errorf("n=4 with lookback 4: out=%v err=%v", out, err)
	}
	assertInt(t, "n=4", out, make([]int, 4))
	// Period 0, factor 0.5: the first body only has to be positive.
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 0.5)
	out, _ = CDLBREAKAWAY(o[10:], h[10:], l[10:], c[10:])
	assertInt(t, "period 0 factor 0.5", out, []int{0, 0, 0, 0, 100})
}

func TestCDLBREAKAWAYLengths(t *testing.T) {
	lb := CDLBREAKAWAYLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLBREAKAWAY(s.O, s.H, s.L, s.C)
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

func TestCDLBREAKAWAYKnownValues(t *testing.T) {
	// Background body 2, so the BodyLong average over the 10 bars before the
	// first candle is 2 (factor 1).
	type B = cdlbreakawayBar
	bu, be := cdlbreakawayBull, cdlbreakawayBear
	w := cdlbreakawayWith
	cases := []struct {
		name string
		bars [5]B
		want int
	}{
		{"bullish", bu, 100},
		{"bearish", be, -100},
		// Bullish near misses.
		{"bull 1st not long", w(bu, 0, B{102, 102.5, 99.5, 100}), 0},      // body 2 not > 2
		{"bull 1st just long", w(bu, 0, B{102.1, 102.5, 99.5, 100}), 100}, // body 2.1
		{"bull 2nd white", w(bu, 1, B{96, 98.5, 95.5, 98}), 0},
		{"bull 2nd body touches 1st", w(bu, 1, B{100, 100.5, 95.5, 96}), 0},
		{"bull 3rd white still ok", w(bu, 2, B{95, 97, 94, 96}), 100},
		{"bull 3rd high equal", w(bu, 2, B{96, 98.5, 94, 95}), 0},
		{"bull 3rd low equal", w(bu, 2, B{96, 97, 95.5, 95}), 0},
		{"bull 4th high equal", w(bu, 3, B{95, 97, 92.5, 93}), 0},
		{"bull 4th low equal", w(bu, 3, B{95, 96, 94, 94.5}), 0},
		{"bull 4th white", w(bu, 3, B{93, 96, 92.5, 95}), 0},
		{"bull 5th black", w(bu, 4, B{99.5, 99.8, 92.8, 99}), 0},
		{"bull 5th closes at 2nd open", w(bu, 4, B{93, 98.5, 92.8, 98}), 0},
		{"bull 5th closes at 1st close", w(bu, 4, B{93, 100.5, 92.8, 100}), 0},
		{"bull 5th closes above 1st close", w(bu, 4, B{93, 101.5, 92.8, 101}), 0},
		{"bull 5th closes just inside", w(bu, 4, B{93, 98.5, 92.8, 98.01}), 100},
		// Bearish near misses.
		{"bear 1st not long", w(be, 0, B{108, 110.5, 107.5, 110}), 0},
		{"bear 2nd black", w(be, 1, B{114, 114.5, 111.5, 112}), 0},
		{"bear 2nd body touches 1st", w(be, 1, B{110, 114.5, 109.5, 114}), 0},
		{"bear 3rd black still ok", w(be, 2, B{115, 116, 113, 114}), -100},
		{"bear 3rd high equal", w(be, 2, B{114, 114.5, 113, 115}), 0},
		{"bear 3rd low equal", w(be, 2, B{114, 116, 111.5, 115}), 0},
		{"bear 4th high equal", w(be, 3, B{115, 116, 114, 116}), 0},
		{"bear 4th low equal", w(be, 3, B{115, 117.5, 113, 117}), 0},
		{"bear 4th black", w(be, 3, B{117, 117.5, 114, 115}), 0},
		{"bear 5th white (doji)", w(be, 4, B{111, 117.2, 110.8, 111}), 0},
		{"bear 5th closes at 2nd open", w(be, 4, B{117, 117.2, 111.8, 112}), 0},
		{"bear 5th closes at 1st close", w(be, 4, B{117, 117.2, 109.8, 110}), 0},
		{"bear 5th closes just inside", w(be, 4, B{117, 117.2, 111.8, 111.99}), -100},
	}
	for _, tc := range cases {
		o, h, l, c := cdlbreakawaySeries(tc.bars)
		out, err := CDLBREAKAWAY(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 15)
		want[14] = tc.want
		assertInt(t, "CDLBREAKAWAY "+tc.name, out, want)
	}

	// The average excludes the first candle itself: raising the background
	// bodies to 10 makes a body-10 first candle no longer long.
	o, h, l, c := cdlbreakawaySeries(cdlbreakawayBull)
	for i := range 10 {
		o[i], c[i], h[i], l[i] = 100, 110, 110.5, 99.5
	}
	out, _ := CDLBREAKAWAY(o, h, l, c)
	assertInt(t, "average equals body", out, make([]int, 15))

	// Flat and monotone series have no breakaway structure.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" && s.Name != "up" && s.Name != "down" {
			continue
		}
		out, _ := CDLBREAKAWAY(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLBREAKAWAYRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlbreakawayRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlbreakawayRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlbreakawayRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlbreakawayRange(0, 13, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlbreakawayRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlbreakawayRange(0, 29, s.O[:29], s.H, s.L, s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("input one short: %v", err)
	}
	if _, _, err := cdlbreakawayRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlbreakawayRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// First output bar: start at the lookback.
	if beg, nb, err := cdlbreakawayRange(14, 14, s.O, s.H, s.L, s.C, out); err != nil || beg != 14 || nb != 1 {
		t.Errorf("single bar at lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values match the full series, including firing bars.
	o, h, l, c := cdlbreakawaySeries(cdlbreakawayBull)
	o2, h2, l2, c2 := cdlbreakawaySeries(cdlbreakawayBear)
	o, h, l, c = append(o, o2...), append(h, h2...), append(l, l2...), append(c, c2...)
	full, _ := CDLBREAKAWAY(o, h, l, c)
	if full[14] != 100 || full[29] != -100 {
		t.Fatalf("constructed patterns did not fire: %v", full)
	}
	buf := make([]int, len(o))
	beg, nb, err = cdlbreakawayRange(5, 25, o, h, l, c, buf)
	if err != nil || beg != 14 || nb != 12 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", buf[:nb], full[beg:beg+nb])
	beg, nb, err = cdlbreakawayRange(20, 29, o, h, l, c, buf)
	if err != nil || beg != 20 || nb != 10 {
		t.Fatalf("sub-range 2: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range 2", buf[:nb], full[beg:beg+nb])
}
