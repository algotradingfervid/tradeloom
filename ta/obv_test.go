package ta

import (
	"errors"
	"math"
	"testing"
)

func TestOBVParams(t *testing.T) {
	if lb := OBVLookback(); lb != 0 {
		t.Errorf("OBVLookback() = %d, want 0", lb)
	}
	s := fixtures(50)[0]
	if _, err := OBV(s.C, s.V[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, err := OBV(s.C, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil volume with real input: err = %v, want ErrBadParam", err)
	}
	if out, err := OBV(nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := OBV([]float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestOBVLengths(t *testing.T) {
	lb := OBVLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := OBV(s.C, s.V)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			assertLeadingNaN(t, lbl("OBV ", s.Name, " n=", n), out, lb)
		}
	}
}

func TestOBVKnownValues(t *testing.T) {
	// Seed = first volume (10); up adds, down subtracts, equal keeps.
	in := []float64{5, 6, 6, 4, 7, 7, 3}
	vol := []float64{10, 20, 30, 40, 50, 60, 70}
	out, err := OBV(in, vol)
	if err != nil {
		t.Fatal(err)
	}
	// 10, 10+20=30, 30 (equal), 30-40=-10, -10+50=40, 40 (equal), 40-70=-30
	assertReal(t, "OBV known", out, []float64{10, 30, 30, -10, 40, 40, -30}, 0)

	// Constant price: OBV stays at the first bar's volume.
	c := fixtures(40)[5]
	out, _ = OBV(c.C, c.V)
	for i, v := range out {
		if v != c.V[0] {
			t.Fatalf("constant: [%d]=%g, want %g", i, v, c.V[0])
		}
	}

	// Strictly rising price: OBV is the cumulative volume.
	u := fixtures(30)[7]
	out, _ = OBV(u.C, u.V)
	sum := 0.0
	for i := range out {
		sum += u.V[i]
		if out[i] != sum {
			t.Fatalf("up: [%d]=%g, want %g", i, out[i], sum)
		}
	}

	// Strictly falling price: first volume minus the following volumes.
	d := fixtures(30)[8]
	out, _ = OBV(d.C, d.V)
	acc := d.V[0]
	for i := range out {
		if i > 0 {
			acc -= d.V[i]
		}
		if out[i] != acc {
			t.Fatalf("down: [%d]=%g, want %g", i, out[i], acc)
		}
	}

	// A NaN price compares false both ways, so OBV is unchanged on that bar
	// and on the next one.
	out, _ = OBV([]float64{1, math.NaN(), 2, 3}, []float64{1, 2, 4, 8})
	assertReal(t, "OBV NaN price", out, []float64{1, 1, 1, 9}, 0)
}

func TestOBVRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := obvRange(-1, 5, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := obvRange(5, 4, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := obvRange(0, -1, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Lookback is 0, so no range lies inside it: a single bar yields one value,
	// the volume at that bar.
	beg, nb, err := obvRange(7, 7, s.C, s.V, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != s.V[7] {
		t.Errorf("single bar: beg=%d nb=%d out=%g err=%v", beg, nb, out[0], err)
	}
	if _, _, err := obvRange(0, 10, s.C[:5], s.V, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inReal: %v", err)
	}
	if _, _, err := obvRange(0, 10, s.C, s.V[:5], out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inVolume: %v", err)
	}
	if _, _, err := obvRange(0, 10, s.C, s.V, out[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outReal: %v", err)
	}
	// Sub-range seeds with inVolume[startIdx].
	beg, nb, _ = obvRange(10, 20, s.C, s.V, out)
	if beg != 10 || nb != 11 || out[0] != s.V[10] {
		t.Errorf("sub-range: beg=%d nb=%d out[0]=%g", beg, nb, out[0])
	}
	// In-place computation over the volume buffer is allowed.
	buf := append([]float64(nil), s.V...)
	want, _ := OBV(s.C, s.V)
	beg, nb, _ = obvRange(0, len(buf)-1, s.C, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
