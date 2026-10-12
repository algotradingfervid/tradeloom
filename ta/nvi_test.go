package ta

import (
	"errors"
	"math"
	"testing"
)

func TestNVIParams(t *testing.T) {
	if lb := NVILookback(); lb != 0 {
		t.Errorf("NVILookback() = %d, want 0", lb)
	}
	s := fixtures(50)[0]
	if _, err := NVI(s.C, s.V[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, err := NVI(s.C, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil volume with non-empty close: err = %v, want ErrBadParam", err)
	}
	if out, err := NVI(nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := NVI([]float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := nviRange(0, 10, s.C[:5], s.V, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short close buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := nviRange(0, 10, s.C, s.V[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short volume buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := nviRange(0, 10, s.C, s.V, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := nviRange(5, 10, s.C, s.V, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: unexpected err %v", err)
	}
}

func TestNVILengths(t *testing.T) {
	lb := NVILookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := NVI(s.C, s.V)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			assertLeadingNaN(t, lbl("NVI ", s.Name, " n=", n), out, lb)
			if n > 0 && out[0] != 1000 {
				t.Errorf("%s n=%d: seed %g, want 1000", s.Name, n, out[0])
			}
		}
	}
}

func TestNVIKnownValues(t *testing.T) {
	// Bar 1: volume falls, +10% -> 1100. Bar 2: volume rises, carried.
	// Bar 3: volume falls, -25% -> 825. Bar 4: volume equal, carried.
	cl := []float64{10, 11, 12, 9, 20}
	vol := []float64{100, 50, 60, 40, 40}
	out, err := NVI(cl, vol)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "NVI hand", out, []float64{1000, 1100, 1100, 825, 825}, 1e-12)

	// Zero previous close: the index is carried forward, no Inf/NaN.
	out, _ = NVI([]float64{0, 5, 10}, []float64{10, 5, 1})
	assertReal(t, "NVI zero close", out, []float64{1000, 1000, 2000}, 0)

	// Overflow: an infinite update keeps the last finite value.
	out, _ = NVI([]float64{1e-300, 1e300, 2e300}, []float64{3, 2, 1})
	assertReal(t, "NVI overflow", out, []float64{1000, 1000, 2000}, 0)

	// NaN close: the update is not finite, so the index is carried.
	out, _ = NVI([]float64{10, math.NaN(), 10}, []float64{3, 2, 1})
	assertReal(t, "NVI NaN close", out, []float64{1000, 1000, 1000}, 0)

	// Constant close: always 1000. Rising volume: always 1000.
	c := fixtures(40)[5]
	out, _ = NVI(c.C, c.V)
	for i, v := range out {
		if v != 1000 {
			t.Fatalf("constant close: [%d]=%g", i, v)
		}
	}
	up := trend("up", 40, 100, 1)
	rising := make([]float64, 40)
	for i := range rising {
		rising[i] = float64(i + 1)
	}
	out, _ = NVI(up.C, rising)
	for i, v := range out {
		if v != 1000 {
			t.Fatalf("rising volume: [%d]=%g", i, v)
		}
	}

	// Positive prices keep the index positive.
	for _, s := range fixtures(500)[:3] {
		out, _ = NVI(s.C, s.V)
		for i, v := range out {
			if !(v > 0) || math.IsInf(v, 0) {
				t.Fatalf("%s: [%d]=%g not positive finite", s.Name, i, v)
			}
		}
	}
}

func TestNVIRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := nviRange(-1, 5, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := nviRange(5, 4, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := nviRange(0, -1, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Single bar: the seed.
	beg, nb, err := nviRange(7, 7, s.C, s.V, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != 1000 {
		t.Errorf("single bar: beg=%d nb=%d out0=%g err=%v", beg, nb, out[0], err)
	}
	// A sub-range re-seeds at startIdx: it equals NVI over the sliced input.
	beg, nb, err = nviRange(10, 29, s.C, s.V, out)
	if err != nil || beg != 10 || nb != 20 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	want, _ := NVI(s.C[10:], s.V[10:])
	assertReal(t, "sub-range", out[:nb], want, 0)
	// In-place computation over the close buffer is allowed.
	buf := append([]float64(nil), s.C...)
	want, _ = NVI(s.C, s.V)
	beg, nb, _ = nviRange(0, len(buf)-1, buf, s.V, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
