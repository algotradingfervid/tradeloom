package ta

import (
	"errors"
	"math"
	"testing"
)

func TestTSIParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := TSI(in, p, 13); !errors.Is(err, ErrBadParam) {
			t.Errorf("first %d: err = %v, want ErrBadParam", p, err)
		}
		if _, err := TSI(in, 25, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("second %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := TSILookback(p, 13); lb != -1 {
			t.Errorf("TSILookback(%d,13) = %d, want -1", p, lb)
		}
		if lb := TSILookback(25, p); lb != -1 {
			t.Errorf("TSILookback(25,%d) = %d, want -1", p, lb)
		}
		out := make([]float64, len(in))
		if _, _, err := tsiRange(0, len(in)-1, in, p, 13, out); !errors.Is(err, ErrBadParam) {
			t.Errorf("range first %d: err = %v", p, err)
		}
		if _, _, err := tsiRange(0, len(in)-1, in, 25, p, out); !errors.Is(err, ErrBadParam) {
			t.Errorf("range second %d: err = %v", p, err)
		}
	}
	for _, p := range [][2]int{{2, 2}, {2, 100000}, {100000, 2}, {100000, 100000}, {25, 13}} {
		if _, err := TSI(in, p[0], p[1]); err != nil {
			t.Errorf("params %v: unexpected err %v", p, err)
		}
		if lb := TSILookback(p[0], p[1]); lb != p[0]+p[1]-1 {
			t.Errorf("TSILookback%v = %d", p, lb)
		}
	}
	if out, err := TSI(nil, 25, 13); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := TSI([]float64{}, 25, 13); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := tsiRange(0, 10, in[:5], 3, 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := tsiRange(0, 10, in, 3, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
}

func TestTSILengths(t *testing.T) {
	for _, p := range [][2]int{{2, 2}, {3, 2}, {2, 5}, {25, 13}, {40, 20}} {
		lb := TSILookback(p[0], p[1])
		if lb != p[0]+p[1]-1 {
			t.Fatalf("TSILookback%v = %d", p, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := TSI(s.Real(), p[0], p[1])
				if err != nil {
					t.Fatalf("%s n=%d p=%v: %v", s.Name, n, p, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%v: len(out)=%d", s.Name, n, p, len(out))
				}
				assertLeadingNaN(t, lbl("TSI ", s.Name, " n=", n, " p=", p), out, lb)
				for i := lb; i < n; i++ {
					if out[i] < -100-1e-9 || out[i] > 100+1e-9 {
						t.Fatalf("%s n=%d p=%v: out[%d]=%g outside [-100,100]", s.Name, n, p, i, out[i])
					}
				}
			}
		}
	}
}

func TestTSIKnownValues(t *testing.T) {
	// Changes m: 1, 2, -1, 2. First EMA(2) seeds at m[0..1] -> 1.5 (|m| 1.5),
	// then -1/6 (7/6), 23/18 (31/18). Second EMA(2) seeds on the first two
	// stage-1 values -> 2/3 over 4/3 = 50, then 29/27 over 43/27.
	in := []float64{1, 2, 4, 3, 5}
	out, err := TSI(in, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	assertReal(t, "TSI known", out, []float64{nan, nan, nan, 50, 100.0 * 29 / 43}, 1e-12)

	// Constant input: every change is zero -> 0 (guarded 0/0).
	c := fixtures(60)[5].Real()
	out, _ = TSI(c, 5, 3)
	for i := 7; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}
	// Strictly rising -> +100; strictly falling -> -100.
	up := fixtures(60)[7].Real()
	out, _ = TSI(up, 5, 3)
	for i := 7; i < len(out); i++ {
		if math.Abs(out[i]-100) > 1e-9 {
			t.Fatalf("up: [%d]=%g", i, out[i])
		}
	}
	down := fixtures(60)[8].Real()
	out, _ = TSI(down, 5, 3)
	for i := 7; i < len(out); i++ {
		if math.Abs(out[i]+100) > 1e-9 {
			t.Fatalf("down: [%d]=%g", i, out[i])
		}
	}
	// Scale invariance: tiny prices give the same reading as large ones.
	w := fixtures(80)[0].Real()
	scaled := make([]float64, len(w))
	for i, v := range w {
		scaled[i] = v * 1e-9
	}
	a, _ := TSI(w, 10, 5)
	b, _ := TSI(scaled, 10, 5)
	assertReal(t, "scale invariance", b, a, 1e-9)
}

func TestTSIUnstableLookback(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	SetUnstablePeriod(UnstEMA, 7)
	if lb := TSILookback(25, 13); lb != 1+24+7+12+7 {
		t.Fatalf("TSILookback with unstable 7 = %d", lb)
	}
}

func TestTSIRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := tsiRange(-1, 5, in, 3, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := tsiRange(5, 4, in, 3, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := tsiRange(0, -1, in, 3, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (lb = 5): no output, no error.
	beg, nb, err := tsiRange(0, 4, in, 3, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// A sub-range equals the matching slice of the full output.
	beg, nb, err = tsiRange(10, 20, in, 3, 3, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Each range call re-seeds its EMAs at startIdx-lookback, so its first
	// value equals a full run over in[startIdx-lookback:].
	ref, _ := TSI(in[10-5:], 3, 3)
	if !closeEnough(out[0], ref[5], 0) {
		t.Errorf("sub-range first value %g, want %g", out[0], ref[5])
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := TSI(in, 4, 3)
	beg, nb, _ = tsiRange(0, len(buf)-1, buf, 4, 3, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
