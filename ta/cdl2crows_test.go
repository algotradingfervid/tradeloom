package ta

import (
	"errors"
	"testing"
)

// cdl2crowsBase returns n identical background bars: open 100, close 102,
// high 103, low 99 (real body 2).
func cdl2crowsBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

type cdl2crowsBar struct{ o, h, l, c float64 }

// cdl2crowsSeries appends three bars to 10 background bars, so the third bar
// is at index 12 (the default lookback).
func cdl2crowsSeries(b1, b2, b3 cdl2crowsBar) (o, h, l, c []float64) {
	o, h, l, c = cdl2crowsBase(10)
	for _, b := range []cdl2crowsBar{b1, b2, b3} {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

// The canonical Two Crows used by several tests.
var (
	cdl2crowsB1 = cdl2crowsBar{100, 106.5, 99.5, 106}  // long white, body 6
	cdl2crowsB2 = cdl2crowsBar{109, 109.5, 106.8, 107} // black, body gaps above 106
	cdl2crowsB3 = cdl2crowsBar{108, 108.2, 102.5, 103} // black, opens in 2nd body, closes in 1st
)

func TestCDL2CROWSParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDL2CROWS(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL2CROWS(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL2CROWS(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL2CROWS(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDL2CROWS(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDL2CROWS([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDL2CROWSLookback(); lb != 12 {
		t.Errorf("default lookback = %d, want 12", lb)
	}
}

func TestCDL2CROWSLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, tc := range []struct{ period, want int }{{0, 2}, {1, 3}, {5, 7}, {10, 12}, {50, 52}} {
		RestoreAllCandleDefaultSettings()
		if err := SetCandleSettings(BodyLong, RangeRealBody, tc.period, 1.0); err != nil {
			t.Fatal(err)
		}
		if lb := CDL2CROWSLookback(); lb != tc.want {
			t.Errorf("BodyLong period %d: lookback %d, want %d", tc.period, lb, tc.want)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyShort, ShadowLong, Near, Far, Equal} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 40, d.Factor)
	}
	if lb := CDL2CROWSLookback(); lb != 12 {
		t.Errorf("other settings changed: lookback %d, want 12", lb)
	}
	// Period 0 with factor 1: "long" compares the body with itself, so the
	// pattern can never fire, and n=2 is exactly the lookback.
	RestoreAllCandleDefaultSettings()
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 1.0)
	o, h, l, c := cdl2crowsSeries(cdl2crowsB1, cdl2crowsB2, cdl2crowsB3)
	out, err := CDL2CROWS(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "period 0", out, make([]int, len(o)))
	out, err = CDL2CROWS(o[:2], h[:2], l[:2], c[:2])
	if err != nil || len(out) != 2 || out[0] != 0 || out[1] != 0 {
		t.Errorf("n=2 with lookback 2: out=%v err=%v", out, err)
	}
	// Period 0, factor 0.5: the first body only has to be positive.
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 0.5)
	out, _ = CDL2CROWS(o[10:], h[10:], l[10:], c[10:])
	assertInt(t, "period 0 factor 0.5", out, []int{0, 0, -100})
}

func TestCDL2CROWSLengths(t *testing.T) {
	lb := CDL2CROWSLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDL2CROWS(s.O, s.H, s.L, s.C)
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

func TestCDL2CROWSKnownValues(t *testing.T) {
	// Background body 2, so BodyLong avg over the 10 bars before the first
	// candle is 2 (factor 1).
	type B = cdl2crowsBar
	cases := []struct {
		name       string
		b1, b2, b3 B
		want       int
	}{
		{"two crows", cdl2crowsB1, cdl2crowsB2, cdl2crowsB3, -100},
		{"1st body not long", B{104, 106.5, 103.5, 106}, cdl2crowsB2, B{108, 108.2, 104.5, 105}, 0}, // body 2 not > 2
		{"1st body just long", B{103.9, 106.5, 103.5, 106}, cdl2crowsB2, B{108, 108.2, 104.5, 105}, -100},
		{"1st black", B{106, 106.5, 99.5, 100}, cdl2crowsB2, cdl2crowsB3, 0},
		{"2nd white", B{100, 106.5, 99.5, 106}, B{107, 109.5, 106.8, 109}, B{108, 108.2, 102.5, 103}, 0},
		{"2nd doji (white)", cdl2crowsB1, B{107, 109.5, 106.8, 107}, B{107, 108.2, 102.5, 103}, 0},
		{"no body gap", cdl2crowsB1, B{109, 109.5, 105, 105.5}, cdl2crowsB3, 0},
		{"body gap zero", cdl2crowsB1, B{109, 109.5, 105.8, 106}, cdl2crowsB3, 0},
		{"3rd white", cdl2crowsB1, cdl2crowsB2, B{103, 108.2, 102.5, 108}, 0},
		{"3rd doji (white)", cdl2crowsB1, cdl2crowsB2, B{108, 108.2, 102.5, 108}, 0},
		{"3rd opens above 2nd open", cdl2crowsB1, cdl2crowsB2, B{109.5, 109.6, 102.5, 103}, 0},
		{"3rd opens at 2nd open", cdl2crowsB1, cdl2crowsB2, B{109, 109.2, 102.5, 103}, 0},
		{"3rd opens at 2nd close", cdl2crowsB1, cdl2crowsB2, B{107, 107.2, 102.5, 103}, 0},
		{"3rd closes at 1st open", cdl2crowsB1, cdl2crowsB2, B{108, 108.2, 99.5, 100}, 0},
		{"3rd closes below 1st open", cdl2crowsB1, cdl2crowsB2, B{108, 108.2, 98.5, 99}, 0},
		{"3rd closes at 1st close", cdl2crowsB1, cdl2crowsB2, B{108, 108.2, 105.5, 106}, 0},
		{"3rd closes just inside", cdl2crowsB1, cdl2crowsB2, B{108, 108.2, 99.5, 100.01}, -100},
	}
	for _, tc := range cases {
		o, h, l, c := cdl2crowsSeries(tc.b1, tc.b2, tc.b3)
		out, err := CDL2CROWS(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 13)
		want[12] = tc.want
		assertInt(t, "CDL2CROWS "+tc.name, out, want)
	}

	// The average excludes the first candle itself: raising the background
	// bodies to 6 makes a body-6 first candle no longer long.
	o, h, l, c := cdl2crowsSeries(cdl2crowsB1, cdl2crowsB2, cdl2crowsB3)
	for i := range 10 {
		o[i], c[i], h[i], l[i] = 100, 106, 106.5, 99.5
	}
	out, _ := CDL2CROWS(o, h, l, c)
	assertInt(t, "average equals body", out, make([]int, 13))

	// Constant and flat series have no white-long/black structure.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" && s.Name != "up" && s.Name != "down" {
			continue
		}
		out, _ := CDL2CROWS(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDL2CROWSRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdl2crowsRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdl2crowsRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdl2crowsRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdl2crowsRange(0, 11, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdl2crowsRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdl2crowsRange(0, 29, s.O[:29], s.H, s.L, s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("input one short: %v", err)
	}
	if _, _, err := cdl2crowsRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdl2crowsRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// First output bar: start at the lookback.
	if beg, nb, err := cdl2crowsRange(12, 12, s.O, s.H, s.L, s.C, out); err != nil || beg != 12 || nb != 1 {
		t.Errorf("single bar at lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values match the full series, including a firing bar.
	o, h, l, c := cdl2crowsSeries(cdl2crowsB1, cdl2crowsB2, cdl2crowsB3)
	o2, h2, l2, c2 := cdl2crowsBase(10)
	o, h, l, c = append(o, o2...), append(h, h2...), append(l, l2...), append(c, c2...)
	full, _ := CDL2CROWS(o, h, l, c)
	if full[12] != -100 {
		t.Fatalf("constructed pattern did not fire: %v", full)
	}
	buf := make([]int, len(o))
	beg, nb, err = cdl2crowsRange(5, 20, o, h, l, c, buf)
	if err != nil || beg != 12 || nb != 9 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", buf[:nb], full[beg:beg+nb])
	beg, nb, err = cdl2crowsRange(14, 22, o, h, l, c, buf)
	if err != nil || beg != 14 || nb != 9 {
		t.Fatalf("sub-range 2: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range 2", buf[:nb], full[beg:beg+nb])
}
