package ta

import (
	"errors"
	"math"
	"testing"
)

// cdldarkcloudcoverSeries returns 10 white background bars (open 100, close
// 102, high 103, low 99: real body 2) followed by a long white candle (open
// 100, close 104, high 105: body 4 > the default BodyLong average 2) and a
// black candle opening at 106 (> 105) and closing at 101.5, inside the white
// body and below the 0.5 penetration level 104 - 4*0.5 = 102.
func cdldarkcloudcoverSeries() (o, h, l, c []float64) {
	for range 10 {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	o = append(o, 100, 106)
	h = append(h, 105, 106.5)
	l = append(l, 99.5, 101)
	c = append(c, 104, 101.5)
	return
}

func TestCDLDARKCLOUDCOVERParams(t *testing.T) {
	s := fixtures(50)[0]
	bad := []float64{-1, -1e-300, math.Nextafter(taRealMax, math.Inf(1)), 1e38, math.Inf(1), math.Inf(-1), math.NaN(), -4e37}
	for _, p := range bad {
		if _, err := CDLDARKCLOUDCOVER(s.O, s.H, s.L, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("penetration %g: err = %v, want ErrBadParam", p, err)
		}
		if lb := CDLDARKCLOUDCOVERLookback(p); lb != -1 {
			t.Errorf("penetration %g: lookback %d, want -1", p, lb)
		}
		if _, _, err := cdldarkcloudcoverRange(0, 49, s.O, s.H, s.L, s.C, p, make([]int, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range penetration %g: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []float64{0, math.Copysign(0, -1), 0.5, 1, 3, taRealMax} {
		if _, err := CDLDARKCLOUDCOVER(s.O, s.H, s.L, s.C, p); err != nil {
			t.Errorf("penetration %g: err = %v", p, err)
		}
		if lb := CDLDARKCLOUDCOVERLookback(p); lb != 11 {
			t.Errorf("penetration %g: lookback %d, want 11", p, lb)
		}
	}
	if _, err := CDLDARKCLOUDCOVER(s.O, s.H[:49], s.L, s.C, 0.5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLDARKCLOUDCOVER(s.O[:10], s.H, s.L, s.C, 0.5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLDARKCLOUDCOVER(s.O, s.H, s.L, s.C[:10], 0.5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLDARKCLOUDCOVER(s.O, s.H, nil, s.C, 0.5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLDARKCLOUDCOVER(nil, nil, nil, nil, 0.5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLDARKCLOUDCOVER([]float64{}, []float64{}, []float64{}, []float64{}, 0.5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// A bad parameter is reported even for empty input.
	if _, err := CDLDARKCLOUDCOVER(nil, nil, nil, nil, -1); !errors.Is(err, ErrBadParam) {
		t.Errorf("empty input, bad param: err = %v", err)
	}
}

func TestCDLDARKCLOUDCOVERLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, p := range []int{0, 1, 5, 30} {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(BodyLong)
		if err := SetCandleSettings(BodyLong, d.RangeType, p, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLDARKCLOUDCOVERLookback(0.5); lb != p+1 {
			t.Errorf("period %d: lookback %d, want %d", p, lb, p+1)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	d := GetCandleSetting(ShadowVeryShort)
	_ = SetCandleSettings(ShadowVeryShort, d.RangeType, 40, d.Factor)
	if lb := CDLDARKCLOUDCOVERLookback(0.5); lb != 11 {
		t.Errorf("ShadowVeryShort changed: lookback %d, want 11", lb)
	}
	// Period 0: lookback 1; one bar gives no output, two give one. The white
	// body is compared with factor*its own body, so body > body is false:
	// it never fires with factor 1, and fires with factor 0.5.
	RestoreAllCandleDefaultSettings()
	d = GetCandleSetting(BodyLong)
	_ = SetCandleSettings(BodyLong, d.RangeType, 0, d.Factor)
	o, h, l, c := cdldarkcloudcoverSeries()
	out, err := CDLDARKCLOUDCOVER(o[10:11], h[10:11], l[10:11], c[10:11], 0.5)
	if err != nil || len(out) != 1 || out[0] != 0 {
		t.Errorf("n=1 with lookback 1: out=%v err=%v", out, err)
	}
	out, _ = CDLDARKCLOUDCOVER(o[10:], h[10:], l[10:], c[10:], 0.5)
	assertInt(t, "period 0 factor 1", out, []int{0, 0})
	_ = SetCandleSettings(BodyLong, d.RangeType, 0, 0.5)
	out, _ = CDLDARKCLOUDCOVER(o[10:], h[10:], l[10:], c[10:], 0.5)
	assertInt(t, "period 0 factor 0.5", out, []int{0, -100})
}

func TestCDLDARKCLOUDCOVERLengths(t *testing.T) {
	for _, p := range []float64{0, 0.3, 0.5, 1, taRealMax} {
		lb := CDLDARKCLOUDCOVERLookback(p)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := CDLDARKCLOUDCOVER(s.O, s.H, s.L, s.C, p)
				if err != nil {
					t.Fatalf("%s n=%d: %v", s.Name, n, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
				}
				for i, v := range out {
					if i < lb && v != 0 {
						t.Fatalf("%s n=%d p=%g: [%d]=%d inside lookback %d", s.Name, n, p, i, v, lb)
					}
					if v != 0 && v != -100 {
						t.Fatalf("%s n=%d p=%g: [%d]=%d, want 0 or -100", s.Name, n, p, i, v)
					}
				}
			}
		}
	}
}

func TestCDLDARKCLOUDCOVERKnownValues(t *testing.T) {
	type mod func(o, h, l, c []float64)
	cases := []struct {
		name string
		pen  float64
		m    mod
		want int
	}{
		{"dark cloud", 0.5, func(o, h, l, c []float64) {}, -100},
		{"first candle black", 0.5, func(o, h, l, c []float64) { o[10], c[10] = 104, 100 }, 0},
		{"first body equals average", 0.5, func(o, h, l, c []float64) { c[10] = 102; c[11] = 100.5 }, 0},
		{"first body just above average", 0.5, func(o, h, l, c []float64) { c[10] = 102.5; c[11] = 101 }, -100},
		{"second candle white", 0.5, func(o, h, l, c []float64) { o[11], c[11] = 101.5, 106 }, 0},
		{"second candle doji is white", 0.5, func(o, h, l, c []float64) { c[11] = 106 }, 0},
		{"open equals prior high", 0.5, func(o, h, l, c []float64) { o[11] = 105 }, 0},
		{"close equals prior open", 0.5, func(o, h, l, c []float64) { c[11] = 100 }, 0},
		{"close below prior open", 0.5, func(o, h, l, c []float64) { c[11] = 99.9 }, 0},
		{"close at penetration level", 0.5, func(o, h, l, c []float64) { c[11] = 102 }, 0},
		{"close just under penetration level", 0.5, func(o, h, l, c []float64) { c[11] = 101.99 }, -100},
		{"penetration 0: close under prior close", 0, func(o, h, l, c []float64) { c[11] = 103.9 }, -100},
		{"penetration 0: close at prior close", 0, func(o, h, l, c []float64) { c[11] = 104 }, 0},
		{"penetration 0.25: level 103", 0.25, func(o, h, l, c []float64) { c[11] = 102.9 }, -100},
		{"penetration 0.25: at level 103", 0.25, func(o, h, l, c []float64) { c[11] = 103 }, 0},
		{"penetration 1: needs close < prior open", 1, func(o, h, l, c []float64) { c[11] = 100.01 }, 0},
		{"penetration max", taRealMax, func(o, h, l, c []float64) {}, 0},
	}
	for _, tc := range cases {
		o, h, l, c := cdldarkcloudcoverSeries()
		tc.m(o, h, l, c)
		out, err := CDLDARKCLOUDCOVER(o, h, l, c, tc.pen)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 12)
		want[11] = tc.want
		assertInt(t, "CDLDARKCLOUDCOVER "+tc.name, out, want)
	}

	// Constant or zero series have no black candles: never fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLDARKCLOUDCOVER(s.O, s.H, s.L, s.C, 0)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLDARKCLOUDCOVERRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdldarkcloudcoverRange(-1, 5, s.O, s.H, s.L, s.C, 0.5, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdldarkcloudcoverRange(5, 4, s.O, s.H, s.L, s.C, 0.5, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdldarkcloudcoverRange(0, -1, s.O, s.H, s.L, s.C, 0.5, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdldarkcloudcoverRange(0, 10, s.O, s.H, s.L, s.C, 0.5, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdldarkcloudcoverRange(0, 29, s.O, s.H, s.L[:20], s.C, 0.5, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdldarkcloudcoverRange(0, 29, s.O, s.H, s.L, s.C, 0.5, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	if _, _, err := cdldarkcloudcoverRange(0, 29, s.O, s.H, s.L, s.C, 0.5, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdldarkcloudcoverRange(20, 29, s.O, s.H, s.L, s.C, 0.5, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLDARKCLOUDCOVER(s.O, s.H, s.L, s.C, 0.5)
	beg, nb, err = cdldarkcloudcoverRange(5, 25, s.O, s.H, s.L, s.C, 0.5, out)
	if err != nil || beg != 11 || nb != 15 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
	// The single pattern bar of the constructed series.
	o, h, l, c := cdldarkcloudcoverSeries()
	beg, nb, err = cdldarkcloudcoverRange(11, 11, o, h, l, c, 0.5, out)
	if err != nil || beg != 11 || nb != 1 || out[0] != -100 {
		t.Errorf("single bar: beg=%d nb=%d out0=%d err=%v", beg, nb, out[0], err)
	}
}
