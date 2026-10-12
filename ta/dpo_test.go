package ta

import (
	"errors"
	"math"
	"testing"
)

func TestDPOParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := DPO(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := DPOLookback(p); lb != -1 {
			t.Errorf("DPOLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := dpoRange(0, len(in)-1, in, p, make([]float64, len(in))); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 20, 100000} {
		if _, err := DPO(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
	}
	for _, c := range []struct{ p, lb int }{{2, 2}, {3, 2}, {4, 3}, {5, 4}, {20, 19}, {100000, 99999}} {
		if lb := DPOLookback(c.p); lb != c.lb {
			t.Errorf("DPOLookback(%d) = %d, want %d", c.p, lb, c.lb)
		}
	}
	if out, err := DPO(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := DPO([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := dpoRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := dpoRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
}

func TestDPOLengths(t *testing.T) {
	for _, period := range []int{2, 3, 4, 5, 20, 31} {
		lb := DPOLookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := DPO(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("DPO ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestDPOKnownValues(t *testing.T) {
	nan := math.NaN()
	// Period 3: t = 3/2+1 = 2, lookback max(2, 2) = 2.
	// out[i] = in[i-2] - (in[i-2]+in[i-1]+in[i])/3.
	in := []float64{2, 4, 8, 16, 32}
	out, err := DPO(in, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "DPO(geom,3)", out, []float64{nan, nan, 2 - 14.0/3, 4 - 28.0/3, 8 - 56.0/3}, 1e-12)

	// Period 2: t = 2, lookback max(1, 2) = 2 (the displacement dominates).
	// out[i] = in[i-2] - (in[i-1]+in[i])/2.
	out, _ = DPO(in, 2)
	assertReal(t, "DPO(geom,2)", out, []float64{nan, nan, 2 - 6, 4 - 12, 8 - 24}, 0)

	// Period 4: t = 3, lookback 3: out[3] = in[0] - (1+2+3+4)/4 = -1.5.
	lin := []float64{1, 2, 3, 4, 5, 6}
	out, _ = DPO(lin, 4)
	assertReal(t, "DPO(1..6,4)", out, []float64{nan, nan, nan, -1.5, -1.5, -1.5}, 0)

	// Period 5: t = 3, lookback 4: out[4] = in[1] - 3 = -1.
	out, _ = DPO(lin, 5)
	assertReal(t, "DPO(1..6,5)", out, []float64{nan, nan, nan, nan, -1, -1}, 0)

	// A constant series is fully detrended to zero.
	c := fixtures(40)[5].Real()
	for _, p := range []int{2, 7, 20} {
		out, _ = DPO(c, p)
		for i := DPOLookback(p); i < len(out); i++ {
			if out[i] != 0 {
				t.Fatalf("constant p=%d: [%d]=%g", p, i, out[i])
			}
		}
	}

	// The moving-average term is bit-identical to SMA at the same bar.
	s := fixtures(200)[0].Real()
	for _, p := range []int{2, 3, 10, 21} {
		out, _ = DPO(s, p)
		sma, _ := SMA(s, p)
		disp := p/2 + 1
		for i := DPOLookback(p); i < len(s); i++ {
			if want := s[i-disp] - sma[i]; out[i] != want {
				t.Fatalf("p=%d [%d]: got %v want %v", p, i, out[i], want)
			}
		}
	}
}

func TestDPORangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := dpoRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := dpoRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := dpoRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := dpoRange(0, 1, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values match the full series.
	full, _ := DPO(in, 6)
	beg, nb, err = dpoRange(2, 20, in, 6, out)
	if err != nil || beg != 5 || nb != 16 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[beg:beg+nb], 0)
	// In-place computation is allowed, including when a read cursor equals outIdx.
	for _, p := range []int{2, 3, 4, 9} {
		buf := append([]float64(nil), in...)
		want, _ := DPO(in, p)
		beg, nb, _ = dpoRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
