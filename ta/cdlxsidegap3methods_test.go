package ta

import (
	"errors"
	"math"
	"testing"
)

func TestCDLXSIDEGAP3METHODSParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLXSIDEGAP3METHODS(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLXSIDEGAP3METHODS(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLXSIDEGAP3METHODS(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLXSIDEGAP3METHODS(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLXSIDEGAP3METHODS(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLXSIDEGAP3METHODS([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLXSIDEGAP3METHODSLookback(); lb != 2 {
		t.Errorf("lookback = %d, want 2", lb)
	}
}

func TestCDLXSIDEGAP3METHODSLookbackSettings(t *testing.T) {
	// The pattern uses no candle settings: the lookback never changes.
	defer RestoreAllCandleDefaultSettings()
	for st := CandleSettingType(0); int(st) < candleSettingCount; st++ {
		d := GetCandleSetting(st)
		if err := SetCandleSettings(st, d.RangeType, 40, d.Factor); err != nil {
			t.Fatal(err)
		}
	}
	if lb := CDLXSIDEGAP3METHODSLookback(); lb != 2 {
		t.Errorf("lookback with long settings = %d, want 2", lb)
	}
}

func TestCDLXSIDEGAP3METHODSLengths(t *testing.T) {
	lb := CDLXSIDEGAP3METHODSLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLXSIDEGAP3METHODS(s.O, s.H, s.L, s.C)
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

func TestCDLXSIDEGAP3METHODSKnownValues(t *testing.T) {
	type bar struct{ o, c float64 }
	// The pattern only reads open and close; high/low are max/min of them +-1.
	run := func(bars ...bar) []int {
		var o, h, l, c []float64
		for _, b := range bars {
			o = append(o, b.o)
			c = append(c, b.c)
			h = append(h, math.Max(b.o, b.c)+1)
			l = append(l, math.Min(b.o, b.c)-1)
		}
		out, err := CDLXSIDEGAP3METHODS(o, h, l, c)
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
		// Upside: white 10->12, white 13->15 (body gap 12 < 13), black 14->11.
		{"up", [3]bar{{10, 12}, {13, 15}, {14, 11}}, 100},
		// Downside: black 15->13, black 12->10 (body gap 12 < 13), white 11->14.
		{"down", [3]bar{{15, 13}, {12, 10}, {11, 14}}, -100},
		{"up no gap (touching bodies)", [3]bar{{10, 12}, {12, 15}, {14, 11}}, 0},
		{"down no gap (touching bodies)", [3]bar{{15, 13}, {13, 10}, {11, 14}}, 0},
		{"up third opens at 2nd top", [3]bar{{10, 12}, {13, 15}, {15, 11}}, 0},
		{"up third opens at 2nd bottom", [3]bar{{10, 12}, {13, 15}, {13, 11}}, 0},
		{"up third closes at 1st top", [3]bar{{10, 12}, {13, 15}, {14, 12}}, 0},
		{"up third closes at 1st bottom", [3]bar{{10, 12}, {13, 15}, {14, 10}}, 0},
		{"up third closes inside gap", [3]bar{{10, 12}, {13, 15}, {14, 12.5}}, 0},
		{"down third opens at 2nd bottom", [3]bar{{15, 13}, {12, 10}, {10, 14}}, 0},
		{"down third closes at 1st top", [3]bar{{15, 13}, {12, 10}, {11, 15}}, 0},
		{"up third same colour", [3]bar{{10, 12}, {13, 15}, {14, 14.5}}, 0},
		{"first two differ in colour", [3]bar{{12, 10}, {13, 15}, {14, 11}}, 0},
		{"up first doji (white, empty body)", [3]bar{{12, 12}, {13, 15}, {14, 11}}, 0},
		// A doji third candle counts as white, so it is "opposite" only after black.
		{"down third doji", [3]bar{{15, 13}, {12, 10}, {11, 11}}, 0},
		{"down with wide 1st body", [3]bar{{20, 13}, {12, 10}, {11, 14}}, -100},
	}
	for _, tc := range cases {
		out := run(tc.bars[0], tc.bars[1], tc.bars[2])
		want := []int{0, 0, tc.want}
		assertInt(t, "CDLXSIDEGAP3METHODS "+tc.name, out, want)
	}

	// Constant/zero series: every candle white with equal prices, never fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLXSIDEGAP3METHODS(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLXSIDEGAP3METHODSRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlxsidegap3methodsRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlxsidegap3methodsRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlxsidegap3methodsRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	beg, nb, err := cdlxsidegap3methodsRange(0, 1, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if _, _, err := cdlxsidegap3methodsRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlxsidegap3methodsRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	if _, nb, err := cdlxsidegap3methodsRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	if beg, nb, err := cdlxsidegap3methodsRange(2, 2, s.O, s.H, s.L, s.C, out); err != nil || beg != 2 || nb != 1 {
		t.Errorf("single bar at lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	full, _ := CDLXSIDEGAP3METHODS(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlxsidegap3methodsRange(1, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 2 || nb != 24 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
