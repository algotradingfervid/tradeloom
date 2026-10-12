package ta

import (
	"errors"
	"math"
	"testing"
)

func TestDXParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := DX(s.H, s.L, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := DXLookback(p); lb != -1 {
			t.Errorf("DXLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := dxRange(0, 49, s.H, s.L, s.C, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("dxRange period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, err := DX(s.H, s.L, s.C, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := DXLookback(p); lb != p {
			t.Errorf("DXLookback(%d) = %d, want %d", p, lb, p)
		}
	}
	// Mismatched input lengths.
	if _, err := DX(s.H, s.L[:49], s.C, 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: err = %v", err)
	}
	if _, err := DX(s.H, s.L, s.C[:10], 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v", err)
	}
	if _, err := DX(s.H[:1], s.L, s.C, 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v", err)
	}
	if _, err := DX(nil, s.L, s.C, 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inHigh with others set: err = %v", err)
	}
	// nil / empty input.
	if out, err := DX(nil, nil, nil, 14); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := DX([]float64{}, []float64{}, []float64{}, 14); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Short buffers in the range form.
	if _, _, err := dxRange(0, 10, s.H, s.L, s.C[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := dxRange(0, 10, s.H, s.L, s.C, 3, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := dxRange(5, 10, s.H, s.L, s.C, 3, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestDXLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30} {
		lb := DXLookback(period)
		if lb != period {
			t.Fatalf("DXLookback(%d) = %d, want %d", period, lb, period)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := DX(s.H, s.L, s.C, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("DX ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestDXKnownValues(t *testing.T) {
	// Period 2 by hand:
	//   bar1: diffP=2,  diffM=-1 -> +DM1=2; TR1=3. Seed: +DM=2, -DM=0, TR=3.
	//   bar2: diffP=-1, diffM=2  -> -DM1=2; TR1=4.
	//         +DM=2-1=1, -DM=0+2=2, TR=3-1.5+4=5.5 -> DX=100*|2-1|/(2+1)
	//   bar3: diffP=2,  diffM=-1 -> +DM1=2; TR1=5.
	//         +DM=1-0.5+2=2.5, -DM=2-1=1, TR=7.75 -> DX=100*1.5/3.5
	h := []float64{10, 12, 11, 13}
	l := []float64{8, 9, 7, 8}
	c := []float64{9, 11, 8, 12}
	nan := math.NaN()
	out, err := DX(h, l, c, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "DX p=2", out, []float64{nan, nan, 100.0 / 3, 100 * 1.5 / 3.5}, 1e-12)

	// A constant series has zero true range: DX is 0.
	cs := fixtures(40)[5]
	out, _ = DX(cs.H, cs.L, cs.C, 7)
	for i := 7; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// A strictly rising or falling series has movement in one direction
	// only: DX is 100.
	for _, idx := range []int{7, 8} {
		s := fixtures(60)[idx]
		out, _ = DX(s.H, s.L, s.C, 14)
		for i := 14; i < len(out); i++ {
			if math.Abs(out[i]-100) > 1e-9 {
				t.Fatalf("%s: [%d]=%g, want 100", s.Name, i, out[i])
			}
		}
	}

	// Bars with range but no directional movement: the DI sum is zero, so
	// the first output is 0 and later ones carry it forward.
	fh := []float64{10, 10, 10, 10, 10, 10}
	fl := []float64{8, 8, 8, 8, 8, 8}
	fc := []float64{9, 9, 9, 9, 9, 9}
	out, _ = DX(fh, fl, fc, 2)
	assertReal(t, "flat range", out, []float64{nan, nan, 0, 0, 0, 0}, 0)

	// After movement, a long run of identical bars decays both DM legs until
	// the DI sum is inside the zero band; from then on the last DX value is
	// carried forward unchanged.
	mh := append([]float64{10, 12, 11, 13}, make([]float64, 300)...)
	ml := append([]float64{8, 9, 7, 8}, make([]float64, 300)...)
	mc := append([]float64{9, 11, 8, 12}, make([]float64, 300)...)
	for i := 4; i < len(mh); i++ {
		mh[i], ml[i], mc[i] = 13, 8, 12
	}
	out, _ = DX(mh, ml, mc, 2)
	last := out[len(out)-1]
	if math.IsNaN(last) || last <= 0 || last > 100 {
		t.Fatalf("carry-forward: last=%g", last)
	}
	for i := len(out) - 100; i < len(out); i++ {
		if out[i] != last {
			t.Fatalf("carry-forward: [%d]=%g, want %g", i, out[i], last)
		}
	}

	// DX is in [0, 100].
	for _, s := range fixtures(300) {
		for _, p := range []int{2, 14} {
			out, _ = DX(s.H, s.L, s.C, p)
			for i := DXLookback(p); i < len(out); i++ {
				if !(out[i] >= 0 && out[i] <= 100+1e-9) {
					t.Fatalf("%s p=%d: [%d]=%g out of [0,100]", s.Name, p, i, out[i])
				}
			}
		}
	}
}

func TestDXUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstDX, 5); err != nil {
		t.Fatal(err)
	}
	if lb := DXLookback(14); lb != 19 {
		t.Errorf("unst=5 lookback(14) = %d, want 19", lb)
	}
	s := fixtures(100)[0]
	out, err := DX(s.H, s.L, s.C, 14)
	if err != nil {
		t.Fatal(err)
	}
	assertLeadingNaN(t, "DX unst=5", out, 19)
}

func TestDXRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := dxRange(-1, 5, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := dxRange(5, 4, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := dxRange(0, -1, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	for _, c := range []struct{ p, end int }{{3, 2}, {2, 1}, {14, 13}} {
		beg, nb, err := dxRange(0, c.end, s.H, s.L, s.C, c.p, out)
		if err != nil || beg != 0 || nb != 0 {
			t.Errorf("inside lookback p=%d end=%d: beg=%d nb=%d err=%v", c.p, c.end, beg, nb, err)
		}
	}
	// Like C, a sub-range restarts the Wilder seed at startIdx-lookback, so
	// it equals the full computation over inputs sliced from that bar.
	for _, p := range []int{2, 4} {
		lb := DXLookback(p)
		sub, _ := DX(s.H[10-lb:21], s.L[10-lb:21], s.C[10-lb:21], p)
		beg, nb, err := dxRange(10, 20, s.H, s.L, s.C, p, out)
		if err != nil || beg != 10 || nb != 11 {
			t.Fatalf("p=%d sub-range: beg=%d nb=%d err=%v", p, beg, nb, err)
		}
		assertReal(t, lbl("sub-range p=", p), out[:nb], sub[lb:], 0)
	}
	// In-place computation over inClose is allowed.
	for _, p := range []int{2, 4} {
		buf := append([]float64(nil), s.C...)
		want, _ := DX(s.H, s.L, s.C, p)
		beg, nb, _ := dxRange(0, len(buf)-1, s.H, s.L, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
