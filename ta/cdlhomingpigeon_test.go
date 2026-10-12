package ta

import (
	"errors"
	"testing"
)

// cdlhomingpigeonBase returns n identical background bars: open 100, close
// 102, high 103, low 99 (real body 2).
func cdlhomingpigeonBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

type cdlhomingpigeonBar struct{ o, h, l, c float64 }

// cdlhomingpigeonSeries appends two bars to 10 background bars, so the second
// bar is at index 11 (the default lookback).
func cdlhomingpigeonSeries(b1, b2 cdlhomingpigeonBar) (o, h, l, c []float64) {
	o, h, l, c = cdlhomingpigeonBase(10)
	for _, b := range []cdlhomingpigeonBar{b1, b2} {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

// The canonical Homing Pigeon. With default settings the BodyLong average for
// the first candle is over bars 0..9 (= 2) and the BodyShort average for the
// second candle is over bars 1..10, which include the first candle:
// (9*2 + 7) / 10 = 2.5 exactly.
var (
	cdlhomingpigeonB1 = cdlhomingpigeonBar{107, 107.5, 99.5, 100}  // long black, body 7
	cdlhomingpigeonB2 = cdlhomingpigeonBar{105, 105.5, 102.5, 103} // short black inside, body 2
)

func TestCDLHOMINGPIGEONParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLHOMINGPIGEON(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLHOMINGPIGEON(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLHOMINGPIGEON(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLHOMINGPIGEON(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLHOMINGPIGEON(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLHOMINGPIGEON([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLHOMINGPIGEONLookback(); lb != 11 {
		t.Errorf("default lookback = %d, want 11", lb)
	}
}

func TestCDLHOMINGPIGEONLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, tc := range []struct{ long, short, want int }{
		{0, 0, 1}, {1, 0, 2}, {0, 1, 2}, {5, 10, 11}, {10, 5, 11}, {50, 3, 51}, {3, 50, 51}, {10, 10, 11},
	} {
		RestoreAllCandleDefaultSettings()
		if err := SetCandleSettings(BodyLong, RangeRealBody, tc.long, 1.0); err != nil {
			t.Fatal(err)
		}
		if err := SetCandleSettings(BodyShort, RangeRealBody, tc.short, 1.0); err != nil {
			t.Fatal(err)
		}
		if lb := CDLHOMINGPIGEONLookback(); lb != tc.want {
			t.Errorf("BodyLong %d BodyShort %d: lookback %d, want %d", tc.long, tc.short, lb, tc.want)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyDoji, ShadowLong, Near, Far, Equal} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 40, d.Factor)
	}
	if lb := CDLHOMINGPIGEONLookback(); lb != 11 {
		t.Errorf("other settings changed: lookback %d, want 11", lb)
	}
	// Both periods 0 with factor 1: "long" compares the first body with
	// itself, so the pattern never fires; n=1 is exactly the lookback.
	RestoreAllCandleDefaultSettings()
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 1.0)
	_ = SetCandleSettings(BodyShort, RangeRealBody, 0, 1.0)
	o, h, l, c := cdlhomingpigeonSeries(cdlhomingpigeonB1, cdlhomingpigeonB2)
	out, err := CDLHOMINGPIGEON(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "period 0", out, make([]int, len(o)))
	out, err = CDLHOMINGPIGEON(o[:1], h[:1], l[:1], c[:1])
	if err != nil || len(out) != 1 || out[0] != 0 {
		t.Errorf("n=1 with lookback 1: out=%v err=%v", out, err)
	}
	// Period 0, BodyLong factor 0.5: the first body only has to be positive,
	// and the second body (2) must be <= itself, which always holds.
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 0.5)
	out, _ = CDLHOMINGPIGEON(o[10:], h[10:], l[10:], c[10:])
	assertInt(t, "period 0 factor 0.5", out, []int{0, 100})
}

func TestCDLHOMINGPIGEONLengths(t *testing.T) {
	lb := CDLHOMINGPIGEONLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLHOMINGPIGEON(s.O, s.H, s.L, s.C)
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

func TestCDLHOMINGPIGEONKnownValues(t *testing.T) {
	type B = cdlhomingpigeonBar
	b1, b2 := cdlhomingpigeonB1, cdlhomingpigeonB2
	cases := []struct {
		name   string
		b1, b2 B
		want   int
	}{
		{"homing pigeon", b1, b2, 100},
		{"1st body not long", B{104, 104.5, 101.5, 102}, B{103.5, 103.8, 102.2, 102.5}, 0}, // body 2 not > 2
		{"1st body just long", B{104.25, 104.5, 101.5, 102}, B{103.5, 103.8, 102.2, 102.5}, 100},
		{"1st white", B{100, 107.5, 99.5, 107}, b2, 0},
		{"2nd white", b1, B{103, 105.5, 102.5, 105}, 0},
		{"2nd doji (white)", b1, B{104, 105.5, 102.5, 104}, 0},
		{"2nd body at short average", b1, B{104.5, 105, 101.5, 102}, 100}, // body 2.5 <= 2.5
		{"2nd body above short average", b1, B{104.75, 105, 101.5, 102}, 0},
		{"2nd opens at 1st open", b1, B{107, 107.5, 104.5, 105}, 0},
		{"2nd opens above 1st open", b1, B{107.5, 107.8, 105.3, 105.5}, 0},
		{"2nd opens just below 1st open", b1, B{106.9, 107.5, 104.5, 105}, 100},
		{"2nd closes at 1st close", b1, B{102, 102.5, 99.5, 100}, 0},
		{"2nd closes below 1st close", b1, B{101.5, 102, 99, 99.5}, 0},
		{"2nd closes just above 1st close", b1, B{102, 102.5, 99.5, 100.5}, 100},
	}
	for _, tc := range cases {
		o, h, l, c := cdlhomingpigeonSeries(tc.b1, tc.b2)
		out, err := CDLHOMINGPIGEON(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 12)
		want[11] = tc.want
		assertInt(t, "CDLHOMINGPIGEON "+tc.name, out, want)
	}

	// The BodyLong average excludes the first candle itself: raising the
	// background bodies to 7 makes a body-7 first candle no longer long.
	o, h, l, c := cdlhomingpigeonSeries(cdlhomingpigeonB1, cdlhomingpigeonB2)
	for i := range 10 {
		o[i], c[i], h[i], l[i] = 100, 107, 107.5, 99.5
	}
	out, _ := CDLHOMINGPIGEON(o, h, l, c)
	assertInt(t, "average equals body", out, make([]int, 12))

	// With BodyLong factor 0.5 a body-1.5 first candle counts as long; the
	// BodyShort average for the second candle includes the first candle:
	// (9*2 + 1.5) / 10 = 1.95.
	_ = SetCandleSettings(BodyLong, RangeRealBody, 10, 0.5)
	defer RestoreAllCandleDefaultSettings()
	o, h, l, c = cdlhomingpigeonSeries(B{103, 103.5, 99.5, 101.5}, B{103, 103.2, 101, 101.5}) // invalid: open not inside
	out, _ = CDLHOMINGPIGEON(o, h, l, c)
	assertInt(t, "open not inside", out, make([]int, 12))
	o, h, l, c = cdlhomingpigeonSeries(B{103, 103.5, 99.5, 101.5}, B{102.9, 103.2, 101, 101.6}) // body 1.3, avg (18+1.5)/10=1.95
	out, _ = CDLHOMINGPIGEON(o, h, l, c)
	want := make([]int, 12)
	want[11] = 100
	assertInt(t, "small first body", out, want)
	RestoreAllCandleDefaultSettings()

	// Constant and trending series have no long-black/short-black structure.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" && s.Name != "up" && s.Name != "down" {
			continue
		}
		out, _ := CDLHOMINGPIGEON(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLHOMINGPIGEONRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlhomingpigeonRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlhomingpigeonRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlhomingpigeonRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlhomingpigeonRange(0, 10, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlhomingpigeonRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlhomingpigeonRange(0, 29, s.O[:29], s.H, s.L, s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("input one short: %v", err)
	}
	if _, _, err := cdlhomingpigeonRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlhomingpigeonRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// First output bar: start at the lookback.
	if beg, nb, err := cdlhomingpigeonRange(11, 11, s.O, s.H, s.L, s.C, out); err != nil || beg != 11 || nb != 1 {
		t.Errorf("single bar at lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values match the full series, including a firing bar.
	o, h, l, c := cdlhomingpigeonSeries(cdlhomingpigeonB1, cdlhomingpigeonB2)
	o2, h2, l2, c2 := cdlhomingpigeonBase(10)
	o, h, l, c = append(o, o2...), append(h, h2...), append(l, l2...), append(c, c2...)
	full, _ := CDLHOMINGPIGEON(o, h, l, c)
	if full[11] != 100 {
		t.Fatalf("constructed pattern did not fire: %v", full)
	}
	buf := make([]int, len(o))
	beg, nb, err = cdlhomingpigeonRange(5, 20, o, h, l, c, buf)
	if err != nil || beg != 11 || nb != 10 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", buf[:nb], full[beg:beg+nb])
	beg, nb, err = cdlhomingpigeonRange(13, 21, o, h, l, c, buf)
	if err != nil || beg != 13 || nb != 9 {
		t.Fatalf("sub-range 2: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range 2", buf[:nb], full[beg:beg+nb])
}
