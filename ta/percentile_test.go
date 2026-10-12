package ta

import (
	"errors"
	"math"
	"sort"
	"testing"
)

// percentileBrute is an independent nearest-rank reference: sort each
// trailing window and take rank ceil(P*N/100) clamped to [1, N].
func percentileBrute(in []float64, n int, p float64) []float64 {
	out := nanSlice(len(in))
	rank := int(math.Ceil(p * float64(n) / 100.0))
	rank = cMaxInt(1, cMinInt(rank, n))
	for t := n - 1; t < len(in); t++ {
		w := append([]float64(nil), in[t-n+1:t+1]...)
		sort.Float64s(w)
		out[t] = w[rank-1]
	}
	return out
}

func TestPERCENTILEParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := PERCENTILE(in, p, 50); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := PERCENTILELookback(p, 50); lb != -1 {
			t.Errorf("PERCENTILELookback(%d) = %d, want -1", p, lb)
		}
	}
	for _, q := range []float64{-1, -1e-12, 100.0000001, 101, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := PERCENTILE(in, 5, q); !errors.Is(err, ErrBadParam) {
			t.Errorf("percentile %g: err = %v, want ErrBadParam", q, err)
		}
		if lb := PERCENTILELookback(5, q); lb != -1 {
			t.Errorf("PERCENTILELookback(5, %g) = %d, want -1", q, lb)
		}
		if _, _, err := percentileRange(0, 10, in, 5, q, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range percentile %g: err = %v, want ErrBadParam", q, err)
		}
	}
	for _, p := range []int{2, 3, 100000} {
		for _, q := range []float64{0, 50, 100} {
			if _, err := PERCENTILE(in, p, q); err != nil {
				t.Errorf("period %d pct %g: unexpected err %v", p, q, err)
			}
			if lb := PERCENTILELookback(p, q); lb != p-1 {
				t.Errorf("PERCENTILELookback(%d,%g) = %d", p, q, lb)
			}
		}
	}
	if out, err := PERCENTILE(nil, 5, 50); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := PERCENTILE([]float64{}, 5, 50); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := percentileRange(0, 10, in[:5], 3, 50, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := percentileRange(0, 10, in, 3, 50, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := percentileRange(0, 10, in, 1, 50, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("range period 1: err = %v, want ErrBadParam", err)
	}
}

func TestPERCENTILELengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 30, 31} {
		for _, pct := range []float64{0, 25, 50, 100} {
			lb := PERCENTILELookback(period, pct)
			if lb != period-1 {
				t.Fatalf("PERCENTILELookback(%d) = %d", period, lb)
			}
			for _, n := range edgeLengths(lb) {
				for _, s := range fixtures(n) {
					out, err := PERCENTILE(s.Real(), period, pct)
					if err != nil {
						t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
					}
					if len(out) != n {
						t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
					}
					assertLeadingNaN(t, lbl("PERCENTILE ", s.Name, " n=", n, " p=", period, " q=", pct), out, lb)
				}
			}
		}
	}
}

func TestPERCENTILEKnownValues(t *testing.T) {
	nan := math.NaN()
	// Window [3,1,4,1,5] sorted is [1,1,3,4,5]; P=50, N=5 -> rank ceil(2.5)=3 -> 3.
	// Window [1,4,1,5,9] sorted is [1,1,4,5,9] -> 4.
	// Window [4,1,5,9,2] sorted is [1,2,4,5,9] -> 4.
	in := []float64{3, 1, 4, 1, 5, 9, 2}
	out, err := PERCENTILE(in, 5, 50)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "PERCENTILE(5,50)", out, []float64{nan, nan, nan, nan, 3, 4, 4}, 0)

	// P=20, N=5: 20*5/100 = 1 exactly -> rank 1 (the minimum), not 2.
	out, _ = PERCENTILE(in, 5, 20)
	assertReal(t, "PERCENTILE(5,20)", out, []float64{nan, nan, nan, nan, 1, 1, 1}, 0)
	// P=0 clamps to rank 1 (minimum); P=100 is rank N (maximum).
	out, _ = PERCENTILE(in, 5, 0)
	assertReal(t, "PERCENTILE(5,0)", out, []float64{nan, nan, nan, nan, 1, 1, 1}, 0)
	out, _ = PERCENTILE(in, 5, 100)
	assertReal(t, "PERCENTILE(5,100)", out, []float64{nan, nan, nan, nan, 5, 9, 9}, 0)
	// P=21, N=5: ceil(1.05) = 2 -> second smallest.
	out, _ = PERCENTILE(in, 5, 21)
	assertReal(t, "PERCENTILE(5,21)", out, []float64{nan, nan, nan, nan, 1, 1, 2}, 0)

	// Period 2: P<=50 is the min of the pair, P>50 the max.
	pair := []float64{5, 2, 8, 8, 1}
	out, _ = PERCENTILE(pair, 2, 50)
	assertReal(t, "PERCENTILE(2,50)", out, []float64{nan, 2, 2, 8, 1}, 0)
	out, _ = PERCENTILE(pair, 2, 51)
	assertReal(t, "PERCENTILE(2,51)", out, []float64{nan, 5, 8, 8, 8}, 0)

	// A constant series reports the constant.
	c := fixtures(40)[5].Real()
	out, _ = PERCENTILE(c, 7, 37)
	for i := 6; i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Brute-force reference on every fixture (finite data): the result is
	// always a window member at the nearest rank, and lies within [min, max].
	for _, s := range fixtures(300) {
		for _, n := range []int{2, 3, 4, 5, 10, 30, 31, 64} {
			for _, p := range []float64{0, 1, 10, 25, 33.33, 50, 66.67, 75, 90, 99, 100} {
				got, err := PERCENTILE(s.Real(), n, p)
				if err != nil {
					t.Fatal(err)
				}
				assertReal(t, lbl("brute ", s.Name, " n=", n, " p=", p), got, percentileBrute(s.Real(), n, p), 0)
			}
		}
	}
}

func TestPERCENTILERangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := percentileRange(-1, 5, in, 3, 50, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := percentileRange(5, 4, in, 3, 50, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := percentileRange(0, -1, in, 3, 50, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := percentileRange(0, 1, in, 3, 50, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// A sub-range matches the corresponding slice of the full output.
	full, _ := PERCENTILE(in, 4, 60)
	beg, nb, err = percentileRange(10, 20, in, 4, 60, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[10:21], 0)
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := PERCENTILE(in, 4, 50)
	beg, nb, _ = percentileRange(0, len(buf)-1, buf, 4, 50, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
