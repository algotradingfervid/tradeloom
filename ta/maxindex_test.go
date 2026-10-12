package ta

import (
	"errors"
	"math"
	"testing"
)

func TestMAXINDEXParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := MAXINDEX(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := MAXINDEXLookback(p); lb != -1 {
			t.Errorf("MAXINDEXLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := maxindexRange(0, 49, in, p, make([]int, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 30, 100000} {
		if _, err := MAXINDEX(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := MAXINDEXLookback(p); lb != p-1 {
			t.Errorf("MAXINDEXLookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	if out, err := MAXINDEX(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := MAXINDEX([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

// maxindexCheckWindow asserts out is 0 inside the lookback and, after it,
// points into the window at the window's maximum.
func maxindexCheckWindow(t *testing.T, label string, in []float64, out []int, period int) {
	t.Helper()
	lb := period - 1
	for i, v := range out {
		if i < lb {
			if v != 0 {
				t.Fatalf("%s: [%d]=%d, want 0 inside lookback %d", label, i, v, lb)
			}
			continue
		}
		if v < i-lb || v > i {
			t.Fatalf("%s: [%d]=%d outside window [%d,%d]", label, i, v, i-lb, i)
		}
		for j := i - lb; j <= i; j++ {
			if in[j] > in[v] {
				t.Fatalf("%s: [%d]=%d (%g) but in[%d]=%g is higher", label, i, v, in[v], j, in[j])
			}
		}
	}
}

func TestMAXINDEXLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 30} {
		lb := MAXINDEXLookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := MAXINDEX(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				maxindexCheckWindow(t, lbl("MAXINDEX ", s.Name, " n=", n, " p=", period), s.Real(), out, period)
			}
		}
	}
}

func TestMAXINDEXKnownValues(t *testing.T) {
	in := []float64{1, 5, 3, 2, 4, 0, 6}
	out, err := MAXINDEX(in, 3)
	if err != nil {
		t.Fatal(err)
	}
	// windows: [0..2]->1, [1..3]->1, [2..4]->4, [3..5]->4, [4..6]->6
	assertInt(t, "MAXINDEX p=3", out, []int{0, 0, 1, 1, 4, 4, 6})

	// Strictly rising: the max is always the current bar.
	up := fixtures(40)[7].Real()
	out, _ = MAXINDEX(up, 7)
	for i := 6; i < len(out); i++ {
		if out[i] != i {
			t.Fatalf("up: [%d]=%d", i, out[i])
		}
	}
	// Strictly falling: the max is always the window's first bar.
	down := fixtures(40)[8].Real()
	out, _ = MAXINDEX(down, 7)
	for i := 6; i < len(out); i++ {
		if out[i] != i-6 {
			t.Fatalf("down: [%d]=%d", i, out[i])
		}
	}
	// Constant input: ties. The tracked max always drops out of the window,
	// so every bar rescans with strict > and keeps the window's first index.
	c := fixtures(10)[5].Real()
	out, _ = MAXINDEX(c, 3)
	assertInt(t, "constant", out, []int{0, 0, 0, 1, 2, 3, 4, 5, 6, 7})
	// Tie on the incremental path (>=) takes the newest index.
	out, _ = MAXINDEX([]float64{1, 5, 2, 5, 5}, 4)
	assertInt(t, "tie >=", out, []int{0, 0, 0, 1, 4}) // bar 3: rescan (strict >) keeps 1; bar 4: >= takes 4
	// Period equal to the input length: one output.
	out, _ = MAXINDEX(in, len(in))
	assertInt(t, "period=n", out, []int{0, 0, 0, 0, 0, 0, 6})
	// Period longer than the input: all zero.
	out, _ = MAXINDEX(in, len(in)+1)
	assertInt(t, "period>n", out, make([]int, len(in)))
}

func TestMAXINDEXRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]int, len(in))
	if _, _, err := maxindexRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := maxindexRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := maxindexRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	beg, nb, err := maxindexRange(0, 1, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if _, _, err := maxindexRange(0, 10, in[:5], 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := maxindexRange(0, 10, in, 3, make([]int, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Sub-range values are absolute indices and match the full computation.
	full, _ := MAXINDEX(in, 4)
	beg, nb, err = maxindexRange(10, 20, in, 4, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range", out[:nb], full[10:21])
}
