package ta

import (
	"errors"
	"math"
	"testing"
)

func TestLINEARREGSLOPEParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := LINEARREGSLOPE(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := LINEARREGSLOPELookback(p); lb != -1 {
			t.Errorf("LINEARREGSLOPELookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := linearregslopeRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, err := LINEARREGSLOPE(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := LINEARREGSLOPELookback(p); lb != p-1 {
			t.Errorf("LINEARREGSLOPELookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	if out, err := LINEARREGSLOPE(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := LINEARREGSLOPE([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestLINEARREGSLOPELengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30} {
		lb := LINEARREGSLOPELookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := LINEARREGSLOPE(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("LINEARREGSLOPE ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestLINEARREGSLOPEKnownValues(t *testing.T) {
	nan := math.NaN()
	// Period 2: SumX=1, SumXSqr=1, Divisor=-1, so m = y[t] - y[t-1].
	out, err := LINEARREGSLOPE([]float64{1, 2, 4, 4, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "LINEARREGSLOPE p2", out, []float64{nan, 1, 2, 0, -4}, 0)

	// Period 3: SumX=3, SumXSqr=5, Divisor=9-15=-6.
	// Window [1,2,6]: SumY=9, SumXY=0*6+1*2+2*1=4, m=(3*4-3*9)/-6=2.5.
	// Window [2,6,4]: SumY=12, SumXY=0*4+1*6+2*2=10, m=(30-36)/-6=1.
	out, _ = LINEARREGSLOPE([]float64{1, 2, 6, 4}, 3)
	assertReal(t, "LINEARREGSLOPE p3", out, []float64{nan, nan, 2.5, 1}, 1e-12)

	// Straight lines have their step as slope; constants have slope 0.
	for _, p := range []int{2, 5, 14, 30} {
		for idx, want := range map[int]float64{5: 0, 6: 0, 7: 1, 8: -1} {
			out, _ = LINEARREGSLOPE(fixtures(200)[idx].Real(), p)
			for i := p - 1; i < len(out); i++ {
				if math.Abs(out[i]-want) > 1e-9 {
					t.Fatalf("fixture %d p=%d: [%d]=%g, want %g", idx, p, i, out[i], want)
				}
			}
		}
	}

	// A huge spike entering and leaving the window drives the magnitude
	// re-anchor; after it leaves, the slope of a line must be exact again.
	in := make([]float64, 100)
	for i := range in {
		in[i] = float64(i)
	}
	in[20] = 1e15
	out, _ = LINEARREGSLOPE(in, 4)
	for i := 24; i < len(out); i++ {
		if math.Abs(out[i]-1) > 1e-9 {
			t.Fatalf("after spike: [%d]=%g, want 1", i, out[i])
		}
	}
}

func TestLINEARREGSLOPERangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := linearregslopeRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := linearregslopeRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := linearregslopeRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (lookback = period-1): no output, no error.
	beg, nb, err := linearregslopeRange(0, 1, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// First computable bar.
	beg, nb, err = linearregslopeRange(0, 2, in, 3, out)
	if err != nil || beg != 2 || nb != 1 {
		t.Errorf("first bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := linearregslopeRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := linearregslopeRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Sub-range values equal the full computation up to rounding: the running
	// sums start at a different bar, so accumulation order differs (as in C).
	full, _ := LINEARREGSLOPE(in, 4)
	beg, nb, _ = linearregslopeRange(10, 20, in, 4, out)
	if beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d", beg, nb)
	}
	assertReal(t, "sub-range", out[:nb], full[10:21], relTol)
	// In-place computation is allowed (C issue #130).
	for _, p := range []int{2, 4} {
		buf := append([]float64(nil), in...)
		want, _ := LINEARREGSLOPE(in, p)
		beg, nb, _ = linearregslopeRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
