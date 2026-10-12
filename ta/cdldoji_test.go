package ta

import (
	"errors"
	"testing"
)

// cdldojiBase returns n identical background bars: open 100, close 102,
// high 103, low 99 (real body 2, high-low 4).
func cdldojiBase(n int) (o, h, l, c []float64) {
	for range n {
		o = append(o, 100)
		h = append(h, 103)
		l = append(l, 99)
		c = append(c, 102)
	}
	return
}

func TestCDLDOJIParams(t *testing.T) {
	s := fixtures(50)[0]
	if _, err := CDLDOJI(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLDOJI(s.O[:10], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v, want ErrBadParam", err)
	}
	if _, err := CDLDOJI(s.O, s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: err = %v, want ErrBadParam", err)
	}
	if out, err := CDLDOJI(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CDLDOJI([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if lb := CDLDOJILookback(); lb != 10 {
		t.Errorf("default lookback = %d, want 10", lb)
	}
}

func TestCDLDOJILookbackSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	for _, p := range []int{0, 1, 5, 30, 100} {
		RestoreAllCandleDefaultSettings()
		d := GetCandleSetting(BodyDoji)
		if err := SetCandleSettings(BodyDoji, d.RangeType, p, d.Factor); err != nil {
			t.Fatal(err)
		}
		if lb := CDLDOJILookback(); lb != p {
			t.Errorf("BodyDoji period %d: lookback %d", p, lb)
		}
	}
	// Other settings do not affect the lookback.
	RestoreAllCandleDefaultSettings()
	d := GetCandleSetting(BodyLong)
	_ = SetCandleSettings(BodyLong, d.RangeType, 40, d.Factor)
	if lb := CDLDOJILookback(); lb != 10 {
		t.Errorf("BodyLong 40: lookback %d, want 10", lb)
	}

	// Period 0: lookback 0, each bar compared with its own high-low range.
	RestoreAllCandleDefaultSettings()
	d = GetCandleSetting(BodyDoji)
	_ = SetCandleSettings(BodyDoji, d.RangeType, 0, d.Factor)
	o := []float64{100, 100}
	h := []float64{101, 101}
	l := []float64{99, 99}
	c := []float64{100.125, 100.25} // body 0.125 <= 0.1*2; body 0.25 > 0.2
	out, err := CDLDOJI(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, "period 0", out, []int{100, 0})
	// n=1 with lookback 0 emits one value.
	out, err = CDLDOJI(o[:1], h[:1], l[:1], c[:1])
	if err != nil || len(out) != 1 || out[0] != 100 {
		t.Errorf("n=1 with lookback 0: out=%v err=%v", out, err)
	}
}

func TestCDLDOJILengths(t *testing.T) {
	lb := CDLDOJILookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CDLDOJI(s.O, s.H, s.L, s.C)
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
				if v != 0 && v != 100 {
					t.Fatalf("%s n=%d: [%d]=%d, want 0 or 100", s.Name, n, i, v)
				}
			}
		}
	}
}

func TestCDLDOJIKnownValues(t *testing.T) {
	// Background bars have high-low 4, so the default BodyDoji threshold
	// is 0.1 * 4 = 0.4 (average of the previous 10 high-low ranges).
	type bar struct{ o, h, l, c float64 }
	cases := []struct {
		name string
		b    bar
		want int
	}{
		{"exact doji", bar{100, 101, 99, 100}, 100},
		{"small body", bar{100, 101, 99, 100.25}, 100},
		{"black small body", bar{100.25, 101, 99, 100}, 100},
		{"body at threshold", bar{100, 101, 99, 100.375}, 100}, // 0.375 exact in binary
		{"body above threshold", bar{100, 101, 99, 100.5}, 0},
		{"big body", bar{100, 103, 99, 102}, 0},
		// Current bar's own range is excluded from the average.
		{"huge own range", bar{100, 200, 0, 101}, 0},
	}
	for _, tc := range cases {
		o, h, l, c := cdldojiBase(10)
		o = append(o, tc.b.o)
		h = append(h, tc.b.h)
		l = append(l, tc.b.l)
		c = append(c, tc.b.c)
		out, err := CDLDOJI(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, 11)
		want[10] = tc.want
		assertInt(t, "CDLDOJI "+tc.name, out, want)
	}

	// The trailing window slides: one wide-range bar (index 10, range 41)
	// raises the threshold only while it is inside the previous-10 window.
	o, h, l, c := cdldojiBase(10)
	o = append(o, 100)
	h = append(h, 140)
	l = append(l, 99)
	c = append(c, 102) // body 2 > 0.4: not a doji itself
	for range 11 {
		o = append(o, 100)
		h = append(h, 101) // range 2
		l = append(l, 99)
		c = append(c, 100.6) // body 0.6
	}
	out, err := CDLDOJI(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	want := make([]int, len(o))
	// Bar k (11..19): window k-10..k-1 holds bar 10 (41), some bars of
	// range 4 and the rest of range 2. Bar 19: 4+41+8*2 = 61 → 0.61 >= 0.6.
	for k := 11; k <= 19; k++ {
		want[k] = 100
	}
	// Bar 20: window 10..19 = 41+9*2 = 59 → 0.59 < 0.6. Bar 21: 20 → 0.2.
	assertInt(t, "sliding window", out, want)

	// Constant and zero series: body 0 <= threshold 0, so every bar after
	// the lookback is a doji.
	for _, s := range fixtures(100) {
		if s.Name != "constant" && s.Name != "zeros" {
			continue
		}
		out, _ := CDLDOJI(s.O, s.H, s.L, s.C)
		for i, v := range out {
			want := 0
			if i >= 10 {
				want = 100
			}
			if v != want {
				t.Fatalf("%s: [%d]=%d want %d", s.Name, i, v, want)
			}
		}
	}
}

func TestCDLDOJIRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]int, 30)
	if _, _, err := cdldojiRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cdldojiRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cdldojiRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cdldojiRange(0, 9, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := cdldojiRange(0, 29, s.O, s.H, s.L[:20], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := cdldojiRange(0, 29, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Output buffer of exactly endIdx-startIdx+1 suffices.
	if _, nb, err := cdldojiRange(20, 29, s.O, s.H, s.L, s.C, make([]int, 10)); err != nil || nb != 10 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
	// Sub-range values match the full series.
	full, _ := CDLDOJI(s.O, s.H, s.L, s.C)
	beg, nb, err = cdldojiRange(5, 25, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 10 || nb != 16 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[beg:beg+nb])
}
