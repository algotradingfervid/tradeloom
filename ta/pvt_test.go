package ta

import (
	"errors"
	"math"
	"testing"
)

func TestPVTParams(t *testing.T) {
	if lb := PVTLookback(); lb != 0 {
		t.Errorf("PVTLookback() = %d, want 0", lb)
	}
	s := fixtures(50)[0]
	if _, err := PVT(s.C, s.V[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, err := PVT(s.C[:49], s.V); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths (short close): err = %v, want ErrBadParam", err)
	}
	if _, err := PVT(s.C, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil volume with close input: err = %v, want ErrBadParam", err)
	}
	if out, err := PVT(nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := PVT([]float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestPVTLengths(t *testing.T) {
	lb := PVTLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := PVT(s.C, s.V)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			assertLeadingNaN(t, lbl("PVT ", s.Name, " n=", n), out, lb)
		}
	}
}

func TestPVTKnownValues(t *testing.T) {
	// PVT[0] = 0; PVT[i] = PVT[i-1] + v[i]*(c[i]-c[i-1])/c[i-1].
	close := []float64{10, 11, 11, 5.5, 0, 4, 8}
	vol := []float64{100, 200, 300, 400, 500, 600, 700}
	out, err := PVT(close, vol)
	if err != nil {
		t.Fatal(err)
	}
	// bar1: +0.1*200 = 20
	// bar2: +0
	// bar3: -0.5*400 = -200 -> -180
	// bar4: -1*500 = -500 -> -680
	// bar5: previous close is 0 -> carried: -680
	// bar6: +1*700 = 700 -> 20
	assertReal(t, "PVT known", out, []float64{0, 20, 20, -180, -680, -680, 20}, 1e-12)

	// Constant price: PVT stays at zero.
	c := fixtures(40)[5]
	out, _ = PVT(c.C, c.V)
	for i, v := range out {
		if v != 0 {
			t.Fatalf("constant: [%d]=%g, want 0", i, v)
		}
	}

	// All-zero closes: every bar has a zero previous close, total stays 0.
	z := fixtures(40)[6]
	out, _ = PVT(z.C, z.V)
	for i, v := range out {
		if v != 0 {
			t.Fatalf("zeros: [%d]=%g, want 0", i, v)
		}
	}

	// Zero volume: PVT stays at zero whatever the price does.
	zv := fixtures(40)[12]
	out, _ = PVT(zv.C, zv.V)
	for i, v := range out {
		if v != 0 {
			t.Fatalf("zero volume: [%d]=%g, want 0", i, v)
		}
	}

	// Strictly rising positive closes: PVT is strictly increasing.
	u := fixtures(30)[7]
	out, _ = PVT(u.C, u.V)
	for i := 1; i < len(out); i++ {
		if !(out[i] > out[i-1]) {
			t.Fatalf("up: [%d]=%g not > [%d]=%g", i, out[i], i-1, out[i-1])
		}
	}
	// Strictly falling positive closes: PVT is strictly decreasing.
	d := fixtures(30)[8]
	out, _ = PVT(d.C, d.V)
	for i := 1; i < len(out); i++ {
		if !(out[i] < out[i-1]) {
			t.Fatalf("down: [%d]=%g not < [%d]=%g", i, out[i], i-1, out[i-1])
		}
	}

	// A NaN close poisons the running total from that bar on (C does the same:
	// NaN != 0 is true).
	out, _ = PVT([]float64{1, math.NaN(), 2, 3}, []float64{1, 2, 4, 8})
	if out[0] != 0 || !math.IsNaN(out[1]) || !math.IsNaN(out[3]) {
		t.Errorf("NaN close: %v", out)
	}
}

func TestPVTRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := pvtRange(-1, 5, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := pvtRange(5, 4, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := pvtRange(0, -1, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Lookback is 0, so no range lies inside it: a single bar yields one
	// value, zero.
	out[0] = 99
	beg, nb, err := pvtRange(7, 7, s.C, s.V, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != 0 {
		t.Errorf("single bar: beg=%d nb=%d out=%g err=%v", beg, nb, out[0], err)
	}
	if _, _, err := pvtRange(0, 10, s.C[:5], s.V, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: %v", err)
	}
	if _, _, err := pvtRange(0, 10, s.C, s.V[:5], out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inVolume: %v", err)
	}
	if _, _, err := pvtRange(0, 10, s.C, s.V, out[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outReal: %v", err)
	}
	// A sub-range restarts at zero: it equals the full series minus its value
	// at startIdx.
	full, _ := PVT(s.C, s.V)
	beg, nb, _ = pvtRange(10, 20, s.C, s.V, out)
	if beg != 10 || nb != 11 || out[0] != 0 {
		t.Errorf("sub-range: beg=%d nb=%d out[0]=%g", beg, nb, out[0])
	}
	for k := 0; k < nb; k++ {
		if !closeEnough(out[k], full[10+k]-full[10], 1e-9) {
			t.Errorf("sub-range [%d]: %g want %g", k, out[k], full[10+k]-full[10])
		}
	}
	// In-place computation over either input buffer is allowed.
	want, _ := PVT(s.C, s.V)
	buf := append([]float64(nil), s.V...)
	beg, nb, _ = pvtRange(0, len(buf)-1, s.C, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place volume", buf, want, 0)
	buf = append([]float64(nil), s.C...)
	beg, nb, _ = pvtRange(0, len(buf)-1, buf, s.V, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place close", buf, want, 0)
}
