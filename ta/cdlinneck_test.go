package ta

import (
	"errors"
	"math"
	"testing"
)

// cdlinneckBase returns n identical background bars: open 100, close 102,
// high 103, low 99 (real body 2, high-low range 4).
func cdlinneckBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

// cdlinneckBar is one open/close pair; high and low are set 0.5 outside the
// body.
type cdlinneckBar struct{ o, c float64 }

// cdlinneckSeries appends the two pattern bars to 10 background bars, so the
// second bar is index 11 (the first evaluated bar with default settings).
func cdlinneckSeries(b1, b2 cdlinneckBar) (o, h, l, c []float64) {
	o, h, l, c = cdlinneckBase(10)
	for _, b := range []cdlinneckBar{b1, b2} {
		o = append(o, b.o)
		c = append(c, b.c)
		h = append(h, cMax(b.o, b.c)+0.5)
		l = append(l, cMin(b.o, b.c)-0.5)
	}
	return
}

// cdlinneckEq is the default Equal band for the 1st candle (bar 10): factor
// 0.05 times the average high-low range of bars 5..9, computed exactly as the
// implementation does.
func cdlinneckEq() float64 {
	sum := 0.0
	for range 5 {
		sum += 4.0
	}
	return 0.05 * (sum / 5) / 1.0
}

// cdlinneckCases are the hand-checked two-bar cases shared by the pure-Go
// and the oracle tests. The 1st candle is black 106->100 (low 99.5) unless
// stated; the BodyLong average for it is 2 (bars 0..9), the Equal band 0.2.
func cdlinneckCases() []struct {
	name   string
	b1, b2 cdlinneckBar
	want   int
} {
	eq := cdlinneckEq()
	up := 100 + eq
	type b = cdlinneckBar
	return []struct {
		name   string
		b1, b2 cdlinneckBar
		want   int
	}{
		{"in-neck", b{106, 100}, b{98, 100.1}, -100},
		{"close equals prior close", b{106, 100}, b{98, 100}, -100},
		{"close at upper band", b{106, 100}, b{98, up}, -100},
		{"close above band", b{106, 100}, b{98, math.Nextafter(up, math.Inf(1))}, 0},
		{"close below prior close", b{106, 100}, b{98, math.Nextafter(100, math.Inf(-1))}, 0},
		// Prior low is 99.5: open must be strictly below it.
		{"open equals prior low", b{106, 100}, b{99.5, 100.1}, 0},
		{"open just below prior low", b{106, 100}, b{math.Nextafter(99.5, math.Inf(-1)), 100.1}, -100},
		{"1st white", b{100, 106}, b{98, 106.1}, 0},
		// Body equal to the average (2) is not long; slightly more is.
		{"1st body equals average", b{102, 100}, b{98, 100.1}, 0},
		{"1st body just above average", b{102.0001, 100}, b{98, 100.1}, -100},
		// A black 2nd candle opening below the prior low cannot close at or
		// above the prior close.
		{"2nd black", b{106, 100}, b{99, 98}, 0},
		// The 2nd candle is white (close >= open) even as a doji, but then it
		// cannot both open below 99.5 and close at or above 100.
		{"2nd doji", b{106, 100}, b{99, 99}, 0},
		// Closing well into the body (thrusting) is not in-neck.
		{"close deep into body", b{106, 100}, b{98, 103}, 0},
	}
}

func TestCDLINNECKParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLINNECK(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLINNECK(s.O[:3], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLINNECK(s.O, s.H, s.L, s.C[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLINNECK(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLINNECK(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLINNECK([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLINNECKLookback(); lb != 11 {
		t.Errorf("default lookback = %d, want 11", lb)
	}
}

func TestCDLINNECKLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	cases := []struct {
		set    CandleSettingType
		period int
		want   int
	}{
		{BodyLong, 20, 21},
		{Equal, 30, 31},
		{BodyLong, 0, 6}, // Equal still 5
		{Equal, 0, 11},   // BodyLong still 10
		{BodyShort, 50, 11},
	}
	for _, tc := range cases {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(tc.set)
		if err := SetCandleSettings(tc.set, d.RangeType, tc.period, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLINNECKLookback(); lb != tc.want {
			t.Errorf("setting %d period %d: lookback %d, want %d", tc.set, tc.period, lb, tc.want)
		}
	}
	// Both average periods 0: only the prior candle is needed.
	RestoreAllCandleDefaultSettings()
	for _, st := range []CandleSettingType{BodyLong, Equal} {
		d := GetCandleSetting(st)
		_ = SetCandleSettings(st, d.RangeType, 0, d.Factor)
	}
	if lb := CDLINNECKLookback(); lb != 1 {
		t.Errorf("all periods 0: lookback %d, want 1", lb)
	}
	for n := 0; n <= 2; n++ {
		o, h, l, c := cdlinneckBase(n)
		out, err := CDLINNECK(o, h, l, c)
		if err != nil || len(out) != n {
			t.Errorf("n=%d with lookback 1: out=%v err=%v", n, out, err)
		}
		for i, v := range out {
			if v != 0 {
				t.Errorf("n=%d: [%d]=%d", n, i, v)
			}
		}
	}
	// With BodyLong period 0 the 1st body is compared with itself (factor 1):
	// never strictly greater, so the pattern cannot fire.
	o, h, l, c := cdlinneckSeries(cdlinneckBar{106, 100}, cdlinneckBar{98, 100.1})
	out, _ := CDLINNECK(o, h, l, c)
	for i, v := range out {
		if v != 0 {
			t.Errorf("periods 0: [%d]=%d", i, v)
		}
	}
	// With BodyLong factor 0.5 and period 0 it fires on the minimal series.
	RestoreAllCandleDefaultSettings()
	_ = SetCandleSettings(BodyLong, RangeRealBody, 0, 0.5)
	_ = SetCandleSettings(Equal, RangeHighLow, 0, 0.05)
	out, _ = CDLINNECK([]float64{106, 98}, []float64{106.5, 100.6}, []float64{99.5, 97.5}, []float64{100, 100.1})
	assertInt(t, "lookback-1 minimal", out, []int{0, -100})
}

func TestCDLINNECKLengths(t *testing.T) {
	lb := CDLINNECKLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLINNECK(s.O, s.H, s.L, s.C)
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
					t.Fatalf("%s n=%d: [%d]=%d, want -100 or 0", s.Name, n, i, v)
				}
			}
		}
	}
}

func TestCDLINNECKKnownValues(t *testing.T) {
	for _, tc := range cdlinneckCases() {
		o, h, l, c := cdlinneckSeries(tc.b1, tc.b2)
		out, err := CDLINNECK(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 12)
		want[11] = tc.want
		assertInt(t, "CDLINNECK "+tc.name, out, want)
	}

	// The pattern repeated: the evaluation at bar 11 fires, and the bars
	// that follow are background-like and do not.
	o, h, l, c := cdlinneckSeries(cdlinneckBar{106, 100}, cdlinneckBar{98, 100.1})
	b0, bh, bl, bc := cdlinneckBase(3)
	o, h, l, c = append(o, b0...), append(h, bh...), append(l, bl...), append(c, bc...)
	out, _ := CDLINNECK(o, h, l, c)
	want := make([]int, 15)
	want[11] = -100
	assertInt(t, "CDLINNECK followed by background", out, want)

	// Constant and zero series have zero bodies: never long, never fire.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLINNECK(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d", s.Name, i, v)
			}
		}
	}
}

func TestCDLINNECKRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlinneckRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlinneckRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlinneckRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlinneckRange(0, 10, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlinneckRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlinneckRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlinneckRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLINNECK(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlinneckRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 11 || nb != 15 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
