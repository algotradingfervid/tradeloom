package ta

import (
	"errors"
	"math"
	"testing"
)

// vwmaAllZeroVolume reports whether a fixture has zero volume on every bar;
// VWMA is then NaN (0/0) on every bar past the lookback for period >= 2.
func vwmaAllZeroVolume(s series) bool {
	for _, v := range s.V {
		if v != 0 {
			return false
		}
	}
	return true
}

func TestVWMAParams(t *testing.T) {
	s := fixtures(50)[0]
	in, vol := s.Real(), s.V
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := VWMA(in, vol, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := VWMALookback(p); lb != -1 {
			t.Errorf("VWMALookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := vwmaRange(0, len(in)-1, in, vol, p, make([]float64, len(in))); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 30, 100000} {
		if _, err := VWMA(in, vol, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := VWMALookback(p); lb != p-1 {
			t.Errorf("VWMALookback(%d) = %d, want %d", p, lb, p-1)
		}
	}
	// Mismatched input lengths.
	if _, err := VWMA(in, vol[:49], 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short volume: err = %v, want ErrBadParam", err)
	}
	if _, err := VWMA(in[:10], vol, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("short real: err = %v, want ErrBadParam", err)
	}
	if _, err := VWMA(in, nil, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil volume with real: err = %v, want ErrBadParam", err)
	}
	// nil / empty input.
	if out, err := VWMA(nil, nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := VWMA([]float64{}, []float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestVWMALengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 30} {
		lb := VWMALookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := VWMA(s.Real(), s.V, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				label := lbl("VWMA ", s.Name, " n=", n, " p=", period)
				if period >= 2 && vwmaAllZeroVolume(s) {
					// nan_inf_output: no weights at all, every element is NaN.
					for i, v := range out {
						if !math.IsNaN(v) {
							t.Fatalf("%s: [%d]=%g, want NaN (all-zero volume)", label, i, v)
						}
					}
					continue
				}
				assertLeadingNaN(t, label, out, lb)
			}
		}
	}
}

func TestVWMAKnownValues(t *testing.T) {
	nan := math.NaN()
	price := []float64{10, 20, 30, 40}
	vol := []float64{1, 3, 1, 0}
	out, err := VWMA(price, vol, 2)
	if err != nil {
		t.Fatal(err)
	}
	// (10*1+20*3)/4 = 17.5; (20*3+30*1)/4 = 22.5; (30*1+40*0)/1 = 30.
	assertReal(t, "VWMA known p=2", out, []float64{nan, 17.5, 22.5, 30}, 1e-15)

	out, _ = VWMA(price, vol, 3)
	// (10+60+30)/5 = 20; (60+30+0)/4 = 22.5.
	assertReal(t, "VWMA known p=3", out, []float64{nan, nan, 20, 22.5}, 1e-15)

	// Equal volumes reduce to SMA.
	eq := []float64{7, 7, 7, 7}
	out, _ = VWMA(price, eq, 3)
	sma, _ := SMA(price, 3)
	assertReal(t, "VWMA equal volume == SMA", out, sma, 1e-12)

	// Period 1 is the identity, even with zero volume.
	out, _ = VWMA(price, vol, 1)
	assertReal(t, "VWMA period 1", out, price, 0)
	z := fixtures(30)[6]
	out, _ = VWMA(z.Real(), z.V, 1)
	assertReal(t, "VWMA period 1 zero volume", out, z.Real(), 0)

	// All-zero window is NaN; a later window with volume recovers.
	out, _ = VWMA([]float64{5, 6, 7, 8}, []float64{0, 0, 0, 2}, 2)
	assertReal(t, "VWMA zero window", out, []float64{nan, nan, nan, 8}, 0)

	// A constant price averages to itself whatever the volume.
	s := fixtures(60)[0]
	c := make([]float64, len(s.V))
	for i := range c {
		c[i] = 42.5
	}
	out, _ = VWMA(c, s.V, 9)
	for i := 8; i < len(out); i++ {
		if !closeEnough(out[i], 42.5, 1e-12) {
			t.Fatalf("constant price: [%d]=%.17g", i, out[i])
		}
	}

	// Bounds: with non-negative volume the result lies within the window's
	// min and max price.
	w := fixtures(300)[1]
	const p = 14
	out, _ = VWMA(w.Real(), w.V, p)
	for i := p - 1; i < len(out); i++ {
		lo, hi := math.Inf(1), math.Inf(-1)
		for k := i - p + 1; k <= i; k++ {
			lo = math.Min(lo, w.C[k])
			hi = math.Max(hi, w.C[k])
		}
		eps := 1e-9 * math.Max(1, hi)
		if out[i] < lo-eps || out[i] > hi+eps {
			t.Fatalf("bounds: [%d]=%g not in [%g, %g]", i, out[i], lo, hi)
		}
	}
}

func TestVWMARangeForm(t *testing.T) {
	s := fixtures(30)[0]
	in, vol := s.Real(), s.V
	out := make([]float64, len(in))
	if _, _, err := vwmaRange(-1, 5, in, vol, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := vwmaRange(taMaxIdx+1, taMaxIdx+2, in, vol, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := vwmaRange(5, 4, in, vol, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := vwmaRange(0, -1, in, vol, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := vwmaRange(0, 1, in, vol, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := vwmaRange(0, 10, in[:5], vol, 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short real buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := vwmaRange(0, 10, in, vol[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short volume buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := vwmaRange(0, 10, in, vol, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Sub-range values match the full computation.
	full, _ := VWMA(in, vol, 4)
	beg, nb, err = vwmaRange(10, 20, in, vol, 4, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[10:21], 0)
	// In-place computation is allowed, over either input.
	for _, which := range []string{"real", "volume"} {
		r := append([]float64(nil), in...)
		v := append([]float64(nil), vol...)
		buf := r
		if which == "volume" {
			buf = v
		}
		beg, nb, _ = vwmaRange(0, len(buf)-1, r, v, 4, buf)
		alignReal(buf, beg, nb)
		assertReal(t, "in-place "+which, buf, full, 0)
	}
}
