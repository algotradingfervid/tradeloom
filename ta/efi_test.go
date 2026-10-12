package ta

import (
	"errors"
	"math"
	"testing"
)

func TestEFIParams(t *testing.T) {
	s := fixtures(50)[0]
	c, v := s.C, s.V
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := EFI(c, v, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := EFILookback(p); lb != -1 {
			t.Errorf("EFILookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := efiRange(0, 49, c, v, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 13, 100000} {
		if _, err := EFI(c, v, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := EFILookback(p); lb != p {
			t.Errorf("EFILookback(%d) = %d, want %d", p, lb, p)
		}
	}
	if _, err := EFI(c, v[:49], 13); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, err := EFI(nil, v, 13); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil close, non-empty volume: err = %v, want ErrBadParam", err)
	}
	if out, err := EFI(nil, nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := EFI([]float64{}, []float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestEFILengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 13, 30} {
		lb := EFILookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := EFI(s.C, s.V, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("EFI ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestEFIKnownValues(t *testing.T) {
	// force[1..4] = (2-1)*1, (4-2)*2, (7-4)*1, (11-7)*1 = 1, 4, 3, 4
	c := []float64{1, 2, 4, 7, 11}
	v := []float64{1, 1, 2, 1, 1}
	nan := math.NaN()

	// Period 1 is the raw Force Index.
	out, err := EFI(c, v, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "EFI p=1", out, []float64{nan, 1, 4, 3, 4}, 0)

	// Period 2: seed (1+4)/2 = 2.5, k = 2/3.
	out, err = EFI(c, v, 2)
	if err != nil {
		t.Fatal(err)
	}
	k := 2.0 / 3.0
	e3 := (3-2.5)*k + 2.5
	e4 := (4-e3)*k + e3
	assertReal(t, "EFI p=2", out, []float64{nan, nan, 2.5, e3, e4}, 1e-14)

	// Period 4: seed is the plain mean of all four forces.
	out, _ = EFI(c, v, 4)
	assertReal(t, "EFI p=4", out, []float64{nan, nan, nan, nan, 3}, 0)

	// A flat close gives exactly zero force, and so does zero volume.
	flat := fixtures(60)[5]
	for _, p := range []int{1, 2, 13} {
		out, _ = EFI(flat.C, flat.V, p)
		for i := p; i < len(out); i++ {
			if out[i] != 0 {
				t.Fatalf("flat p=%d: [%d]=%g", p, i, out[i])
			}
		}
	}
	zv := fixtures(60)[12]
	out, _ = EFI(zv.C, zv.V, 5)
	for i := 5; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("zero volume: [%d]=%g", i, out[i])
		}
	}

	// Strictly rising close with positive volume: force and EFI are positive;
	// falling: negative.
	up := fixtures(60)[7]
	out, _ = EFI(up.C, up.V, 13)
	for i := 13; i < len(out); i++ {
		if !(out[i] > 0) {
			t.Fatalf("up: [%d]=%g, want > 0", i, out[i])
		}
	}
	down := fixtures(60)[8]
	out, _ = EFI(down.C, down.V, 13)
	for i := 13; i < len(out); i++ {
		if !(out[i] < 0) {
			t.Fatalf("down: [%d]=%g, want < 0", i, out[i])
		}
	}

	// Period 1 matches (close[t]-close[t-1])*volume[t] exactly on a fixture.
	w := fixtures(80)[0]
	out, _ = EFI(w.C, w.V, 1)
	for i := 1; i < len(out); i++ {
		if want := (w.C[i] - w.C[i-1]) * w.V[i]; out[i] != want {
			t.Fatalf("p=1 identity: [%d]=%g want %g", i, out[i], want)
		}
	}
}

func TestEFIRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	c, v := s.C, s.V
	out := make([]float64, len(c))
	if _, _, err := efiRange(-1, 5, c, v, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := efiRange(5, 4, c, v, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := efiRange(0, -1, c, v, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := efiRange(0, 2, c, v, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := efiRange(0, 10, c[:5], v, 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short close buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := efiRange(0, 10, c, v[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short volume buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := efiRange(0, 10, c, v, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Output aliasing the close input is allowed (C carries prevClose).
	for _, p := range []int{1, 4} {
		buf := append([]float64(nil), c...)
		want, _ := EFI(c, v, p)
		beg, nb, err = efiRange(0, len(buf)-1, buf, v, p, buf)
		if err != nil {
			t.Fatal(err)
		}
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
	// A sub-range starts its seed window at startIdx-lookback+1.
	beg, nb, err = efiRange(10, 20, c, v, 3, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
}

func TestEFIUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	SetUnstablePeriod(UnstEMA, 4)
	if lb := EFILookback(13); lb != 17 {
		t.Fatalf("EFILookback(13) with unst 4 = %d, want 17", lb)
	}
	s := fixtures(60)[0]
	out, err := EFI(s.C, s.V, 13)
	if err != nil {
		t.Fatal(err)
	}
	assertLeadingNaN(t, "EFI unst", out, 17)
}
