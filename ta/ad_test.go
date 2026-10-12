package ta

import (
	"errors"
	"math"
	"testing"
)

func TestADParams(t *testing.T) {
	if lb := ADLookback(); lb != 0 {
		t.Errorf("ADLookback() = %d, want 0", lb)
	}
	s := fixtures(50)[0]
	// Mismatched input lengths, each input in turn.
	if _, err := AD(s.H[:49], s.L, s.C, s.V); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high: err = %v, want ErrBadParam", err)
	}
	if _, err := AD(s.H, s.L[:49], s.C, s.V); !errors.Is(err, ErrBadParam) {
		t.Errorf("short low: err = %v, want ErrBadParam", err)
	}
	if _, err := AD(s.H, s.L, s.C[:49], s.V); !errors.Is(err, ErrBadParam) {
		t.Errorf("short close: err = %v, want ErrBadParam", err)
	}
	if _, err := AD(s.H, s.L, s.C, s.V[:49]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short volume: err = %v, want ErrBadParam", err)
	}
	if _, err := AD(s.H, s.L, s.C, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil volume with non-empty others: err = %v, want ErrBadParam", err)
	}
	// nil / empty input -> empty output, nil error.
	if out, err := AD(nil, nil, nil, nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	e := []float64{}
	if out, err := AD(e, e, e, e); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestADLengths(t *testing.T) {
	lb := ADLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := AD(s.H, s.L, s.C, s.V)
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			assertLeadingNaN(t, lbl("AD ", s.Name, " n=", n), out, lb)
		}
	}
}

func TestADKnownValues(t *testing.T) {
	// Bar 0: MFM = ((8-6)-(10-8))/4 = 0        -> AD 0
	// Bar 1: MFM = ((10-6)-(10-10))/4 = 1, v=50 -> AD 50
	// Bar 2: MFM = ((6-6)-(10-6))/4 = -1, v=20  -> AD 30
	// Bar 3: high == low, skipped              -> AD 30
	// Bar 4: MFM = ((9-6)-(10-9))/4 = 0.5, v=8  -> AD 34
	// Bar 5: high < low (malformed), skipped   -> AD 34
	h := []float64{10, 10, 10, 7, 10, 5}
	l := []float64{6, 6, 6, 7, 6, 9}
	c := []float64{8, 10, 6, 7, 9, 7}
	v := []float64{100, 50, 20, 1000, 8, 1000}
	out, err := AD(h, l, c, v)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "AD hand", out, []float64{0, 50, 30, 30, 34, 34}, 0)

	// Constant and all-zero series have zero range: AD stays 0.
	for _, s := range []series{fixtures(40)[5], fixtures(40)[6]} {
		out, _ = AD(s.H, s.L, s.C, s.V)
		for i := range out {
			if out[i] != 0 {
				t.Fatalf("%s: [%d]=%g want 0", s.Name, i, out[i])
			}
		}
	}

	// Zero volume adds nothing.
	s := fixtures(100)[12]
	out, _ = AD(s.H, s.L, s.C, s.V)
	for i := range out {
		if out[i] != 0 {
			t.Fatalf("zero volume: [%d]=%g want 0", i, out[i])
		}
	}

	// Bounds: on well-formed bars each step is within [-volume, +volume].
	for _, s := range fixtures(300) {
		out, _ = AD(s.H, s.L, s.C, s.V)
		prev := 0.0
		for i := range out {
			d := out[i] - prev
			tol := 1e-9 * math.Max(1, math.Max(math.Abs(out[i]), math.Abs(prev)))
			if math.Abs(d) > s.V[i]+tol {
				t.Fatalf("%s: [%d] step %g exceeds volume %g", s.Name, i, d, s.V[i])
			}
			prev = out[i]
		}
	}

	// A NaN volume on a bar with range poisons the running sum from there on.
	nan := math.NaN()
	out, _ = AD([]float64{2, 2, 2}, []float64{0, 0, 0}, []float64{2, 2, 2}, []float64{1, nan, 1})
	assertReal(t, "AD NaN", out, []float64{1, nan, nan}, 0)
	// A NaN high makes high-low NaN, which fails tmp > 0: the bar is skipped.
	out, _ = AD([]float64{2, nan, 2}, []float64{0, 0, 0}, []float64{2, 2, 2}, []float64{1, 1, 1})
	assertReal(t, "AD NaN high", out, []float64{1, 1, 2}, 0)
}

func TestADRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := adRange(-1, 5, s.H, s.L, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := adRange(5, 4, s.H, s.L, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := adRange(0, -1, s.H, s.L, s.C, s.V, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// No lookback: a single bar at 0 produces one value.
	beg, nb, err := adRange(0, 0, s.H, s.L, s.C, s.V, out)
	if err != nil || beg != 0 || nb != 1 {
		t.Errorf("single bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// A sub-range restarts the sum at 0 at startIdx.
	beg, nb, err = adRange(7, 12, s.H, s.L, s.C, s.V, out)
	if err != nil || beg != 7 || nb != 6 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	want, _ := AD(s.H[7:13], s.L[7:13], s.C[7:13], s.V[7:13])
	assertReal(t, "sub-range restart", out[:nb], want, 0)
	// Too-short buffers.
	if _, _, err := adRange(0, 10, s.H[:5], s.L, s.C, s.V, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := adRange(0, 10, s.H, s.L, s.C, s.V[:10], out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short volume buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := adRange(0, 10, s.H, s.L, s.C, s.V, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// In-place computation over an input is allowed.
	want, _ = AD(s.H, s.L, s.C, s.V)
	buf := append([]float64(nil), s.V...)
	beg, nb, _ = adRange(0, len(buf)-1, s.H, s.L, s.C, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
