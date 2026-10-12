package ta

import (
	"errors"
	"testing"
)

// cdl3whitesoldiersBar is one OHLC candle.
type cdl3whitesoldiersBar struct{ o, h, l, c float64 }

// cdl3whitesoldiersSeries returns 10 identical background bars (open 100,
// high 102, low 99, close 101: body 1, high-low 3, upper shadow 1) followed
// by the given bars.
func cdl3whitesoldiersSeries(bars ...cdl3whitesoldiersBar) (o, h, l, c []float64) {
	for range 10 {
		o = append(o, 100)
		h = append(h, 102)
		l = append(l, 99)
		c = append(c, 101)
	}
	for _, b := range bars {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

// cdl3whitesoldiersSoldiers are three white candles that form the pattern
// after the background (every one has high-low 3, body 2, upper shadow 0.1).
var cdl3whitesoldiersSoldiers = []cdl3whitesoldiersBar{
	{100, 102.1, 99.1, 102},
	{101.5, 103.6, 100.6, 103.5},
	{103, 105.1, 102.1, 105},
}

func TestCDL3WHITESOLDIERSParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDL3WHITESOLDIERS(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3WHITESOLDIERS(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3WHITESOLDIERS(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3WHITESOLDIERS(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDL3WHITESOLDIERS(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDL3WHITESOLDIERS([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDL3WHITESOLDIERSLookback(); lb != 12 {
		t.Errorf("default lookback = %d, want 12", lb)
	}
}

func TestCDL3WHITESOLDIERSLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{ShadowVeryShort, 20, 22},
		{BodyShort, 30, 32},
		{Far, 40, 42},
		{Near, 50, 52},
		{BodyShort, 0, 12}, // ShadowVeryShort still 10
		{BodyLong, 99, 12}, // unused setting
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDL3WHITESOLDIERSLookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Every average period 0: only the two prior candles are needed.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{ShadowVeryShort, BodyShort, Far, Near} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDL3WHITESOLDIERSLookback(); lb != 2 {
		t.Errorf("all periods 0: lookback %d, want 2", lb)
	}
	for _, n := range []int{1, 2, 3} {
		o, h, l, c := cdl3whitesoldiersSeries()
		out, err := CDL3WHITESOLDIERS(o[:n], h[:n], l[:n], c[:n])
		if err != nil || len(out) != n {
			t.Errorf("n=%d with lookback 2: out=%v err=%v", n, out, err)
		}
	}
}

func TestCDL3WHITESOLDIERSLengths(t *testing.T) {
	lb := CDL3WHITESOLDIERSLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDL3WHITESOLDIERS(s.O, s.H, s.L, s.C)
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

func TestCDL3WHITESOLDIERSKnownValues(t *testing.T) {
	// With default settings, at the third soldier (bar 12):
	// ShadowVeryShort avg = 0.1*3 = 0.3 for each candle (every prior bar
	// has high-low 3); Near avg = 0.2*3 = 0.6; Far avg = 0.6*3 = 1.8;
	// BodyShort avg = (8*1 + 2*2)/10 = 1.2.
	a, b, c3 := cdl3whitesoldiersSoldiers[0], cdl3whitesoldiersSoldiers[1], cdl3whitesoldiersSoldiers[2]
	cases := []struct {
		name string
		bars []cdl3whitesoldiersBar
		want int
	}{
		{"soldiers", []cdl3whitesoldiersBar{a, b, c3}, 100},
		{"third black", []cdl3whitesoldiersBar{a, b, {105, 105.1, 102.1, 103.6}}, 0},
		{"first black", []cdl3whitesoldiersBar{{102, 102.1, 99.1, 100}, b, c3}, 0},
		{"third long upper shadow", []cdl3whitesoldiersBar{a, b, {103, 105.4, 102.4, 105}}, 0}, // 0.4 not < 0.3
		{"second long upper shadow", []cdl3whitesoldiersBar{a, {101.5, 103.9, 100.9, 103.5}, c3}, 0},
		{"third opens too high", []cdl3whitesoldiersBar{a, b, {104.2, 106.3, 103.3, 106.2}}, 0}, // 104.2 > 103.5+0.6
		{"third opens near", []cdl3whitesoldiersBar{a, b, {104, 106.1, 103.1, 106}}, 100},       // 104 <= 104.1
		{"third opens below second", []cdl3whitesoldiersBar{a, b, {101.4, 105.1, 102.1, 105}}, 0},
		{"third close not higher", []cdl3whitesoldiersBar{a, b, {101.6, 103.6, 100.6, 103.5}}, 0},
		{"second close not higher", []cdl3whitesoldiersBar{a, {101.5, 102.1, 99.1, 102}, c3}, 0},
		{"third body short", []cdl3whitesoldiersBar{a, b, {103, 104.2, 101.2, 104.1}}, 0}, // 1.1 not > 1.2
		{"third body far shorter", []cdl3whitesoldiersBar{
			{98, 102.1, 99.1, 102},     // body 4, high-low 3
			{99, 103.6, 100.6, 103.5},  // body 4.5
			{101, 105.1, 102.1, 103.6}, // body 2.6 <= 4.5-1.8
		}, 0},
	}
	for _, tc := range cases {
		o, h, l, c := cdl3whitesoldiersSeries(tc.bars...)
		out, err := CDL3WHITESOLDIERS(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 13)
		want[12] = tc.want
		assertInt(t, "CDL3WHITESOLDIERS "+tc.name, out, want)
	}

	// Flat series have no white candle with a short upper shadow strictly
	// below a zero average: never a pattern. Falling candles are black.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" && s.Name != "down" {
			continue
		}
		out, _ := CDL3WHITESOLDIERS(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDL3WHITESOLDIERSRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdl3whitesoldiersRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdl3whitesoldiersRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdl3whitesoldiersRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdl3whitesoldiersRange(0, 11, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdl3whitesoldiersRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdl3whitesoldiersRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdl3whitesoldiersRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range of the constructed series matches the full series.
	o, h, l, c := cdl3whitesoldiersSeries(cdl3whitesoldiersSoldiers...)
	full, _ := CDL3WHITESOLDIERS(o, h, l, c)
	beg, nb, err = cdl3whitesoldiersRange(5, 12, o, h, l, c, out)
	if err != nil || beg != 12 || nb != 1 || out[0] != 100 {
		t.Fatalf("sub-range: beg=%d nb=%d out0=%d err=%v", beg, nb, out[0], err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
