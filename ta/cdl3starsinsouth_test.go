package ta

import (
	"errors"
	"testing"
)

// cdl3starsinsouthBase returns n identical background bars: open 100,
// close 102, high 103, low 99 (real body 2, high-low 4).
func cdl3starsinsouthBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

type cdl3starsinsouthBar struct{ o, h, l, c float64 }

// cdl3starsinsouthPattern is a hand-checked pattern on top of 12 background
// bars (bars 12, 13, 14). With default settings:
//   - bar 12: black, body 5 > BodyLong avg 2 (bars 2..11); lower shadow 6 >
//     ShadowLong (period 0: its own body 5).
//   - bar 13: black, body 2.5 < 5; open 98 in (95, 100.5]; low 92 < 95 and
//     >= 89; lower shadow 3.5 > ShadowVeryShort avg 0.1*(9*4+11.5)/10 = 0.475.
//   - bar 14: black, body 1 < BodyShort avg (8*2+5+2.5)/10 = 2.35; lower and
//     upper shadows 0.2 < ShadowVeryShort avg 0.1*(8*4+11.5+6.5)/10 = 0.5;
//     low 93.8 > 92 and high 95.2 < 98.5.
var cdl3starsinsouthPattern = [3]cdl3starsinsouthBar{
	{100, 100.5, 89, 95},
	{98, 98.5, 92, 95.5},
	{95, 95.2, 93.8, 94},
}

