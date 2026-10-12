package ta

import (
	"errors"
	"testing"
)

// cdlupsidegap2crowsBase returns n identical background bars: open 100,
// close 102, high 103, low 99 (real body 2).
func cdlupsidegap2crowsBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

type cdlupsidegap2crowsBar struct{ o, h, l, c float64 }

// cdlupsidegap2crowsSeries appends three bars to 10 background bars, so the
// third bar is at index 12 (the default lookback).
func cdlupsidegap2crowsSeries(b1, b2, b3 cdlupsidegap2crowsBar) (o, h, l, c []float64) {
	o, h, l, c = cdlupsidegap2crowsBase(10)
	for _, b := range []cdlupsidegap2crowsBar{b1, b2, b3} {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

// The canonical Upside Gap Two Crows used by several tests. With the default
// settings the BodyLong average for the first candle is 2 (ten background
// bodies) and the BodyShort average for the second candle is
// (9*2 + 6) / 10 = 2.4 (it includes the first candle).
var (
	cdlupsidegap2crowsB1 = cdlupsidegap2crowsBar{100, 106.5, 99.5, 106}  // long white, body 6
	cdlupsidegap2crowsB2 = cdlupsidegap2crowsBar{108, 108.5, 106.8, 107} // small black, body gaps above 106
	cdlupsidegap2crowsB3 = cdlupsidegap2crowsBar{109, 109.5, 106, 106.5} // black, engulfs 2nd, closes above 1st close
)

func TestCDLUPSIDEGAP2CROWSParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLUPSIDEGAP2CROWS(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLUPSIDEGAP2CROWS(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLUPSIDEGAP2CROWS(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLUPSIDEGAP2CROWS(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLUPSIDEGAP2CROWS(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLUPSIDEGAP2CROWS([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLUPSIDEGAP2CROWSLookback(); lb != 12 {
		t.Errorf("default lookback = %d, want 12", lb)
	}
}

func TestCDLUPSIDEGAP2CROWSLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, tc := range []struct{ long, short, want int }{
		{0, 0, 2}, {1, 0, 3}, {0, 1, 3}, {5, 10, 12}, {10, 5, 12}, {50, 3, 52}, {3, 50, 52},
	} {
		RestoreAllCandleDefaultSettings()
		if err := SetCandleSettings(BodyLong, RangeRealBody, tc.long, 1.0); err != nil {
			t.Fatal(err)
		}
		if err := SetCandleSettings(BodyShort, RangeRealBody, tc.short, 1.0); err != nil {
			t.Fatal(err)
		}
		if lb := CDLUPSIDEGAP2CROWSLookback(); lb != tc.want {
			t.Errorf("BodyLong %d BodyShort %d: lookback %d, want %d", tc.long, tc.short, lb, tc.want)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyDoji, ShadowLong, Near, Far, Equal} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 40, d.Factor)
	}
	if lb := CDLUPSIDEGAP2CROWSLookback(); lb != 12 {
		t.Errorf("other settings changed: lookback %d, want 12", lb)
	}
	// Both periods 0: lookback 2, the averages become the candle's own
	// range. With BodyLong factor 1 "long" compares the body with itself and
	// can never fire.
	RestoreAllCandleDefaultSettings()
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 1.0)
	_ = SetCandleSettings(BodyShort, RangeRealBody, 0, 1.0)
	o, h, l, c := cdlupsidegap2crowsSeries(cdlupsidegap2crowsB1, cdlupsidegap2crowsB2, cdlupsidegap2crowsB3)
	out, err := CDLUPSIDEGAP2CROWS(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "periods 0", out, make([]int, len(o)))
	out, err = CDLUPSIDEGAP2CROWS(o[:2], h[:2], l[:2], c[:2])
	if err != nil || len(out) != 2 || out[0] != 0 || out[1] != 0 {
		t.Errorf("n=2 with lookback 2: out=%v err=%v", out, err)
	}
	// BodyLong factor 0.5 with period 0: the first body only has to be
	// positive; BodyShort period 0 factor 1: body <= itself always holds.
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 0.5)
	out, _ = CDLUPSIDEGAP2CROWS(o[10:], h[10:], l[10:], c[10:])
	assertInt(t, "periods 0, long factor 0.5", out, []int{0, 0, -100})
}

func TestCDLUPSIDEGAP2CROWSLengths(t *testing.T) {
	lb := CDLUPSIDEGAP2CROWSLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLUPSIDEGAP2CROWS(s.O, s.H, s.L, s.C)
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
				if v != 0 && v != -100 {
					t.Fatalf("%s n=%d: [%d]=%d, want 0 or -100", s.Name, n, i, v)
				}
			}
		}
	}
}

func TestCDLUPSIDEGAP2CROWSKnownValues(t *testing.T) {
	type B = cdlupsidegap2crowsBar
	b1, b2, b3 := cdlupsidegap2crowsB1, cdlupsidegap2crowsB2, cdlupsidegap2crowsB3
	cases := []struct {
		name       string
		b1, b2, b3 B
		want       int
	}{
		{"upside gap two crows", b1, b2, b3, -100},
		{"1st body not long", B{104, 106.5, 103.5, 106}, b2, b3, 0}, // body 2 not > 2
		{"1st body just long", B{103.9, 106.5, 103.5, 106}, b2, b3, -100},
		{"1st black", B{106, 106.5, 99.5, 100}, b2, B{109, 109.5, 100, 100.5}, 0},
		{"1st doji (white, body 0)", B{106, 106.5, 105.5, 106}, b2, b3, 0},
		{"2nd white", b1, B{107, 108.5, 106.8, 108}, b3, 0},
		{"2nd doji (white)", b1, B{107.5, 108.5, 106.8, 107.5}, b3, 0},
		{"2nd body not short", b1, B{110, 110.5, 106.8, 107}, B{111, 111.5, 106, 106.5}, 0}, // body 3 > 2.4
		{"2nd body just short", b1, B{109.25, 109.5, 106.8, 107}, B{110, 110.5, 106, 106.5}, -100},
		{"no body gap", b1, B{107, 107.5, 105, 105.5}, B{108, 108.5, 105.5, 106.1}, 0},
		{"body gap zero", b1, B{107, 107.5, 105.8, 106}, B{108, 108.5, 105.5, 106.1}, 0},
		{"3rd white", b1, b2, B{106.5, 109.5, 106, 109}, 0},
		{"3rd opens at 2nd open", b1, b2, B{108, 108.5, 106, 106.5}, 0},
		{"3rd opens below 2nd open", b1, b2, B{107.9, 108.5, 106, 106.5}, 0},
		{"3rd closes at 2nd close", b1, b2, B{109, 109.5, 106.9, 107}, 0},
		{"3rd closes at 1st close", b1, b2, B{109, 109.5, 105.5, 106}, 0},
		{"3rd closes below 1st close", b1, b2, B{109, 109.5, 105, 105.5}, 0},
		{"3rd closes just above 1st close", b1, b2, B{109, 109.5, 105.5, 106.01}, -100},
	}
	for _, tc := range cases {
		o, h, l, c := cdlupsidegap2crowsSeries(tc.b1, tc.b2, tc.b3)
		out, err := CDLUPSIDEGAP2CROWS(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 13)
		want[12] = tc.want
		assertInt(t, "CDLUPSIDEGAP2CROWS "+tc.name, out, want)
	}

	// The BodyLong average excludes the first candle itself: raising the
	// background bodies to 6 makes a body-6 first candle no longer long.
	o, h, l, c := cdlupsidegap2crowsSeries(cdlupsidegap2crowsB1, cdlupsidegap2crowsB2, cdlupsidegap2crowsB3)
	for i := range 10 {
		o[i], c[i], h[i], l[i] = 100, 106, 106.5, 99.5
	}
	out, _ := CDLUPSIDEGAP2CROWS(o, h, l, c)
	assertInt(t, "average equals body", out, make([]int, 13))

	// The BodyShort average for the second candle includes the first candle:
	// with tiny background bodies (0.1) the short average is
	// (9*0.1 + 6)/10 = 0.69, so a body-0.5 second candle is still short.
	o, h, l, c = cdlupsidegap2crowsSeries(cdlupsidegap2crowsB1, B{107.5, 108, 106.8, 107}, B{108, 108.5, 106, 106.5})
	for i := range 10 {
		o[i], c[i], h[i], l[i] = 100, 100.1, 100.2, 99.9
	}
	out, _ = CDLUPSIDEGAP2CROWS(o, h, l, c)
	want := make([]int, 13)
	want[12] = -100
	assertInt(t, "short average includes 1st", out, want)

	// Constant and monotone series never have the white/black/black shape.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" && s.Name != "up" && s.Name != "down" {
			continue
		}
		out, _ := CDLUPSIDEGAP2CROWS(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLUPSIDEGAP2CROWSRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlupsidegap2crowsRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlupsidegap2crowsRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlupsidegap2crowsRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlupsidegap2crowsRange(0, 11, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlupsidegap2crowsRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlupsidegap2crowsRange(0, 29, s.O[:29], s.H, s.L, s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("input one short: %v", err)
	}
	if _, _, err := cdlupsidegap2crowsRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlupsidegap2crowsRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// First output bar: start at the lookback.
	if beg, nb, err := cdlupsidegap2crowsRange(12, 12, s.O, s.H, s.L, s.C, out); err != nil || beg != 12 || nb != 1 {
		t.Errorf("single bar at lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values match the full series, including a firing bar.
	o, h, l, c := cdlupsidegap2crowsSeries(cdlupsidegap2crowsB1, cdlupsidegap2crowsB2, cdlupsidegap2crowsB3)
	o2, h2, l2, c2 := cdlupsidegap2crowsBase(10)
	o, h, l, c = append(o, o2...), append(h, h2...), append(l, l2...), append(c, c2...)
	full, _ := CDLUPSIDEGAP2CROWS(o, h, l, c)
	if full[12] != -100 {
		t.Fatalf("constructed pattern did not fire: %v", full)
	}
	buf := make([]int, len(o))
	beg, nb, err = cdlupsidegap2crowsRange(5, 20, o, h, l, c, buf)
	if err != nil || beg != 12 || nb != 9 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", buf[:nb], full[beg:beg+nb])
	beg, nb, err = cdlupsidegap2crowsRange(12, 12, o, h, l, c, buf)
	if err != nil || beg != 12 || nb != 1 || buf[0] != -100 {
		t.Fatalf("single firing bar: beg=%d nb=%d err=%v out=%d", beg, nb, err, buf[0])
	}
	beg, nb, err = cdlupsidegap2crowsRange(14, 22, o, h, l, c, buf)
	if err != nil || beg != 14 || nb != 9 {
		t.Fatalf("sub-range 2: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range 2", buf[:nb], full[beg:beg+nb])
}
