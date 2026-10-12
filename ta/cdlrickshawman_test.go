package ta

import (
	"errors"
	"testing"
)

// cdlrickshawmanBase returns n identical background bars: open 100,
// close 104, high 106, low 96 (high-low 10). With default settings BodyDoji
// (HighLow, period 10, factor 0.1) averages exactly 1 and Near (HighLow,
// period 5, factor 0.2) exactly 2 against it. ShadowLong (RealBody,
// period 0, factor 1) is the current bar's own body.
func cdlrickshawmanBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 106)
		l = append(l, 96)
		c = append(c, 104)
	}
	return
}

func TestCDLRICKSHAWMANParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLRICKSHAWMAN(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLRICKSHAWMAN(s.O[:10], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLRICKSHAWMAN(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLRICKSHAWMAN(s.O, s.H, s.L, s.C[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLRICKSHAWMAN(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLRICKSHAWMAN([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLRICKSHAWMANLookback(); lb != 10 {
		t.Errorf("default lookback = %d, want 10", lb)
	}
}

func TestCDLRICKSHAWMANLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	type change struct {
		set    CandleSettingType
		period int
	}
	cases := []struct {
		changes []change
		want    int
	}{
		{[]change{{BodyDoji, 20}}, 20},
		{[]change{{ShadowLong, 30}}, 30},
		{[]change{{Near, 25}}, 25},
		{[]change{{ShadowLong, 5}}, 10},         // BodyDoji still 10
		{[]change{{BodyDoji, 0}}, 5},            // Near default 5
		{[]change{{BodyDoji, 3}, {Near, 0}}, 3}, // ShadowLong default 0
		{[]change{{BodyDoji, 0}, {Near, 0}}, 0},
		{[]change{{BodyLong, 50}}, 10}, // unrelated setting
	}
	for k, tc := range cases {
		RestoreAllCandleDefaultSettings()
		for _, ch := range tc.changes {
			d := GetCandleSetting(ch.set)
			if err := SetCandleSettings(ch.set, d.RangeType, ch.period, d.Factor); err != nil {
				t.Fatal(err)
			}
		}
		if lb := CDLRICKSHAWMANLookback(); lb != tc.want {
			t.Errorf("case %d: lookback %d, want %d", k, lb, tc.want)
		}
	}
	// All periods 0: lookback 0, every bar is judged on itself.
	// BodyDoji avg = 0.1*range, Near avg = 0.2*range.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyDoji, Near} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	// Range 10, body 0 <= 1, shadows 5 > 0, body at midpoint.
	out, err := CDLRICKSHAWMAN([]float64{100}, []float64{105}, []float64{95}, []float64{100})
	if err != nil || len(out) != 1 || out[0] != 100 {
		t.Errorf("n=1 rickshaw man with lookback 0: out=%v err=%v", out, err)
	}
	// Range 10, body 2 > 1.
	out, err = CDLRICKSHAWMAN([]float64{100}, []float64{106}, []float64{96}, []float64{102})
	if err != nil || len(out) != 1 || out[0] != 0 {
		t.Errorf("n=1 big body with lookback 0: out=%v err=%v", out, err)
	}
}

func TestCDLRICKSHAWMANLengths(t *testing.T) {
	lb := CDLRICKSHAWMANLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLRICKSHAWMAN(s.O, s.H, s.L, s.C)
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

func TestCDLRICKSHAWMANKnownValues(t *testing.T) {
	// Against the background BodyDoji avg is exactly 1 and Near avg exactly
	// 2; ShadowLong avg is the tested bar's own body. Every boundary below
	// is exact in binary floating point.
	type bar struct{ o, h, l, c float64 }
	cases := []struct {
		name string
		b    bar
		want int
	}{
		{"classic", bar{100, 105, 95, 100}, 100},              // body 0, shadows 5, mid 100
		{"dragonfly shape", bar{100, 100, 90, 100}, 0},        // upper 0 not > 0
		{"gravestone shape", bar{100, 110, 100, 100}, 0},      // lower 0 not > 0
		{"four price", bar{100, 100, 100, 100}, 0},            // no shadows
		{"body low of mid", bar{100, 101, 91, 100}, 0},        // mid 96: 100 > 96+2
		{"min at mid+near", bar{100, 104, 92, 100}, 100},      // mid 98: 100 <= 100
		{"min beyond mid+near", bar{100, 104, 91, 100}, 0},    // mid 97.5: 100 > 99.5
		{"max at mid-near", bar{100, 108, 96, 100}, 100},      // mid 102: 100 >= 100
		{"max beyond mid-near", bar{100, 109, 96, 100}, 0},    // mid 102.5: 100 < 100.5
		{"white body equal avg", bar{100, 103, 97, 101}, 100}, // body 1 <= 1, shadows 3, 2 > 1
		{"black body equal avg", bar{101, 103, 97, 100}, 100}, // body 1 <= 1, lower 3, upper 2
		{"body too big", bar{100, 103, 97, 101.5}, 0},         // body 1.5 > 1
		{"upper equal body", bar{100, 102, 97, 101}, 0},       // upper 1 not > 1
		{"lower equal body", bar{100, 104, 99, 101}, 0},       // lower 1 not > 1
		{"black lower from close", bar{101, 103, 99, 100}, 0}, // lower 1 not > body 1
		{"black upper from open", bar{101, 102, 97, 100}, 0},  // upper 1 not > body 1
		{"black both long", bar{101, 102.5, 98.5, 100}, 100},  // shadows 1.5 > 1, mid 100.5
		{"long white body", bar{100, 106, 96, 104}, 0},        // background bar
	}
	for _, tc := range cases {
		o, h, l, c := cdlrickshawmanBase(10)
		o = append(o, tc.b.o)
		h = append(h, tc.b.h)
		l = append(l, tc.b.l)
		c = append(c, tc.b.c)
		out, err := CDLRICKSHAWMAN(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 11)
		want[10] = tc.want
		assertInt(t, "CDLRICKSHAWMAN "+tc.name, out, want)
	}

	// The Near average excludes the current candle and slides: bar 10 has
	// range 35, so bar 11's Near average is (4*10+35)/5*0.2 = 3 and a body
	// 2.5 above the midpoint qualifies, though it would not against the
	// plain background (Near 2). Bar 10 itself is too far from its midpoint.
	o, h, l, c := cdlrickshawmanBase(10)
	o = append(o, 100, 100)
	h = append(h, 120, 104)
	l = append(l, 85, 91)
	c = append(c, 100, 100)
	out, _ := CDLRICKSHAWMAN(o, h, l, c)
	assertInt(t, "sliding average", out, []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100})
	// Same bar 11 after a plain background bar: no pattern.
	o[10], h[10], l[10], c[10] = 100, 106, 96, 104
	out, _ = CDLRICKSHAWMAN(o, h, l, c)
	assertInt(t, "plain background", out, []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})

	// All-zero bars have no shadows: never a rickshaw man.
	for _, s := range fixtures(100) {
		if s.Name != "zeros" {
			continue
		}
		out, _ := CDLRICKSHAWMAN(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLRICKSHAWMANRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlrickshawmanRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlrickshawmanRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlrickshawmanRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlrickshawmanRange(0, 9, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// First bar after the lookback.
	beg, nb, err = cdlrickshawmanRange(0, 10, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 10 || nb != 1 {
		t.Errorf("first output: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlrickshawmanRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlrickshawmanRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlrickshawmanRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLRICKSHAWMAN(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlrickshawmanRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 10 || nb != 16 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
