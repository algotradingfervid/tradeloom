package ta

import (
	"errors"
	"math"
	"testing"
)

func TestMINMAXINDEXParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, _, err := MINMAXINDEX(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := MINMAXINDEXLookback(p); lb != -1 {
			t.Errorf("MINMAXINDEXLookback(%d) = %d, want -1", p, lb)
		}
	}
	for _, p := range []int{2, 3, 30, 100000} {
		if _, _, err := MINMAXINDEX(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := MINMAXINDEXLookback(p); lb != p-1 {
			t.Errorf("MINMAXINDEXLookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	for _, in := range [][]float64{nil, {}} {
		mn, mx, err := MINMAXINDEX(in, 5)
		if err != nil || len(mn) != 0 || len(mx) != 0 {
			t.Errorf("empty input: min=%v max=%v err=%v, want empty and nil", mn, mx, err)
		}
	}
}

func TestMINMAXINDEXLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 30} {
		lb := MINMAXINDEXLookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				mn, mx, err := MINMAXINDEX(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(mn) != n || len(mx) != n {
					t.Fatalf("%s n=%d p=%d: len(min)=%d len(max)=%d", s.Name, n, period, len(mn), len(mx))
				}
				in := s.Real()
				for i := range n {
					if i < lb {
						if mn[i] != 0 || mx[i] != 0 {
							t.Fatalf("%s n=%d p=%d: [%d] = (%d,%d), want 0 inside lookback", s.Name, n, period, i, mn[i], mx[i])
						}
						continue
					}
					// Each index lies in the window and points at its extreme.
					if mn[i] < i-lb || mn[i] > i || mx[i] < i-lb || mx[i] > i {
						t.Fatalf("%s n=%d p=%d: [%d] = (%d,%d) outside window", s.Name, n, period, i, mn[i], mx[i])
					}
					for k := i - lb; k <= i; k++ {
						if in[k] < in[mn[i]] || in[k] > in[mx[i]] {
							t.Fatalf("%s n=%d p=%d: [%d] = (%d,%d) not the window extremes", s.Name, n, period, i, mn[i], mx[i])
						}
					}
				}
			}
		}
	}
}

func TestMINMAXINDEXKnownValues(t *testing.T) {
	in := []float64{3, 1, 4, 1, 5, 9, 2, 6}
	mn, mx, err := MINMAXINDEX(in, 3)
	if err != nil {
		t.Fatal(err)
	}
	// Derived by hand from the C algorithm: a rescan of the window keeps the
	// first extreme (strict comparison); an incremental update takes the
	// newest bar on a tie (bar 3 ties bar 1 at value 1 and wins).
	assertInt(t, "min", mn, []int{0, 0, 1, 3, 3, 3, 6, 6})
	assertInt(t, "max", mx, []int{0, 0, 2, 2, 4, 5, 5, 5})

	// Constant series: every window is a rescan from its trailing bar
	// (the stored index always falls out), so both indexes are i-(p-1).
	c := fixtures(40)[5].Real()
	mn, mx, _ = MINMAXINDEX(c, 7)
	for i := 6; i < len(c); i++ {
		if mn[i] != i-6 || mx[i] != i-6 {
			t.Fatalf("constant: [%d] = (%d,%d), want %d", i, mn[i], mx[i], i-6)
		}
	}

	// Strictly rising: min is the oldest bar, max the newest. Falling: reverse.
	up := fixtures(40)[7].Real()
	mn, mx, _ = MINMAXINDEX(up, 5)
	down := fixtures(40)[8].Real()
	dmn, dmx, _ := MINMAXINDEX(down, 5)
	for i := 4; i < 40; i++ {
		if mn[i] != i-4 || mx[i] != i || dmn[i] != i || dmx[i] != i-4 {
			t.Fatalf("trend [%d]: up (%d,%d) down (%d,%d)", i, mn[i], mx[i], dmn[i], dmx[i])
		}
	}

	// Period 2 on a series whose bars are compared pairwise.
	mn, mx, _ = MINMAXINDEX([]float64{2, 1, 3, 3}, 2)
	assertInt(t, "p2 min", mn, []int{0, 1, 1, 2})
	assertInt(t, "p2 max", mx, []int{0, 0, 2, 3})
}

func TestMINMAXINDEXRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	mn := make([]int, len(in))
	mx := make([]int, len(in))
	if _, _, err := minmaxindexRange(-1, 5, in, 3, mn, mx); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := minmaxindexRange(5, 4, in, 3, mn, mx); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := minmaxindexRange(0, -1, in, 3, mn, mx); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := minmaxindexRange(0, 5, in, 1, mn, mx); !errors.Is(err, ErrBadParam) {
		t.Errorf("period 1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := minmaxindexRange(0, 1, in, 3, mn, mx)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := minmaxindexRange(0, 10, in[:5], 3, mn, mx); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := minmaxindexRange(0, 10, in, 3, mn[:10], mx); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outMinIdx: err = %v, want ErrBadParam", err)
	}
	if _, _, err := minmaxindexRange(0, 10, in, 3, mn, mx[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outMaxIdx: err = %v, want ErrBadParam", err)
	}
	// Same output buffer for both outputs is rejected, like C.
	if _, _, err := minmaxindexRange(0, 10, in, 3, mn, mn); !errors.Is(err, ErrBadParam) {
		t.Errorf("aliased outputs: err = %v, want ErrBadParam", err)
	}
	// A sub-range matches the full series.
	wmn, wmx, _ := MINMAXINDEX(in, 4)
	beg, nb, err = minmaxindexRange(10, 20, in, 4, mn, mx)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range min", mn[:nb], wmn[10:21])
	assertInt(t, "sub-range max", mx[:nb], wmx[10:21])
}