func cdl3starsinsouthBuild(bars [3]cdl3starsinsouthBar) (o, h, l, c []float64) {
	o, h, l, c = cdl3starsinsouthBase(12)
	for _, b := range bars {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

func TestCDL3STARSINSOUTHParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDL3STARSINSOUTH(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3STARSINSOUTH(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3STARSINSOUTH(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3STARSINSOUTH(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDL3STARSINSOUTH(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDL3STARSINSOUTH([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDL3STARSINSOUTHLookback(); lb != 12 {
		t.Errorf("default lookback = %d, want 12", lb)
	}
}

func TestCDL3STARSINSOUTHLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyLong, 20, 22},
		{BodyShort, 15, 17},
		{ShadowLong, 30, 32},
		{ShadowVeryShort, 25, 27},
		{ShadowVeryShort, 0, 12}, // BodyLong/BodyShort still 10
		{Near, 40, 12},           // not used by this pattern
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDL3STARSINSOUTHLookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Every average period 0: only the two prior candles are needed.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, BodyShort, ShadowLong, ShadowVeryShort} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDL3STARSINSOUTHLookback(); lb != 2 {
		t.Errorf("all periods 0: lookback %d, want 2", lb)
	}
	for n := 0; n <= 3; n++ {
		o, h, l, c := cdl3starsinsouthBase(n)
		out, err := CDL3STARSINSOUTH(o, h, l, c)
		if err != nil || len(out) != n {
			t.Errorf("n=%d with lookback 2: out=%v err=%v", n, out, err)
		}
	}
}

func TestCDL3STARSINSOUTHLengths(t *testing.T) {
	lb := CDL3STARSINSOUTHLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDL3STARSINSOUTH(s.O, s.H, s.L, s.C)
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

func TestCDL3STARSINSOUTHKnownValues(t *testing.T) {
	p := cdl3starsinsouthPattern
	mod := func(k int, b cdl3starsinsouthBar) [3]cdl3starsinsouthBar {
		q := p
		q[k] = b
		return q
	}
	cases := []struct {
		name string
		bars [3]cdl3starsinsouthBar
		want int
	}{
		{"pattern", p, 100},
		{"1st white", mod(0, cdl3starsinsouthBar{95, 100.5, 89, 100}), 0},
		{"2nd white", mod(1, cdl3starsinsouthBar{95.5, 98.5, 92, 98}), 0},
		{"3rd white", mod(2, cdl3starsinsouthBar{94, 95.2, 93.8, 95}), 0},
		{"3rd doji counts as white", mod(2, cdl3starsinsouthBar{94.5, 94.6, 94.4, 94.5}), 0},
		{"1st body not long", mod(0, cdl3starsinsouthBar{97, 97.5, 89, 95}), 0},              // body 2 not > 2
		{"1st lower shadow = body", mod(0, cdl3starsinsouthBar{100, 100.5, 90, 95}), 0},      // 5 not > 5
		{"2nd body not smaller", mod(1, cdl3starsinsouthBar{100.5, 100.5, 92, 95.5}), 0},     // 5 not < 5
		{"2nd opens at 1st close", mod(1, cdl3starsinsouthBar{95, 98.5, 92, 93}), 0},         // open not > close1
		{"2nd opens above 1st high", mod(1, cdl3starsinsouthBar{100.6, 100.6, 92, 98.2}), 0}, // open > high1
		{"2nd opens at 1st high", mod(1, cdl3starsinsouthBar{100.5, 100.5, 92, 98}), 100},    // open <= high1
		{"2nd low not below close1", mod(1, cdl3starsinsouthBar{98, 98.5, 95, 95.5}), 0},     // low 95 not < 95
		{"2nd low below 1st low", mod(1, cdl3starsinsouthBar{98, 98.5, 88.9, 95.5}), 0},      // low < low1
		{"2nd low at 1st low", mod(1, cdl3starsinsouthBar{98, 98.5, 89, 95.5}), 100},         // low >= low1
		{"2nd no lower shadow", mod(1, cdl3starsinsouthBar{98, 98.5, 95.4, 95.5}), 0},        // 0.1 not > 0.475
		{"3rd body too big", mod(2, cdl3starsinsouthBar{96, 96.1, 93.4, 93.5}), 0},           // body 2.5 not < avg
		{"3rd lower shadow long", mod(2, cdl3starsinsouthBar{95, 95.2, 93.3, 94}), 0},        // 0.7 not < 0.5
		{"3rd upper shadow long", mod(2, cdl3starsinsouthBar{95, 95.6, 93.8, 94}), 0},        // 0.6 not < 0.5
		{"3rd low at 2nd low", mod(2, cdl3starsinsouthBar{92.5, 92.6, 92, 92.1}), 0},         // low not > low2
		{"3rd high at 2nd high", mod(2, cdl3starsinsouthBar{98.2, 98.5, 97.3, 97.5}), 0},     // high not < high2
	}
	for _, tc := range cases {
		o, h, l, c := cdl3starsinsouthBuild(tc.bars)
		out, err := CDL3STARSINSOUTH(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 15)
		want[14] = tc.want
		assertInt(t, "CDL3STARSINSOUTH "+tc.name, out, want)
	}

	// Constant series: every candle is "white" (close >= open): never fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDL3STARSINSOUTH(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDL3STARSINSOUTHRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdl3starsinsouthRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdl3starsinsouthRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdl3starsinsouthRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdl3starsinsouthRange(0, 11, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// First bar after the lookback.
	beg, nb, err = cdl3starsinsouthRange(0, 12, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 12 || nb != 1 {
		t.Errorf("first bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdl3starsinsouthRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdl3starsinsouthRange(0, 29, s.O[:29], s.H, s.L, s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("input one short: %v", err)
	}
	if _, _, err := cdl3starsinsouthRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdl3starsinsouthRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDL3STARSINSOUTH(s.O, s.H, s.L, s.C)
	beg, nb, err = cdl3starsinsouthRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 12 || nb != 14 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])

	// Sub-range on the constructed pattern fires at the right bar.
	o, h, l, c := cdl3starsinsouthBuild(cdl3starsinsouthPattern)
	beg, nb, err = cdl3starsinsouthRange(14, 14, o, h, l, c, out)
	if err != nil || beg != 14 || nb != 1 || out[0] != 100 {
		t.Errorf("pattern bar only: beg=%d nb=%d out=%d err=%v", beg, nb, out[0], err)
	}
}
