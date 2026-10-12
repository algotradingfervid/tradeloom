package ta

import (
	"errors"
	"math"
	"testing"
)

func TestMIDPOINTParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := MIDPOINT(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := MIDPOINTLookback(p); lb != -1 {
			t.Errorf("MIDPOINTLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := midpointRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, err := MIDPOINT(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := MIDPOINTLookback(p); lb != p-1 {
			t.Errorf("MIDPOINTLookback(%d) = %d", p, lb)
		}
	}
	if out, err := MIDPOINT(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := MIDPOINT([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := midpointRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := midpointRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
}

func TestMIDPOINTLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30, 31, 100} {
		lb := MIDPOINTLookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := MIDPOINT(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("MIDPOINT ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

// midpointNaive is the formula from midpoint.md, used as an independent check.
func midpointNaive(in []float64, p int) []float64 {
	out := nanSlice(len(in))
	for i := p - 1; i < len(in); i++ {
		hi, lo := in[i-p+1], in[i-p+1]
		for j := i - p + 2; j <= i; j++ {
			hi = math.Max(hi, in[j])
			lo = math.Min(lo, in[j])
		}
		out[i] = (hi + lo) / 2
	}
	return out
}

func TestMIDPOINTKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{1, 5, 3, 2, 8, 4}
	out, err := MIDPOINT(in, 3)
	if err != nil {
		t.Fatal(err)
	}
	// windows: {1,5,3}->3, {5,3,2}->3.5, {3,2,8}->5, {2,8,4}->5
	assertReal(t, "MIDPOINT hand", out, []float64{nan, nan, 3, 3.5, 5, 5}, 0)

	// Matches the naive formula on every fixture, across block boundaries.
	for _, p := range []int{2, 3, 4, 7, 14, 31} {
		for _, s := range fixtures(257) {
			got, _ := MIDPOINT(s.Real(), p)
			assertReal(t, lbl("naive ", s.Name, " p=", p), got, midpointNaive(s.Real(), p), 0)
		}
	}

	// A constant series has its own value as midpoint.
	c := fixtures(40)[5].Real()
	out, _ = MIDPOINT(c, 7)
	for i := 6; i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Bounds: the midpoint lies within [min, max] of its window.
	s := fixtures(200)[0].Real()
	out, _ = MIDPOINT(s, 10)
	for i := 9; i < len(s); i++ {
		lo, hi := s[i], s[i]
		for j := i - 9; j <= i; j++ {
			lo = math.Min(lo, s[j])
			hi = math.Max(hi, s[j])
		}
		if out[i] < lo || out[i] > hi {
			t.Fatalf("bounds: [%d]=%g not in [%g,%g]", i, out[i], lo, hi)
		}
	}
}

func TestMIDPOINTRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := midpointRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := midpointRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := midpointRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	beg, nb, err := midpointRange(0, 1, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-ranges agree with the full series.
	full, _ := MIDPOINT(in, 4)
	for _, rg := range [][2]int{{0, 29}, {3, 3}, {5, 17}, {29, 29}, {0, 3}, {10, 29}} {
		o := make([]float64, len(in))
		beg, nb, err := midpointRange(rg[0], rg[1], in, 4, o)
		if err != nil {
			t.Fatal(err)
		}
		assertReal(t, lbl("subrange ", rg), o[:nb], full[beg:beg+nb], 0)
	}
	// In-place computation is allowed.
	for _, p := range []int{2, 4, 7} {
		buf := append([]float64(nil), in...)
		want, _ := MIDPOINT(in, p)
		beg, nb, _ = midpointRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
