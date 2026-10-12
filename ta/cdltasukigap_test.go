package ta

import (
	"errors"
	"testing"
)

type cdltasukigapBar struct{ o, h, l, c float64 }

// cdltasukigapSeries appends three bars to 10 background bars (open 100,
// close 102, high 103, low 99: high-low range 4, body 2), so the third bar is
// at index 12.
func cdltasukigapSeries(bars [3]cdltasukigapBar) (o, h, l, c []float64) {
	for range 10 {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	for _, b := range bars {
		o = append(o, b.o)
		h = append(h, b.h)
		l = append(l, b.l)
		c = append(c, b.c)
	}
	return
}

// Canonical upside and downside tasuki gaps. The first candle has range 4
// like the background, so the Near average at bar 11 sees a high-low range
// of 4. Both gap and third bodies are 3.
var (
	cdltasukigapUp = [3]cdltasukigapBar{
		{100, 103, 99, 102},      // 1st: body [100,102]
		{104, 108, 103.5, 107},   // white, body [104,107] gaps above 102
		{106, 106.5, 102.5, 103}, // black, opens in [104,107], closes in (102,104)
	}
	cdltasukigapDown = [3]cdltasukigapBar{
		{110, 111, 107, 108},     // 1st: body [108,110]
		{106, 106.5, 102.5, 103}, // black, body [103,106] gaps below 108
		{104, 107.5, 103.5, 107}, // white, opens in [103,106], closes in (106,108)
	}
)

func cdltasukigapWith(base [3]cdltasukigapBar, k int, b cdltasukigapBar) [3]cdltasukigapBar {
	base[k] = b
	return base
}

// cdltasukigapExactSettings sets a dyadic factor so the Near band is exact:
// 0.25 * 4 = 1.
func cdltasukigapExactSettings(t *testing.T) {
	t.Helper()
	if err := SetCandleSettings(Near, RangeHighLow, 5, 0.25); err != nil {
		t.Fatal(err)
	}
}

type cdltasukigapCase struct {
	name string
	bars [3]cdltasukigapBar
	want int // under cdltasukigapExactSettings
}

// cdltasukigapExactCases are hand-checked against the rules in
// cdltasukigap.md / ta_CDLTASUKIGAP.c with a Near band of exactly 1
// (strict <): every strict comparison is probed at equality and just inside.
func cdltasukigapExactCases() []cdltasukigapCase {
	type B = cdltasukigapBar
	up, dn := cdltasukigapUp, cdltasukigapDown
	w := cdltasukigapWith
	return []cdltasukigapCase{
		{"up", up, 100},
		{"down", dn, -100},
		{"up, 1st black", w(up, 0, B{102, 103, 99, 100}), 100},
		{"down, 1st white", w(dn, 0, B{108, 111, 107, 110}), -100},
		// Near: |body2 - body3| < 1 (body2 = 3).
		{"up body3 = 4", w(up, 2, B{106.5, 107, 102, 102.5}), 0},
		{"up body3 = 3.75", w(up, 2, B{106.5, 107, 102.5, 102.75}), 100},
		{"up body3 = 2", w(up, 2, B{105.5, 106, 102.5, 103.5}), 0},
		{"up body3 = 2.25", w(up, 2, B{105.5, 106, 102.5, 103.25}), 100},
		{"down body3 = 3.5", w(dn, 2, B{103.5, 107.5, 103, 107}), -100},
		{"down body3 = 4", w(dn, 2, B{103.5, 108, 103, 107.5}), 0},
		{"down body3 = 4.25", w(dn, 2, B{103.25, 108, 103, 107.5}), 0},
		// Up: open3 < close2.
		{"up open3 = close2", w(up, 2, B{107, 107.5, 103, 103.5}), 0},
		{"up open3 < close2", w(up, 2, B{106.75, 107, 103, 103.5}), 100},
		// Up: open3 > open2.
		{"up open3 = open2", w(w(up, 1, B{104, 108, 103.5, 106}), 2, B{104, 104.5, 102.25, 102.5}), 0},
		{"up open3 > open2", w(w(up, 1, B{104, 108, 103.5, 106}), 2, B{104.25, 104.5, 102.25, 102.5}), 100},
		// Up: close3 < open2.
		{"up close3 = open2", w(up, 2, B{106.5, 107, 103.5, 104}), 0},
		{"up close3 < open2", w(up, 2, B{106.5, 107, 103.5, 103.75}), 100},
		// Up: close3 > max body of 1st (inside the gap).
		{"up close3 = top of 1st", w(w(up, 1, B{104, 108, 103.5, 106.5}), 2, B{104.5, 105, 101.5, 102}), 0},
		{"up close3 above 1st", w(w(up, 1, B{104, 108, 103.5, 106.5}), 2, B{104.5, 105, 101.5, 102.25}), 100},
		// Down: open3 < open2.
		{"down open3 = open2", w(w(dn, 1, B{106, 106.5, 103.5, 104}), 2, B{106, 108, 105.5, 107.5}), 0},
		{"down open3 < open2", w(w(dn, 1, B{106, 106.5, 103.5, 104}), 2, B{105.75, 108, 105.5, 107.5}), -100},
		// Down: open3 > close2.
		{"down open3 = close2", w(dn, 2, B{103, 107, 102.5, 106.5}), 0},
		{"down open3 > close2", w(dn, 2, B{103.25, 107, 102.5, 106.5}), -100},
		// Down: close3 > open2.
		{"down close3 = open2", w(dn, 2, B{103.5, 106.5, 103, 106}), 0},
		{"down close3 > open2", w(dn, 2, B{103.5, 106.5, 103, 106.25}), -100},
		// Down: close3 < min body of 1st (inside the gap).
		{"down close3 = bottom of 1st", w(dn, 2, B{104.5, 108.5, 104, 108}), 0},
		{"down close3 below 1st", w(dn, 2, B{104.5, 108.5, 104, 107.75}), -100},
		// Gaps: the 2nd body must gap away from the 1st.
		{"up no gap", w(up, 1, B{102, 106, 101.5, 105}), 0},
		{"up 2nd body touches 1st", w(w(up, 0, B{100, 104.5, 99, 104}), 2, B{106, 106.5, 102.5, 104.5}), 0},
		{"down 2nd body touches 1st", w(dn, 0, B{106, 111, 105.5, 108}), 0},
		// Colours: a doji (close == open) counts as white.
		{"up 2nd black", w(up, 1, B{107, 108, 103.5, 104}), 0},
		{"up 3rd white", w(up, 2, B{103, 106.5, 102.5, 106}), 0},
		{"up 3rd doji", w(up, 2, B{105, 106, 104, 105}), 0},
		{"down 2nd white", w(dn, 1, B{103, 106.5, 102.5, 106}), 0},
		{"down 3rd black", w(dn, 2, B{107, 107.5, 103.5, 104}), 0},
	}
}

func TestCDLTASUKIGAPParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLTASUKIGAP(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLTASUKIGAP(s.O[:1], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLTASUKIGAP(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLTASUKIGAP(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLTASUKIGAP(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLTASUKIGAP([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLTASUKIGAPLookback(); lb != 7 {
		t.Errorf("default lookback = %d, want 7", lb)
	}
}

func TestCDLTASUKIGAPLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, tc := range []struct{ near, want int }{{0, 2}, {1, 3}, {5, 7}, {10, 12}, {50, 52}} {
		RestoreAllCandleDefaultSettings()
		if err := SetCandleSettings(Near, RangeHighLow, tc.near, 0.2); err != nil {
			t.Fatal(err)
		}
		if lb := CDLTASUKIGAPLookback(); lb != tc.want {
			t.Errorf("near %d: lookback %d, want %d", tc.near, lb, tc.want)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, BodyShort, BodyDoji, ShadowLong, Far, Equal} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 40, d.Factor)
	}
	if lb := CDLTASUKIGAPLookback(); lb != 7 {
		t.Errorf("other settings changed: lookback %d, want 7", lb)
	}

	// Period 0: the average uses bar i-1's own range; lookback 2, so n=3 is
	// the smallest input with an output.
	RestoreAllCandleDefaultSettings()
	_ = SetCandleSettings(Near, RangeHighLow, 0, 0.25)
	o, h, l, c := cdltasukigapSeries(cdltasukigapUp)
	out, err := CDLTASUKIGAP(o[10:], h[10:], l[10:], c[10:])
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "period 0", out, []int{0, 0, 100})
	out, err = CDLTASUKIGAP(o[10:12], h[10:12], l[10:12], c[10:12])
	if err != nil || len(out) != 2 {
		t.Fatalf("n=2 with lookback 2: out=%v err=%v", out, err)
	}
	assertInt(t, "n=2", out, []int{0, 0})
	// Period 0: bar 2's range is 4.5, so the band is 1.125 and a body
	// difference of exactly 1 now passes, 1.25 does not.
	bars := cdltasukigapWith(cdltasukigapUp, 2, cdltasukigapBar{106.5, 107, 102, 102.5})
	o, h, l, c = cdltasukigapSeries(bars)
	out, _ = CDLTASUKIGAP(o[10:], h[10:], l[10:], c[10:])
	assertInt(t, "period 0 diff 1", out, []int{0, 0, 100})
	bars[2] = cdltasukigapBar{106.5, 107, 102, 102.25}
	o, h, l, c = cdltasukigapSeries(bars)
	out, _ = CDLTASUKIGAP(o[10:], h[10:], l[10:], c[10:])
	assertInt(t, "period 0 diff 1.25", out, []int{0, 0, 0})
}

func TestCDLTASUKIGAPLengths(t *testing.T) {
	lb := CDLTASUKIGAPLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLTASUKIGAP(s.O, s.H, s.L, s.C)
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
				if v != 0 && v != -100 && v != 100 {
					t.Fatalf("%s n=%d: [%d]=%d, want 0 or +-100", s.Name, n, i, v)
				}
			}
		}
	}
}

func TestCDLTASUKIGAPKnownValues(t *testing.T) {
	type B = cdltasukigapBar
	up, dn := cdltasukigapUp, cdltasukigapDown
	w := cdltasukigapWith
	run := func(label string, bars [3]B, want int) {
		t.Helper()
		o, h, l, c := cdltasukigapSeries(bars)
		out, err := CDLTASUKIGAP(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		exp := make([]int, 13)
		exp[12] = want
		assertInt(t, "CDLTASUKIGAP "+label, out, exp)
	}

	// Default settings: Near band 0.2*4 = 0.8.
	run("default up", up, 100)
	run("default down", dn, -100)
	run("default up diff 0.5", w(up, 2, B{106, 106.5, 102.5, 103.5}), 100)
	run("default up diff 0.75", w(up, 2, B{106.5, 107, 102.5, 102.75}), 100)
	run("default up diff 1", w(up, 2, B{106.5, 107, 102, 102.5}), 0)

	defer RestoreAllCandleDefaultSettings()
	cdltasukigapExactSettings(t)
	for _, tc := range cdltasukigapExactCases() {
		run(tc.name, tc.bars, tc.want)
	}

	// The average excludes bar i-1 itself: a huge range on the gap candle
	// does not widen the band (diff 1 still rejected).
	run("2nd range huge", w(w(up, 1, B{104, 200, 103.5, 107}), 2, B{106.5, 107, 102, 102.5}), 0)
	// But the five bars before it do: widening their ranges to 8 doubles the
	// band to 2, so a difference of 1.25 now passes.
	o, h, l, c := cdltasukigapSeries(w(w(up, 1, B{104, 108, 103.5, 107}), 2, B{106.5, 107, 102, 102.25}))
	out, _ := CDLTASUKIGAP(o, h, l, c)
	if out[12] != 0 {
		t.Errorf("narrow background diff 1.25: got %d, want 0", out[12])
	}
	for i := 6; i <= 10; i++ {
		h[i], l[i] = h[i]+2, l[i]-2
	}
	out, _ = CDLTASUKIGAP(o, h, l, c)
	if out[12] != 100 {
		t.Errorf("wider background: got %d, want 100", out[12])
	}
	// Bar 5 is outside the window for bar 12: widening it changes nothing.
	o, h, l, c = cdltasukigapSeries(w(w(up, 1, B{104, 108, 103.5, 107}), 2, B{106.5, 107, 102, 102.25}))
	h[5], l[5] = 200, 0
	out, _ = CDLTASUKIGAP(o, h, l, c)
	if out[12] != 0 {
		t.Errorf("bar outside window widened: got %d, want 0", out[12])
	}

	// Flat and monotone series never fire (no body gaps / no opposite colour).
	RestoreAllCandleDefaultSettings()
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" && s.Name != "zigzag" {
			continue
		}
		out, _ := CDLTASUKIGAP(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLTASUKIGAPRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdltasukigapRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdltasukigapRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdltasukigapRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdltasukigapRange(0, 6, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdltasukigapRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdltasukigapRange(0, 29, s.O[:29], s.H, s.L, s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("input one short: %v", err)
	}
	if _, _, err := cdltasukigapRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	if _, nb, err := cdltasukigapRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	if beg, nb, err := cdltasukigapRange(7, 7, s.O, s.H, s.L, s.C, out); err != nil || beg != 7 || nb != 1 {
		t.Errorf("single bar at lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values match the full series, including firing bars.
	o, h, l, c := cdltasukigapSeries(cdltasukigapUp)
	o2, h2, l2, c2 := cdltasukigapSeries(cdltasukigapDown)
	o, h, l, c = append(o, o2...), append(h, h2...), append(l, l2...), append(c, c2...)
	full, _ := CDLTASUKIGAP(o, h, l, c)
	if full[12] != 100 || full[25] != -100 {
		t.Fatalf("constructed patterns did not fire: %v", full)
	}
	buf := make([]int, len(o))
	beg, nb, err = cdltasukigapRange(3, 20, o, h, l, c, buf)
	if err != nil || beg != 7 || nb != 14 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", buf[:nb], full[beg:beg+nb])
	beg, nb, err = cdltasukigapRange(15, 25, o, h, l, c, buf)
	if err != nil || beg != 15 || nb != 11 {
		t.Fatalf("sub-range 2: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range 2", buf[:nb], full[beg:beg+nb])
	beg, nb, err = cdltasukigapRange(12, 12, o, h, l, c, buf)
	if err != nil || beg != 12 || nb != 1 || buf[0] != 100 {
		t.Fatalf("single firing bar: beg=%d nb=%d v=%d err=%v", beg, nb, buf[0], err)
	}
}
