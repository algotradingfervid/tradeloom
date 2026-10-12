package ta

import (
	"errors"
	"math"
	"testing"
)

func TestLINEARREGINTERCEPTParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := LINEARREGINTERCEPT(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := LINEARREGINTERCEPTLookback(p); lb != -1 {
			t.Errorf("LINEARREGINTERCEPTLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := linearreginterceptRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, err := LINEARREGINTERCEPT(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := LINEARREGINTERCEPTLookback(p); lb != p-1 {
			t.Errorf("LINEARREGINTERCEPTLookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	if out, err := LINEARREGINTERCEPT(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := LINEARREGINTERCEPT([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestLINEARREGINTERCEPTLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30} {
		lb := LINEARREGINTERCEPTLookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := LINEARREGINTERCEPT(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("LINEARREGINTERCEPT ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestLINEARREGINTERCEPTKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{1, 2, 4, 4, 0}
	// Period 3, x = 0,1,2 from the oldest bar of each window:
	// [1,2,4]: slope 1.5, mean 7/3  -> b = 7/3 - 1.5 = 5/6
	// [2,4,4]: slope 1,   mean 10/3 -> b = 10/3 - 1  = 7/3
	// [4,4,0]: slope -2,  mean 8/3  -> b = 8/3 + 2   = 14/3
	out, err := LINEARREGINTERCEPT(in, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "LINEARREGINTERCEPT hand p3", out, []float64{nan, nan, 5.0 / 6, 7.0 / 3, 14.0 / 3}, 1e-12)

	// Period 2: the line passes through both points, so b is the older value.
	out, _ = LINEARREGINTERCEPT(in, 2)
	assertReal(t, "LINEARREGINTERCEPT p2", out, []float64{nan, 1, 2, 4, 4}, 1e-12)

	// A straight line is fitted exactly: b is the value at the window's oldest bar.
	for _, p := range []int{2, 5, 14} {
		up := fixtures(100)[7].Real()
		out, _ = LINEARREGINTERCEPT(up, p)
		for i := p - 1; i < len(out); i++ {
			if math.Abs(out[i]-up[i-p+1]) > 1e-9 {
				t.Fatalf("trend p=%d: [%d]=%g, want %g", p, i, out[i], up[i-p+1])
			}
		}
	}

	// Constant input returns the constant; all-zero input returns zero.
	for _, c := range []struct {
		idx  int
		want float64
	}{{5, 100}, {6, 0}} {
		out, _ = LINEARREGINTERCEPT(fixtures(60)[c.idx].Real(), 5)
		for i := 4; i < len(out); i++ {
			if math.Abs(out[i]-c.want) > 1e-9 {
				t.Fatalf("fixture %d: [%d]=%g, want %g", c.idx, i, out[i], c.want)
			}
		}
	}
}

func TestLINEARREGINTERCEPTRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := linearreginterceptRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := linearreginterceptRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := linearreginterceptRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (lookback = period-1): no output, no error.
	beg, nb, err := linearreginterceptRange(0, 1, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// First computable bar.
	beg, nb, err = linearreginterceptRange(0, 2, in, 3, out)
	if err != nil || beg != 2 || nb != 1 {
		t.Errorf("first bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := linearreginterceptRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := linearreginterceptRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Sub-range values equal the full computation up to rounding: the running
	// sums start at a different bar, so accumulation order differs (as in C).
	full, _ := LINEARREGINTERCEPT(in, 4)
	beg, nb, _ = linearreginterceptRange(10, 20, in, 4, out)
	if beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d", beg, nb)
	}
	assertReal(t, "sub-range", out[:nb], full[10:21], relTol)
	// In-place computation is allowed (C issue #130).
	for _, p := range []int{2, 4} {
		buf := append([]float64(nil), in...)
		want, _ := LINEARREGINTERCEPT(in, p)
		beg, nb, _ = linearreginterceptRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
