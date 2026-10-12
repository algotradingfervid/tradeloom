package ta

import (
	"errors"
	"testing"
)

// cdlidentical3crowsSeries returns 10 white background bars (open 100, close
// 102, high 103, low 99; high-low 4) followed by three identical crows at bars
// 10, 11, 12. With default settings (ShadowVeryShort: HighLow, 10, 0.1;
// Equal: HighLow, 5, 0.05) the lower-shadow thresholds are 0.4, 0.382 and
// 0.364 (every crow's lower shadow is 0.1), and the open tolerances are 0.2
// around the 1st close (bars 5..9 average 4) and 0.182 around the 2nd close
// (bars 6..10 average 3.64).
func cdlidentical3crowsSeries() (o, h, l, c []float64) {
	for range 10 {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	o = append(o, 104, 102, 100)
	h = append(h, 104.1, 102.1, 100.1)
	l = append(l, 101.9, 99.9, 97.9)
	c = append(c, 102, 100, 98)
	return
}

func TestCDLIDENTICAL3CROWSParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLIDENTICAL3CROWS(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLIDENTICAL3CROWS(s.O[:10], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLIDENTICAL3CROWS(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLIDENTICAL3CROWS(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLIDENTICAL3CROWS(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLIDENTICAL3CROWS([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLIDENTICAL3CROWSLookback(); lb != 12 {
		t.Errorf("default lookback = %d, want 12", lb)
	}
}

func TestCDLIDENTICAL3CROWSLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, tc := range []struct{ svs, eq, want int }{
		{0, 0, 2}, {1, 0, 3}, {0, 1, 3}, {10, 5, 12}, {5, 10, 12}, {3, 30, 32}, {30, 3, 32},
	} {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(ShadowVeryShort)
		if err := SetCandleSettings(ShadowVeryShort, d.RangeType, tc.svs, d.Factor); err != nil {
			t.Fatal(err)
		}
		d = GetCandleSetting(Equal)
		if err := SetCandleSettings(Equal, d.RangeType, tc.eq, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLIDENTICAL3CROWSLookback(); lb != tc.want {
			t.Errorf("svs %d eq %d: lookback %d, want %d", tc.svs, tc.eq, lb, tc.want)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	d := GetCandleSetting(BodyLong)
	_ = SetCandleSettings(BodyLong, d.RangeType, 40, d.Factor)
	if lb := CDLIDENTICAL3CROWSLookback(); lb != 12 {
		t.Errorf("BodyLong changed: lookback %d, want 12", lb)
	}
	// Both periods 0: lookback 2; with 2 bars there is no output, with 3 one.
	RestoreAllCandleDefaultSettings()
	d = GetCandleSetting(ShadowVeryShort)
	_ = SetCandleSettings(ShadowVeryShort, d.RangeType, 0, d.Factor)
	d = GetCandleSetting(Equal)
	_ = SetCandleSettings(Equal, d.RangeType, 0, d.Factor)
	o, h, l, c := cdlidentical3crowsSeries()
	out, err := CDLIDENTICAL3CROWS(o[10:12], h[10:12], l[10:12], c[10:12])
	if err != nil || len(out) != 2 || out[0] != 0 || out[1] != 0 {
		t.Errorf("n=2 with lookback 2: out=%v err=%v", out, err)
	}
	// Period 0 compares each lower shadow (0.1) with 0.1*its own range (0.22)
	// and each open with 0.05*the prior candle's own range (0.11).
	out, err = CDLIDENTICAL3CROWS(o[10:], h[10:], l[10:], c[10:])
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "period 0", out, []int{0, 0, -100})
	// With period 0, a 2nd open 0.12 above the 1st close is outside 0.11.
	o2 := append([]float64(nil), o[10:]...)
	o2[1] = 102.12
	out, _ = CDLIDENTICAL3CROWS(o2, h[10:], l[10:], c[10:])
	assertInt(t, "period 0 open far", out, []int{0, 0, 0})
}

func TestCDLIDENTICAL3CROWSLengths(t *testing.T) {
	lb := CDLIDENTICAL3CROWSLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLIDENTICAL3CROWS(s.O, s.H, s.L, s.C)
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

func TestCDLIDENTICAL3CROWSKnownValues(t *testing.T) {
	type mod func(o, h, l, c []float64)
	cases := []struct {
		name string
		m    mod
		want int
	}{
		{"crows", func(o, h, l, c []float64) {}, -100},
		{"1st crow white", func(o, h, l, c []float64) { o[10], c[10] = 102, 104 }, 0},
		{"1st crow doji is white", func(o, h, l, c []float64) { c[10] = 104 }, 0},
		{"2nd crow doji is white", func(o, h, l, c []float64) { c[11] = 102 }, 0},
		{"3rd crow doji is white", func(o, h, l, c []float64) { c[12] = 100 }, 0},
		{"2nd opens 0.15 above 1st close", func(o, h, l, c []float64) { o[11] = 102.15 }, -100},
		{"2nd opens 0.15 below 1st close", func(o, h, l, c []float64) { o[11] = 101.85 }, -100},
		{"2nd opens 0.25 above 1st close", func(o, h, l, c []float64) { o[11] = 102.25 }, 0},
		{"2nd opens 0.25 below 1st close", func(o, h, l, c []float64) { o[11] = 101.75 }, 0},
		{"3rd opens 0.17 above 2nd close", func(o, h, l, c []float64) { o[12] = 100.17 }, -100},
		{"3rd opens 0.17 below 2nd close", func(o, h, l, c []float64) { o[12] = 99.83 }, -100},
		{"3rd opens 0.19 above 2nd close", func(o, h, l, c []float64) { o[12] = 100.19 }, 0},
		{"3rd opens 0.19 below 2nd close", func(o, h, l, c []float64) { o[12] = 99.81 }, 0},
		{"2nd close not below 1st", func(o, h, l, c []float64) {
			o[11], c[11], l[11], h[11] = 102.15, 102.05, 101.95, 102.2
			o[12] = 102.05
		}, 0},
		{"3rd close not below 2nd", func(o, h, l, c []float64) { o[12], c[12], l[12] = 100.15, 100.05, 99.95 }, 0},
		{"3rd close equals 2nd", func(o, h, l, c []float64) { o[12], c[12], l[12] = 100.15, 100, 99.9 }, 0},
		{"1st lower shadow long", func(o, h, l, c []float64) { l[10] = 101.5 }, 0},
		{"2nd lower shadow long", func(o, h, l, c []float64) { l[11] = 99.5 }, 0},
		{"3rd lower shadow long", func(o, h, l, c []float64) { l[12] = 97.5 }, 0},
		{"zero lower shadows", func(o, h, l, c []float64) { l[10], l[11], l[12] = 102, 100, 98 }, -100},
	}
	for _, tc := range cases {
		o, h, l, c := cdlidentical3crowsSeries()
		tc.m(o, h, l, c)
		out, err := CDLIDENTICAL3CROWS(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 13)
		want[12] = tc.want
		assertInt(t, "CDLIDENTICAL3CROWS "+tc.name, out, want)
	}

	// Constant or zero series have no black candles: never fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLIDENTICAL3CROWS(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLIDENTICAL3CROWSRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlidentical3crowsRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlidentical3crowsRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlidentical3crowsRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlidentical3crowsRange(0, 11, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlidentical3crowsRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlidentical3crowsRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	if _, _, err := cdlidentical3crowsRange(0, 29, s.O, s.H, s.L, s.C, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlidentical3crowsRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLIDENTICAL3CROWS(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlidentical3crowsRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 12 || nb != 14 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
	// Sub-range on the constructed pattern: the single last bar.
	o, h, l, c := cdlidentical3crowsSeries()
	beg, nb, err = cdlidentical3crowsRange(12, 12, o, h, l, c, out)
	if err != nil || beg != 12 || nb != 1 || out[0] != -100 {
		t.Errorf("single bar: beg=%d nb=%d out0=%d err=%v", beg, nb, out[0], err)
	}
}
