package ta

import (
	"errors"
	"testing"
)

// cdltristarBar is one candle.
type cdltristarBar struct{ o, h, l, c float64 }

// cdltristarSeries appends the three pattern bars to 10 background bars
// (open 100, close 102, high 104, low 99: high-low range 5), so the 3rd doji
// is bar 12, the first evaluated bar with default settings, and the BodyDoji
// average (taken at the 1st doji, bar 10, over bars 0..9) is 0.1*5 = 0.5.
func cdltristarSeries(b1, b2, b3 cdltristarBar) (o, h, l, c []float64) {
	for range 10 {
		o = append(o, 100)
		h = append(h, 104)
		l = append(l, 99)
		c = append(c, 102)
	}
	for _, b := range []cdltristarBar{b1, b2, b3} {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

func TestCDLTRISTARParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLTRISTAR(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLTRISTAR(s.O[:3], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLTRISTAR(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLTRISTAR(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLTRISTAR(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLTRISTAR([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLTRISTARLookback(); lb != 12 {
		t.Errorf("default lookback = %d, want 12", lb)
	}
}

func TestCDLTRISTARLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyDoji, 20, 22},
		{BodyDoji, 1, 3},
		{BodyDoji, 0, 2},
		{BodyLong, 50, 12}, // unrelated setting
		{BodyShort, 0, 12},
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLTRISTARLookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}

	// BodyDoji period 0: lookback 2, the average is the 1st candle's own
	// range times the factor.
	RestoreAllCandleDefaultSettings()
	d := GetCandleSetting(BodyDoji)
	_ = SetCandleSettings(BodyDoji, d.RangeType, 0, d.Factor)
	for n := 0; n <= 3; n++ {
		o, h, l, c := cdltristarSeries(cdltristarBar{}, cdltristarBar{}, cdltristarBar{})
		out, err := CDLTRISTAR(o[:n], h[:n], l[:n], c[:n])
		if err != nil || len(out) != n {
			t.Errorf("n=%d with lookback 2: out=%v err=%v", n, out, err)
		}
		for i, v := range out {
			if v != 0 {
				t.Errorf("n=%d: [%d]=%d", n, i, v)
			}
		}
	}
	// 1st candle range 5 -> threshold 0.5; all bodies 0.2.
	o := []float64{100, 101, 100.6}
	h := []float64{102.5, 101.5, 101}
	l := []float64{97.5, 100.5, 100}
	c := []float64{100.2, 101.2, 100.8}
	out, err := CDLTRISTAR(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "period 0 bearish", out, []int{0, 0, -100})
	// Same bars but 1st range 1 -> threshold 0.1 < 0.2: no doji.
	h[0], l[0] = 100.5, 99.5
	out, _ = CDLTRISTAR(o, h, l, c)
	assertInt(t, "period 0 not doji", out, []int{0, 0, 0})
}

func TestCDLTRISTARLengths(t *testing.T) {
	lb := CDLTRISTARLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLTRISTAR(s.O, s.H, s.L, s.C)
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

// cdltristarCases are hand-checked patterns: the BodyDoji threshold is 0.5
// (see cdltristarSeries) and the result is at bar 12.
var cdltristarCases = []struct {
	name       string
	b1, b2, b3 cdltristarBar
	want       int
}{
	// d1 body [100,100.2]; d2 body [101,101.2] gaps up; d3 top 100.8 < 101.2.
	{"bearish", cdltristarBar{100, 100.5, 99.5, 100.2}, cdltristarBar{101, 101.5, 100.5, 101.2}, cdltristarBar{100.6, 101, 100, 100.8}, -100},
	// d2 body [99,99.2] gaps down; d3 bottom 99.4 > 99.
	{"bullish", cdltristarBar{100, 100.5, 99.5, 100.2}, cdltristarBar{99.2, 99.5, 98.5, 99}, cdltristarBar{99.8, 100, 99.2, 99.4}, 100},
	// Bodies exactly at the threshold 0.5 are doji (<=).
	{"bodies equal threshold", cdltristarBar{100, 100.5, 99.5, 100.5}, cdltristarBar{101, 101.5, 100.5, 101.5}, cdltristarBar{100.5, 101, 100, 101}, -100},
	{"1st body above threshold", cdltristarBar{100, 100.6, 99.5, 100.5000001}, cdltristarBar{101, 101.5, 100.5, 101.2}, cdltristarBar{100.6, 101, 100, 100.8}, 0},
	{"2nd body above threshold", cdltristarBar{100, 100.5, 99.5, 100.2}, cdltristarBar{101, 101.6, 100.5, 101.5000001}, cdltristarBar{100.6, 101, 100, 100.8}, 0},
	{"3rd body above threshold", cdltristarBar{100, 100.5, 99.5, 100.2}, cdltristarBar{101, 101.5, 100.5, 101.2}, cdltristarBar{100.5, 101.1, 100, 101.0000001}, 0},
	// Three doji without a star gap: 0.
	{"no gap", cdltristarBar{100, 100.5, 99.5, 100.2}, cdltristarBar{100.1, 100.5, 99.5, 100.3}, cdltristarBar{100, 100.5, 99.5, 100.2}, 0},
	// 2nd body touching the 1st does not gap (strict).
	{"2nd touches 1st (up)", cdltristarBar{100, 100.5, 99.5, 100.2}, cdltristarBar{100.2, 100.8, 100, 100.4}, cdltristarBar{100, 100.5, 99.5, 100.1}, 0},
	{"2nd touches 1st (down)", cdltristarBar{100, 100.5, 99.5, 100.2}, cdltristarBar{99.8, 100.2, 99.5, 100}, cdltristarBar{100, 100.5, 99.5, 100.1}, 0},
	// 3rd top equal to the 2nd top: not lower (strict) -> 0.
	{"3rd top equals 2nd top", cdltristarBar{100, 100.5, 99.5, 100.2}, cdltristarBar{101, 101.5, 100.5, 101.2}, cdltristarBar{100.9, 101.5, 100.5, 101.2}, 0},
	{"3rd higher than 2nd", cdltristarBar{100, 100.5, 99.5, 100.2}, cdltristarBar{101, 101.5, 100.5, 101.2}, cdltristarBar{101.3, 101.8, 101, 101.4}, 0},
	// 3rd bottom equal to the 2nd bottom -> 0.
	{"3rd bottom equals 2nd bottom", cdltristarBar{100, 100.5, 99.5, 100.2}, cdltristarBar{99.2, 99.5, 98.5, 99}, cdltristarBar{99, 99.5, 98.5, 99.3}, 0},
	{"3rd lower than 2nd", cdltristarBar{100, 100.5, 99.5, 100.2}, cdltristarBar{99.2, 99.5, 98.5, 99}, cdltristarBar{98.8, 99, 98, 98.6}, 0},
	// The 3rd need not gap from the 2nd: overlapping is fine.
	{"bullish 3rd overlaps 2nd", cdltristarBar{100, 100.5, 99.5, 100.2}, cdltristarBar{99.2, 99.5, 98.5, 99}, cdltristarBar{99.1, 99.5, 98.5, 99.3}, 100},
	// Perfect doji (open == close) everywhere.
	{"exact doji bearish", cdltristarBar{100, 100.5, 99.5, 100}, cdltristarBar{101, 101.5, 100.5, 101}, cdltristarBar{100.5, 101, 100, 100.5}, -100},
}

func TestCDLTRISTARKnownValues(t *testing.T) {
	for _, tc := range cdltristarCases {
		o, h, l, c := cdltristarSeries(tc.b1, tc.b2, tc.b3)
		out, err := CDLTRISTAR(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 13)
		want[12] = tc.want
		assertInt(t, "CDLTRISTAR "+tc.name, out, want)
	}

	// The average is taken at the 1st doji (bar i-2), so its period covers
	// bars 0..9 only: a huge-range 2nd or 3rd candle does not widen it.
	o, h, l, c := cdltristarSeries(cdltristarBar{100, 100.5, 99.5, 100.2}, cdltristarBar{101, 200, 50, 101.2}, cdltristarBar{100.6, 300, 10, 100.8})
	out, _ := CDLTRISTAR(o, h, l, c)
	if out[12] != -100 {
		t.Errorf("wide 2nd/3rd ranges: [12]=%d, want -100", out[12])
	}

	// Constant and zero series: every candle is a doji but no star gap.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLTRISTAR(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLTRISTARRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdltristarRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdltristarRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdltristarRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdltristarRange(0, 11, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdltristarRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdltristarRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdltristarRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLTRISTAR(s.O, s.H, s.L, s.C)
	beg, nb, err = cdltristarRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 12 || nb != 14 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
