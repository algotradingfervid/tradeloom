package ta

import (
	"errors"
	"testing"
)

// cdlbeltholdBase returns n identical background bars: open 100, close 101,
// high 101.5, low 99.5 (real body 1, high-low 2).
func cdlbeltholdBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 101.5)
		l = append(l, 99.5)
		c = append(c, 101)
	}
	return
}

func TestCDLBELTHOLDParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLBELTHOLD(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLBELTHOLD(s.O[:3], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLBELTHOLD(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLBELTHOLD(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLBELTHOLD(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLBELTHOLD([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLBELTHOLDLookback(); lb != 10 {
		t.Errorf("default lookback = %d, want 10", lb)
	}
}

func TestCDLBELTHOLDLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyLong, 20, 20},
		{ShadowVeryShort, 30, 30},
		{BodyLong, 0, 10}, // ShadowVeryShort still 10
		{ShadowVeryShort, 0, 10},
		{BodyShort, 50, 10}, // unrelated setting
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLBELTHOLDLookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Every average period 0: lookback 0, every bar is evaluated.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, ShadowVeryShort} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLBELTHOLDLookback(); lb != 0 {
		t.Errorf("all periods 0: lookback %d, want 0", lb)
	}
	// BodyLong compares the body with itself (factor 1): never "long".
	o, h, l, c := cdlbeltholdBase(1)
	out, err := CDLBELTHOLD(o, h, l, c)
	if err != nil || len(out) != 1 || out[0] != 0 {
		t.Errorf("n=1 with lookback 0: out=%v err=%v", out, err)
	}
	// With BodyLong factor 0.5 a marubozu fires on the very first bar.
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 0.5)
	out, err = CDLBELTHOLD([]float64{100}, []float64{103}, []float64{100}, []float64{103})
	if err != nil || len(out) != 1 || out[0] != 100 {
		t.Errorf("n=1 marubozu with lookback 0: out=%v err=%v", out, err)
	}
}

func TestCDLBELTHOLDLengths(t *testing.T) {
	lb := CDLBELTHOLDLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLBELTHOLD(s.O, s.H, s.L, s.C)
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
				if v != 0 && v != 100 && v != -100 {
					t.Fatalf("%s n=%d: [%d]=%d, want -100, 0 or 100", s.Name, n, i, v)
				}
			}
		}
	}
}

func TestCDLBELTHOLDKnownValues(t *testing.T) {
	// Background bars: body 1, high-low 2, so with default settings
	// BodyLong avg = 1 and ShadowVeryShort avg = 0.1*2 = 0.2.
	type bar struct{ o, h, l, c float64 }
	cases := []struct {
		name string
		b    bar
		want int
	}{
		{"bullish", bar{100, 103.5, 99.9, 103}, 100},             // body 3 > 1, lower 0.1 < 0.2
		{"bullish marubozu", bar{100, 103, 100, 103}, 100},       // lower 0
		{"bullish long upper", bar{100, 110, 100, 103}, 100},     // upper shadow irrelevant
		{"bearish", bar{103, 103.1, 99, 100}, -100},              // upper 0.1 < 0.2
		{"bearish marubozu", bar{103, 103, 100, 100}, -100},      // upper 0
		{"bearish long lower", bar{103, 103, 90, 100}, -100},     // lower shadow irrelevant
		{"bullish lower too long", bar{100, 103, 99.7, 103}, 0},  // lower 0.3
		{"bearish upper too long", bar{103, 103.5, 100, 100}, 0}, // upper 0.5
		{"body equal average", bar{100, 101, 100, 101}, 0},       // body 1 not > 1
		{"body short", bar{100, 100.5, 100, 100.5}, 0},           // body 0.5
		{"doji", bar{100, 100, 100, 100}, 0},                     // white, body 0
	}
	for _, tc := range cases {
		o, h, l, c := cdlbeltholdBase(10)
		o = append(o, tc.b.o)
		h = append(h, tc.b.h)
		l = append(l, tc.b.l)
		c = append(c, tc.b.c)
		out, err := CDLBELTHOLD(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 11)
		want[10] = tc.want
		assertInt(t, "CDLBELTHOLD "+tc.name, out, want)
	}

	// The current bar is excluded from the averages: a series of identical
	// marubozus never fires because each body equals the average.
	n := 30
	o, h, l, c := make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
	for i := range n {
		o[i], h[i], l[i], c[i] = 100, 103, 100, 103
	}
	out, _ := CDLBELTHOLD(o, h, l, c)
	assertInt(t, "identical marubozus", out, make([]int, n))

	// Constant series: zero body is never long.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLBELTHOLD(s.O, s.H, s.L, s.C)
		assertInt(t, s.Name, out, make([]int, 100))
	}
}

func TestCDLBELTHOLDRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlbeltholdRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlbeltholdRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlbeltholdRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlbeltholdRange(0, 9, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlbeltholdRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlbeltholdRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlbeltholdRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Single bar at the lookback.
	if beg, nb, err := cdlbeltholdRange(10, 10, s.O, s.H, s.L, s.C, out); err != nil || beg != 10 || nb != 1 {
		t.Errorf("single bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLBELTHOLD(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlbeltholdRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 10 || nb != 16 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
