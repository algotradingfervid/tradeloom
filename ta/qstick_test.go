package ta

import (
	"errors"
	"math"
	"testing"
)

func TestQSTICKParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := QSTICK(s.O, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := QSTICKLookback(p); lb != -1 {
			t.Errorf("QSTICKLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := qstickRange(0, 49, s.O, s.C, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 10, 100000} {
		if _, err := QSTICK(s.O, s.C, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := QSTICKLookback(p); lb != p-1 {
			t.Errorf("QSTICKLookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	// Mismatched input lengths.
	if _, err := QSTICK(s.O, s.C[:49], 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, err := QSTICK(nil, s.C, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil open, non-empty close: err = %v, want ErrBadParam", err)
	}
	// nil / empty input → empty output.
	if out, err := QSTICK(nil, nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := QSTICK([]float64{}, []float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestQSTICKLengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 10, 30} {
		lb := QSTICKLookback(period)
		if lb != period-1 {
			t.Fatalf("QSTICKLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := QSTICK(s.O, s.C, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("QSTICK ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestQSTICKKnownValues(t *testing.T) {
	nan := math.NaN()
	open := []float64{10, 12, 11, 15, 14, 20}
	close := []float64{11, 11, 14, 15, 16, 17}
	// bodies: 1, -1, 3, 0, 2, -3
	out, err := QSTICK(open, close, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "QSTICK p=3", out, []float64{nan, nan, 1, 2.0 / 3, 5.0 / 3, -1.0 / 3}, 0)

	// Period 1 is the raw body.
	out, _ = QSTICK(open, close, 1)
	assertReal(t, "QSTICK period 1", out, []float64{1, -1, 3, 0, 2, -3}, 0)

	// Period equal to the length gives a single value: the mean body.
	out, _ = QSTICK(open, close, 6)
	assertReal(t, "QSTICK p=6", out, []float64{nan, nan, nan, nan, nan, 1.0 / 3}, 0)

	// Constant and doji bars (open == close) average to zero.
	c := fixtures(40)[5]
	out, _ = QSTICK(c.O, c.C, 7)
	for i := 6; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// A steady trend with constant body averages to that body.
	up := fixtures(40)[7]
	out, _ = QSTICK(up.O, up.C, 5)
	for i := 4; i < len(out); i++ {
		if math.Abs(out[i]-0.8) > 1e-9 {
			t.Fatalf("up trend: [%d]=%g, want 0.8", i, out[i])
		}
	}
	down := fixtures(40)[8]
	out, _ = QSTICK(down.O, down.C, 5)
	for i := 4; i < len(out); i++ {
		if math.Abs(out[i]+0.8) > 1e-9 {
			t.Fatalf("down trend: [%d]=%g, want -0.8", i, out[i])
		}
	}

	// Zigzag bodies alternate +5/-5: even period averages to 0.
	z := fixtures(30)[9]
	out, _ = QSTICK(z.O, z.C, 2)
	for i := 1; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("zigzag p=2: [%d]=%g", i, out[i])
		}
	}

	// Bounded by the min/max body in the window.
	w := fixtures(200)[0]
	out, _ = QSTICK(w.O, w.C, 10)
	for i := 9; i < len(out); i++ {
		lo, hi := math.Inf(1), math.Inf(-1)
		for k := i - 9; k <= i; k++ {
			b := w.C[k] - w.O[k]
			lo, hi = math.Min(lo, b), math.Max(hi, b)
		}
		if out[i] < lo-1e-9 || out[i] > hi+1e-9 {
			t.Fatalf("bounds: [%d]=%g not in [%g,%g]", i, out[i], lo, hi)
		}
	}

	// Equals SMA of (close - open), bit for bit (same arithmetic as the C).
	body := make([]float64, len(w.C))
	for i := range body {
		body[i] = w.C[i] - w.O[i]
	}
	want, _ := SMA(body, 10)
	assertReal(t, "QSTICK == SMA(body)", out, want, 0)
}

func TestQSTICKRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := qstickRange(-1, 5, s.O, s.C, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := qstickRange(0, -1, s.O, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := qstickRange(5, 4, s.O, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := qstickRange(0, 1, s.O, s.C, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := qstickRange(0, 10, s.O[:5], s.C, 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short open: %v", err)
	}
	if _, _, err := qstickRange(0, 10, s.O, s.C[:5], 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short close: %v", err)
	}
	if _, _, err := qstickRange(0, 10, s.O, s.C, 3, out[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Sub-range values equal the full computation.
	full, _ := QSTICK(s.O, s.C, 4)
	beg, nb, err = qstickRange(10, 20, s.O, s.C, 4, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[10:21], 0)
	// In-place computation over either input is allowed.
	for _, which := range []string{"open", "close"} {
		o := append([]float64(nil), s.O...)
		c := append([]float64(nil), s.C...)
		buf := o
		if which == "close" {
			buf = c
		}
		beg, nb, _ = qstickRange(0, 29, o, c, 4, buf)
		alignReal(buf, beg, nb)
		assertReal(t, "in-place "+which, buf, full, 0)
	}
}
