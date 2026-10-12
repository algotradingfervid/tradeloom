package ta

import (
	"errors"
	"testing"
)

// cdladvanceblockBase returns n identical white background bars: open 100,
// close 101, high 101.5, low 99.5 (real body 1, high-low 2, shadows 0.5+0.5).
func cdladvanceblockBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 101.5)
		l = append(l, 99.5)
		c = append(c, 101)
	}
	return
}

type cdladvanceblockBar struct{ o, h, l, c float64 }

// cdladvanceblockSeries appends three pattern bars to 12 background bars
// (lookback 12 with default settings), so the answer is at index 14.
func cdladvanceblockSeries(bars [3]cdladvanceblockBar) (o, h, l, c []float64) {
	o, h, l, c = cdladvanceblockBase(12)
	for _, b := range bars {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

func TestCDLADVANCEBLOCKParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLADVANCEBLOCK(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLADVANCEBLOCK(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLADVANCEBLOCK(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLADVANCEBLOCK(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLADVANCEBLOCK(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLADVANCEBLOCK([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLADVANCEBLOCKLookback(); lb != 12 {
		t.Errorf("default lookback = %d, want 12", lb)
	}
}

func TestCDLADVANCEBLOCKLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{ShadowLong, 20, 22},
		{ShadowShort, 30, 32},
		{Far, 15, 17},
		{Near, 40, 42},
		{BodyLong, 25, 27},
		{BodyLong, 0, 12},   // ShadowShort still 10
		{BodyShort, 50, 12}, // not used
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLADVANCEBLOCKLookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Every average period 0: only the two prior candles are needed.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, Far, Near, ShadowLong, ShadowShort} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLADVANCEBLOCKLookback(); lb != 2 {
		t.Errorf("all periods 0: lookback %d, want 2", lb)
	}
	for _, n := range []int{1, 2, 3} {
		o, h, l, c := cdladvanceblockBase(n)
		out, err := CDLADVANCEBLOCK(o, h, l, c)
		if err != nil || len(out) != n {
			t.Errorf("n=%d with lookback 2: out=%v err=%v", n, out, err)
		}
	}
}

func TestCDLADVANCEBLOCKLengths(t *testing.T) {
	lb := CDLADVANCEBLOCKLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLADVANCEBLOCK(s.O, s.H, s.L, s.C)
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

func TestCDLADVANCEBLOCKKnownValues(t *testing.T) {
	// Background bars (0..11): body 1, high-low 2, shadows 1. With default
	// settings, for the pattern ending at i=14:
	//   BodyLong avg (bars 0..11 bodies) = 1        -> 1st body must be > 1
	//   ShadowShort[2] (bars 2..11)  = 10/10/2 = 0.5 -> 1st upper shadow < 0.5
	//   Near[2] (bars 7..11) = 0.2*2 = 0.4
	//   Near[1] (bars 8..12) = 0.2*(8+range12)/5
	//   Far[2]  (bars 7..11) = 0.6*2 = 1.2
	//   Far[1]  (bars 8..12) = 0.6*(8+range12)/5
	//   ShadowLong: period 0, real body -> 3rd body
	// With bar 12 = 100/104.2/99.8/104 (range 4.4): Near[1]=0.496, Far[1]=1.488.
	a := cdladvanceblockBar{100, 104.2, 99.8, 104}
	cases := []struct {
		name string
		b, c cdladvanceblockBar
		want int
	}{
		// Branch 3: bodies 4>3>2 and 3rd upper shadow 1 > ShadowShort[0]
		// (bars 4..13: (8+0.4+0.3)/20 = 0.435).
		{"smaller bodies + upper shadow", cdladvanceblockBar{103, 106.2, 102.9, 106}, cdladvanceblockBar{105, 108, 104.9, 107}, -100},
		// Same but 3rd upper shadow 0.1 and 2nd 0.2 (< ShadowShort[1] 0.47): no branch.
		{"no weakness", cdladvanceblockBar{103, 106.2, 102.9, 106}, cdladvanceblockBar{105, 107.1, 104.9, 107}, 0},
		// 3rd opens above close1 + Near[1] = 106.496.
		{"3rd opens too high", cdladvanceblockBar{103, 106.2, 102.9, 106}, cdladvanceblockBar{106.6, 108, 106.5, 107}, 0},
		// 3rd close not higher than 2nd close.
		{"close not higher", cdladvanceblockBar{103, 106.2, 102.9, 106}, cdladvanceblockBar{105, 108, 104.9, 106}, 0},
		// 2nd opens below 1st open.
		{"2nd opens low", cdladvanceblockBar{99.9, 106.2, 99.8, 106}, cdladvanceblockBar{105, 108, 104.9, 107}, 0},
		// 3rd is black.
		{"3rd black", cdladvanceblockBar{103, 106.2, 102.9, 106}, cdladvanceblockBar{107, 108, 104.9, 106.5}, 0},
		// Branch 1: 2nd body 1.5 < 4-1.2; 3rd body 1.8 < 1.5+0.496.
		{"2nd far smaller", cdladvanceblockBar{103, 104.6, 102.9, 104.5}, cdladvanceblockBar{104, 105.85, 103.9, 105.8}, -100},
		// 3rd body 2.1 not < 1.996.
		{"2nd far smaller, 3rd carries on", cdladvanceblockBar{103, 104.6, 102.9, 104.5}, cdladvanceblockBar{104, 106.15, 103.9, 106.1}, 0},
		// Branch 2: 3rd body 1 < 3-1.488; shadows small.
		{"3rd far smaller", cdladvanceblockBar{103, 106.1, 102.9, 106}, cdladvanceblockBar{105.5, 106.55, 105.4, 106.5}, -100},
		// 3rd body 1.7 not < 1.512.
		{"3rd not far smaller", cdladvanceblockBar{103, 106.1, 102.9, 106}, cdladvanceblockBar{105.5, 107.25, 105.4, 107.2}, 0},
		// Branch 4: 2nd body 4 (= 1st, so branch 3 fails), 3rd body 3 with
		// upper shadow 3.5 > 3.
		{"3rd long upper shadow", cdladvanceblockBar{103, 107.2, 102.9, 107}, cdladvanceblockBar{105, 111.5, 104.9, 108}, -100},
		// Upper shadow 2.9 not > 3.
		{"3rd upper shadow too short", cdladvanceblockBar{103, 107.2, 102.9, 107}, cdladvanceblockBar{105, 110.9, 104.9, 108}, 0},
	}
	for _, tc := range cases {
		o, h, l, c := cdladvanceblockSeries([3]cdladvanceblockBar{a, tc.b, tc.c})
		out, err := CDLADVANCEBLOCK(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 15)
		want[14] = tc.want
		assertInt(t, "CDLADVANCEBLOCK "+tc.name, out, want)
	}

	// 1st candle disqualified: upper shadow 0.6 not < 0.5, or body 0.95 not > 1.
	for _, first := range []cdladvanceblockBar{{100, 104.6, 99.8, 104}, {103.05, 104.2, 102.9, 104}} {
		o, h, l, c := cdladvanceblockSeries([3]cdladvanceblockBar{first, {103.5, 106.2, 103.4, 106}, {105, 108, 104.9, 107}})
		out, _ := CDLADVANCEBLOCK(o, h, l, c)
		if out[14] != 0 {
			t.Errorf("1st candle %v: got %d, want 0", first, out[14])
		}
	}

	// Constant or all-zero series: closes are not rising, never a pattern.
	// A strictly rising series of identical candles has no weakening: never.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" && s.Name != "down" {
			continue
		}
		out, _ := CDLADVANCEBLOCK(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLADVANCEBLOCKRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdladvanceblockRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdladvanceblockRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdladvanceblockRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdladvanceblockRange(0, 11, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdladvanceblockRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdladvanceblockRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdladvanceblockRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Single bar at the lookback.
	if beg, nb, err := cdladvanceblockRange(12, 12, s.O, s.H, s.L, s.C, out); err != nil || beg != 12 || nb != 1 {
		t.Errorf("single bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLADVANCEBLOCK(s.O, s.H, s.L, s.C)
	beg, nb, err = cdladvanceblockRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 12 || nb != 14 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
