package ta

import (
	"errors"
	"testing"
)

// cdlstalledpatternBase returns n identical white background bars: open 100,
// close 101, high 101.5, low 99.5 (real body 1, high-low 2).
func cdlstalledpatternBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 101.5)
		l = append(l, 99.5)
		c = append(c, 101)
	}
	return
}

type cdlstalledpatternBar struct{ o, h, l, c float64 }

// cdlstalledpatternSeries appends three pattern bars to 12 background bars
// (lookback 12 with default settings), so the answer is at index 14.
func cdlstalledpatternSeries(bars [3]cdlstalledpatternBar) (o, h, l, c []float64) {
	o, h, l, c = cdlstalledpatternBase(12)
	for _, b := range bars {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

// cdlstalledpatternCases are hand-checked patterns (see KnownValues for the
// derivation of the thresholds).
var cdlstalledpatternCases = []struct {
	name    string
	a, b, c cdlstalledpatternBar
	want    int
}{
	{"stalled", cdlstalledpatternBar{100, 104.2, 99.8, 104}, cdlstalledpatternBar{103, 106.1, 102.9, 106}, cdlstalledpatternBar{105.8, 106.6, 105.7, 106.5}, -100},
	// 3rd body 2 not < BodyShort avg 1.5.
	{"3rd body not small", cdlstalledpatternBar{100, 104.2, 99.8, 104}, cdlstalledpatternBar{103, 106.1, 102.9, 106}, cdlstalledpatternBar{105, 107.1, 104.9, 107}, 0},
	// 2nd upper shadow 0.3 not < ShadowVeryShort avg 0.224.
	{"2nd upper shadow", cdlstalledpatternBar{100, 104.2, 99.8, 104}, cdlstalledpatternBar{103, 106.3, 102.9, 106}, cdlstalledpatternBar{105.8, 106.6, 105.7, 106.5}, 0},
	// 2nd opens at 104.3 <= close1 + Near[2] = 104.4: fires.
	{"2nd opens near", cdlstalledpatternBar{100, 104.2, 99.8, 104}, cdlstalledpatternBar{104.3, 107.5, 104.2, 107.4}, cdlstalledpatternBar{107, 107.7, 106.9, 107.6}, -100},
	// 2nd opens at 104.5 > 104.4.
	{"2nd opens too high", cdlstalledpatternBar{100, 104.2, 99.8, 104}, cdlstalledpatternBar{104.5, 107.7, 104.4, 107.6}, cdlstalledpatternBar{107, 107.9, 106.9, 107.8}, 0},
	// 2nd opens at the 1st open (o1 > o2 is strict).
	{"2nd opens at 1st open", cdlstalledpatternBar{100, 104.2, 99.8, 104}, cdlstalledpatternBar{100, 106.1, 99.9, 106}, cdlstalledpatternBar{105.8, 106.6, 105.7, 106.5}, 0},
	// 2nd body 1.2 not > BodyLong[1] avg 1.3.
	{"2nd body not long", cdlstalledpatternBar{100, 104.2, 99.8, 104}, cdlstalledpatternBar{103, 104.3, 102.9, 104.2}, cdlstalledpatternBar{104, 104.6, 103.9, 104.5}, 0},
	// 1st body 0.9 not > 1.
	{"1st body not long", cdlstalledpatternBar{103.1, 104.2, 103, 104}, cdlstalledpatternBar{103.5, 106.1, 103.4, 106}, cdlstalledpatternBar{105.8, 106.6, 105.7, 106.5}, 0},
	// 3rd black.
	{"3rd black", cdlstalledpatternBar{100, 104.2, 99.8, 104}, cdlstalledpatternBar{103, 106.1, 102.9, 106}, cdlstalledpatternBar{106.5, 106.6, 105.7, 106.2}, 0},
	// 3rd close not higher than 2nd close.
	{"close not higher", cdlstalledpatternBar{100, 104.2, 99.8, 104}, cdlstalledpatternBar{103, 106.1, 102.9, 106}, cdlstalledpatternBar{105.5, 106.1, 105.4, 106}, 0},
	// 2nd close not higher than 1st close.
	{"2nd close not higher", cdlstalledpatternBar{100, 104.2, 99.8, 104}, cdlstalledpatternBar{101, 104.05, 100.9, 104}, cdlstalledpatternBar{103.8, 104.6, 103.7, 104.5}, 0},
	// 2nd black.
	{"2nd black", cdlstalledpatternBar{100, 104.2, 99.8, 104}, cdlstalledpatternBar{106, 106.1, 102.9, 103}, cdlstalledpatternBar{105.8, 106.6, 105.7, 106.5}, 0},
}

func TestCDLSTALLEDPATTERNParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLSTALLEDPATTERN(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLSTALLEDPATTERN(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLSTALLEDPATTERN(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLSTALLEDPATTERN(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLSTALLEDPATTERN(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLSTALLEDPATTERN([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLSTALLEDPATTERNLookback(); lb != 12 {
		t.Errorf("default lookback = %d, want 12", lb)
	}
}

func TestCDLSTALLEDPATTERNLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyLong, 20, 22},
		{BodyShort, 30, 32},
		{ShadowVeryShort, 15, 17},
		{Near, 40, 42},
		{BodyLong, 0, 12},     // BodyShort still 10
		{ShadowShort, 50, 12}, // not used
		{Far, 50, 12},         // not used
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLSTALLEDPATTERNLookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Every average period 0: only the two prior candles are needed.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, BodyShort, ShadowVeryShort, Near} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLSTALLEDPATTERNLookback(); lb != 2 {
		t.Errorf("all periods 0: lookback %d, want 2", lb)
	}
	for _, n := range []int{1, 2, 3} {
		o, h, l, c := cdlstalledpatternBase(n)
		out, err := CDLSTALLEDPATTERN(o, h, l, c)
		if err != nil || len(out) != n {
			t.Errorf("n=%d with lookback 2: out=%v err=%v", n, out, err)
		}
	}
}

func TestCDLSTALLEDPATTERNLengths(t *testing.T) {
	lb := CDLSTALLEDPATTERNLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLSTALLEDPATTERN(s.O, s.H, s.L, s.C)
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

func TestCDLSTALLEDPATTERNKnownValues(t *testing.T) {
	// Background bars (0..11): body 1, high-low 2. With default settings and
	// 1st bar = 100/104.2/99.8/104 (body 4, range 4.4), for i=14:
	//   BodyLong[2] (bars 2..11)         = 1     -> 1st body must be > 1
	//   BodyLong[1] (bars 3..12)         = 13/10 = 1.3
	//   ShadowVeryShort (bars 3..12)     = 0.1*(18+4.4)/10 = 0.224
	//   Near[2] (bars 7..11)             = 0.2*10/5 = 0.4
	//   Near[1] (bars 8..12)             = 0.2*12.4/5 = 0.496
	//   BodyShort (bars 4..13)           = (8+4+body13)/10
	// The "rides on the shoulder" test o0 >= c1 - |c0-o0| - Near[1] reduces to
	// c0 >= c1 - Near[1] for a white 3rd candle, so it is implied by c0 > c1.
	for _, tc := range cdlstalledpatternCases {
		o, h, l, c := cdlstalledpatternSeries([3]cdlstalledpatternBar{tc.a, tc.b, tc.c})
		out, err := CDLSTALLEDPATTERN(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 15)
		want[14] = tc.want
		assertInt(t, "CDLSTALLEDPATTERN "+tc.name, out, want)
	}

	// Constant, all-zero or falling series: closes are not rising, never a pattern.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" && s.Name != "down" {
			continue
		}
		out, _ := CDLSTALLEDPATTERN(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLSTALLEDPATTERNRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlstalledpatternRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlstalledpatternRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlstalledpatternRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlstalledpatternRange(0, 11, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlstalledpatternRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlstalledpatternRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlstalledpatternRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Single bar at the lookback.
	if beg, nb, err := cdlstalledpatternRange(12, 12, s.O, s.H, s.L, s.C, out); err != nil || beg != 12 || nb != 1 {
		t.Errorf("single bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLSTALLEDPATTERN(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlstalledpatternRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 12 || nb != 14 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
