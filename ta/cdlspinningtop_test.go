package ta

import (
	"errors"
	"testing"
)

// cdlspinningtopBase returns n identical background bars: open 100, close
// 102, high 103, low 99 (real body 2, upper shadow 1, lower shadow 1).
func cdlspinningtopBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

func TestCDLSPINNINGTOPParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLSPINNINGTOP(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLSPINNINGTOP(s.O[:10], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLSPINNINGTOP(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLSPINNINGTOP(s.O, s.H, s.L, s.C[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLSPINNINGTOP(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLSPINNINGTOP([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLSPINNINGTOPLookback(); lb != 10 {
		t.Errorf("default lookback = %d, want 10", lb)
	}
}

func TestCDLSPINNINGTOPLookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, p := range []int{0, 1, 7, 30, 100} {
		RestoreAllCandleDefaultSettings()
		b := GetCandleSetting(BodyShort)
		if err := SetCandleSettings(BodyShort, b.RangeType, p, b.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLSPINNINGTOPLookback(); lb != p {
			t.Errorf("BodyShort %d: lookback %d, want %d", p, lb, p)
		}
	}
	// Unrelated settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	d := GetCandleSetting(BodyLong)
	_ = SetCandleSettings(BodyLong, d.RangeType, 40, d.Factor)
	if lb := CDLSPINNINGTOPLookback(); lb != 10 {
		t.Errorf("BodyLong 40: lookback %d, want 10", lb)
	}

	// Period 0: lookback 0, each bar compared with its own range. With
	// RealBody, body < body is never true, so use HighLow * 0.5.
	RestoreAllCandleDefaultSettings()
	_ = SetCandleSettings(BodyShort, RangeHighLow, 0, 0.5)
	o := []float64{100, 100}
	h := []float64{103, 101}
	l := []float64{98, 99}
	c := []float64{101, 101} // bar0: body 1 < 2.5, shadows 2 > 1; bar1: upper 0
	out, err := CDLSPINNINGTOP(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "period 0", out, []int{100, 0})
	out, err = CDLSPINNINGTOP(o[:1], h[:1], l[:1], c[:1])
	if err != nil || len(out) != 1 || out[0] != 100 {
		t.Errorf("n=1 with lookback 0: out=%v err=%v", out, err)
	}
}

func TestCDLSPINNINGTOPLengths(t *testing.T) {
	lb := CDLSPINNINGTOPLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLSPINNINGTOP(s.O, s.H, s.L, s.C)
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

func TestCDLSPINNINGTOPKnownValues(t *testing.T) {
	// Background bars have real body 2, so the default BodyShort threshold
	// (RealBody, period 10, factor 1) is 2: the body must be < 2, and each
	// shadow must exceed the bar's own body.
	type bar struct{ o, h, l, c float64 }
	cases := []struct {
		name string
		b    bar
		want int
	}{
		{"white spinning top", bar{100, 103, 98, 101}, 100},
		{"black spinning top", bar{101, 103, 98, 100}, -100},
		{"doji with shadows", bar{100, 101, 99, 100}, 100},
		{"doji without lower shadow", bar{100, 101, 100, 100}, 0},
		{"doji without upper shadow", bar{100, 100, 99, 100}, 0},
		{"upper shadow equals body", bar{100, 102, 98, 101}, 0},
		{"lower shadow equals body", bar{100, 103, 99, 101}, 0},
		{"black upper shadow equals body", bar{101, 102, 98, 100}, 0},
		{"black lower shadow equals body", bar{101, 103, 99, 100}, 0},
		{"body at threshold", bar{100, 105, 97, 102}, 0}, // 2 < 2 false
		{"body just below threshold", bar{100, 105, 97, 101.75}, 100},
	}
	for _, tc := range cases {
		o, h, l, c := cdlspinningtopBase(10)
		o = append(o, tc.b.o)
		h = append(h, tc.b.h)
		l = append(l, tc.b.l)
		c = append(c, tc.b.c)
		out, err := CDLSPINNINGTOP(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 11)
		want[10] = tc.want
		assertInt(t, "CDLSPINNINGTOP "+tc.name, out, want)
	}

	// The BodyShort window slides: one bar with body 12 (index 10) raises
	// the threshold to (9*2+12)/10 = 3 while it is inside the previous-10
	// window. Following bars have body 2.5 with shadows 3.
	o, h, l, c := cdlspinningtopBase(10)
	o = append(o, 100)
	h = append(h, 113)
	l = append(l, 99)
	c = append(c, 112) // shadows 1 < body 12, not a spinning top
	for range 11 {
		o = append(o, 100)
		h = append(h, 105.5)
		l = append(l, 97)
		c = append(c, 102.5)
	}
	out, err := CDLSPINNINGTOP(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	want := make([]int, len(o))
	// Bar k (11..20): window k-10..k-1 contains bar 10 (body 12), so the
	// average is >= 3 > 2.5. Bar 21: window 11..20 is all 2.5, so
	// 2.5 < 2.5 is false.
	for k := 11; k <= 20; k++ {
		want[k] = 100
	}
	assertInt(t, "sliding window", out, want)

	// Constant and zero series: shadows 0 > body 0 is false, so nothing fires.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLSPINNINGTOP(s.O, s.H, s.L, s.C)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d want 0", s.Name, i, v)
			}
		}
	}
}

func TestCDLSPINNINGTOPRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdlspinningtopRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdlspinningtopRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdlspinningtopRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdlspinningtopRange(0, 9, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdlspinningtopRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdlspinningtopRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdlspinningtopRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLSPINNINGTOP(s.O, s.H, s.L, s.C)
	beg, nb, err = cdlspinningtopRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 10 || nb != 16 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
