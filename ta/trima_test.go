package ta

import (
	"errors"
	"math"
	"testing"
)

func TestTRIMAParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32} {
		if _, err := TRIMA(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := TRIMALookback(p); lb != -1 {
			t.Errorf("TRIMALookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := trimaRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 99999, 100000} {
		if _, err := TRIMA(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := TRIMALookback(p); lb != p-1 {
			t.Errorf("TRIMALookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	if out, err := TRIMA(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := TRIMA([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestTRIMALengths(t *testing.T) {
	for _, period := range []int{1, 2, 3, 4, 5, 30, 31, 100000} {
		lb := TRIMALookback(period)
		if lb != period-1 {
			t.Fatalf("TRIMALookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := TRIMA(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("TRIMA ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

// trimaNaive computes TRIMA as an explicitly weighted average, from the
// weight description in trima.md.
func trimaNaive(in []float64, period int) []float64 {
	w := make([]float64, period)
	sum := 0.0
	for k := range period {
		w[k] = float64(min(k+1, period-k))
		sum += w[k]
	}
	out := make([]float64, len(in))
	for i := range out {
		if i < period-1 {
			out[i] = math.NaN()
			continue
		}
		acc := 0.0
		for k := range period {
			acc += w[k] * in[i-period+1+k]
		}
		out[i] = acc / sum
	}
	return out
}

func TestTRIMAKnownValues(t *testing.T) {
	nan := math.NaN()
	// 4-period: (1a+2b+2c+1d)/6 over 1..6 -> 2.5, 3.5, 4.5.
	in := []float64{1, 2, 3, 4, 5, 6}
	out, err := TRIMA(in, 4)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "TRIMA(1..6,4)", out, []float64{nan, nan, nan, 2.5, 3.5, 4.5}, 1e-15)

	// 5-period: (1a+2b+3c+2d+1e)/9 on a spike: weights show up directly.
	spike := []float64{0, 0, 9, 0, 0, 0, 0}
	out, _ = TRIMA(spike, 5)
	assertReal(t, "TRIMA spike,5", out, []float64{nan, nan, nan, nan, 3, 2, 1}, 1e-15)

	// 4-period spike: weights 1,2,2,1 over 6.
	spike = []float64{6, 0, 0, 0, 0, 0}
	out, _ = TRIMA(spike, 4)
	assertReal(t, "TRIMA spike,4", out, []float64{nan, nan, nan, 1, 0, 0}, 1e-15)

	// Period 1 is the identity.
	src := fixtures(40)[0].Real()
	out, _ = TRIMA(src, 1)
	assertReal(t, "TRIMA period 1", out, src, 0)

	// Equals the explicit weighted average for odd and even periods.
	for _, p := range []int{2, 3, 4, 7, 10, 31} {
		out, _ = TRIMA(src, p)
		assertReal(t, lbl("TRIMA naive p=", p), out, trimaNaive(src, p), 1e-12)
	}

	// Equivalent to SMA of SMA (trima.md).
	for _, p := range []int{6, 9} {
		a, b := (p+1)/2, (p+1)/2
		if p%2 == 0 {
			a, b = p/2, p/2+1
		}
		s1, _ := SMA(src, a)
		s2, _ := SMA(s1[a-1:], b)
		out, _ = TRIMA(src, p)
		assertReal(t, lbl("TRIMA sma-of-sma p=", p), out[a-1:], s2, 1e-12)
	}

	// A constant series averages to itself; output stays within input bounds.
	c := fixtures(40)[5].Real()
	out, _ = TRIMA(c, 7)
	for i := 6; i < len(out); i++ {
		if math.Abs(out[i]-100) > 1e-12 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}
	out, _ = TRIMA(src, 8)
	for i := 7; i < len(out); i++ {
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, v := range src[i-7 : i+1] {
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
		if out[i] < lo-1e-9 || out[i] > hi+1e-9 {
			t.Fatalf("bounds: [%d]=%g not in [%g,%g]", i, out[i], lo, hi)
		}
	}
}

func TestTRIMARangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := trimaRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := trimaRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := trimaRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := trimaRange(0, 1, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := trimaRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := trimaRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Sub-range values match the full computation.
	full, _ := TRIMA(in, 5)
	beg, nb, err = trimaRange(2, 20, in, 5, out)
	if err != nil || beg != 4 || nb != 17 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[4:21], 0)
	// In-place computation is allowed, odd and even.
	for _, p := range []int{4, 5} {
		buf := append([]float64(nil), in...)
		want, _ := TRIMA(in, p)
		beg, nb, _ = trimaRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
