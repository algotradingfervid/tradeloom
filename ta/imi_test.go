package ta

import (
	"errors"
	"math"
	"testing"
)

func TestIMIParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := IMI(s.O, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := IMILookback(p); lb != -1 {
			t.Errorf("IMILookback(%d) = %d, want -1", p, lb)
		}
	}
	for _, p := range []int{2, 14, 100000} {
		if _, err := IMI(s.O, s.C, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := IMILookback(p); lb != p-1 {
			t.Errorf("IMILookback(%d) = %d", p, lb)
		}
	}
	if _, err := IMI(s.O, s.C[:49], 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, err := IMI(nil, s.C, 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil open, non-empty close: err = %v, want ErrBadParam", err)
	}
	if out, err := IMI(nil, nil, 14); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := IMI([]float64{}, []float64{}, 14); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestIMILengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30} {
		lb := IMILookback(period)
		if lb != period-1 {
			t.Fatalf("IMILookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := IMI(s.O, s.C, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("IMI ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestIMIKnownValues(t *testing.T) {
	// Bodies: +2, -1, +3, 0, -4.
	open := []float64{10, 12, 11, 14, 14}
	close := []float64{12, 11, 14, 14, 10}
	out, err := IMI(open, close, 3)
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	// [2]: up 2+3=5, down 1 -> 500/6. [3]: up 3, down 1 -> 75.
	// [4]: up 3, down 0+4 -> 300/7.
	assertReal(t, "IMI p=3", out, []float64{nan, nan, 100 * (5.0 / 6.0), 75, 100 * (3.0 / 7.0)}, 1e-15)

	// All-flat window (#112 guard): 50, not NaN.
	c := fixtures(40)[5]
	out, _ = IMI(c.O, c.C, 7)
	for i := 6; i < len(out); i++ {
		if out[i] != 50 {
			t.Fatalf("constant: [%d]=%g, want 50", i, out[i])
		}
	}
	// Only up bodies -> 100; only down bodies -> 0.
	up := fixtures(40)[7]
	out, _ = IMI(up.O, up.C, 5)
	for i := 4; i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("up: [%d]=%g, want 100", i, out[i])
		}
	}
	down := fixtures(40)[8]
	out, _ = IMI(down.O, down.C, 5)
	for i := 4; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("down: [%d]=%g, want 0", i, out[i])
		}
	}
	// Zigzag with an even period: equal up and down bodies -> 50.
	z := fixtures(40)[9]
	out, _ = IMI(z.O, z.C, 4)
	for i := 3; i < len(out); i++ {
		if out[i] != 50 {
			t.Fatalf("zigzag: [%d]=%g, want 50", i, out[i])
		}
	}
	// Bounds on every fixture.
	for _, s := range fixtures(300) {
		out, _ := IMI(s.O, s.C, 14)
		for i := 13; i < len(out); i++ {
			if !(out[i] >= 0 && out[i] <= 100) {
				t.Fatalf("%s: [%d]=%g out of [0,100]", s.Name, i, out[i])
			}
		}
	}
}

func TestIMIRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, len(s.C))
	if _, _, err := imiRange(-1, 5, s.O, s.C, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := imiRange(5, 4, s.O, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := imiRange(0, -1, s.O, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := imiRange(0, 5, s.O, s.C, 1, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("period 1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := imiRange(0, 1, s.O, s.C, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := imiRange(0, 10, s.O[:5], s.C, 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short open: %v", err)
	}
	if _, _, err := imiRange(0, 10, s.O, s.C[:5], 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short close: %v", err)
	}
	if _, _, err := imiRange(0, 10, s.O, s.C, 3, out[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Sub-range matches the public form.
	want, _ := IMI(s.O, s.C, 5)
	beg, nb, err = imiRange(10, 20, s.O, s.C, 5, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], want[10:21], 0)
}
