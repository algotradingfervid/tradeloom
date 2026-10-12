package ta

import (
	"errors"
	"math"
	"testing"
)

func TestAVGPRICEParams(t *testing.T) {
	if lb := AVGPRICELookback(); lb != 0 {
		t.Errorf("AVGPRICELookback() = %d, want 0", lb)
	}
	s := fixtures(50)[0]
	// Mismatched input lengths, each input in turn.
	if _, err := AVGPRICE(s.O[:49], s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short open: err = %v, want ErrBadParam", err)
	}
	if _, err := AVGPRICE(s.O, s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high: err = %v, want ErrBadParam", err)
	}
	if _, err := AVGPRICE(s.O, s.H, s.L[:49], s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short low: err = %v, want ErrBadParam", err)
	}
	if _, err := AVGPRICE(s.O, s.H, s.L, s.C[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short close: err = %v, want ErrBadParam", err)
	}
	if _, err := AVGPRICE(nil, s.H, s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil open with non-empty others: err = %v, want ErrBadParam", err)
	}
	// nil / empty input -> empty output, nil error.
	if out, err := AVGPRICE(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := AVGPRICE([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestAVGPRICELengths(t *testing.T) {
	lb := AVGPRICELookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := AVGPRICE(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			assertLeadingNaN(t, lbl("AVGPRICE ", s.Name, " n=", n), out, lb)
		}
	}
}

func TestAVGPRICEKnownValues(t *testing.T) {
	o := []float64{10, 1, 4, 0}
	h := []float64{12, 6, 9, 3.5}
	l := []float64{6, 0, 3, -0.5}
	c := []float64{8, 1, 8, 1}
	out, err := AVGPRICE(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "AVGPRICE hand", out, []float64{9, 2, 6, 1}, 0)

	// A constant series maps to itself; all-zero maps to zero.
	for _, s := range []series{fixtures(40)[5], fixtures(40)[6]} {
		out, _ = AVGPRICE(s.O, s.H, s.L, s.C)
		for i := range out {
			if out[i] != s.C[i] {
				t.Fatalf("%s: [%d]=%g want %g", s.Name, i, out[i], s.C[i])
			}
		}
	}

	// Bounds: with well-formed bars the average price lies in [low, high].
	for _, s := range fixtures(300) {
		if s.Name == "negative" {
			continue
		}
		out, _ = AVGPRICE(s.O, s.H, s.L, s.C)
		for i := range out {
			if out[i] < s.L[i]*(1-1e-15) || out[i] > s.H[i]*(1+1e-15) {
				t.Fatalf("%s: [%d]=%g outside [%g, %g]", s.Name, i, out[i], s.L[i], s.H[i])
			}
		}
	}

	// NaN in one bar propagates only to that bar.
	z := []float64{0, 0, 0}
	out, _ = AVGPRICE([]float64{4, math.NaN(), 4}, z, z, z)
	assertReal(t, "AVGPRICE NaN", out, []float64{1, math.NaN(), 1}, 0)
}

func TestAVGPRICERangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := avgpriceRange(-1, 5, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := avgpriceRange(5, 4, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := avgpriceRange(0, -1, s.O, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// No lookback: a single bar at 0 produces one value (nothing lies inside the lookback).
	beg, nb, err := avgpriceRange(0, 0, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 0 || nb != 1 {
		t.Errorf("single bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	beg, nb, err = avgpriceRange(7, 12, s.O, s.H, s.L, s.C, out)
	if err != nil || beg != 7 || nb != 6 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := avgpriceRange(0, 10, s.O[:5], s.H, s.L, s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short open buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := avgpriceRange(0, 10, s.O, s.H, s.L, s.C[:10], out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short close buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := avgpriceRange(0, 10, s.O, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// In-place computation over an input is allowed.
	want, _ := AVGPRICE(s.O, s.H, s.L, s.C)
	buf := append([]float64(nil), s.C...)
	beg, nb, _ = avgpriceRange(0, len(buf)-1, s.O, s.H, s.L, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
