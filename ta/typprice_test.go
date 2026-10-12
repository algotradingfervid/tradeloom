package ta

import (
	"errors"
	"math"
	"testing"
)

func TestTYPPRICEParams(t *testing.T) {
	if lb := TYPPRICELookback(); lb != 0 {
		t.Errorf("TYPPRICELookback() = %d, want 0", lb)
	}
	s := fixtures(50)[0]
	// Mismatched input lengths, each input in turn.
	if _, err := TYPPRICE(s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high: err = %v, want ErrBadParam", err)
	}
	if _, err := TYPPRICE(s.H, s.L[:49], s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short low: err = %v, want ErrBadParam", err)
	}
	if _, err := TYPPRICE(s.H, s.L, s.C[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short close: err = %v, want ErrBadParam", err)
	}
	if _, err := TYPPRICE(s.H, nil, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil low with non-empty others: err = %v, want ErrBadParam", err)
	}
	// nil / empty input -> empty output, nil error.
	if out, err := TYPPRICE(nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := TYPPRICE([]float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestTYPPRICELengths(t *testing.T) {
	lb := TYPPRICELookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := TYPPRICE(s.H, s.L, s.C)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			assertLeadingNaN(t, lbl("TYPPRICE ", s.Name, " n=", n), out, lb)
		}
	}
}

func TestTYPPRICEKnownValues(t *testing.T) {
	h := []float64{12, 6, 9, 3.5}
	l := []float64{6, 0, 3, -0.5}
	c := []float64{9, 3, 9, 0.5}
	out, err := TYPPRICE(h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "TYPPRICE hand", out, []float64{9, 3, 7, 3.5 / 3}, 0)

	// A constant series maps to itself; all-zero maps to zero.
	for _, s := range []series{fixtures(40)[5], fixtures(40)[6]} {
		out, _ = TYPPRICE(s.H, s.L, s.C)
		for i := range out {
			if out[i] != s.C[i] {
				t.Fatalf("%s: [%d]=%g want %g", s.Name, i, out[i], s.C[i])
			}
		}
	}

	// Bounds: with well-formed bars the typical price lies in [low, high].
	for _, s := range fixtures(300) {
		if s.Name == "negative" {
			continue
		}
		out, _ = TYPPRICE(s.H, s.L, s.C)
		for i := range out {
			if out[i] < s.L[i]*(1-1e-15) || out[i] > s.H[i]*(1+1e-15) {
				t.Fatalf("%s: [%d]=%g outside [%g, %g]", s.Name, i, out[i], s.L[i], s.H[i])
			}
		}
	}

	// NaN in one bar propagates only to that bar.
	h2 := []float64{3, math.NaN(), 3}
	out, _ = TYPPRICE(h2, []float64{0, 0, 0}, []float64{0, 0, 0})
	assertReal(t, "TYPPRICE NaN", out, []float64{1, math.NaN(), 1}, 0)
}

func TestTYPPRICERangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := typpriceRange(-1, 5, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := typpriceRange(5, 4, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := typpriceRange(0, -1, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// No lookback: a single bar at 0 produces one value.
	beg, nb, err := typpriceRange(0, 0, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 1 {
		t.Errorf("single bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	beg, nb, err = typpriceRange(7, 12, s.H, s.L, s.C, out)
	if err != nil || beg != 7 || nb != 6 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := typpriceRange(0, 10, s.H[:5], s.L, s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := typpriceRange(0, 10, s.H, s.L, s.C[:10], out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short close buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := typpriceRange(0, 10, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// In-place computation over an input is allowed.
	want, _ := TYPPRICE(s.H, s.L, s.C)
	buf := append([]float64(nil), s.C...)
	beg, nb, _ = typpriceRange(0, len(buf)-1, s.H, s.L, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
