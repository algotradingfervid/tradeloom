package ta

import (
	"errors"
	"math"
	"testing"
)

func TestDEMAParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := DEMA(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := DEMALookback(p); lb != -1 {
			t.Errorf("DEMALookback(%d) = %d, want -1", p, lb)
		}
		out := make([]float64, len(in))
		if _, _, err := demaRange(0, len(in)-1, in, p, out); !errors.Is(err, ErrBadParam) {
			t.Errorf("demaRange period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 30, 100000} {
		if _, err := DEMA(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := DEMALookback(p); lb != 2*(p-1) {
			t.Errorf("DEMALookback(%d) = %d, want %d", p, lb, 2*(p-1))
		}
	}
	if out, err := DEMA(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := DEMA([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Period 100000 on a short series: all NaN, no error.
	out, err := DEMA(in, 100000)
	if err != nil || len(out) != len(in) {
		t.Fatalf("p=100000: len=%d err=%v", len(out), err)
	}
	assertLeadingNaN(t, "DEMA p=100000", out, len(in))
}

func TestDEMALengths(t *testing.T) {
	for _, period := range []int{1, 2, 3, 5, 30} {
		lb := DEMALookback(period)
		if lb != 2*(period-1) {
			t.Fatalf("DEMALookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := DEMA(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("DEMA ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestDEMAKnownValues(t *testing.T) {
	nan := math.NaN()
	// Period 3, k = 0.5, input 1..6.
	// EMA1: seed (1+2+3)/3 = 2 at bar 2; bar 3: 3; bar 4: 4; bar 5: 5.
	// EMA2: seed (2+3+4)/3 = 3 at bar 4; bar 5: (5-3)*0.5+3 = 4.
	// DEMA bar 4: 2*4-3 = 5; bar 5: 2*5-4 = 6.
	in := []float64{1, 2, 3, 4, 5, 6}
	out, err := DEMA(in, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "DEMA(1..6,3)", out, []float64{nan, nan, nan, nan, 5, 6}, 0)

	// Hand-derived non-linear case, period 2 (k = 2/3), input 0, 3, 0, 6:
	// EMA1: seed 1.5 at bar 1; bar 2: 1.5 + (0-1.5)*2/3 = 0.5;
	//       bar 3: 0.5 + (6-0.5)*2/3 = 0.5+11/3 = 25/6.
	// EMA2: seed (1.5+0.5)/2 = 1 at bar 2; bar 3: 1 + (25/6-1)*2/3 = 28/9.
	// DEMA bar 2: 2*0.5-1 = 0; bar 3: 25/3 - 28/9 = 47/9.
	out, _ = DEMA([]float64{0, 3, 0, 6}, 2)
	assertReal(t, "DEMA(0,3,0,6;2)", out, []float64{nan, nan, 0, 47.0 / 9}, 1e-15)

	// Matches the formula 2*EMA - EMA(EMA) built from EMA itself.
	s := fixtures(200)[0].Real()
	for _, p := range []int{2, 5, 14} {
		e1, _ := EMA(s, p)
		e2, _ := EMA(e1[p-1:], p)
		want := make([]float64, len(s))
		for i := range want {
			want[i] = nan
		}
		for i := 2 * (p - 1); i < len(s); i++ {
			want[i] = 2*e1[i] - e2[i-(p-1)]
		}
		got, _ := DEMA(s, p)
		assertReal(t, lbl("DEMA vs EMA p=", p), got, want, 1e-12)
	}

	// Period 1 is the identity (yaml flag period1_identity), including
	// values that differ by more than a factor of two.
	id := []float64{1, 1000, -3, 1e-9, 5e12, 0}
	out, _ = DEMA(id, 1)
	assertReal(t, "DEMA period 1", out, id, 0)

	// A constant series stays constant.
	c := fixtures(60)[5].Real()
	out, _ = DEMA(c, 7)
	for i := 12; i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}
}

func TestDEMARangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := demaRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := demaRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := demaRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (lookback 4 at period 3).
	beg, nb, err := demaRange(0, 3, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := demaRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := demaRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// A range starting inside the lookback seeds from bar 0 like the full
	// computation, so it reproduces it. (A later startIdx reseeds at
	// startIdx-lookback, as C does, and so gives different values; the
	// oracle tests cover those.)
	full, _ := DEMA(in, 3)
	beg, nb, err = demaRange(2, 20, in, 3, out)
	if err != nil || beg != 4 || nb != 17 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[4:21], 0)
	beg, nb, err = demaRange(10, 20, in, 3, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range 10..20: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// In-place computation is allowed.
	for _, p := range []int{1, 4} {
		buf := append([]float64(nil), in...)
		want, _ := DEMA(in, p)
		beg, nb, _ = demaRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}

func TestDEMAUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	SetUnstablePeriod(UnstEMA, 5)
	if lb := DEMALookback(3); lb != 2*(3-1+5) {
		t.Errorf("lookback with unstable 5: %d", lb)
	}
	if lb := DEMALookback(1); lb != 10 {
		t.Errorf("period-1 lookback with unstable 5: %d", lb)
	}
	in := fixtures(40)[0].Real()
	out, err := DEMA(in, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertLeadingNaN(t, "period 1 unstable", out, 10)
	assertReal(t, "period 1 unstable copy", out[10:], in[10:], 0)
}
