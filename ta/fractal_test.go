package ta

import (
	"errors"
	"math"
	"testing"
)

func TestFRACTALParams(t *testing.T) {
	s := fixtures(50)[0]
	bad := []int{0, -1, 100001, math.MinInt32}
	for _, p := range bad {
		if _, _, err := FRACTAL(s.H, s.L, p, 2); !errors.Is(err, ErrBadParam) {
			t.Errorf("left %d: err = %v, want ErrBadParam", p, err)
		}
		if _, _, err := FRACTAL(s.H, s.L, 2, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("right %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := FRACTALLookback(p, 2); lb != -1 {
			t.Errorf("FRACTALLookback(%d,2) = %d, want -1", p, lb)
		}
		if lb := FRACTALLookback(2, p); lb != -1 {
			t.Errorf("FRACTALLookback(2,%d) = %d, want -1", p, lb)
		}
		out := make([]int, 50)
		out2 := make([]int, 50)
		if _, _, err := fractalRange(0, 49, s.H, s.L, p, 2, out, out2); !errors.Is(err, ErrBadParam) {
			t.Errorf("range left %d: err = %v, want ErrBadParam", p, err)
		}
		if _, _, err := fractalRange(0, 49, s.H, s.L, 2, p, out, out2); !errors.Is(err, ErrBadParam) {
			t.Errorf("range right %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range [][2]int{{1, 1}, {1, 100000}, {100000, 1}, {100000, 100000}, {2, 2}} {
		hi, lo, err := FRACTAL(s.H, s.L, p[0], p[1])
		if err != nil {
			t.Errorf("params %v: unexpected err %v", p, err)
		}
		if len(hi) != 50 || len(lo) != 50 {
			t.Errorf("params %v: len %d/%d", p, len(hi), len(lo))
		}
		if lb := FRACTALLookback(p[0], p[1]); lb != p[0]+p[1] {
			t.Errorf("FRACTALLookback%v = %d", p, lb)
		}
	}
	// Mismatched input lengths.
	if _, _, err := FRACTAL(s.H, s.L[:49], 2, 2); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, _, err := FRACTAL(nil, s.L, 2, 2); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil high, non-empty low: err = %v, want ErrBadParam", err)
	}
	// nil / empty input -> empty output.
	if hi, lo, err := FRACTAL(nil, nil, 2, 2); err != nil || len(hi) != 0 || len(lo) != 0 {
		t.Errorf("nil input: hi=%v lo=%v err=%v, want empty and nil", hi, lo, err)
	}
	if hi, lo, err := FRACTAL([]float64{}, []float64{}, 2, 2); err != nil || len(hi) != 0 || len(lo) != 0 {
		t.Errorf("empty input: hi=%v lo=%v err=%v, want empty and nil", hi, lo, err)
	}
}

func TestFRACTALLengths(t *testing.T) {
	for _, p := range [][2]int{{1, 1}, {2, 2}, {1, 4}, {4, 1}, {7, 3}, {20, 20}} {
		lb := FRACTALLookback(p[0], p[1])
		if lb != p[0]+p[1] {
			t.Fatalf("FRACTALLookback%v = %d", p, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				hi, lo, err := FRACTAL(s.H, s.L, p[0], p[1])
				if err != nil {
					t.Fatalf("%s n=%d p=%v: %v", s.Name, n, p, err)
				}
				if len(hi) != n || len(lo) != n {
					t.Fatalf("%s n=%d p=%v: len %d/%d", s.Name, n, p, len(hi), len(lo))
				}
				for i := range n {
					if i < lb && (hi[i] != 0 || lo[i] != 0) {
						t.Fatalf("%s n=%d p=%v: [%d] = %d/%d inside lookback", s.Name, n, p, i, hi[i], lo[i])
					}
					if (hi[i] != 0 && hi[i] != 100) || (lo[i] != 0 && lo[i] != 100) {
						t.Fatalf("%s n=%d p=%v: [%d] = %d/%d not in {0,100}", s.Name, n, p, i, hi[i], lo[i])
					}
				}
			}
		}
	}
}

func TestFRACTALKnownValues(t *testing.T) {
	// Classic 5-bar fractal (L=2, R=2). Flag at i names pivot bar i-2.
	h := []float64{1, 2, 5, 2, 1, 3, 3, 1, 0}
	l := []float64{5, 4, 1, 4, 5, 2, 2, 6, 7}
	hi, lo, err := FRACTAL(h, l, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	// i=4: bar 2 is a strict swing high (5) and strict swing low (1).
	// i=7/8: bars 5 and 6 tie (3,3 / 2,2): strictness forbids a pivot.
	assertInt(t, "FRACTAL hi", hi, []int{0, 0, 0, 0, 100, 0, 0, 0, 0})
	assertInt(t, "FRACTAL lo", lo, []int{0, 0, 0, 0, 100, 0, 0, 0, 0})

	// Outside bar: swing high and swing low at once (L=1, R=1).
	hi, lo, _ = FRACTAL([]float64{1, 5, 1}, []float64{3, 0, 3}, 1, 1)
	assertInt(t, "outside hi", hi, []int{0, 0, 100})
	assertInt(t, "outside lo", lo, []int{0, 0, 100})

	// Each side is decided on its own: tied high, free low.
	hi, lo, _ = FRACTAL([]float64{1, 5, 5, 1}, []float64{3, 0, 3, 3}, 1, 1)
	assertInt(t, "tie hi", hi, []int{0, 0, 0, 0})
	assertInt(t, "tie lo", lo, []int{0, 0, 100, 0})

	// Asymmetric arms (L=1, R=3): bar 1 must beat bar 0 and bars 2..4.
	hi, lo, _ = FRACTAL([]float64{4, 9, 8, 7, 6, 10}, []float64{4, 1, 2, 3, 0, 5}, 1, 3)
	assertInt(t, "asym hi", hi, []int{0, 0, 0, 0, 100, 0})
	// lo at i=4: bar1 low 1 vs 4,2,3,0 -> 0 (bar 4 is lower); i=5: bar 2 (2) vs 1,3,0,5 -> 0.
	assertInt(t, "asym lo", lo, []int{0, 0, 0, 0, 0, 0})

	// Constant, strictly rising and strictly falling series never pivot.
	for _, idx := range []int{5, 6, 7, 8, 9} {
		s := fixtures(60)[idx]
		hi, lo, _ := FRACTAL(s.H, s.L, 2, 2)
		for i := range hi {
			if hi[i] != 0 || lo[i] != 0 {
				t.Fatalf("%s: [%d] = %d/%d, want 0/0", s.Name, i, hi[i], lo[i])
			}
		}
	}

	// A random walk must produce some pivots on both sides.
	s := fixtures(500)[0]
	hi, lo, _ = FRACTAL(s.H, s.L, 2, 2)
	nh, nl := 0, 0
	for i := range hi {
		nh += hi[i] / 100
		nl += lo[i] / 100
	}
	if nh == 0 || nl == 0 {
		t.Errorf("walk: %d swing highs, %d swing lows; want some of each", nh, nl)
	}
}

func TestFRACTALRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	hi := make([]int, 30)
	lo := make([]int, 30)
	if _, _, err := fractalRange(-1, 5, s.H, s.L, 2, 2, hi, lo); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := fractalRange(5, 4, s.H, s.L, 2, 2, hi, lo); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := fractalRange(0, -1, s.H, s.L, 2, 2, hi, lo); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := fractalRange(0, 3, s.H, s.L, 2, 2, hi, lo)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := fractalRange(0, 29, s.H[:10], s.L, 2, 2, hi, lo); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: %v", err)
	}
	if _, _, err := fractalRange(0, 29, s.H, s.L[:10], 2, 2, hi, lo); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: %v", err)
	}
	if _, _, err := fractalRange(0, 29, s.H, s.L, 2, 2, hi[:5], lo); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outSwingHigh: %v", err)
	}
	if _, _, err := fractalRange(0, 29, s.H, s.L, 2, 2, hi, lo[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outSwingLow: %v", err)
	}
	// Same output buffer twice is rejected, like C.
	if _, _, err := fractalRange(0, 29, s.H, s.L, 2, 2, hi, hi); !errors.Is(err, ErrBadParam) {
		t.Errorf("aliased outputs: %v", err)
	}
	// A sub-range matches the full computation.
	wantHi, wantLo, _ := FRACTAL(s.H, s.L, 2, 2)
	beg, nb, err = fractalRange(2, 20, s.H, s.L, 2, 2, hi, lo)
	if err != nil || beg != 4 || nb != 17 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertInt(t, "sub-range hi", hi[:nb], wantHi[beg:beg+nb])
	assertInt(t, "sub-range lo", lo[:nb], wantLo[beg:beg+nb])
}
