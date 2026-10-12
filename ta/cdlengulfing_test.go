package ta

import (
	"errors"
	"math"
	"testing"
)

func TestCDLENGULFINGParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLENGULFING(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLENGULFING(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLENGULFING(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLENGULFING(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLENGULFING(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLENGULFING([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLENGULFINGLookback(); lb != 2 {
		t.Errorf("lookback = %d, want 2", lb)
	}
}

func TestCDLENGULFINGLookbackSettings(t *testing.T) {
	// The pattern uses no candle settings: the lookback never changes.
	defer RestoreAllCandleDefaultSettings()
	for st := CandleSettingType(0); int(st) < candleSettingCount; st++ {
		d := GetCandleSetting(st)
		if err := SetCandleSettings(st, d.RangeType, 40, d.Factor); err != nil {
			t.Fatal(err)
		}
	}
	if lb := CDLENGULFINGLookback(); lb != 2 {
		t.Errorf("lookback with long settings = %d, want 2", lb)
	}
}

func TestCDLENGULFINGLengths(t *testing.T) {
	lb := CDLENGULFINGLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLENGULFING(s.O, s.H, s.L, s.C)
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
				case 0, 80, -80, 100, -100:
				default:
					t.Fatalf("%s n=%d: [%d]=%d, want 0, +-80 or +-100", s.Name, n, i, v)
				}
			}
		}
	}
}

func TestCDLENGULFINGKnownValues(t *testing.T) {
	type bar struct{ o, c float64 }
	// The pattern only reads open and close; high/low are max/min of them.
	// Bar 0 is a filler (lookback is 2), bars 1 and 2 form the pattern.
	run := func(bars ...bar) []int {
		o := []float64{50}
		h := []float64{50}
		l := []float64{50}
		c := []float64{50}
		for _, b := range bars {
			o = append(o, b.o)
			c = append(c, b.c)
			h = append(h, math.Max(b.o, b.c)+1)
			l = append(l, math.Min(b.o, b.c)-1)
		}
		out, err := CDLENGULFING(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	cases := []struct {
		name string
		bars [2]bar
		want int
	}{
		{"bullish strict", [2]bar{{10, 9}, {8.5, 10.5}}, 100},
		{"bearish strict", [2]bar{{9, 10}, {10.5, 8.5}}, -100},
		{"bullish open equals prior close", [2]bar{{10, 9}, {9, 10.5}}, 80},
		{"bullish close equals prior open", [2]bar{{10, 9}, {8.5, 10}}, 80},
		{"bullish both ends equal", [2]bar{{10, 9}, {9, 10}}, 0},
		{"bearish open equals prior close", [2]bar{{9, 10}, {10, 8.5}}, -80},
		{"bearish close equals prior open", [2]bar{{9, 10}, {10.5, 9}}, -80},
		{"bearish both ends equal", [2]bar{{9, 10}, {10, 9}}, 0},
		{"bullish not engulfing top", [2]bar{{10, 9}, {8.5, 9.5}}, 0},
		{"bullish not engulfing bottom", [2]bar{{10, 9}, {9.5, 10.5}}, 0},
		{"bearish not engulfing", [2]bar{{9, 10}, {9.5, 8.5}}, 0},
		{"same colours white", [2]bar{{9, 10}, {8.5, 10.5}}, 0},
		{"same colours black", [2]bar{{10, 9}, {10.5, 8.5}}, 0},
		{"doji first counts white", [2]bar{{10, 10}, {10.5, 9.5}}, -100},
		{"doji first, black touches open", [2]bar{{10, 10}, {10, 9.5}}, -80},
		{"doji second is white", [2]bar{{10, 9}, {9.5, 9.5}}, 0},
		{"doji second white on prior close", [2]bar{{10, 9}, {9, 9}}, 0},
	}
	for _, tc := range cases {
		out := run(tc.bars[0], tc.bars[1])
		assertInt(t, "CDLENGULFING "+tc.name, out, []int{0, 0, tc.want})
	}

	// Lookback is 2: an engulfing at bar 1 is not reported.
	out, _ := CDLENGULFING([]float64{10, 8.5, 9}, []float64{11, 11, 11}, []float64{8, 8, 8}, []float64{9, 10.5, 9.5})
	assertInt(t, "engulfing at bar 1", out, []int{0, 0, 0})

	// Constant/zero series: every candle white with equal prices, never fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLENGULFING(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLENGULFINGRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlengulfingRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlengulfingRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlengulfingRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	beg, nb, err := cdlengulfingRange(0, 1, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if _, _, err := cdlengulfingRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlengulfingRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	if _, nb, err := cdlengulfingRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	if beg, nb, err := cdlengulfingRange(2, 2, s.O, s.H, s.L, s.C, out); err != nil || beg != 2 || nb != 1 {
		t.Errorf("single bar at lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	full, _ := CDLENGULFING(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlengulfingRange(1, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 2 || nb != 24 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
