package ta

import (
	"errors"
	"math"
	"testing"
)

func TestCDL3OUTSIDEParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDL3OUTSIDE(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3OUTSIDE(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3OUTSIDE(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDL3OUTSIDE(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDL3OUTSIDE(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDL3OUTSIDE([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDL3OUTSIDELookback(); lb != 3 {
		t.Errorf("lookback = %d, want 3", lb)
	}
}

func TestCDL3OUTSIDELookbackSettings(t *testing.T) {
	// The pattern uses no candle settings: the lookback never changes.
	defer RestoreAllCandleDefaultSettings()
	for st := CandleSettingType(0); int(st) < candleSettingCount; st++ {
		d := GetCandleSetting(st)
		if err := SetCandleSettings(st, d.RangeType, 40, d.Factor); err != nil {
			t.Fatal(err)
		}
	}
	if lb := CDL3OUTSIDELookback(); lb != 3 {
		t.Errorf("lookback with long settings = %d, want 3", lb)
	}
}

func TestCDL3OUTSIDELengths(t *testing.T) {
	lb := CDL3OUTSIDELookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDL3OUTSIDE(s.O, s.H, s.L, s.C)
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
					t.Fatalf("%s n=%d: [%d]=%d, want 0 or +-100", s.Name, n, i, v)
				}
			}
		}
	}
}

func TestCDL3OUTSIDEKnownValues(t *testing.T) {
	type bar struct{ o, c float64 }
	// The pattern only reads open and close; high/low are max/min of them.
	run := func(bars ...bar) []int {
		var o, h, l, c []float64
		o = append(o, 50)
		h = append(h, 50)
		l = append(l, 50)
		c = append(c, 50)
		for _, b := range bars {
			o = append(o, b.o)
			c = append(c, b.c)
			h = append(h, math.Max(b.o, b.c)+1)
			l = append(l, math.Min(b.o, b.c)-1)
		}
		out, err := CDL3OUTSIDE(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	cases := []struct {
		name string
		bars [3]bar
		want int
	}{
		{"up", [3]bar{{10, 9}, {8.5, 10.5}, {10.5, 11}}, 100},
		{"down", [3]bar{{9, 10}, {10.5, 8.5}, {8.5, 8}}, -100},
		{"up third equal close", [3]bar{{10, 9}, {8.5, 10.5}, {10.5, 10.5}}, 0}, // close not > 10.5
		{"down third equal close", [3]bar{{9, 10}, {10.5, 8.5}, {8.5, 8.5}}, 0},
		{"up third lower", [3]bar{{10, 9}, {8.5, 10.5}, {10.5, 10}}, 0},
		{"up close equals prior open", [3]bar{{10, 9}, {8.5, 10}, {10, 11}}, 0},   // c1 not > o0
		{"up open equals prior close", [3]bar{{10, 9}, {9, 10.5}, {10.5, 11}}, 0}, // o1 not < c0
		{"down open equals prior close", [3]bar{{9, 10}, {10, 8.5}, {8.5, 8}}, 0},
		{"down close equals prior open", [3]bar{{9, 10}, {10.5, 9}, {9, 8}}, 0},
		{"first doji is white", [3]bar{{10, 10}, {9.5, 10.5}, {10.5, 11}}, 0}, // doji counts as white
		{"down after white doji", [3]bar{{10, 10}, {10.5, 9.5}, {9.5, 9}}, -100},
		{"second doji white not engulfing", [3]bar{{10, 9}, {9.5, 9.5}, {9.5, 12}}, 0},
		{"same colours", [3]bar{{9, 10}, {8.5, 10.5}, {10.5, 11}}, 0},
	}
	for _, tc := range cases {
		out := run(tc.bars[0], tc.bars[1], tc.bars[2])
		want := []int{0, 0, 0, tc.want}
		assertInt(t, "CDL3OUTSIDE "+tc.name, out, want)
	}

	// Constant/zero series: every candle white with equal prices, never fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDL3OUTSIDE(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDL3OUTSIDERangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdl3outsideRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdl3outsideRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdl3outsideRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	beg, nb, err := cdl3outsideRange(0, 2, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if _, _, err := cdl3outsideRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdl3outsideRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	if _, nb, err := cdl3outsideRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	if beg, nb, err := cdl3outsideRange(3, 3, s.O, s.H, s.L, s.C, out); err != nil || beg != 3 || nb != 1 {
		t.Errorf("single bar at lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	full, _ := CDL3OUTSIDE(s.O, s.H, s.L, s.C)
	beg, nb, err = cdl3outsideRange(1, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 3 || nb != 23 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
