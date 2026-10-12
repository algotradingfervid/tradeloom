package ta

import (
	"errors"
	"math"
	"testing"
)

func TestWADParams(t *testing.T) {
	if lb := WADLookback(); lb != 0 {
		t.Errorf("WADLookback() = %d, want 0", lb)
	}
	s := fixtures(50)[0]
	// Mismatched input lengths, each input in turn.
	if _, err := WAD(s.H[:49], s.L, s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high: err = %v, want ErrBadParam", err)
	}
	if _, err := WAD(s.H, s.L[:49], s.C); !errors.Is(err, ErrBadParam) {
		t.Errorf("short low: err = %v, want ErrBadParam", err)
	}
	if _, err := WAD(s.H, s.L, s.C[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short close: err = %v, want ErrBadParam", err)
	}
	if _, err := WAD(s.H, s.L, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil close with non-empty others: err = %v, want ErrBadParam", err)
	}
	// nil / empty input -> empty output, nil error.
	if out, err := WAD(nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	e := []float64{}
	if out, err := WAD(e, e, e); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestWADLengths(t *testing.T) {
	lb := WADLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := WAD(s.H, s.L, s.C)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			assertLeadingNaN(t, lbl("WAD ", s.Name, " n=", n), out, lb)
		}
	}
}

func TestWADKnownValues(t *testing.T) {
	// Bar 0: no previous close                          -> 0
	// Bar 1: 11 > 10, TRL = min(10, 9) = 9, AD = 2      -> 2
	// Bar 2: 12.5 > 11, TRL = min(11, 11) = 11, AD 1.5  -> 3.5
	// Bar 3: 10.5 < 12.5, TRH = max(12.5, 12) = 12.5, AD = -2 -> 1.5
	// Bar 4: 10.5 == 10.5, AD = 0                       -> 1.5
	// Bar 5: 13 > 10.5, TRL = min(10.5, 12) = 10.5 (gap up), AD 2.5 -> 4
	// Bar 6: 8 < 13, TRH = max(13, 9) = 13 (gap down), AD = -5 -> -1
	h := []float64{11, 12, 13, 12, 11, 14, 9}
	l := []float64{9, 9, 11, 10, 10, 12, 7}
	c := []float64{10, 11, 12.5, 10.5, 10.5, 13, 8}
	out, err := WAD(h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "WAD hand", out, []float64{0, 2, 3.5, 1.5, 1.5, 4, -1}, 0)

	// Constant and all-zero series: every close equals the previous one.
	for _, s := range []series{fixtures(40)[5], fixtures(40)[6]} {
		out, _ = WAD(s.H, s.L, s.C)
		for i := range out {
			if out[i] != 0 {
				t.Fatalf("%s: [%d]=%g want 0", s.Name, i, out[i])
			}
		}
	}

	// Strictly rising closes make the line non-decreasing; falling, non-increasing.
	up := fixtures(100)[7]
	out, _ = WAD(up.H, up.L, up.C)
	for i := 1; i < len(out); i++ {
		if out[i] < out[i-1] {
			t.Fatalf("up: [%d]=%g < [%d]=%g", i, out[i], i-1, out[i-1])
		}
	}
	down := fixtures(100)[8]
	out, _ = WAD(down.H, down.L, down.C)
	for i := 1; i < len(out); i++ {
		if out[i] > out[i-1] {
			t.Fatalf("down: [%d]=%g > [%d]=%g", i, out[i], i-1, out[i-1])
		}
	}

	// Bounds: on well-formed bars each step is bounded by the true range and
	// has the sign of the close-to-close move.
	for _, s := range fixtures(300) {
		out, _ = WAD(s.H, s.L, s.C)
		for i := 1; i < len(out); i++ {
			d := out[i] - out[i-1]
			tr := math.Max(s.H[i], s.C[i-1]) - math.Min(s.L[i], s.C[i-1])
			tol := 1e-9 * math.Max(1, math.Max(math.Abs(out[i]), math.Abs(out[i-1])))
			if math.Abs(d) > tr+tol {
				t.Fatalf("%s: [%d] step %g exceeds true range %g", s.Name, i, d, tr)
			}
			mv := s.C[i] - s.C[i-1]
			if (mv > 0 && d < -tol) || (mv < 0 && d > tol) || (mv == 0 && d != 0) {
				t.Fatalf("%s: [%d] step %g has wrong sign for move %g", s.Name, i, d, mv)
			}
		}
	}

	nan := math.NaN()
	// A NaN close fails both comparisons (adds nothing) and becomes prevClose,
	// so the next bar adds nothing either; the bar after that resumes.
	out, _ = WAD([]float64{5, 5, 5, 5}, []float64{0, 0, 0, 0}, []float64{1, nan, 3, 4})
	assertReal(t, "WAD NaN close", out, []float64{0, 0, 0, 4}, 0)
	// A NaN low on an up bar: prevClose < NaN is false, so the NaN is used and
	// poisons the sum.
	out, _ = WAD([]float64{5, 5, 5}, []float64{0, nan, 0}, []float64{1, 2, 3})
	assertReal(t, "WAD NaN low", out, []float64{0, nan, nan}, 0)
	// A NaN high on an up bar is never read (TRL = low = 0: steps 2 and 3).
	out, _ = WAD([]float64{5, nan, 5}, []float64{0, 0, 0}, []float64{1, 2, 3})
	assertReal(t, "WAD NaN high up", out, []float64{0, 2, 5}, 0)
}

func TestWADRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := wadRange(-1, 5, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := wadRange(5, 4, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := wadRange(0, -1, s.H, s.L, s.C, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// No lookback: a single bar produces one value, always 0.
	beg, nb, err := wadRange(9, 9, s.H, s.L, s.C, out)
	if err != nil || beg != 9 || nb != 1 || out[0] != 0 {
		t.Errorf("single bar: beg=%d nb=%d out=%g err=%v", beg, nb, out[0], err)
	}
	// A sub-range restarts the sum at 0 at startIdx.
	beg, nb, err = wadRange(7, 12, s.H, s.L, s.C, out)
	if err != nil || beg != 7 || nb != 6 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	want, _ := WAD(s.H[7:13], s.L[7:13], s.C[7:13])
	assertReal(t, "sub-range restart", out[:nb], want, 0)
	// Too-short buffers.
	if _, _, err := wadRange(0, 10, s.H[:5], s.L, s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := wadRange(0, 10, s.H, s.L[:10], s.C, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short low buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := wadRange(0, 10, s.H, s.L, s.C[:10], out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short close buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := wadRange(0, 10, s.H, s.L, s.C, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// In-place computation over any input is allowed.
	want, _ = WAD(s.H, s.L, s.C)
	for k := 0; k < 3; k++ {
		h := append([]float64(nil), s.H...)
		l := append([]float64(nil), s.L...)
		c := append([]float64(nil), s.C...)
		buf := [][]float64{h, l, c}[k]
		beg, nb, _ = wadRange(0, len(buf)-1, h, l, c, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place input ", k), buf, want, 0)
	}
}
