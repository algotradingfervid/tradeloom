package ta

import (
	"errors"
	"math"
	"testing"
)

func TestVHFParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := VHF(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := VHFLookback(p); lb != -1 {
			t.Errorf("VHFLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := vhfRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 28, 100000} {
		if _, err := VHF(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := VHFLookback(p); lb != p {
			t.Errorf("VHFLookback(%d) = %d, want %d", p, lb, p)
		}
	}
	if out, err := VHF(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := VHF([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := vhfRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := vhfRange(0, 10, in, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
}

func TestVHFLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 28, 56} {
		lb := VHFLookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := VHF(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("VHF ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestVHFKnownValues(t *testing.T) {
	nan := math.NaN()
	// p=3: num = range of 3 newest closes, den = sum of 3 newest |changes|.
	// bar3: closes 2,4,3 -> num 2; changes |2-1|+|4-2|+|3-4| = 4 -> 0.5
	// bar4: closes 4,3,5 -> num 2; changes 2+1+2 = 5 -> 0.4
	// bar5: closes 3,5,6 -> num 3; changes 1+2+1 = 4 -> 0.75
	in := []float64{1, 2, 4, 3, 5, 6}
	out, err := VHF(in, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "VHF hand", out, []float64{nan, nan, nan, 0.5, 0.4, 0.75}, 1e-15)

	// A unit-step trend: num = p-1 (range of p closes), den = p (p changes).
	up := []float64{10, 11, 12, 13, 14, 15, 16}
	out, _ = VHF(up, 4)
	for i := 4; i < len(out); i++ {
		if out[i] != 0.75 {
			t.Fatalf("unit trend: [%d]=%g, want 0.75", i, out[i])
		}
	}

	// A constant (flat) window reports exactly 0, never NaN.
	c := fixtures(40)[5].Real()
	out, _ = VHF(c, 7)
	for i := 7; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Bounds: VHF in [0, 1] on every non-degenerate fixture.
	for _, s := range fixtures(300) {
		out, _ := VHF(s.Real(), 14)
		for i := 14; i < len(out); i++ {
			if !(out[i] >= 0 && out[i] <= 1) {
				t.Fatalf("%s: [%d]=%g outside [0,1]", s.Name, i, out[i])
			}
		}
	}

	// Tiny prices are not zeroed by an epsilon band (issue #253): a
	// unit-step trend scaled to 1e-16 still gives (p-1)/p = 0.5 at p=2.
	tiny := []float64{1e-16, 2e-16, 3e-16, 4e-16}
	out, _ = VHF(tiny, 2)
	assertReal(t, "tiny trend", out, []float64{nan, nan, 0.5, 0.5}, 1e-12)
}

func TestVHFRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := vhfRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := vhfRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := vhfRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := vhfRange(0, 2, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values equal the full computation.
	full, _ := VHF(in, 5)
	beg, nb, err = vhfRange(2, 20, in, 5, out)
	if err != nil || beg != 5 || nb != 16 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[5:21], 0)
}
