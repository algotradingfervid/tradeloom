package ta

import (
	"errors"
	"math"
	"testing"
)

func TestAPOParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := APO(in, p, 26, MATypeSMA); !errors.Is(err, ErrBadParam) {
			t.Errorf("fast %d: err = %v, want ErrBadParam", p, err)
		}
		if _, err := APO(in, 12, p, MATypeSMA); !errors.Is(err, ErrBadParam) {
			t.Errorf("slow %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := APOLookback(p, 26, MATypeSMA); lb != -1 {
			t.Errorf("APOLookback(fast %d) = %d, want -1", p, lb)
		}
		if lb := APOLookback(12, p, MATypeSMA); lb != -1 {
			t.Errorf("APOLookback(slow %d) = %d, want -1", p, lb)
		}
	}
	for _, mt := range []MAType{-1, 14, 99} {
		if _, err := APO(in, 12, 26, mt); !errors.Is(err, ErrBadParam) {
			t.Errorf("MAType %d: err = %v, want ErrBadParam", mt, err)
		}
		if lb := APOLookback(12, 26, mt); lb != -1 {
			t.Errorf("APOLookback(MAType %d) = %d, want -1", mt, lb)
		}
	}
	for _, pp := range [][2]int{{2, 2}, {2, 100000}, {100000, 2}, {100000, 100000}, {12, 26}} {
		if _, err := APO(in, pp[0], pp[1], MATypeSMA); err != nil {
			t.Errorf("periods %v: unexpected err %v", pp, err)
		}
	}
	for _, mt := range maAllTypes {
		if _, err := APO(in, 3, 5, mt); err != nil {
			t.Errorf("MAType %d: unexpected err %v", mt, err)
		}
	}
	if out, err := APO(nil, 12, 26, MATypeEMA); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := APO([]float64{}, 12, 26, MATypeEMA); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Default MAType is EMA.
	if a, b := APOLookback(12, 26, MATypeDefault), APOLookback(12, 26, MATypeEMA); a != b {
		t.Errorf("default lookback %d, EMA %d", a, b)
	}
}

func TestAPOLengths(t *testing.T) {
	cases := []struct {
		fast, slow int
		mt         MAType
	}{
		{2, 3, MATypeSMA}, {12, 26, MATypeEMA}, {26, 12, MATypeSMA}, {5, 5, MATypeWMA},
		{3, 10, MATypeT3}, {3, 10, MATypeMAMA}, {3, 10, MATypeDisabled}, {12, 26, MATypeDefault},
	}
	for _, c := range cases {
		lb := APOLookback(c.fast, c.slow, c.mt)
		if lb < 0 {
			t.Fatalf("APOLookback(%d,%d,%d) = %d", c.fast, c.slow, c.mt, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := APO(s.Real(), c.fast, c.slow, c.mt)
				if err != nil {
					t.Fatalf("%s n=%d %v: %v", s.Name, n, c, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d %v: len(out)=%d", s.Name, n, c, len(out))
				}
				assertLeadingNaN(t, lbl("APO ", s.Name, " n=", n, " ", c), out, lb)
			}
		}
	}
	if lb := APOLookback(12, 26, MATypeSMA); lb != 25 {
		t.Errorf("APOLookback(12,26,SMA) = %d, want 25", lb)
	}
	if lb := APOLookback(26, 12, MATypeSMA); lb != 25 {
		t.Errorf("APOLookback(26,12,SMA) = %d, want 25", lb)
	}
	if lb := APOLookback(12, 26, MATypeDisabled); lb != 0 {
		t.Errorf("APOLookback(disabled) = %d, want 0", lb)
	}
}

func TestAPOKnownValues(t *testing.T) {
	// SMA(2) - SMA(3) by hand:
	// SMA2: [_, 2.5, 3, 5, 6.5, 6]; SMA3: [_, _, 7/3, 14/3, 5, 20/3].
	in := []float64{1, 4, 2, 8, 5, 7}
	nan := math.NaN()
	want := []float64{nan, nan, 3 - 7.0/3, 5 - 14.0/3, 1.5, 6 - 20.0/3}
	out, err := APO(in, 2, 3, MATypeSMA)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "APO SMA(2,3)", out, want, 1e-12)

	// A slow period shorter than the fast one is swapped.
	sw, _ := APO(in, 3, 2, MATypeSMA)
	assertReal(t, "APO swapped", sw, out, 0)

	// APO equals MA(fast) - MA(slow) for every MA type.
	s := fixtures(200)[0].Real()
	for _, mt := range maAllTypes {
		got, err := APO(s, 5, 17, mt)
		if err != nil {
			t.Fatal(err)
		}
		// APO maps MATypeDefault to EMA (MA maps it to SMA).
		mmt := mt
		if mmt == MATypeDefault {
			mmt = MATypeEMA
		}
		f, _ := MA(s, 5, mmt)
		sl, _ := MA(s, 17, mmt)
		lb := APOLookback(5, 17, mt)
		exp := make([]float64, len(s))
		for i := range exp {
			if i < lb {
				exp[i] = nan
			} else {
				exp[i] = f[i] - sl[i]
			}
		}
		assertReal(t, lbl("APO vs MA mt=", mt), got, exp, 0)
	}

	// A constant series has zero oscillation.
	c := fixtures(100)[5].Real()
	for _, mt := range []MAType{MATypeSMA, MATypeEMA, MATypeWMA} {
		out, _ = APO(c, 4, 9, mt)
		lb := APOLookback(4, 9, mt)
		for i := lb; i < len(out); i++ {
			if math.Abs(out[i]) > 1e-12 {
				t.Fatalf("constant mt=%d: [%d]=%g", mt, i, out[i])
			}
		}
	}

	// MATypeDisabled: both MAs are the identity, so APO is 0 everywhere.
	out, _ = APO(s, 5, 17, MATypeDisabled)
	for i, v := range out {
		if v != 0 {
			t.Fatalf("disabled: [%d]=%g", i, v)
		}
	}

	// Equal periods give exactly 0.
	out, _ = APO(s, 7, 7, MATypeEMA)
	for i := APOLookback(7, 7, MATypeEMA); i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("equal periods: [%d]=%g", i, out[i])
		}
	}
}

func TestAPORangeForm(t *testing.T) {
	in := fixtures(60)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := apoRange(-1, 5, in, 3, 5, MATypeSMA, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := apoRange(5, 4, in, 3, 5, MATypeSMA, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := apoRange(0, 5, in, 1, 5, MATypeSMA, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("bad fast: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := apoRange(0, 3, in, 3, 5, MATypeSMA, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := apoRange(0, 10, in[:5], 3, 5, MATypeSMA, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := apoRange(0, 10, in, 3, 5, MATypeSMA, out[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Sub-range values match the full series (SMA has no seeding memory).
	fullS, _ := APO(in, 3, 5, MATypeSMA)
	beg, nb, err = apoRange(10, 30, in, 3, 5, MATypeSMA, out)
	if err != nil || beg != 10 || nb != 21 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range SMA", out[:nb], fullS[beg:beg+nb], 1e-9)
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := APO(in, 4, 9, MATypeWMA)
	beg, nb, _ = apoRange(0, len(buf)-1, buf, 4, 9, MATypeWMA, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
