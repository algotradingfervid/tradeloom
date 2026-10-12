package ta

import (
	"errors"
	"math"
	"testing"
)

func TestMARKETFIParams(t *testing.T) {
	if lb := MARKETFILookback(); lb != 0 {
		t.Errorf("MARKETFILookback() = %d, want 0", lb)
	}
	s := fixtures(50)[0]
	if _, err := MARKETFI(s.H, s.L, s.V); err != nil {
		t.Errorf("unexpected err %v", err)
	}
	// Mismatched input lengths.
	for i, in := range [][3][]float64{
		{s.H[:49], s.L, s.V},
		{s.H, s.L[:49], s.V},
		{s.H, s.L, s.V[:49]},
		{s.H, nil, s.V},
	} {
		if _, err := MARKETFI(in[0], in[1], in[2]); !errors.Is(err, ErrBadParam) {
			t.Errorf("mismatch %d: err = %v, want ErrBadParam", i, err)
		}
	}
	// nil / empty input -> empty output.
	if out, err := MARKETFI(nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := MARKETFI([]float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Short input buffers in the range form.
	out := make([]float64, 50)
	for i, in := range [][3][]float64{
		{s.H[:5], s.L, s.V},
		{s.H, s.L[:5], s.V},
		{s.H, s.L, s.V[:5]},
	} {
		if _, _, err := marketfiRange(0, 10, in[0], in[1], in[2], out); !errors.Is(err, ErrBadParam) {
			t.Errorf("short input %d: err = %v, want ErrBadParam", i, err)
		}
	}
}

func TestMARKETFILengths(t *testing.T) {
	lb := MARKETFILookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := MARKETFI(s.H, s.L, s.V)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			assertLeadingNaN(t, lbl("MARKETFI ", s.Name, " n=", n), out, lb)
			for i, v := range out {
				if math.IsInf(v, 0) {
					t.Fatalf("%s n=%d: [%d] = %g", s.Name, n, i, v)
				}
			}
		}
	}
}

func TestMARKETFIKnownValues(t *testing.T) {
	high := []float64{10, 12, 5, 7, 3, 8, 9}
	low := []float64{8, 9, 5, 4, 1, 6, 6}
	vol := []float64{4, 6, 10, 0, -2, math.Copysign(0, -1), 1e-300}
	// (10-8)/4, (12-9)/6, 0/10, zero volume -> 0, negative volume divides,
	// -0 volume compares equal to 0 -> 0, tiny volume still divides exactly.
	want := []float64{0.5, 0.5, 0, 0, -1, 0, 3e300}
	out, err := MARKETFI(high, low, vol)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "MARKETFI known", out, want, 0)

	// NaN volume is not == 0, so it divides and propagates NaN (as C does).
	out, _ = MARKETFI([]float64{2}, []float64{1}, []float64{math.NaN()})
	if !math.IsNaN(out[0]) {
		t.Errorf("NaN volume: got %g, want NaN", out[0])
	}

	// Zero range or zero volume: always 0.
	for _, idx := range []int{5, 6} { // constant, zeros
		s := fixtures(40)[idx]
		out, _ = MARKETFI(s.H, s.L, s.V)
		for i, v := range out {
			if v != 0 {
				t.Fatalf("%s: [%d]=%g, want 0", s.Name, i, v)
			}
		}
	}
	// Zero-volume walk: all 0 despite a non-zero range.
	s := fixtures(40)[12]
	out, _ = MARKETFI(s.H, s.L, s.V)
	for i, v := range out {
		if v != 0 {
			t.Fatalf("walk-zero-vol: [%d]=%g, want 0", i, v)
		}
	}
	// Well-formed bars (high >= low) with positive volume are non-negative
	// and equal the formula exactly.
	s = fixtures(200)[0]
	out, _ = MARKETFI(s.H, s.L, s.V)
	for i := range out {
		if out[i] < 0 || out[i] != (s.H[i]-s.L[i])/s.V[i] {
			t.Fatalf("walk: [%d]=%g", i, out[i])
		}
	}
}

func TestMARKETFIRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := marketfiRange(-1, 5, s.H, s.L, s.V, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := marketfiRange(5, 4, s.H, s.L, s.V, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := marketfiRange(0, -1, s.H, s.L, s.V, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Lookback is 0, so there is no range "inside the lookback": a single
	// bar at 0 produces exactly one value.
	beg, nb, err := marketfiRange(0, 0, s.H, s.L, s.V, out)
	if err != nil || beg != 0 || nb != 1 {
		t.Errorf("single bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values equal the full-series values.
	full, _ := MARKETFI(s.H, s.L, s.V)
	beg, nb, err = marketfiRange(7, 19, s.H, s.L, s.V, out)
	if err != nil || beg != 7 || nb != 13 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[7:20], 0)
	// Too-short output buffer.
	if _, _, err := marketfiRange(0, 29, s.H, s.L, s.V, make([]float64, 29)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// In-place computation (output aliases volume) is allowed.
	buf := append([]float64(nil), s.V...)
	beg, nb, _ = marketfiRange(0, len(buf)-1, s.H, s.L, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, full, 0)
}
