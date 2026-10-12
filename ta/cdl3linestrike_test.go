package ta

import (
	"errors"
	"testing"
)

// cdl3linestrikeBar is one OHLC bar.
type cdl3linestrikeBar struct{ o, h, l, c float64 }

// cdl3linestrikeSeries returns 5 background bars (open 100, close 102,
// high 103, low 99: high-low 4) followed by the given bars, as OHLC slices.
// With the default Near setting (HighLow, 5, 0.2) and bars whose high-low is
// 4, the Near average of each candle is 0.2*4 = 0.8.
func cdl3linestrikeSeries(bars ...cdl3linestrikeBar) (o, h, l, c []float64) {
	all := []cdl3linestrikeBar{}
	for range 5 {
		all = append(all, cdl3linestrikeBar{100, 103, 99, 102})
	}
	all = append(all, bars...)
	for _, b := range all {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

// cdl3linestrikeWhite returns three rising white candles (high-low 4 each)
// that form the first part of a bullish three-line strike.
func cdl3linestrikeWhite() []cdl3linestrikeBar {
	return []cdl3linestrikeBar{
		{100, 103, 99, 102},
		{101, 104, 100, 103},
		{102, 105, 101, 104},
	}
}

// cdl3linestrikeBlack is the mirror: three falling black candles.
func cdl3linestrikeBlack() []cdl3linestrikeBar {
	return []cdl3linestrikeBar{
		{102, 103, 99, 100},
		{101, 102, 98, 99},
		{100, 101, 97, 98},
	}
}

func TestCDL3LINESTRIKEParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDL3LINESTRIKE(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3LINESTRIKE(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3LINESTRIKE(s.O[:3], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3LINESTRIKE(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDL3LINESTRIKE(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDL3LINESTRIKE([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDL3LINESTRIKELookback(); lb != 8 {
		t.Errorf("default lookback = %d, want 8", lb)
	}
}

func TestCDL3LINESTRIKELookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, p := range []int{0, 1, 10, 40} {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(Near)
		if err := SetCandleSettings(Near, d.RangeType, p, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDL3LINESTRIKELookback(); lb != p+3 {
			t.Errorf("Near period %d: lookback %d, want %d", p, lb, p+3)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	d := GetCandleSetting(BodyLong)
	_ = SetCandleSettings(BodyLong, d.RangeType, 50, d.Factor)
	if lb := CDL3LINESTRIKELookback(); lb != 8 {
		t.Errorf("BodyLong period 50: lookback %d, want 8", lb)
	}
	// Near period 0: lookback 3, the 4-bar pattern can fire at index 3 and
	// the Near average is the candle's own range.
	RestoreAllCandleDefaultSettings()
	d = GetCandleSetting(Near)
	_ = SetCandleSettings(Near, d.RangeType, 0, d.Factor)
	bars := append(cdl3linestrikeWhite(), cdl3linestrikeBar{105, 106, 98, 99})
	var o, h, l, c []float64
	for _, b := range bars {
		o, h, l, c = append(o, b.o), append(h, b.h), append(l, b.l), append(c, b.c)
	}
	out, err := CDL3LINESTRIKE(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "near period 0", out, []int{0, 0, 0, 100})
	out, err = CDL3LINESTRIKE(o[:3], h[:3], l[:3], c[:3])
	if err != nil || len(out) != 3 || out[0] != 0 || out[1] != 0 || out[2] != 0 {
		t.Errorf("n=3 with lookback 3: out=%v err=%v", out, err)
	}
}

func TestCDL3LINESTRIKELengths(t *testing.T) {
	lb := CDL3LINESTRIKELookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDL3LINESTRIKE(s.O, s.H, s.L, s.C)
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

func TestCDL3LINESTRIKEKnownValues(t *testing.T) {
	type tc struct {
		name  string
		three []cdl3linestrikeBar
		edit  func(b []cdl3linestrikeBar) // modifies the three bars in place
		last  cdl3linestrikeBar
		want  int
	}
	none := func([]cdl3linestrikeBar) {}
	cases := []tc{
		// Three white, rising closes, 4th black opens above 104, closes below 100.
		{"bullish", cdl3linestrikeWhite(), none, cdl3linestrikeBar{105, 106, 98, 99}, 100},
		// Three black, falling closes, 4th white opens below 98, closes above 102.
		{"bearish", cdl3linestrikeBlack(), none, cdl3linestrikeBar{97, 104, 96, 103}, -100},
		{"4th same color", cdl3linestrikeWhite(), none, cdl3linestrikeBar{105, 107, 104, 106}, 0},
		{"4th opens at prior close", cdl3linestrikeWhite(), none, cdl3linestrikeBar{104, 105, 98, 99}, 0},
		{"4th closes at first open", cdl3linestrikeWhite(), none, cdl3linestrikeBar{105, 106, 99, 100}, 0},
		{"4th closes above first open", cdl3linestrikeWhite(), none, cdl3linestrikeBar{105, 106, 100, 100.5}, 0},
		{"bear 4th opens at prior close", cdl3linestrikeBlack(), none, cdl3linestrikeBar{98, 104, 97, 103}, 0},
		{"bear 4th closes at first open", cdl3linestrikeBlack(), none, cdl3linestrikeBar{97, 104, 96, 102}, 0},
		{"mixed colors", cdl3linestrikeWhite(), func(b []cdl3linestrikeBar) {
			b[1] = cdl3linestrikeBar{103.5, 104, 102, 103} // black, opens within near of 1st body
		}, cdl3linestrikeBar{105, 106, 98, 99}, 0},
		{"closes not rising", cdl3linestrikeWhite(), func(b []cdl3linestrikeBar) {
			b[2] = cdl3linestrikeBar{102, 104, 101, 103} // close 103 == prior close
		}, cdl3linestrikeBar{105, 106, 98, 99}, 0},
		// 2nd opens at 102.5: above 1st body top 102 but within near (0.8).
		{"2nd opens near above", cdl3linestrikeWhite(), func(b []cdl3linestrikeBar) {
			b[1] = cdl3linestrikeBar{102.5, 104, 100.5, 103}
		}, cdl3linestrikeBar{105, 106, 98, 99}, 100},
		// 2nd opens at 103.5: beyond 102 + 0.8.
		{"2nd opens too high", cdl3linestrikeWhite(), func(b []cdl3linestrikeBar) {
			b[1] = cdl3linestrikeBar{103.5, 104.5, 100.5, 104}
			b[2] = cdl3linestrikeBar{103.5, 106, 101.5, 105}
		}, cdl3linestrikeBar{106, 107, 98, 99}, 0},
		// 3rd opens at 99.5: below 2nd body bottom 101 by more than 0.8.
		{"3rd opens too low", cdl3linestrikeWhite(), func(b []cdl3linestrikeBar) {
			b[2] = cdl3linestrikeBar{99.5, 105, 99, 104}
		}, cdl3linestrikeBar{105, 106, 98, 99}, 0},
		// 3rd opens at 100.5: below 101 but within 0.8.
		{"3rd opens near below", cdl3linestrikeWhite(), func(b []cdl3linestrikeBar) {
			b[2] = cdl3linestrikeBar{100.5, 105, 100, 104}
		}, cdl3linestrikeBar{105, 106, 98, 99}, 100},
	}
	for _, k := range cases {
		three := k.three
		k.edit(three)
		o, h, l, c := cdl3linestrikeSeries(append(three, k.last)...)
		out, err := CDL3LINESTRIKE(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 9)
		want[8] = k.want
		assertInt(t, "CDL3LINESTRIKE "+k.name, out, want)
	}

	// A constant series is all white (close >= open), so the 4th candle is
	// never of the opposite color: never fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" && s.Name != "up" {
			continue
		}
		out, _ := CDL3LINESTRIKE(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDL3LINESTRIKERangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdl3linestrikeRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdl3linestrikeRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdl3linestrikeRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdl3linestrikeRange(0, 7, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdl3linestrikeRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdl3linestrikeRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdl3linestrikeRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Single bar at the lookback boundary.
	if beg, nb, err := cdl3linestrikeRange(8, 8, s.O, s.H, s.L, s.C, out); err != nil || beg != 8 || nb != 1 {
		t.Errorf("single bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDL3LINESTRIKE(s.O, s.H, s.L, s.C)
	beg, nb, err = cdl3linestrikeRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 8 || nb != 18 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
