package ta

import (
	"errors"
	"testing"
)

func TestCDLHIKKAKEParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLHIKKAKE(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLHIKKAKE(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLHIKKAKE(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLHIKKAKE(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLHIKKAKE(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLHIKKAKE([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLHIKKAKELookback(); lb != 5 {
		t.Errorf("lookback = %d, want 5", lb)
	}
}

func TestCDLHIKKAKELookbackSettings(t *testing.T) {
	// The pattern uses no candle settings: the lookback never changes.
	defer RestoreAllCandleDefaultSettings()
	for st := CandleSettingType(0); int(st) < candleSettingCount; st++ {
		d := GetCandleSetting(st)
		if err := SetCandleSettings(st, d.RangeType, 40, d.Factor); err != nil {
			t.Fatal(err)
		}
	}
	if lb := CDLHIKKAKELookback(); lb != 5 {
		t.Errorf("lookback with long settings = %d, want 5", lb)
	}
}

func TestCDLHIKKAKELengths(t *testing.T) {
	lb := CDLHIKKAKELookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLHIKKAKE(s.O, s.H, s.L, s.C)
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
				case 0, 100, -100, 200, -200:
				default:
					t.Fatalf("%s n=%d: [%d]=%d, want 0, +-100 or +-200", s.Name, n, i, v)
				}
			}
		}
	}
}

// cdlhikkakeBars builds OHLC from (high, low, close) triples; open is the
// close (the pattern never reads open).
func cdlhikkakeBars(hlc ...[3]float64) (o, h, l, c []float64) {
	for _, b := range hlc {
		h = append(h, b[0])
		l = append(l, b[1])
		c = append(c, b[2])
		o = append(o, b[2])
	}
	return
}

func TestCDLHIKKAKEKnownValues(t *testing.T) {
	filler := [3]float64{20, 0, 10}
	mother := [3]float64{15, 5, 10}
	inside := [3]float64{14, 6, 10} // lower high, higher low than mother
	bull := [3]float64{13, 4, 8}    // lower high, lower low than inside
	bear := [3]float64{16, 7, 12}   // higher high, higher low than inside
	quiet := [3]float64{13, 5, 10}  // no new pattern, close inside [6, 14]
	up := [3]float64{16, 9, 15}     // close 15 > inside high 14
	down := [3]float64{10, 3, 4}    // close 4 < inside low 6

	cases := []struct {
		name string
		bars [][3]float64
		want []int
	}{
		{"bull confirmed", [][3]float64{filler, filler, filler, mother, inside, bull, quiet, up, up}, []int{0, 0, 0, 0, 0, 100, 0, 200, 0}},
		{"bull confirmed on 3rd bar", [][3]float64{filler, filler, filler, mother, inside, bull, quiet, quiet, up}, []int{0, 0, 0, 0, 0, 100, 0, 0, 200}},
		{"bull window expired", [][3]float64{filler, filler, filler, mother, inside, bull, quiet, quiet, quiet, up}, []int{0, 0, 0, 0, 0, 100, 0, 0, 0, 0}},
		{"bull confirmed once only", [][3]float64{filler, filler, filler, mother, inside, bull, up, up, up}, []int{0, 0, 0, 0, 0, 100, 200, 0, 0}},
		{"bull never confirmed by down", [][3]float64{filler, filler, filler, mother, inside, bull, down, quiet, quiet}, []int{0, 0, 0, 0, 0, 100, 0, 0, 0}},
		{"bear confirmed", [][3]float64{filler, filler, filler, mother, inside, bear, down}, []int{0, 0, 0, 0, 0, -100, -200}},
		{"bear not confirmed by up", [][3]float64{filler, filler, filler, mother, inside, bear, up, up, up}, []int{0, 0, 0, 0, 0, -100, 0, 0, 0}},
		// Hikkake at bar 4 lies in the lookback but is found by the warm-up
		// loop, so its confirmation at bar 5 is reported.
		{"warm-up hikkake confirmed", [][3]float64{filler, filler, mother, inside, bull, up}, []int{0, 0, 0, 0, 0, 200}},
		// Equal highs are not an inside bar.
		{"equal highs", [][3]float64{filler, filler, filler, mother, {15, 6, 10}, bull}, []int{0, 0, 0, 0, 0, 0}},
		// Equal lows on the breakout bar are not a breakout.
		{"equal low breakout", [][3]float64{filler, filler, filler, mother, inside, {13, 6, 8}}, []int{0, 0, 0, 0, 0, 0}},
	}
	for _, tc := range cases {
		o, h, l, c := cdlhikkakeBars(tc.bars...)
		out, err := CDLHIKKAKE(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		assertInt(t, "CDLHIKKAKE "+tc.name, out, tc.want)
	}

	// A new hikkake on what would be a confirmation bar replaces it: bar 6
	// is inside bar 5, and bar 7 breaks it upwards (a bearish hikkake) while
	// also closing above bar 4's high 14, which would confirm bar 5's bull.
	o, h, l, c := cdlhikkakeBars(filler, filler, filler, mother, inside, bull, [3]float64{12, 5, 10}, [3]float64{13, 6, 15})
	out, _ := CDLHIKKAKE(o, h, l, c)
	assertInt(t, "new hikkake overrides confirmation", out, []int{0, 0, 0, 0, 0, 100, 0, -100})

	// Constant/zero series never have an inside bar.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" && s.Name != "up" && s.Name != "down" {
			continue
		}
		out, _ := CDLHIKKAKE(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLHIKKAKERangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlhikkakeRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlhikkakeRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlhikkakeRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	beg, nb, err := cdlhikkakeRange(0, 4, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if _, _, err := cdlhikkakeRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlhikkakeRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	if _, nb, err := cdlhikkakeRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	if beg, nb, err := cdlhikkakeRange(5, 5, s.O, s.H, s.L, s.C, out); err != nil || beg != 5 || nb != 1 {
		t.Errorf("single bar at lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}

	// The warm-up carries a hikkake from before startIdx into the range.
	o, h, l, c := cdlhikkakeBars([3]float64{20, 0, 10}, [3]float64{20, 0, 10}, [3]float64{20, 0, 10},
		[3]float64{15, 5, 10}, [3]float64{14, 6, 10}, [3]float64{13, 4, 8}, [3]float64{13, 5, 10}, [3]float64{16, 9, 15})
	buf := make([]int, 8)
	beg, nb, err = cdlhikkakeRange(7, 7, o, h, l, c, buf)
	if err != nil || beg != 7 || nb != 1 || buf[0] != 200 {
		t.Errorf("warm-up confirmation: beg=%d nb=%d err=%v out=%d", beg, nb, err, buf[0])
	}
}
