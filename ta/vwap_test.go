package ta

import (
	"errors"
	"math"
	"testing"
)

func TestVWAPParams(t *testing.T) {
	s := fixtures(50)[0]
	if lb := VWAPLookback(); lb != 0 {
		t.Errorf("VWAPLookback() = %d, want 0", lb)
	}
	if _, err := VWAP(s.H, s.L[:49], s.C, s.V); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched low: err = %v, want ErrBadParam", err)
	}
	if _, err := VWAP(s.H, s.L, s.C[:10], s.V); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched close: err = %v, want ErrBadParam", err)
	}
	if _, err := VWAP(s.H, s.L, s.C, s.V[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched volume: err = %v, want ErrBadParam", err)
	}
	if _, err := VWAP(s.H, s.L, s.C, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil volume with data: err = %v, want ErrBadParam", err)
	}
	if out, err := VWAP(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := VWAP([]float64{}, []float64{}, []float64{}, []float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	out := make([]float64, 50)
	if _, _, err := vwapRange(0, 10, s.H, s.L, s.C, s.V[:5], out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short volume buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := vwapRange(0, 10, s.H[:5], s.L, s.C, s.V, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := vwapRange(0, 10, s.H, s.L, s.C, s.V, out[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
}

func TestVWAPLengths(t *testing.T) {
	lb := VWAPLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := VWAP(s.H, s.L, s.C, s.V)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			assertLeadingNaN(t, lbl("VWAP ", s.Name, " n=", n), out, lb)
			// A successful call never emits NaN or Inf on finite input.
			for i, v := range out {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("%s n=%d: [%d]=%g", s.Name, n, i, v)
				}
			}
		}
	}
}

func TestVWAPKnownValues(t *testing.T) {
	// TP = (H+L+C)/3: bar0 (12+6+9)/3 = 9, bar1 (21+15+18)/3 = 18, bar2 (6+3+3)/3 = 4.
	h := []float64{12, 21, 6}
	l := []float64{6, 15, 3}
	c := []float64{9, 18, 3}
	v := []float64{1, 2, 3}
	out, err := VWAP(h, l, c, v)
	if err != nil {
		t.Fatal(err)
	}
	// 9; (9+36)/3 = 15; (9+36+12)/6 = 9.5
	assertReal(t, "VWAP hand", out, []float64{9, 15, 9.5}, 0)

	// Leading zero volume carries 0, then the first traded bar sets VWAP;
	// a later zero-volume bar repeats the previous value.
	out, _ = VWAP(h, l, c, []float64{0, 2, 0})
	assertReal(t, "VWAP zero vol", out, []float64{0, 18, 18}, 0)

	// A non-finite bar is skipped, not absorbed.
	nan, inf := math.NaN(), math.Inf(1)
	out, _ = VWAP(h, l, []float64{9, nan, 3}, v)
	assertReal(t, "VWAP NaN close", out, []float64{9, 9, (9.0 + 12) / 4}, 0)
	out, _ = VWAP(h, l, c, []float64{1, inf, 3})
	assertReal(t, "VWAP Inf volume", out, []float64{9, 9, (9.0 + 12) / 4}, 0)
	out, _ = VWAP([]float64{nan}, []float64{1}, []float64{1}, []float64{1})
	assertReal(t, "VWAP all skipped", out, []float64{0}, 0)

	// Constant input averages to itself.
	cs := fixtures(40)[5]
	out, _ = VWAP(cs.H, cs.L, cs.C, cs.V)
	for i, x := range out {
		if x != 100 {
			t.Fatalf("constant: [%d]=%g", i, x)
		}
	}
	// All zeros (incl. volume): output is 0 everywhere.
	z := fixtures(40)[6]
	out, _ = VWAP(z.H, z.L, z.C, z.V)
	for i, x := range out {
		if x != 0 {
			t.Fatalf("zeros: [%d]=%g", i, x)
		}
	}

	// Bounds: VWAP stays within [min TP, max TP] of the bars seen so far
	// (for bars with positive volume).
	for _, s := range fixtures(300) {
		if s.Name == "walk-zero-vol" || s.Name == "zeros" {
			continue
		}
		out, _ := VWAP(s.H, s.L, s.C, s.V)
		lo, hi := math.Inf(1), math.Inf(-1)
		for i := range out {
			tp := (s.H[i] + s.L[i] + s.C[i]) / 3
			lo, hi = math.Min(lo, tp), math.Max(hi, tp)
			eps := 1e-12 * math.Max(math.Abs(lo), math.Abs(hi))
			if out[i] < lo-eps || out[i] > hi+eps {
				t.Fatalf("%s: [%d]=%g outside [%g, %g]", s.Name, i, out[i], lo, hi)
			}
		}
	}
}

func TestVWAPRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := vwapRange(-1, 5, s.H, s.L, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := vwapRange(5, 4, s.H, s.L, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := vwapRange(0, -1, s.H, s.L, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields one value.
	beg, nb, err := vwapRange(7, 7, s.H, s.L, s.C, s.V, out)
	if err != nil || beg != 7 || nb != 1 {
		t.Errorf("single bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if want := (s.H[7] + s.L[7] + s.C[7]) / 3.0; out[0] != want {
		t.Errorf("single bar: got %g want %g", out[0], want)
	}
	// Sub-range restarts the sums at startIdx.
	sub, _ := VWAP(s.H[10:], s.L[10:], s.C[10:], s.V[10:])
	beg, nb, _ = vwapRange(10, 29, s.H, s.L, s.C, s.V, out)
	if beg != 10 || nb != 20 {
		t.Fatalf("sub-range: beg=%d nb=%d", beg, nb)
	}
	assertReal(t, "sub-range", out[:nb], sub, 0)
	// In-place computation into the close buffer is allowed.
	want, _ := VWAP(s.H, s.L, s.C, s.V)
	buf := append([]float64(nil), s.C...)
	beg, nb, _ = vwapRange(0, 29, s.H, s.L, buf, s.V, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
