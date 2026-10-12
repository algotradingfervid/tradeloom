package ta

import (
	"errors"
	"testing"
)

// cdlhikkakemodBar is one OHLC bar.
type cdlhikkakemodBar struct{ o, h, l, c float64 }

// cdlhikkakemodBg is the background bar: range 4 (high-low).
var cdlhikkakemodBg = cdlhikkakemodBar{100, 103, 99, 102}

// cdlhikkakemodWide returns a wide bar (high 120, low 80) closing at c. It
// is never an inside bar of the pattern bars, so it cannot start a new
// pattern; it only tests confirmation.
func cdlhikkakemodWide(c float64) cdlhikkakemodBar { return cdlhikkakemodBar{c, 120, 80, c} }

// cdlhikkakemodSeries puts 10 background bars before the given bars, so
// bars[0] is index 10 and the 4th pattern bar, bars[3], is index 13.
func cdlhikkakemodSeries(bars ...cdlhikkakemodBar) (o, h, l, c []float64) {
	for range 10 {
		bars = append([]cdlhikkakemodBar{cdlhikkakemodBg}, bars...)
	}
	for _, b := range bars {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

// The pattern bars used below. With default settings (Near: HighLow, 5,
// 0.2) the Near average for the 2nd candle (index 11) covers indexes 6..10:
// four background ranges of 4 and the 1st candle's range 20, so it is
// 0.2*36/5 = 1.44. The bullish 2nd candle must close <= 92+1.44, the
// bearish one >= 108-1.44.
var (
	cdlhikkakemodFirst  = cdlhikkakemodBar{100, 110, 90, 100}
	cdlhikkakemodThird  = cdlhikkakemodBar{100, 106, 94, 100}
	cdlhikkakemodBull4  = cdlhikkakemodBar{100, 105, 93, 100}
	cdlhikkakemodBear4  = cdlhikkakemodBar{100, 107, 95, 100}
	cdlhikkakemodNearAv = 0.2 * 36.0 / 5.0
)

// cdlhikkakemodOverwrite is a bearish pattern whose confirmation bar is
// also the 4th bar of a new bullish pattern (see TestCDLHIKKAKEMODKnownValues).
var cdlhikkakemodOverwrite = []cdlhikkakemodBar{
	cdlhikkakemodFirst,
	cdlhikkakemodSecond(107.5),
	cdlhikkakemodThird,
	cdlhikkakemodBear4,     // 13: A's bearish 4th; B's 1st (107/95)
	{100, 106, 96, 97},     // 14: B's 2nd, close near its low
	{100, 105, 96.5, 100},  // 15: B's 3rd
	{95, 104, 93, 93.5},    // 16: B's bullish 4th; close < 94
	cdlhikkakemodWide(106), // 17: confirms B
}

func cdlhikkakemodSecond(c float64) cdlhikkakemodBar { return cdlhikkakemodBar{100, 108, 92, c} }

func TestCDLHIKKAKEMODParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLHIKKAKEMOD(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLHIKKAKEMOD(s.O[:3], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLHIKKAKEMOD(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLHIKKAKEMOD(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLHIKKAKEMOD(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLHIKKAKEMOD([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLHIKKAKEMODLookback(); lb != 10 {
		t.Errorf("default lookback = %d, want 10", lb)
	}
}

func TestCDLHIKKAKEMODLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{Near, 0, 6}, // max(1, 0) + 5
		{Near, 1, 6},
		{Near, 2, 7},
		{Near, 20, 25},
		{BodyLong, 50, 10},
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLHIKKAKEMODLookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
		for _, n := range edgeLengths(tc.want) {
			s := fixtures(n)[0]
			out, err := CDLHIKKAKEMOD(s.O, s.H, s.L, s.C)
			if err != nil || len(out) != n {
				t.Fatalf("n=%d: len %d err %v", n, len(out), err)
			}
			for i := 0; i < tc.want && i < n; i++ {
				if out[i] != 0 {
					t.Fatalf("n=%d: [%d]=%d inside lookback", n, i, out[i])
				}
			}
		}
	}
}

func TestCDLHIKKAKEMODLengths(t *testing.T) {
	lb := CDLHIKKAKEMODLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLHIKKAKEMOD(s.O, s.H, s.L, s.C)
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
				switch v {
				case -200, -100, 0, 100, 200:
				default:
					t.Fatalf("%s n=%d: [%d]=%d, want one of -200,-100,0,100,200", s.Name, n, i, v)
				}
			}
		}
	}
}

func TestCDLHIKKAKEMODKnownValues(t *testing.T) {
	type b = cdlhikkakemodBar
	w := cdlhikkakemodWide
	first, third := cdlhikkakemodFirst, cdlhikkakemodThird
	bull4, bear4 := cdlhikkakemodBull4, cdlhikkakemodBear4
	lowLim := 92 + cdlhikkakemodNearAv
	highLim := 108 - cdlhikkakemodNearAv
	sec := cdlhikkakemodSecond
	cases := []struct {
		name string
		bars []b
		want map[int]int // index -> value; all others 0
	}{
		{"bullish", []b{first, sec(92.5), third, bull4}, map[int]int{13: 100}},
		{"bearish", []b{first, sec(107.5), third, bear4}, map[int]int{13: -100}},
		{"bull 2nd close at the near limit", []b{first, sec(lowLim), third, bull4}, map[int]int{13: 100}},
		{"bull 2nd close above the near limit", []b{first, sec(lowLim + 0.01), third, bull4}, nil},
		{"bear 2nd close at the near limit", []b{first, sec(highLim), third, bear4}, map[int]int{13: -100}},
		{"bear 2nd close below the near limit", []b{first, sec(highLim - 0.01), third, bear4}, nil},
		// 2nd close near the low does not make a bearish breakout fire.
		{"bear breakout, 2nd near low", []b{first, sec(92.5), third, bear4}, nil},
		{"bull breakout, 2nd near high", []b{first, sec(107.5), third, bull4}, nil},
		// Inside-bar conditions are strict.
		{"2nd high equals 1st high", []b{first, {100, 110, 92, 92.5}, third, bull4}, nil},
		{"2nd low equals 1st low", []b{first, {100, 108, 90, 90.5}, {100, 106, 94, 100}, bull4}, nil},
		{"3rd high equals 2nd high", []b{first, sec(92.5), {100, 108, 94, 100}, bull4}, nil},
		{"3rd low equals 2nd low", []b{first, sec(92.5), {100, 106, 92, 100}, {100, 105, 91, 100}}, nil},
		// 4th must break both sides the same way.
		{"4th high equals 3rd high", []b{first, sec(92.5), third, {100, 106, 93, 100}}, nil},
		{"4th low equals 3rd low", []b{first, sec(92.5), third, {100, 105, 94, 100}}, nil},
		{"4th outside bar", []b{first, sec(92.5), third, {100, 107, 93, 100}}, nil},
		{"4th inside bar", []b{first, sec(92.5), third, {100, 105, 95, 100}}, nil},
		// Confirmation: close beyond the 3rd candle's extreme within 3 bars.
		{"bull confirmed next bar", []b{first, sec(92.5), third, bull4, w(106.5)}, map[int]int{13: 100, 14: 200}},
		{"bull confirmed 3rd bar", []b{first, sec(92.5), third, bull4, w(100), w(100), w(106.5)}, map[int]int{13: 100, 16: 200}},
		{"bull confirmation too late", []b{first, sec(92.5), third, bull4, w(100), w(100), w(100), w(106.5)}, map[int]int{13: 100}},
		{"bull close equals 3rd high", []b{first, sec(92.5), third, bull4, w(106)}, map[int]int{13: 100}},
		{"bull confirmed only once", []b{first, sec(92.5), third, bull4, w(107), w(108)}, map[int]int{13: 100, 14: 200}},
		{"bull: close below 3rd low ignored", []b{first, sec(92.5), third, bull4, w(90)}, map[int]int{13: 100}},
		{"bear confirmed next bar", []b{first, sec(107.5), third, bear4, w(93.5)}, map[int]int{13: -100, 14: -200}},
		{"bear confirmed 3rd bar", []b{first, sec(107.5), third, bear4, w(100), w(100), w(93.5)}, map[int]int{13: -100, 16: -200}},
		{"bear confirmation too late", []b{first, sec(107.5), third, bear4, w(100), w(100), w(100), w(93.5)}, map[int]int{13: -100}},
		{"bear close equals 3rd low", []b{first, sec(107.5), third, bear4, w(94)}, map[int]int{13: -100}},
		{"bear: close above 3rd high ignored", []b{first, sec(107.5), third, bear4, w(110)}, map[int]int{13: -100}},
	}
	for _, tc := range cases {
		o, h, l, c := cdlhikkakemodSeries(tc.bars...)
		out, err := CDLHIKKAKEMOD(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, len(o))
		for k, v := range tc.want {
			want[k] = v
		}
		assertInt(t, "CDLHIKKAKEMOD "+tc.name, out, want)
	}

	// A new hikkake on the would-be confirmation bar overwrites it. Pattern
	// A is bearish (bars 10..13, 3rd low 94). Pattern B is bars 13..16: the
	// Near average for its 2nd candle (14) covers ranges 4,20,16,12,12, so
	// 0.2*64/5 = 2.56 and its close 97 <= 96+2.56 is near the low. Bar 16 is
	// B's bullish 4th and also closes below 94, which would confirm A; only
	// B's +100 is reported. Bar 17 then confirms B (close > 105).
	o, h, l, c := cdlhikkakemodSeries(cdlhikkakemodOverwrite...)
	out, err := CDLHIKKAKEMOD(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	want := make([]int, len(o))
	want[13], want[16], want[17] = -100, 100, 200
	assertInt(t, "CDLHIKKAKEMOD overwrite", out, want)

	// Constant and zero series have no inside bars: never fire.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLHIKKAKEMOD(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLHIKKAKEMODRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlhikkakemodRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlhikkakemodRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlhikkakemodRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlhikkakemodRange(0, 9, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if _, _, err := cdlhikkakemodRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlhikkakemodRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	if _, nb, err := cdlhikkakemodRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	full, _ := CDLHIKKAKEMOD(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlhikkakemodRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 10 || nb != 16 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])

	// A pattern just before startIdx is replayed, so its confirmation is
	// still reported inside the range.
	o, h, l, c := cdlhikkakemodSeries(cdlhikkakemodFirst, cdlhikkakemodSecond(92.5), cdlhikkakemodThird,
		cdlhikkakemodBull4, cdlhikkakemodWide(100), cdlhikkakemodWide(106.5))
	for start := 14; start <= 15; start++ {
		out := make([]int, 2)
		beg, nb, err := cdlhikkakemodRange(start, 15, o, h, l, c, out)
		if err != nil || beg != start || nb != 16-start || out[nb-1] != 200 {
			t.Errorf("start %d: beg=%d nb=%d out=%v err=%v, want confirmation 200 at 15", start, beg, nb, out, err)
		}
	}
}
