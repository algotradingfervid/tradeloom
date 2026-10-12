package ta

import (
	"errors"
	"math"
	"testing"
)

func TestPVIParams(t *testing.T) {
	if lb := PVILookback(); lb != 0 {
		t.Errorf("PVILookback() = %d, want 0", lb)
	}
	s := fixtures(50)[0]
	if _, err := PVI(s.C, s.V[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, err := PVI(s.C[:49], s.V); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths (close short): err = %v, want ErrBadParam", err)
	}
	if _, err := PVI(s.C, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil volume with close input: err = %v, want ErrBadParam", err)
	}
	if out, err := PVI(nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := PVI([]float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestPVILengths(t *testing.T) {
	lb := PVILookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := PVI(s.C, s.V)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			// PVI never emits NaN: non-finite updates are discarded.
			assertLeadingNaN(t, lbl("PVI ", s.Name, " n=", n), out, lb)
			for i, v := range out {
				if math.IsInf(v, 0) {
					t.Fatalf("%s n=%d: [%d] is Inf", s.Name, n, i)
				}
			}
		}
	}
}

func TestPVIKnownValues(t *testing.T) {
	nan := math.NaN()
	inf := math.Inf(1)

	// Seed 1000; compounds the % close change only when volume rises.
	// bar1: vol 1->2 up, +50%  -> 1500
	// bar2: vol 2->1 down      -> 1500
	// bar3: vol 1->3 up, +100% -> 3000
	// bar4: vol 3->4 up, -50%  -> 1500
	// bar5: vol 4->4 equal     -> 1500
	out, err := PVI([]float64{10, 15, 15, 30, 15, 99}, []float64{1, 2, 1, 3, 4, 4})
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "PVI known", out, []float64{1000, 1500, 1500, 3000, 1500, 1500}, 0)

	// Single bar: the seed.
	out, _ = PVI([]float64{42}, []float64{7})
	assertReal(t, "PVI single", out, []float64{1000}, 0)

	// Zero previous close: carried forward instead of dividing by zero.
	out, _ = PVI([]float64{0, 5, 10}, []float64{1, 2, 3})
	assertReal(t, "PVI zero close", out, []float64{1000, 1000, 2000}, 0)

	// NaN close makes the update NaN on that bar and the next: both discarded.
	out, _ = PVI([]float64{10, nan, 20, 40}, []float64{1, 2, 3, 4})
	assertReal(t, "PVI NaN close", out, []float64{1000, 1000, 1000, 2000}, 0)

	// Inf close: Inf update, then Inf/Inf = NaN update; both discarded.
	out, _ = PVI([]float64{10, inf, 20, 40}, []float64{1, 2, 3, 4})
	assertReal(t, "PVI Inf close", out, []float64{1000, 1000, 1000, 2000}, 0)

	// NaN volume compares false: no update on that bar or the next.
	out, _ = PVI([]float64{10, 20, 40, 80}, []float64{1, nan, 3, 4})
	assertReal(t, "PVI NaN volume", out, []float64{1000, 1000, 1000, 2000}, 0)

	// Overflow guard: an update past MaxFloat64 keeps the last finite value.
	out, _ = PVI([]float64{1, 1e300, 1e308, 5e307}, []float64{1, 2, 3, 4})
	x := 1000.0 + (1e300-1)/1*1000.0
	if out[1] != x || out[2] != x || math.IsInf(out[2], 0) {
		t.Errorf("PVI overflow: got %v, want [1] = [2] = %g", out, x)
	}
	if want := x + (5e307-1e308)/1e308*x; !closeEnough(out[3], want, 1e-15) {
		t.Errorf("PVI after overflow: got %g want %g", out[3], want)
	}

	// Constant price, constant volume, all-zero series: the index stays at 1000.
	for _, s := range []series{fixtures(40)[5], fixtures(40)[6], fixtures(40)[12]} {
		out, _ = PVI(s.C, s.V)
		for i, v := range out {
			if v != 1000 {
				t.Fatalf("%s: [%d]=%g, want 1000", s.Name, i, v)
			}
		}
	}

	// Strictly falling volume: never updates.
	out, _ = PVI([]float64{1, 2, 3, 4}, []float64{4, 3, 2, 1})
	assertReal(t, "PVI falling volume", out, []float64{1000, 1000, 1000, 1000}, 0)

	// Positive prices keep the index strictly positive.
	for _, s := range fixtures(500)[:5] {
		out, _ = PVI(s.C, s.V)
		for i, v := range out {
			if !(v > 0) {
				t.Fatalf("%s: [%d]=%g, want > 0", s.Name, i, v)
			}
		}
	}
}

func TestPVIRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := pviRange(-1, 5, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := pviRange(5, 4, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := pviRange(0, -1, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Lookback is 0, so no range lies inside it: a single bar yields the seed.
	beg, nb, err := pviRange(7, 7, s.C, s.V, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != 1000 {
		t.Errorf("single bar: beg=%d nb=%d out=%g err=%v", beg, nb, out[0], err)
	}
	if _, _, err := pviRange(0, 10, s.C[:5], s.V, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: %v", err)
	}
	if _, _, err := pviRange(0, 10, s.C, s.V[:5], out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inVolume: %v", err)
	}
	if _, _, err := pviRange(0, 10, s.C, s.V, out[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outReal: %v", err)
	}
	// Sub-range reseeds at 1000 on startIdx.
	beg, nb, _ = pviRange(10, 20, s.C, s.V, out)
	if beg != 10 || nb != 11 || out[0] != 1000 {
		t.Errorf("sub-range: beg=%d nb=%d out[0]=%g", beg, nb, out[0])
	}
	// In-place computation over either input buffer is allowed.
	want, _ := PVI(s.C, s.V)
	buf := append([]float64(nil), s.V...)
	beg, nb, _ = pviRange(0, len(buf)-1, s.C, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place volume", buf, want, 0)
	buf = append([]float64(nil), s.C...)
	beg, nb, _ = pviRange(0, len(buf)-1, buf, s.V, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place close", buf, want, 0)
}
