package ta

import (
	"errors"
	"math"
	"testing"
)

func TestMFIParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := MFI(s.H, s.L, s.C, s.V, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := MFILookback(p); lb != -1 {
			t.Errorf("MFILookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := mfiRange(0, 49, s.H, s.L, s.C, s.V, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, err := MFI(s.H, s.L, s.C, s.V, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := MFILookback(p); lb != p {
			t.Errorf("MFILookback(%d) = %d, want %d", p, lb, p)
		}
	}
	// Mismatched input lengths, each input in turn.
	if _, err := MFI(s.H[:49], s.L, s.C, s.V, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high: err = %v, want ErrBadParam", err)
	}
	if _, err := MFI(s.H, s.L[:49], s.C, s.V, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short low: err = %v, want ErrBadParam", err)
	}
	if _, err := MFI(s.H, s.L, s.C[:49], s.V, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short close: err = %v, want ErrBadParam", err)
	}
	if _, err := MFI(s.H, s.L, s.C, s.V[:49], 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short volume: err = %v, want ErrBadParam", err)
	}
	if _, err := MFI(s.H, s.L, s.C, nil, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil volume with non-empty others: err = %v, want ErrBadParam", err)
	}
	// nil / empty input -> empty output, nil error.
	if out, err := MFI(nil, nil, nil, nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	e := []float64{}
	if out, err := MFI(e, e, e, e, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// A bad period is reported even on empty input.
	if _, err := MFI(nil, nil, nil, nil, 1); !errors.Is(err, ErrBadParam) {
		t.Errorf("empty input, bad period: err = %v, want ErrBadParam", err)
	}
}

func TestMFILengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 51} {
		lb := MFILookback(period)
		if lb != period {
			t.Fatalf("MFILookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := MFI(s.H, s.L, s.C, s.V, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("MFI ", s.Name, " n=", n, " p=", period), out, lb)
				for i := lb; i < n; i++ {
					if out[i] < 0 || out[i] > 100 {
						t.Fatalf("%s n=%d p=%d: [%d]=%g outside [0,100]", s.Name, n, period, i, out[i])
					}
				}
			}
		}
	}
}

func TestMFIKnownValues(t *testing.T) {
	nan := math.NaN()
	// high == low == close, so the typical price is the close.
	// TP: 10 11 10 12 12 12 12; flows: -, +22, -30, +48, 0, 0, 0.
	c := []float64{10, 11, 10, 12, 12, 12, 12}
	v := []float64{1, 2, 3, 4, 5, 6, 7}
	out, err := MFI(c, c, c, v, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{nan, nan, nan,
		100 * (70.0 / 100.0), // bars 1..3: pos 22+48, neg 30
		100 * (48.0 / 78.0),  // bars 2..4: pos 48, neg 30
		100,                  // bars 3..5: pos 48, neg 0
		0,                    // bars 4..6: no money flow at all -> 0
	}
	assertReal(t, "MFI known", out, want, 1e-15)

	// Typical price uses all three components: (3+1+2)/3=2, then (6+2+4)/3=4
	// (up, flow 4*10), then (3+1+2)/3=2 (down, flow 2*10).
	h := []float64{3, 6, 3}
	l := []float64{1, 2, 1}
	cl := []float64{2, 4, 2}
	vv := []float64{10, 10, 10}
	out, _ = MFI(h, l, cl, vv, 2)
	assertReal(t, "MFI typical price", out, []float64{nan, nan, 100 * (40.0 / 60.0)}, 1e-15)

	// Constant series: no money flow -> 0. Rising -> 100. Falling -> 0.
	for _, tc := range []struct {
		idx  int
		want float64
	}{{5, 0}, {6, 0}, {7, 100}, {8, 0}} {
		s := fixtures(60)[tc.idx]
		out, _ := MFI(s.H, s.L, s.C, s.V, 7)
		for i := 7; i < len(out); i++ {
			if out[i] != tc.want {
				t.Fatalf("%s: [%d]=%g want %g", s.Name, i, out[i], tc.want)
			}
		}
	}

	// Zero volume everywhere -> 0; scaling the volume by a power of two leaves
	// the index unchanged.
	s := fixtures(80)[12]
	out, _ = MFI(s.H, s.L, s.C, s.V, 10)
	for i := 10; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("zero volume: [%d]=%g", i, out[i])
		}
	}
	s = fixtures(80)[0]
	base, _ := MFI(s.H, s.L, s.C, s.V, 10)
	v2 := make([]float64, len(s.V))
	for i := range v2 {
		v2[i] = s.V[i] * 4
	}
	scaled, _ := MFI(s.H, s.L, s.C, v2, 10)
	assertReal(t, "MFI volume scaling", scaled, base, 0)
}

func TestMFIRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := mfiRange(-1, 5, s.H, s.L, s.C, s.V, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := mfiRange(5, 4, s.H, s.L, s.C, s.V, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := mfiRange(0, -1, s.H, s.L, s.C, s.V, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (lookback 3 -> bars 0..2): no output.
	beg, nb, err := mfiRange(0, 2, s.H, s.L, s.C, s.V, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	for i, bufs := range [][4][]float64{
		{s.H[:5], s.L, s.C, s.V}, {s.H, s.L[:5], s.C, s.V},
		{s.H, s.L, s.C[:5], s.V}, {s.H, s.L, s.C, s.V[:5]},
	} {
		if _, _, err := mfiRange(0, 10, bufs[0], bufs[1], bufs[2], bufs[3], 3, out); !errors.Is(err, ErrBadParam) {
			t.Errorf("short input %d: err = %v, want ErrBadParam", i, err)
		}
	}
	if _, _, err := mfiRange(0, 29, s.H, s.L, s.C, s.V, 3, make([]float64, 29)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: err = %v, want ErrBadParam", err)
	}
	// A sub-range matches the full computation.
	full, _ := MFI(s.H, s.L, s.C, s.V, 4)
	beg, nb, err = mfiRange(10, 20, s.H, s.L, s.C, s.V, 4, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// The sub-range rebuilds its running sums from bar 6, so they differ from
	// the full run's add-then-subtract sums in the last ulps (C does the same).
	assertReal(t, "MFI sub-range", out[:nb], full[10:21], relTol)
	// In-place: the output may alias an input.
	buf := append([]float64(nil), s.C...)
	beg, nb, _ = mfiRange(0, len(buf)-1, s.H, s.L, buf, s.V, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "MFI in-place", buf, full, 0)
}
