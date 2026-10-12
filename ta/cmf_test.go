package ta

import (
	"errors"
	"math"
	"testing"
)

func TestCMFParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := CMF(s.H, s.L, s.C, s.V, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := CMFLookback(p); lb != -1 {
			t.Errorf("CMFLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := cmfRange(0, 49, s.H, s.L, s.C, s.V, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 20, 100000} {
		if _, err := CMF(s.H, s.L, s.C, s.V, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := CMFLookback(p); lb != p-1 {
			t.Errorf("CMFLookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	// Mismatched input lengths, each input in turn.
	if _, err := CMF(s.H[:49], s.L, s.C, s.V, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high: err = %v, want ErrBadParam", err)
	}
	if _, err := CMF(s.H, s.L[:49], s.C, s.V, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short low: err = %v, want ErrBadParam", err)
	}
	if _, err := CMF(s.H, s.L, s.C[:49], s.V, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short close: err = %v, want ErrBadParam", err)
	}
	if _, err := CMF(s.H, s.L, s.C, s.V[:49], 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short volume: err = %v, want ErrBadParam", err)
	}
	if _, err := CMF(s.H, s.L, s.C, nil, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil volume with non-empty others: err = %v, want ErrBadParam", err)
	}
	// nil / empty input -> empty output, nil error.
	if out, err := CMF(nil, nil, nil, nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	e := []float64{}
	if out, err := CMF(e, e, e, e, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// A bad period is reported even on empty input.
	if _, err := CMF(nil, nil, nil, nil, 1); !errors.Is(err, ErrBadParam) {
		t.Errorf("empty input, bad period: err = %v, want ErrBadParam", err)
	}
}

func TestCMFLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 20, 51} {
		lb := CMFLookback(period)
		if lb != period-1 {
			t.Fatalf("CMFLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := CMF(s.H, s.L, s.C, s.V, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("CMF ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestCMFKnownValues(t *testing.T) {
	// Per-bar money flow volume:
	// Bar 0: ((8-6)-(10-8))/4 * 100 = 0
	// Bar 1: ((10-6)-(10-10))/4 * 50 = 50
	// Bar 2: ((6-6)-(10-6))/4 * 20 = -20
	// Bar 3: high == low -> 0 (volume 30 still counts)
	// Bar 4: ((9-6)-(10-9))/4 * 8 = 4
	// Bar 5: high < low (malformed) -> 0 (volume 2 still counts)
	// Period 2 windows: [0,1] 50/150, [1,2] 30/70, [2,3] -20/50, [3,4] 4/38, [4,5] 4/10.
	h := []float64{10, 10, 10, 7, 10, 5}
	l := []float64{6, 6, 6, 7, 6, 9}
	c := []float64{8, 10, 6, 7, 9, 7}
	v := []float64{100, 50, 20, 30, 8, 2}
	out, err := CMF(h, l, c, v, 2)
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	assertReal(t, "CMF hand p=2", out, []float64{nan, 50.0 / 150, 30.0 / 70, -20.0 / 50, 4.0 / 38, 4.0 / 10}, 1e-15)
	// Period 3 windows: [0..2] 30/170, [1..3] 30/100, [2..4] -16/58, [3..5] 4/40.
	out, _ = CMF(h, l, c, v, 3)
	assertReal(t, "CMF hand p=3", out, []float64{nan, nan, 30.0 / 170, 30.0 / 100, -16.0 / 58, 4.0 / 40}, 1e-15)

	// Closing at the high on every bar gives +1, at the low -1.
	s := fixtures(60)[0]
	out, _ = CMF(s.H, s.L, s.H, s.V, 10)
	for i := 9; i < len(out); i++ {
		if math.Abs(out[i]-1) > 1e-12 {
			t.Fatalf("close at high: [%d]=%g want 1", i, out[i])
		}
	}
	out, _ = CMF(s.H, s.L, s.L, s.V, 10)
	for i := 9; i < len(out); i++ {
		if math.Abs(out[i]+1) > 1e-12 {
			t.Fatalf("close at low: [%d]=%g want -1", i, out[i])
		}
	}

	// Constant (zero range) and all-zero (zero volume) series give exactly 0.
	for _, s := range []series{fixtures(40)[5], fixtures(40)[6], fixtures(40)[12]} {
		out, _ = CMF(s.H, s.L, s.C, s.V, 7)
		for i := 6; i < len(out); i++ {
			if out[i] != 0 {
				t.Fatalf("%s: [%d]=%g want 0", s.Name, i, out[i])
			}
		}
	}

	// Bounds: on well-formed bars with non-negative volume CMF is in [-1, 1].
	for _, s := range fixtures(300) {
		for _, p := range []int{2, 14, 20} {
			out, _ = CMF(s.H, s.L, s.C, s.V, p)
			for i := p - 1; i < len(out); i++ {
				if out[i] < -1-1e-9 || out[i] > 1+1e-9 {
					t.Fatalf("%s p=%d: [%d]=%g outside [-1,1]", s.Name, p, i, out[i])
				}
			}
		}
	}

	// A NaN volume makes sumVol NaN for good (it is never subtracted out to a
	// number), and NaN > 0 is false, so every later output is 0, never NaN.
	out, _ = CMF([]float64{2, 2, 2, 2, 2}, []float64{0, 0, 0, 0, 0}, []float64{2, 2, 2, 2, 2},
		[]float64{1, 1, nan, 1, 1}, 2)
	assertReal(t, "CMF NaN volume", out, []float64{nan, 1, 0, 0, 0}, 0)
}

func TestCMFRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := cmfRange(-1, 5, s.H, s.L, s.C, s.V, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cmfRange(5, 4, s.H, s.L, s.C, s.V, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cmfRange(0, -1, s.H, s.L, s.C, s.V, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := cmfRange(0, 1, s.H, s.L, s.C, s.V, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Start inside the lookback is moved up to it.
	full, _ := CMF(s.H, s.L, s.C, s.V, 5)
	beg, nb, err = cmfRange(1, 29, s.H, s.L, s.C, s.V, 5, out)
	if err != nil || beg != 4 || nb != 26 {
		t.Errorf("start in lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "start in lookback", out[:nb], full[4:], 1e-12)
	// A single bar after the lookback.
	beg, nb, err = cmfRange(10, 10, s.H, s.L, s.C, s.V, 5, out)
	if err != nil || beg != 10 || nb != 1 {
		t.Errorf("single bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "single bar", out[:1], full[10:11], 1e-12)
	// Too-short buffers.
	if _, _, err := cmfRange(0, 10, s.H[:5], s.L, s.C, s.V, 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := cmfRange(0, 10, s.H, s.L, s.C, s.V[:10], 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short volume buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := cmfRange(0, 10, s.H, s.L, s.C, s.V, 3, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// In-place computation over any input is allowed.
	for k := 0; k < 4; k++ {
		in := [][]float64{
			append([]float64(nil), s.H...), append([]float64(nil), s.L...),
			append([]float64(nil), s.C...), append([]float64(nil), s.V...),
		}
		buf := in[k]
		beg, nb, _ = cmfRange(0, len(buf)-1, in[0], in[1], in[2], in[3], 5, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place input ", k), buf, full, 0)
	}
}
