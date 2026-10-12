package ta

import (
	"errors"
	"math"
	"testing"
)

func TestRVOLParams(t *testing.T) {
	in := fixtures(50)[0].V
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := RVOL(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := RVOLLookback(p); lb != -1 {
			t.Errorf("RVOLLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := rvolRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 20, 100000} {
		if _, err := RVOL(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := RVOLLookback(p); lb != p {
			t.Errorf("RVOLLookback(%d) = %d, want %d", p, lb, p)
		}
	}
	if out, err := RVOL(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := RVOL([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Parameter validation precedes the empty-input shortcut.
	if _, err := RVOL(nil, 0); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input bad period: err = %v, want ErrBadParam", err)
	}
	if _, _, err := rvolRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := rvolRange(0, 10, in, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := rvolRange(5, 10, in, 3, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

// rvolDeadVolume reports fixtures whose volume window can be all zero, where
// RVOL legitimately emits NaN/Inf (nan_inf_output flag).
func rvolDeadVolume(name string) bool {
	return name == "zeros" || name == "walk-zero-vol"
}

func TestRVOLLengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 20, 30} {
		lb := RVOLLookback(period)
		if lb != period {
			t.Fatalf("RVOLLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := RVOL(s.V, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				label := lbl("RVOL ", s.Name, " n=", n, " p=", period)
				if rvolDeadVolume(s.Name) {
					// 0/0: every element is NaN, inside and after the lookback.
					for i, v := range out {
						if !math.IsNaN(v) {
							t.Fatalf("%s: [%d]=%g, want NaN on zero volume", label, i, v)
						}
					}
					continue
				}
				assertLeadingNaN(t, label, out, lb)
			}
		}
	}
}

func TestRVOLKnownValues(t *testing.T) {
	nan := math.NaN()
	// Period 2: baseline is the mean of the two preceding bars.
	// bar2: 30/((10+20)/2)=2; bar3: 40/((20+30)/2)=1.6; bar4: 10/((30+40)/2)=2/7.
	in := []float64{10, 20, 30, 40, 10}
	out, err := RVOL(in, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "RVOL period 2", out, []float64{nan, nan, 2, 1.6, 10.0 / 35.0}, 1e-15)

	// Period 1: ratio to the previous bar.
	out, _ = RVOL(in, 1)
	assertReal(t, "RVOL period 1", out, []float64{nan, 2, 1.5, 40.0 / 30.0, 0.25}, 1e-15)

	// A spike is measured against the preceding window only, at full size.
	spike := []float64{100, 100, 100, 100, 500}
	out, _ = RVOL(spike, 4)
	assertReal(t, "RVOL spike", out, []float64{nan, nan, nan, nan, 5}, 0)

	// A constant series has relative volume exactly 1.
	c := fixtures(40)[5].V
	out, _ = RVOL(c, 7)
	for i := 7; i < len(out); i++ {
		if out[i] != 1 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Dead window: zero baseline gives +Inf for a positive bar, NaN for a zero bar.
	dead := []float64{0, 0, 0, 5, 0}
	out, _ = RVOL(dead, 3)
	if !math.IsInf(out[3], 1) {
		t.Errorf("dead window, positive bar: got %g, want +Inf", out[3])
	}
	out, _ = RVOL([]float64{0, 0, 0, 0}, 3)
	if !math.IsNaN(out[3]) {
		t.Errorf("dead window, zero bar: got %g, want NaN", out[3])
	}

	// Non-negative volume gives a non-negative ratio.
	for _, s := range fixtures(300) {
		if rvolDeadVolume(s.Name) || s.Name == "negative" {
			continue
		}
		out, _ := RVOL(s.V, 14)
		for i := 14; i < len(out); i++ {
			if !(out[i] >= 0) {
				t.Fatalf("%s: [%d]=%g, want >= 0", s.Name, i, out[i])
			}
		}
	}
}

func TestRVOLRangeForm(t *testing.T) {
	in := fixtures(30)[0].V
	out := make([]float64, len(in))
	if _, _, err := rvolRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := rvolRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := rvolRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (bars 0..2 for period 3): no output, no error.
	beg, nb, err := rvolRange(0, 2, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// First bar after the lookback.
	beg, nb, err = rvolRange(0, 3, in, 3, out)
	if err != nil || beg != 3 || nb != 1 {
		t.Errorf("first output: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// A sub-range matches the full computation.
	full, _ := RVOL(in, 3)
	beg, nb, _ = rvolRange(10, 20, in, 3, out)
	if beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d", beg, nb)
	}
	assertReal(t, "sub-range", out[:nb], full[10:21], 0)
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := RVOL(in, 4)
	beg, nb, _ = rvolRange(0, len(buf)-1, buf, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
