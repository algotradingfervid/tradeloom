package ta

import (
	"errors"
	"math"
	"testing"
)

func TestADRParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := ADR(s.H, s.L, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := ADRLookback(p); lb != -1 {
			t.Errorf("ADRLookback(%d) = %d, want -1", p, lb)
		}
	}
	for _, p := range []int{1, 2, 14, 100000} {
		if _, err := ADR(s.H, s.L, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := ADRLookback(p); lb != p-1 {
			t.Errorf("ADRLookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	if _, err := ADR(s.H, s.L[:49], 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, err := ADR(s.H, nil, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil low with non-empty high: err = %v, want ErrBadParam", err)
	}
	if out, err := ADR(nil, nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := ADR([]float64{}, []float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestADRLengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 14, 30} {
		lb := ADRLookback(period)
		if lb != period-1 {
			t.Fatalf("ADRLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := ADR(s.H, s.L, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("ADR ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestADRKnownValues(t *testing.T) {
	// Ranges: 1, 2, 3, 4, 5, 6.
	high := []float64{11, 22, 33, 44, 55, 66}
	low := []float64{10, 20, 30, 40, 50, 60}
	out, err := ADR(high, low, 3)
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	assertReal(t, "ADR(ranges 1..6,3)", out, []float64{nan, nan, 2, 3, 4, 5}, 0)

	// Period 1 is the bar range itself.
	out, _ = ADR(high, low, 1)
	assertReal(t, "ADR period 1", out, []float64{1, 2, 3, 4, 5, 6}, 0)

	// Constant and all-zero series have zero range.
	for _, idx := range []int{5, 6} {
		s := fixtures(40)[idx]
		out, _ = ADR(s.H, s.L, 7)
		for i := 6; i < len(out); i++ {
			if out[i] != 0 {
				t.Fatalf("%s: [%d]=%g, want 0", s.Name, i, out[i])
			}
		}
	}

	// ADR equals SMA(high - low) bit for bit (same accumulation order), and
	// is non-negative on well-formed bars.
	for _, s := range fixtures(300) {
		diff := make([]float64, len(s.H))
		for i := range diff {
			diff[i] = s.H[i] - s.L[i]
		}
		for _, p := range []int{1, 2, 14, 50} {
			got, _ := ADR(s.H, s.L, p)
			want, _ := SMA(diff, p)
			assertReal(t, lbl("ADR vs SMA ", s.Name, " p=", p), got, want, 0)
			for i := p - 1; i < len(got); i++ {
				if got[i] < 0 {
					t.Fatalf("%s p=%d: negative ADR %g at %d", s.Name, p, got[i], i)
				}
			}
		}
	}
}

func TestADRRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, len(s.H))
	if _, _, err := adrRange(-1, 5, s.H, s.L, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := adrRange(5, 4, s.H, s.L, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := adrRange(0, -1, s.H, s.L, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := adrRange(0, 10, s.H, s.L, 0, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("period 0: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := adrRange(0, 1, s.H, s.L, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := adrRange(0, 10, s.H[:5], s.L, 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high: err = %v, want ErrBadParam", err)
	}
	if _, _, err := adrRange(0, 10, s.H, s.L[:5], 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short low: err = %v, want ErrBadParam", err)
	}
	if _, _, err := adrRange(5, 20, s.H, s.L, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short out: err = %v, want ErrBadParam", err)
	}
	// Sub-range values match the full series.
	full, _ := ADR(s.H, s.L, 4)
	beg, nb, err = adrRange(10, 20, s.H, s.L, 4, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[10:21], 0)
	// In-place computation over either input is allowed.
	for which := 0; which < 2; which++ {
		h := append([]float64(nil), s.H...)
		l := append([]float64(nil), s.L...)
		buf := h
		if which == 1 {
			buf = l
		}
		beg, nb, _ = adrRange(0, len(buf)-1, h, l, 4, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place ", which), buf, full, 0)
	}
}
