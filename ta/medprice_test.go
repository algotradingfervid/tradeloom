package ta

import (
	"errors"
	"math"
	"testing"
)

func TestMEDPRICEParams(t *testing.T) {
	if lb := MEDPRICELookback(); lb != 0 {
		t.Errorf("MEDPRICELookback() = %d, want 0", lb)
	}
	s := fixtures(50)[0]
	// Mismatched input lengths, each input in turn.
	if _, err := MEDPRICE(s.H[:49], s.L); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high: err = %v, want ErrBadParam", err)
	}
	if _, err := MEDPRICE(s.H, s.L[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short low: err = %v, want ErrBadParam", err)
	}
	if _, err := MEDPRICE(s.H, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil low with non-empty high: err = %v, want ErrBadParam", err)
	}
	if _, err := MEDPRICE(nil, s.L); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil high with non-empty low: err = %v, want ErrBadParam", err)
	}
	// nil / empty input -> empty output, nil error.
	if out, err := MEDPRICE(nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := MEDPRICE([]float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestMEDPRICELengths(t *testing.T) {
	lb := MEDPRICELookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := MEDPRICE(s.H, s.L)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			assertLeadingNaN(t, lbl("MEDPRICE ", s.Name, " n=", n), out, lb)
		}
	}
}

func TestMEDPRICEKnownValues(t *testing.T) {
	h := []float64{12, 6, 9, 3.5, 1}
	l := []float64{6, 0, 3, -0.5, -3}
	out, err := MEDPRICE(h, l)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "MEDPRICE hand", out, []float64{9, 3, 6, 1.5, -1}, 0)

	// A constant series maps to itself; all-zero maps to zero.
	for _, s := range []series{fixtures(40)[5], fixtures(40)[6]} {
		out, _ = MEDPRICE(s.H, s.L)
		for i := range out {
			if out[i] != s.C[i] {
				t.Fatalf("%s: [%d]=%g want %g", s.Name, i, out[i], s.C[i])
			}
		}
	}

	// Bounds: the median price lies in [min(high,low), max(high,low)] and
	// is symmetric in its two inputs.
	for _, s := range fixtures(300) {
		out, _ = MEDPRICE(s.H, s.L)
		sw, _ := MEDPRICE(s.L, s.H)
		for i := range out {
			lo, hi := math.Min(s.H[i], s.L[i]), math.Max(s.H[i], s.L[i])
			if out[i] < lo || out[i] > hi {
				t.Fatalf("%s: [%d]=%g outside [%g, %g]", s.Name, i, out[i], lo, hi)
			}
			if out[i] != sw[i] {
				t.Fatalf("%s: [%d] not symmetric: %g vs %g", s.Name, i, out[i], sw[i])
			}
		}
	}

	// NaN in one bar propagates only to that bar.
	h2 := []float64{4, math.NaN(), 4}
	out, _ = MEDPRICE(h2, []float64{0, 0, 0})
	assertReal(t, "MEDPRICE NaN", out, []float64{2, math.NaN(), 2}, 0)
}

func TestMEDPRICERangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := medpriceRange(-1, 5, s.H, s.L, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := medpriceRange(5, 4, s.H, s.L, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := medpriceRange(0, -1, s.H, s.L, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// No lookback: a single bar at 0 produces one value.
	beg, nb, err := medpriceRange(0, 0, s.H, s.L, out)
	if err != nil || beg != 0 || nb != 1 {
		t.Errorf("single bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	beg, nb, err = medpriceRange(7, 12, s.H, s.L, out)
	if err != nil || beg != 7 || nb != 6 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := medpriceRange(0, 10, s.H[:5], s.L, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := medpriceRange(0, 10, s.H, s.L[:10], out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short low buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := medpriceRange(0, 10, s.H, s.L, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// In-place computation over an input is allowed.
	want, _ := MEDPRICE(s.H, s.L)
	buf := append([]float64(nil), s.L...)
	beg, nb, _ = medpriceRange(0, len(buf)-1, s.H, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
