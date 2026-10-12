package ta

import (
	"errors"
	"math"
	"testing"
)

func TestMIDPRICEParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := MIDPRICE(s.H, s.L, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := MIDPRICELookback(p); lb != -1 {
			t.Errorf("MIDPRICELookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := midpriceRange(0, 49, s.H, s.L, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, err := MIDPRICE(s.H, s.L, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := MIDPRICELookback(p); lb != p-1 {
			t.Errorf("MIDPRICELookback(%d) = %d", p, lb)
		}
	}
	if _, err := MIDPRICE(s.H, s.L[:49], 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, err := MIDPRICE(nil, s.L, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil high with non-empty low: err = %v, want ErrBadParam", err)
	}
	if out, err := MIDPRICE(nil, nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := MIDPRICE([]float64{}, []float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestMIDPRICELengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30} {
		lb := MIDPRICELookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := MIDPRICE(s.H, s.L, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("MIDPRICE ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

// midpriceNaive is the textbook formula, used to check the block scan.
func midpriceNaive(h, l []float64, p int) []float64 {
	out := make([]float64, len(h))
	for i := range out {
		if i < p-1 {
			out[i] = math.NaN()
			continue
		}
		hi, lo := h[i-p+1], l[i-p+1]
		for j := i - p + 2; j <= i; j++ {
			hi = math.Max(hi, h[j])
			lo = math.Min(lo, l[j])
		}
		out[i] = (hi + lo) / 2
	}
	return out
}

func TestMIDPRICEKnownValues(t *testing.T) {
	nan := math.NaN()
	h := []float64{10, 12, 11, 15, 13, 14}
	l := []float64{8, 9, 7, 10, 12, 11}
	out, err := MIDPRICE(h, l, 3)
	if err != nil {
		t.Fatal(err)
	}
	// Bar 2: (12+7)/2, bar 3: (15+7)/2, bar 4: (15+7)/2, bar 5: (15+10)/2.
	assertReal(t, "MIDPRICE p=3", out, []float64{nan, nan, 9.5, 11, 11, 12.5}, 0)

	// Period 2 hand-check.
	out, _ = MIDPRICE(h, l, 2)
	assertReal(t, "MIDPRICE p=2", out, []float64{nan, (12 + 8) / 2.0, (12 + 7) / 2.0, (15 + 7) / 2.0, (15 + 10) / 2.0, (14 + 11) / 2.0}, 0)

	// High == low == constant gives the constant back.
	c := fixtures(40)[5]
	out, _ = MIDPRICE(c.C, c.C, 7)
	for i := 6; i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Block scan equals the naive formula exactly, across block boundaries.
	for _, s := range fixtures(257) {
		for _, p := range []int{2, 3, 4, 7, 14, 31, 64, 257} {
			got, _ := MIDPRICE(s.H, s.L, p)
			assertReal(t, lbl("naive ", s.Name, " p=", p), got, midpriceNaive(s.H, s.L, p), 0)
			// Bounds: lowest low <= mid <= highest high over the window.
			for i := p - 1; i < len(got); i++ {
				lo, hi := math.Inf(1), math.Inf(-1)
				for j := i - p + 1; j <= i; j++ {
					lo, hi = math.Min(lo, s.L[j]), math.Max(hi, s.H[j])
				}
				if got[i] < lo || got[i] > hi {
					t.Fatalf("%s p=%d [%d]=%g outside [%g, %g]", s.Name, p, i, got[i], lo, hi)
				}
			}
		}
	}
}

func TestMIDPRICERangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := midpriceRange(-1, 5, s.H, s.L, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := midpriceRange(5, 4, s.H, s.L, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := midpriceRange(0, -1, s.H, s.L, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	beg, nb, err := midpriceRange(0, 1, s.H, s.L, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if _, _, err := midpriceRange(0, 10, s.H[:5], s.L, 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := midpriceRange(0, 10, s.H, s.L[:5], 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short low buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := midpriceRange(0, 10, s.H, s.L, 3, out[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Sub-ranges agree with the full series.
	full, _ := MIDPRICE(s.H, s.L, 4)
	for _, rg := range [][2]int{{0, 29}, {3, 3}, {5, 20}, {29, 29}, {2, 9}} {
		o := make([]float64, 30)
		beg, nb, err := midpriceRange(rg[0], rg[1], s.H, s.L, 4, o)
		if err != nil {
			t.Fatal(err)
		}
		wantBeg := rg[0]
		if wantBeg < 3 {
			wantBeg = 3
		}
		if beg != wantBeg || nb != rg[1]-wantBeg+1 {
			t.Fatalf("range %v: beg=%d nb=%d", rg, beg, nb)
		}
		assertReal(t, lbl("sub-range ", rg), o[:nb], full[beg:beg+nb], 0)
	}
	// In-place over the whole series (output aliasing the high input).
	buf := append([]float64(nil), s.H...)
	want, _ := MIDPRICE(s.H, s.L, 5)
	beg, nb, _ = midpriceRange(0, len(buf)-1, buf, s.L, 5, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
