package ta

import (
	"errors"
	"math"
	"testing"
)

func TestMINParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := MIN(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := MINLookback(p); lb != -1 {
			t.Errorf("MINLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := minRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 30, 100000} {
		if _, err := MIN(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := MINLookback(p); lb != p-1 {
			t.Errorf("MINLookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	if out, err := MIN(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := MIN([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := minRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := minRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
}

func TestMINLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 30, 31} {
		lb := MINLookback(period)
		if lb != period-1 {
			t.Fatalf("MINLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := MIN(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("MIN ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

// minNaive is the formula from min.md: min(inReal[i-p+1 .. i]).
func minNaive(in []float64, p int) []float64 {
	out := nanSlice(len(in))
	for i := p - 1; i < len(in); i++ {
		h := in[i-p+1]
		for j := i - p + 2; j <= i; j++ {
			if in[j] < h {
				h = in[j]
			}
		}
		out[i] = h
	}
	return out
}

func TestMINKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{1, 3, 2, 5, 4, 0, -1}
	out, err := MIN(in, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "MIN(...,3)", out, []float64{nan, nan, 1, 2, 2, 0, -1}, 0)

	out, _ = MIN(in, 2)
	assertReal(t, "MIN(...,2)", out, []float64{nan, 1, 2, 2, 4, 0, -1}, 0)

	// Window equal to the series: a single value, the overall min.
	out, _ = MIN(in, 7)
	assertReal(t, "MIN(...,7)", out, []float64{nan, nan, nan, nan, nan, nan, -1}, 0)

	// Constant input stays constant.
	c := fixtures(40)[5].Real()
	out, _ = MIN(c, 7)
	for i := 6; i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Rising series: min is the oldest bar; falling: the current bar.
	up := fixtures(50)[7].Real()
	out, _ = MIN(up, 5)
	for i := 4; i < len(up); i++ {
		if out[i] != up[i-4] {
			t.Fatalf("up: [%d]=%g want %g", i, out[i], up[i-4])
		}
	}
	down := fixtures(50)[8].Real()
	out, _ = MIN(down, 5)
	for i := 4; i < len(down); i++ {
		if out[i] != down[i] {
			t.Fatalf("down: [%d]=%g want %g", i, out[i], down[i])
		}
	}

	// The block algorithm equals the definition on every fixture, at block
	// boundaries and with partial last blocks; output <= every window value.
	for _, p := range []int{2, 3, 4, 7, 30, 31, 64} {
		for _, n := range []int{p - 1, p, p + 1, 2*p - 1, 2 * p, 2*p + 1, 3*p + 2, 257} {
			for _, s := range fixtures(n) {
				got, _ := MIN(s.Real(), p)
				assertReal(t, lbl("naive ", s.Name, " n=", n, " p=", p), got, minNaive(s.Real(), p), 0)
			}
		}
	}
}

func TestMINRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := minRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := minRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := minRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := minRange(0, 1, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-ranges agree with the full series.
	full, _ := MIN(in, 4)
	for _, rg := range [][2]int{{0, 29}, {3, 3}, {5, 17}, {29, 29}, {2, 6}} {
		o := make([]float64, len(in))
		beg, nb, err := minRange(rg[0], rg[1], in, 4, o)
		if err != nil {
			t.Fatal(err)
		}
		wantBeg := max(rg[0], 3)
		if beg != wantBeg || nb != rg[1]-wantBeg+1 {
			t.Fatalf("range %v: beg=%d nb=%d", rg, beg, nb)
		}
		assertReal(t, lbl("subrange ", rg), o[:nb], full[beg:beg+nb], 0)
	}
	// In-place computation is allowed.
	for _, p := range []int{2, 4, 9} {
		buf := append([]float64(nil), in...)
		want, _ := MIN(in, p)
		beg, nb, _ = minRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
