package ta

import (
	"errors"
	"testing"
)

// cdlunique3riverBase returns n identical background bars: open 100, close
// 102, high 103, low 99 (white, real body 2).
func cdlunique3riverBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

type cdlunique3riverBar struct{ o, h, l, c float64 }

// cdlunique3riverSeries appends three bars to 10 background bars, so the
// third bar is at index 12 (the default lookback).
func cdlunique3riverSeries(b1, b2, b3 cdlunique3riverBar) (o, h, l, c []float64) {
	o, h, l, c = cdlunique3riverBase(10)
	for _, b := range []cdlunique3riverBar{b1, b2, b3} {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

// The canonical Unique 3 River. With default settings the BodyLong average
// for the first candle (bar 10) is over bars 0..9 (= 2), and the BodyShort
// average for the third candle (bar 12) is over bars 2..11, which include the
// first two candles: (8*2 + 10 + 4) / 10 = 3 exactly.
var (
	cdlunique3riverB1 = cdlunique3riverBar{110, 111, 99, 100}   // long black, body 10
	cdlunique3riverB2 = cdlunique3riverBar{106, 106.5, 97, 102} // black harami, lower low, body 4
	cdlunique3riverB3 = cdlunique3riverBar{98, 99.5, 97.5, 99}  // short white, body 1, opens above 97
)

func TestCDLUNIQUE3RIVERParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLUNIQUE3RIVER(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLUNIQUE3RIVER(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLUNIQUE3RIVER(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLUNIQUE3RIVER(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLUNIQUE3RIVER(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLUNIQUE3RIVER([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLUNIQUE3RIVERLookback(); lb != 12 {
		t.Errorf("default lookback = %d, want 12", lb)
	}
}

func TestCDLUNIQUE3RIVERLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, tc := range []struct{ long, short, want int }{
		{0, 0, 2}, {1, 0, 3}, {0, 1, 3}, {5, 10, 12}, {10, 5, 12}, {50, 3, 52}, {3, 50, 52}, {10, 10, 12},
	} {
		RestoreAllCandleDefaultSettings()
		if err := SetCandleSettings(BodyLong, RangeRealBody, tc.long, 1.0); err != nil {
			t.Fatal(err)
		}
		if err := SetCandleSettings(BodyShort, RangeRealBody, tc.short, 1.0); err != nil {
			t.Fatal(err)
		}
		if lb := CDLUNIQUE3RIVERLookback(); lb != tc.want {
			t.Errorf("BodyLong %d BodyShort %d: lookback %d, want %d", tc.long, tc.short, lb, tc.want)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyDoji, ShadowLong, Near, Far, Equal} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 40, d.Factor)
	}
	if lb := CDLUNIQUE3RIVERLookback(); lb != 12 {
		t.Errorf("other settings changed: lookback %d, want 12", lb)
	}
	// Both periods 0 with factor 1: "long" compares the first body with
	// itself, so the pattern never fires; n=2 is exactly the lookback.
	RestoreAllCandleDefaultSettings()
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 1.0)
	_ = SetCandleSettings(BodyShort, RangeRealBody, 0, 1.0)
	o, h, l, c := cdlunique3riverSeries(cdlunique3riverB1, cdlunique3riverB2, cdlunique3riverB3)
	out, err := CDLUNIQUE3RIVER(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "period 0", out, make([]int, len(o)))
	out, err = CDLUNIQUE3RIVER(o[:2], h[:2], l[:2], c[:2])
	if err != nil || len(out) != 2 || out[0] != 0 || out[1] != 0 {
		t.Errorf("n=2 with lookback 2: out=%v err=%v", out, err)
	}
	// Period 0, BodyLong factor 0.5 and BodyShort factor 2: the first body
	// only has to be positive and the third body (1) < 2*itself.
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 0.5)
	_ = SetCandleSettings(BodyShort, RangeRealBody, 0, 2.0)
	out, _ = CDLUNIQUE3RIVER(o[10:], h[10:], l[10:], c[10:])
	assertInt(t, "period 0 factors", out, []int{0, 0, 100})
	// A white doji third candle (body 0) is not < 2*0.
	o[12], c[12] = 98, 98
	out, _ = CDLUNIQUE3RIVER(o[10:], h[10:], l[10:], c[10:])
	assertInt(t, "period 0 doji", out, []int{0, 0, 0})
}

func TestCDLUNIQUE3RIVERLengths(t *testing.T) {
	lb := CDLUNIQUE3RIVERLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLUNIQUE3RIVER(s.O, s.H, s.L, s.C)
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

// cdlunique3riverCases are the canonical pattern and its near misses, each
// changing one condition at its boundary. Averages: BodyLong for bar 10 is 2;
// BodyShort for bar 12 is (16 + body1 + body2) / 10.
var cdlunique3riverCases = []struct {
	name       string
	b1, b2, b3 cdlunique3riverBar
	want       int
}{
	{"unique 3 river", cdlunique3riverB1, cdlunique3riverB2, cdlunique3riverB3, 100},
	// 1st candle
	{"1st white", cdlunique3riverBar{100, 111, 99, 110}, cdlunique3riverB2, cdlunique3riverB3, 0},
	{"1st body at long average", cdlunique3riverBar{102, 103, 99, 100}, cdlunique3riverBar{101.5, 102, 97.5, 101}, cdlunique3riverB3, 0}, // body 2 not > 2
	{"1st body just long", cdlunique3riverBar{102.25, 103, 99, 100}, cdlunique3riverBar{101.5, 102, 97.5, 101}, cdlunique3riverB3, 100},  // short avg 1.875
	// 2nd candle
	{"2nd white", cdlunique3riverB1, cdlunique3riverBar{102, 106.5, 97, 106}, cdlunique3riverB3, 0},
	{"2nd doji (white)", cdlunique3riverB1, cdlunique3riverBar{104, 106.5, 97, 104}, cdlunique3riverB3, 0},
	{"2nd closes at 1st close", cdlunique3riverB1, cdlunique3riverBar{105, 106, 97, 100}, cdlunique3riverB3, 0},
	{"2nd closes just above 1st close", cdlunique3riverB1, cdlunique3riverBar{105, 106, 97, 100.5}, cdlunique3riverB3, 100},
	{"2nd opens at 1st open", cdlunique3riverB1, cdlunique3riverBar{110, 110.5, 97, 102}, cdlunique3riverB3, 100},
	{"2nd opens above 1st open", cdlunique3riverB1, cdlunique3riverBar{110.5, 111, 97, 102}, cdlunique3riverB3, 0},
	{"2nd low at 1st low", cdlunique3riverB1, cdlunique3riverBar{105, 106, 99, 102}, cdlunique3riverBar{99.25, 100.5, 99, 100.25}, 0},
	{"2nd low just below 1st low", cdlunique3riverB1, cdlunique3riverBar{105, 106, 98.9, 102}, cdlunique3riverBar{99.25, 100.5, 99, 100.25}, 100},
	// 3rd candle
	{"3rd black", cdlunique3riverB1, cdlunique3riverB2, cdlunique3riverBar{99, 99.5, 97.5, 98}, 0},
	{"3rd white doji", cdlunique3riverB1, cdlunique3riverB2, cdlunique3riverBar{98, 99.5, 97.5, 98}, 100},
	{"3rd opens at 2nd low", cdlunique3riverB1, cdlunique3riverB2, cdlunique3riverBar{97, 99.5, 96.5, 98}, 0},
	{"3rd opens just above 2nd low", cdlunique3riverB1, cdlunique3riverB2, cdlunique3riverBar{97.25, 99.5, 96.5, 98}, 100},
	{"3rd body at short average", cdlunique3riverB1, cdlunique3riverB2, cdlunique3riverBar{98, 101.5, 97.5, 101}, 0}, // body 3 not < 3
	{"3rd body just short", cdlunique3riverB1, cdlunique3riverB2, cdlunique3riverBar{98, 101.5, 97.5, 100.75}, 100},
}

func TestCDLUNIQUE3RIVERKnownValues(t *testing.T) {
	for _, tc := range cdlunique3riverCases {
		o, h, l, c := cdlunique3riverSeries(tc.b1, tc.b2, tc.b3)
		out, err := CDLUNIQUE3RIVER(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 13)
		want[12] = tc.want
		assertInt(t, "CDLUNIQUE3RIVER "+tc.name, out, want)
	}

	// The BodyLong average excludes the first candle itself: raising the
	// background bodies to 10 makes a body-10 first candle no longer long.
	o, h, l, c := cdlunique3riverSeries(cdlunique3riverB1, cdlunique3riverB2, cdlunique3riverB3)
	for i := range 10 {
		o[i], c[i], h[i], l[i] = 100, 110, 111, 99
	}
	out, _ := CDLUNIQUE3RIVER(o, h, l, c)
	assertInt(t, "average equals body", out, make([]int, 13))

	// The BodyShort average includes the first two candles: shrinking them
	// pulls the average below the third body. B1 body 2.5, B2 body 0.5:
	// (16 + 2.5 + 0.5) / 10 = 1.9, so a third body of 1.9 is not short.
	b1 := cdlunique3riverBar{102.5, 103, 99, 100}
	b2 := cdlunique3riverBar{101.5, 102, 98, 101}
	o, h, l, c = cdlunique3riverSeries(b1, b2, cdlunique3riverBar{98.5, 100.5, 98, 100.5})
	out, _ = CDLUNIQUE3RIVER(o, h, l, c)
	assertInt(t, "third body 2", out, make([]int, 13))
	o, h, l, c = cdlunique3riverSeries(b1, b2, cdlunique3riverBar{98.5, 100.5, 98, 99.5})
	out, _ = CDLUNIQUE3RIVER(o, h, l, c)
	want := make([]int, 13)
	want[12] = 100
	assertInt(t, "third body 1", out, want)

	// Constant and trending series have no black/black/white structure.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" && s.Name != "up" && s.Name != "down" {
			continue
		}
		out, _ := CDLUNIQUE3RIVER(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLUNIQUE3RIVERRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlunique3riverRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlunique3riverRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlunique3riverRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlunique3riverRange(0, 11, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlunique3riverRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlunique3riverRange(0, 29, s.O[:29], s.H, s.L, s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("input one short: %v", err)
	}
	if _, _, err := cdlunique3riverRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlunique3riverRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// First output bar: start at the lookback.
	if beg, nb, err := cdlunique3riverRange(12, 12, s.O, s.H, s.L, s.C, out); err != nil || beg != 12 || nb != 1 {
		t.Errorf("single bar at lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values match the full series, including a firing bar.
	o, h, l, c := cdlunique3riverSeries(cdlunique3riverB1, cdlunique3riverB2, cdlunique3riverB3)
	o2, h2, l2, c2 := cdlunique3riverBase(10)
	o, h, l, c = append(o, o2...), append(h, h2...), append(l, l2...), append(c, c2...)
	full, _ := CDLUNIQUE3RIVER(o, h, l, c)
	if full[12] != 100 {
		t.Fatalf("constructed pattern did not fire: %v", full)
	}
	buf := make([]int, len(o))
	beg, nb, err = cdlunique3riverRange(5, 20, o, h, l, c, buf)
	if err != nil || beg != 12 || nb != 9 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", buf[:nb], full[beg:beg+nb])
	beg, nb, err = cdlunique3riverRange(12, 22, o, h, l, c, buf)
	if err != nil || beg != 12 || nb != 11 {
		t.Fatalf("sub-range 2: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range 2", buf[:nb], full[beg:beg+nb])
	beg, nb, err = cdlunique3riverRange(14, 22, o, h, l, c, buf)
	if err != nil || beg != 14 || nb != 9 {
		t.Fatalf("sub-range 3: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range 3", buf[:nb], full[beg:beg+nb])
}
