package ta

import (
	"errors"
	"math"
	"testing"
)

func TestTEMAParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32} {
		if _, err := TEMA(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := TEMALookback(p); lb != -1 {
			t.Errorf("TEMALookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := temaRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 100000} {
		if _, err := TEMA(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
	}
	if lb := TEMALookback(100000); lb != 3*99999 {
		t.Errorf("TEMALookback(100000) = %d", lb)
	}
	if out, err := TEMA(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := TEMA([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestTEMALengths(t *testing.T) {
	for _, period := range []int{1, 2, 3, 5, 30} {
		lb := TEMALookback(period)
		if lb != 3*(period-1) {
			t.Fatalf("TEMALookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := TEMA(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("TEMA ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestTEMAUnstableLookback(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	SetUnstablePeriod(UnstEMA, 4)
	if lb := TEMALookback(5); lb != 3*(4+4) {
		t.Fatalf("TEMALookback(5) with UnstEMA=4: %d", lb)
	}
	in := fixtures(60)[0].Real()
	out, err := TEMA(in, 5)
	if err != nil {
		t.Fatal(err)
	}
	assertLeadingNaN(t, "TEMA unst", out, 24)
}

func TestTEMAKnownValues(t *testing.T) {
	// Period 2, k = 2/3, input 1..5:
	//   EMA1: seed (1+2)/2 = 1.5 at bar 1, then 2.5, 3.5, 4.5
	//   EMA2: seed (1.5+2.5)/2 = 2 at bar 2, then 3, 4
	//   EMA3: seed (2+3)/2 = 2.5 at bar 3, then 3.5
	//   TEMA[3] = 2.5 + 3*3.5 - 3*3 = 4;  TEMA[4] = 3.5 + 3*4.5 - 3*4 = 5
	nan := math.NaN()
	out, err := TEMA([]float64{1, 2, 3, 4, 5}, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "TEMA(1..5,2)", out, []float64{nan, nan, nan, 4, 5}, 1e-12)

	// Period 1 is the identity.
	in := fixtures(40)[0].Real()
	out, _ = TEMA(in, 1)
	assertReal(t, "TEMA period 1", out, in, 0)

	// A constant series stays constant.
	c := fixtures(60)[5].Real()
	out, _ = TEMA(c, 7)
	for i := 18; i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("constant: [%d]=%.17g", i, out[i])
		}
	}

	// Composition: TEMA = 3*EMA1 - 3*EMA2 + EMA3 with EMA2 = EMA(EMA1) and
	// EMA3 = EMA(EMA2), each EMA seeded on the previous one's first outputs.
	for _, p := range []int{2, 3, 10} {
		x := fixtures(200)[1].Real()
		lb := p - 1
		e1, _ := EMA(x, p)
		e2s, _ := EMA(e1[lb:], p)
		e3s, _ := EMA(e2s[lb:], p)
		want := nanSlice(len(x))
		for i := 3 * lb; i < len(x); i++ {
			want[i] = 3*e1[i] - 3*e2s[i-lb] + e3s[i-2*lb]
		}
		got, _ := TEMA(x, p)
		assertReal(t, lbl("TEMA composition p=", p), got, want, 1e-9)
	}
}

func TestTEMARangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := temaRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := temaRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := temaRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (6 for p=3): no output, no error.
	beg, nb, err := temaRange(0, 5, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := temaRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := temaRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// A range starting at 0 and ending early equals the prefix of the full
	// computation. (A later startIdx re-seeds the EMAs at startIdx-lookback,
	// as in C, so it does not equal a slice of the full output.)
	full, _ := TEMA(in, 3)
	beg, nb, err = temaRange(0, 20, in, 3, out)
	if err != nil || beg != 6 || nb != 15 {
		t.Fatalf("prefix range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "prefix range", out[:nb], full[6:21], 0)
	beg, nb, err = temaRange(10, 20, in, 3, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	reseeded, _ := TEMA(in[4:21], 3)
	assertReal(t, "sub-range", out[:nb], reseeded[6:], 0)
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := TEMA(in, 4)
	beg, nb, _ = temaRange(0, len(buf)-1, buf, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
