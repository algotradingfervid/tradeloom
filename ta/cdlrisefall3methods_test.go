package ta

import (
	"errors"
	"testing"
)

// cdlrisefall3methodsBar is one OHLC bar.
type cdlrisefall3methodsBar struct{ o, h, l, c float64 }

// cdlrisefall3methodsBackground is the number of filler bars before a pattern.
const cdlrisefall3methodsBackground = 12

// cdlrisefall3methodsSeries returns 12 background bars (open 100, close 101,
// high 102, low 99: real body 1) followed by the given bars, as OHLC slices.
// With the default BodyLong and BodyShort settings (RealBody, 10, 1.0) the
// first pattern candle is compared with an average body of 1.
func cdlrisefall3methodsSeries(bars ...cdlrisefall3methodsBar) (o, h, l, c []float64) {
	all := []cdlrisefall3methodsBar{}
	for range cdlrisefall3methodsBackground {
		all = append(all, cdlrisefall3methodsBar{100, 102, 99, 101})
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

// cdlrisefall3methodsRising returns a rising three methods: a long white
// candle (body 4), three small black candles (body 0.5) with falling closes
// inside its range, and a long white candle (body 3.5) opening above the
// prior close and closing above the first close.
//
// Averages with the default settings and the 12-bar background: BodyShort
// for candles 2..4 is (9+4)/10 = 1.3, (8+4+0.5)/10 = 1.25, (7+4+1)/10 = 1.2;
// BodyLong for candle 5 is (6+4+1.5)/10 = 1.15.
func cdlrisefall3methodsRising() []cdlrisefall3methodsBar {
	return []cdlrisefall3methodsBar{
		{100, 104.5, 99.5, 104},
		{103.5, 104, 102.5, 103},
		{103, 103.5, 102, 102.5},
		{102.5, 103, 101.5, 102},
		{102.5, 106.5, 102, 106},
	}
}

// cdlrisefall3methodsMirror reflects bars around 102 (p -> 204-p), turning a
// rising pattern into a falling one with identical body sizes.
func cdlrisefall3methodsMirror(bars []cdlrisefall3methodsBar) []cdlrisefall3methodsBar {
	out := make([]cdlrisefall3methodsBar, len(bars))
	for k, b := range bars {
		out[k] = cdlrisefall3methodsBar{204 - b.o, 204 - b.l, 204 - b.h, 204 - b.c}
	}
	return out
}

func TestCDLRISEFALL3METHODSParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLRISEFALL3METHODS(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLRISEFALL3METHODS(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLRISEFALL3METHODS(s.O[:3], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLRISEFALL3METHODS(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLRISEFALL3METHODS(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLRISEFALL3METHODS([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLRISEFALL3METHODSLookback(); lb != 14 {
		t.Errorf("default lookback = %d, want 14", lb)
	}
}

func TestCDLRISEFALL3METHODSLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	set := func(st CandleSettingType, p int) {
		d := GetCandleSetting(st)
		if err := SetCandleSettings(st, d.RangeType, p, d.Factor); err != nil {
			t.Fatal(err)
		}
	}
	for _, pp := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {3, 20}, {20, 3}, {40, 40}} {
		RestoreAllCandleDefaultSettings()
		set(BodyLong, pp[0])
		set(BodyShort, pp[1])
		want := max(pp[0], pp[1]) + 4
		if lb := CDLRISEFALL3METHODSLookback(); lb != want {
			t.Errorf("BodyLong %d BodyShort %d: lookback %d, want %d", pp[0], pp[1], lb, want)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	set(Near, 50)
	if lb := CDLRISEFALL3METHODSLookback(); lb != 14 {
		t.Errorf("Near period 50: lookback %d, want 14", lb)
	}

	// Both periods 0: lookback 4, the 5-bar pattern can fire at index 4.
	// Each body is compared with factor * its own body, so lower the BodyLong
	// factor and raise the BodyShort factor.
	RestoreAllCandleDefaultSettings()
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 0.5)
	_ = SetCandleSettings(BodyShort, RangeRealBody, 0, 2.0)
	bars := cdlrisefall3methodsRising()
	var o, h, l, c []float64
	for _, b := range bars {
		o, h, l, c = append(o, b.o), append(h, b.h), append(l, b.l), append(c, b.c)
	}
	out, err := CDLRISEFALL3METHODS(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "periods 0", out, []int{0, 0, 0, 0, 100})
	out, err = CDLRISEFALL3METHODS(o[:4], h[:4], l[:4], c[:4])
	if err != nil || len(out) != 4 {
		t.Fatalf("n=4 with lookback 4: out=%v err=%v", out, err)
	}
	assertInt(t, "n=4", out, []int{0, 0, 0, 0})
	// Factor 1 with period 0: a body is never strictly longer than itself.
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 1.0)
	out, _ = CDLRISEFALL3METHODS(o, h, l, c)
	assertInt(t, "periods 0 factor 1", out, []int{0, 0, 0, 0, 0})
}

func TestCDLRISEFALL3METHODSLengths(t *testing.T) {
	lb := CDLRISEFALL3METHODSLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLRISEFALL3METHODS(s.O, s.H, s.L, s.C)
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

func TestCDLRISEFALL3METHODSKnownValues(t *testing.T) {
	type tc struct {
		name string
		edit func(b []cdlrisefall3methodsBar) // modifies the rising bars in place
		want int
		// asym marks a case whose mirror is not simply -want (a doji is
		// white, so reflecting it does not flip its color); mirrorWant
		// is then the expected mirrored output.
		asym       bool
		mirrorWant int
	}
	none := func([]cdlrisefall3methodsBar) {}
	cases := []tc{
		{"rising", none, 100, false, 0},
		{"5th closes at 1st close", func(b []cdlrisefall3methodsBar) { b[4].c = 104 }, 0, false, 0},
		{"5th closes just above 1st close", func(b []cdlrisefall3methodsBar) { b[4].c = 104.25 }, 100, false, 0},
		{"5th opens at prior close", func(b []cdlrisefall3methodsBar) { b[4].o = 102 }, 0, false, 0},
		{"5th black", func(b []cdlrisefall3methodsBar) { b[4] = cdlrisefall3methodsBar{106, 106.5, 102, 102.5} }, 0, false, 0},
		{"5th short", func(b []cdlrisefall3methodsBar) { b[4] = cdlrisefall3methodsBar{103.5, 104.5, 103, 104.5} }, 0, false, 0},
		{"1st not long", func(b []cdlrisefall3methodsBar) { b[0] = cdlrisefall3methodsBar{103, 104.5, 99.5, 104} }, 0, false, 0},
		{"2nd white", func(b []cdlrisefall3methodsBar) { b[1] = cdlrisefall3methodsBar{103, 104, 102.5, 103.5} }, 0, false, 0},
		{"3rd white", func(b []cdlrisefall3methodsBar) { b[2] = cdlrisefall3methodsBar{102.5, 103.5, 102, 102.75} }, 0, false, 0},
		{"4th white", func(b []cdlrisefall3methodsBar) { b[3] = cdlrisefall3methodsBar{101.5, 103, 101.5, 102} }, 0, false, 0},
		// Doji (open == close) counts as white: color mismatch in the rising
		// pattern, but the mirrored doji is still white, which is exactly
		// what the falling pattern needs, so the mirror fires.
		{"4th doji", func(b []cdlrisefall3methodsBar) { b[3] = cdlrisefall3methodsBar{102, 103, 101.5, 102} }, 0, true, -100},
		{"2nd body long", func(b []cdlrisefall3methodsBar) { b[1] = cdlrisefall3methodsBar{104.5, 104.5, 102.5, 103} }, 0, false, 0},
		{"3rd closes at 2nd close", func(b []cdlrisefall3methodsBar) { b[2] = cdlrisefall3methodsBar{103.5, 103.5, 102.5, 103} }, 0, false, 0},
		{"4th closes at 3rd close", func(b []cdlrisefall3methodsBar) { b[3] = cdlrisefall3methodsBar{103, 103, 102, 102.5} }, 0, false, 0},
		// 2nd body bottom at the 1st high: does not hold within.
		{"2nd above 1st high", func(b []cdlrisefall3methodsBar) { b[1] = cdlrisefall3methodsBar{105, 105.5, 104.5, 104.5} }, 0, false, 0},
		// 2nd body bottom just below the 1st high: holds within.
		{"2nd touches 1st range", func(b []cdlrisefall3methodsBar) { b[1] = cdlrisefall3methodsBar{105, 105.5, 104.25, 104.4} }, 100, false, 0},
	}
	for _, k := range cases {
		bars := cdlrisefall3methodsRising()
		k.edit(bars)
		for _, mirror := range []bool{false, true} {
			pb, want := bars, k.want
			if mirror {
				pb, want = cdlrisefall3methodsMirror(bars), -k.want
				if k.asym {
					want = k.mirrorWant
				}
			}
			o, h, l, c := cdlrisefall3methodsSeries(pb...)
			if mirror {
				// Mirror the background too so the whole series is reflected.
				for j := range cdlrisefall3methodsBackground {
					o[j], h[j], l[j], c[j] = 204-o[j], 204-l[j], 204-h[j], 204-c[j]
				}
			}
			out, err := CDLRISEFALL3METHODS(o, h, l, c)
			if err != nil {
				t.Fatal(err)
			}
			wantOut := make([]int, len(o))
			wantOut[len(o)-1] = want
			assertInt(t, lbl("CDLRISEFALL3METHODS ", k.name, " mirror=", mirror), out, wantOut)
		}
	}

	// Constant and steadily trending series never alternate colors.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" && s.Name != "up" && s.Name != "down" {
			continue
		}
		out, _ := CDLRISEFALL3METHODS(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLRISEFALL3METHODSRangeForm(t *testing.T) {
	s := fixtures(40)[0]
	out := make([]int, 40)
	if _, _, err := cdlrisefall3methodsRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlrisefall3methodsRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlrisefall3methodsRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlrisefall3methodsRange(0, 13, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlrisefall3methodsRange(0, 39, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlrisefall3methodsRange(0, 39, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlrisefall3methodsRange(30, 39, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Single bar at the lookback boundary.
	if beg, nb, err := cdlrisefall3methodsRange(14, 14, s.O, s.H, s.L, s.C, out); err != nil || beg != 14 || nb != 1 {
		t.Errorf("single bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLRISEFALL3METHODS(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlrisefall3methodsRange(5, 35, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 14 || nb != 22 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
