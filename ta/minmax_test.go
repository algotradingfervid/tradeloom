package ta

import (
	"errors"
	"math"
	"testing"
)

func TestMINMAXParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, _, err := MINMAX(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := MINMAXLookback(p); lb != -1 {
			t.Errorf("MINMAXLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := minmaxRange(0, 49, in, p, make([]float64, 50), make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 30, 100000} {
		if _, _, err := MINMAX(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := MINMAXLookback(p); lb != p-1 {
			t.Errorf("MINMAXLookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	if lo, hi, err := MINMAX(nil, 5); err != nil || len(lo) != 0 || len(hi) != 0 {
		t.Errorf("nil input: lo=%v hi=%v err=%v, want empty and nil", lo, hi, err)
	}
	if lo, hi, err := MINMAX([]float64{}, 5); err != nil || len(lo) != 0 || len(hi) != 0 {
		t.Errorf("empty input: lo=%v hi=%v err=%v, want empty and nil", lo, hi, err)
	}
	if _, _, err := minmaxRange(0, 10, in[:5], 3, make([]float64, 20), make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := minmaxRange(0, 10, in, 3, make([]float64, 5), make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outMin buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := minmaxRange(0, 10, in, 3, make([]float64, 20), make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outMax buffer: err = %v, want ErrBadParam", err)
	}
	// C rejects outMin == outMax.
	same := make([]float64, 50)
	if _, _, err := minmaxRange(0, 49, in, 3, same, same); !errors.Is(err, ErrBadParam) {
		t.Errorf("aliased outputs: err = %v, want ErrBadParam", err)
	}
}

func TestMINMAXLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 30, 31} {
		lb := MINMAXLookback(period)
		if lb != period-1 {
			t.Fatalf("MINMAXLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				lo, hi, err := MINMAX(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(lo) != n || len(hi) != n {
					t.Fatalf("%s n=%d p=%d: len=%d,%d", s.Name, n, period, len(lo), len(hi))
				}
				assertLeadingNaN(t, lbl("MINMAX min ", s.Name, " n=", n, " p=", period), lo, lb)
				assertLeadingNaN(t, lbl("MINMAX max ", s.Name, " n=", n, " p=", period), hi, lb)
			}
		}
	}
}

// minmaxNaive is the formula from minmax.md.
func minmaxNaive(in []float64, p int) (lo, hi []float64) {
	lo, hi = nanSlice(len(in)), nanSlice(len(in))
	for i := p - 1; i < len(in); i++ {
		l, h := in[i-p+1], in[i-p+1]
		for j := i - p + 2; j <= i; j++ {
			if in[j] > h {
				h = in[j]
			}
			if in[j] < l {
				l = in[j]
			}
		}
		lo[i], hi[i] = l, h
	}
	return lo, hi
}

func TestMINMAXKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{1, 3, 2, 5, 4, 0, -1}
	lo, hi, err := MINMAX(in, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "MINMAX min(...,3)", lo, []float64{nan, nan, 1, 2, 2, 0, -1}, 0)
	assertReal(t, "MINMAX max(...,3)", hi, []float64{nan, nan, 3, 5, 5, 5, 4}, 0)

	lo, hi, _ = MINMAX(in, 2)
	assertReal(t, "MINMAX min(...,2)", lo, []float64{nan, 1, 2, 2, 4, 0, -1}, 0)
	assertReal(t, "MINMAX max(...,2)", hi, []float64{nan, 3, 3, 5, 5, 4, 0}, 0)

	// Window equal to the series: a single value, the overall extrema.
	lo, hi, _ = MINMAX(in, 7)
	assertReal(t, "MINMAX min(...,7)", lo, []float64{nan, nan, nan, nan, nan, nan, -1}, 0)
	assertReal(t, "MINMAX max(...,7)", hi, []float64{nan, nan, nan, nan, nan, nan, 5}, 0)

	// Constant input stays constant.
	c := fixtures(40)[5].Real()
	lo, hi, _ = MINMAX(c, 7)
	for i := 6; i < len(c); i++ {
		if lo[i] != 100 || hi[i] != 100 {
			t.Fatalf("constant: [%d]=%g,%g", i, lo[i], hi[i])
		}
	}

	// Rising: min is the oldest bar, max the current; falling: the reverse.
	up := fixtures(50)[7].Real()
	lo, hi, _ = MINMAX(up, 5)
	for i := 4; i < len(up); i++ {
		if lo[i] != up[i-4] || hi[i] != up[i] {
			t.Fatalf("up: [%d]=%g,%g", i, lo[i], hi[i])
		}
	}
	down := fixtures(50)[8].Real()
	lo, hi, _ = MINMAX(down, 5)
	for i := 4; i < len(down); i++ {
		if lo[i] != down[i] || hi[i] != down[i-4] {
			t.Fatalf("down: [%d]=%g,%g", i, lo[i], hi[i])
		}
	}

	// The block algorithm equals the definition on every fixture, at block
	// boundaries and with partial last blocks; min <= max everywhere; and
	// the outputs equal MIN and MAX.
	for _, p := range []int{2, 3, 4, 7, 30, 31, 64} {
		for _, n := range []int{p - 1, p, p + 1, 2*p - 1, 2 * p, 2*p + 1, 3*p + 2, 257} {
			for _, s := range fixtures(n) {
				lo, hi, _ := MINMAX(s.Real(), p)
				wlo, whi := minmaxNaive(s.Real(), p)
				assertReal(t, lbl("naive min ", s.Name, " n=", n, " p=", p), lo, wlo, 0)
				assertReal(t, lbl("naive max ", s.Name, " n=", n, " p=", p), hi, whi, 0)
				for i := p - 1; i < n; i++ {
					if lo[i] > hi[i] {
						t.Fatalf("%s n=%d p=%d [%d]: min %g > max %g", s.Name, n, p, i, lo[i], hi[i])
					}
				}
				mx, _ := MAX(s.Real(), p)
				assertReal(t, lbl("vs MAX ", s.Name, " n=", n, " p=", p), hi, mx, 0)
			}
		}
	}
}

func TestMINMAXRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	o1, o2 := make([]float64, len(in)), make([]float64, len(in))
	if _, _, err := minmaxRange(-1, 5, in, 3, o1, o2); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := minmaxRange(5, 4, in, 3, o1, o2); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := minmaxRange(0, -1, in, 3, o1, o2); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := minmaxRange(0, 1, in, 3, o1, o2)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-ranges agree with the full series.
	flo, fhi, _ := MINMAX(in, 4)
	for _, rg := range [][2]int{{0, 29}, {3, 3}, {5, 17}, {29, 29}, {2, 6}} {
		lo, hi := make([]float64, len(in)), make([]float64, len(in))
		beg, nb, err := minmaxRange(rg[0], rg[1], in, 4, lo, hi)
		if err != nil {
			t.Fatal(err)
		}
		wantBeg := max(rg[0], 3)
		if beg != wantBeg || nb != rg[1]-wantBeg+1 {
			t.Fatalf("range %v: beg=%d nb=%d", rg, beg, nb)
		}
		assertReal(t, lbl("subrange min ", rg), lo[:nb], flo[beg:beg+nb], 0)
		assertReal(t, lbl("subrange max ", rg), hi[:nb], fhi[beg:beg+nb], 0)
	}
	// In-place computation is allowed: either output may alias the input.
	for _, p := range []int{2, 4, 9} {
		wlo, whi, _ := MINMAX(in, p)

		buf := append([]float64(nil), in...)
		other := make([]float64, len(in))
		beg, nb, _ = minmaxRange(0, len(buf)-1, buf, p, buf, other)
		alignReal(buf, beg, nb)
		alignReal(other, beg, nb)
		assertReal(t, lbl("in-place min p=", p), buf, wlo, 0)
		assertReal(t, lbl("in-place min/other p=", p), other, whi, 0)

		buf = append([]float64(nil), in...)
		other = make([]float64, len(in))
		beg, nb, _ = minmaxRange(0, len(buf)-1, buf, p, other, buf)
		alignReal(buf, beg, nb)
		alignReal(other, beg, nb)
		assertReal(t, lbl("in-place max p=", p), buf, whi, 0)
		assertReal(t, lbl("in-place max/other p=", p), other, wlo, 0)
	}
}
