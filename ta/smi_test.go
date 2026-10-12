package ta

import (
	"errors"
	"math"
	"testing"
)

// smiParamSets are (time, fast, slow, signal) tuples used across the tests.
var smiParamSets = [][4]int{
	{13, 2, 25, 9}, // defaults
	{2, 2, 2, 2},   // all minimum
	{5, 3, 7, 4},
	{14, 3, 3, 3},
	{3, 10, 2, 6},
}

func TestSMIParams(t *testing.T) {
	s := fixtures(80)[0]
	valid := [4]int{13, 2, 25, 9}
	for k := 0; k < 4; k++ {
		for _, bad := range []int{1, 0, -1, 100001, math.MinInt32} {
			p := valid
			p[k] = bad
			if _, _, err := SMI(s.H, s.L, s.C, p[0], p[1], p[2], p[3]); !errors.Is(err, ErrBadParam) {
				t.Errorf("param %d = %d: err = %v, want ErrBadParam", k, bad, err)
			}
			if lb := SMILookback(p[0], p[1], p[2], p[3]); lb != -1 {
				t.Errorf("SMILookback%v = %d, want -1", p, lb)
			}
			out := make([]float64, len(s.C))
			out2 := make([]float64, len(s.C))
			if _, _, err := smiRange(0, len(s.C)-1, s.H, s.L, s.C, p[0], p[1], p[2], p[3], out, out2); !errors.Is(err, ErrBadParam) {
				t.Errorf("range param %d = %d: err = %v, want ErrBadParam", k, bad, err)
			}
		}
		for _, good := range []int{2, 3, 100000} {
			p := valid
			p[k] = good
			if _, _, err := SMI(s.H, s.L, s.C, p[0], p[1], p[2], p[3]); err != nil {
				t.Errorf("param %d = %d: unexpected err %v", k, good, err)
			}
			if lb := SMILookback(p[0], p[1], p[2], p[3]); lb < 0 {
				t.Errorf("SMILookback%v = %d", p, lb)
			}
		}
	}
	// Mismatched input lengths.
	if _, _, err := SMI(s.H, s.L[:79], s.C, 13, 2, 25, 9); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: err = %v", err)
	}
	if _, _, err := SMI(s.H, s.L, s.C[:10], 13, 2, 25, 9); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v", err)
	}
	if _, _, err := SMI(s.H[:1], s.L, s.C, 13, 2, 25, 9); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: err = %v", err)
	}
	// nil / empty input.
	if a, b, err := SMI(nil, nil, nil, 13, 2, 25, 9); err != nil || len(a) != 0 || len(b) != 0 {
		t.Errorf("nil input: a=%v b=%v err=%v", a, b, err)
	}
	if a, b, err := SMI([]float64{}, []float64{}, []float64{}, 13, 2, 25, 9); err != nil || len(a) != 0 || len(b) != 0 {
		t.Errorf("empty input: a=%v b=%v err=%v", a, b, err)
	}
	// Lookback formula.
	if lb := SMILookback(13, 2, 25, 9); lb != 12+24+1+8 {
		t.Errorf("SMILookback defaults = %d, want 45", lb)
	}
	if lb := SMILookback(2, 2, 2, 2); lb != 4 {
		t.Errorf("SMILookback mins = %d, want 4", lb)
	}
	if lb := SMILookback(100000, 100000, 100000, 100000); lb != 4*99999 {
		t.Errorf("SMILookback max = %d", lb)
	}
}

func TestSMILengths(t *testing.T) {
	for _, p := range smiParamSets {
		lb := SMILookback(p[0], p[1], p[2], p[3])
		if lb != p[0]-1+p[1]-1+p[2]-1+p[3]-1 {
			t.Fatalf("SMILookback%v = %d", p, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				a, b, err := SMI(s.H, s.L, s.C, p[0], p[1], p[2], p[3])
				if err != nil {
					t.Fatalf("%s n=%d p=%v: %v", s.Name, n, p, err)
				}
				if len(a) != n || len(b) != n {
					t.Fatalf("%s n=%d p=%v: len=%d,%d", s.Name, n, p, len(a), len(b))
				}
				assertLeadingNaN(t, lbl("SMI ", s.Name, " n=", n, " p=", p), a, lb)
				assertLeadingNaN(t, lbl("SMI signal ", s.Name, " n=", n, " p=", p), b, lb)
			}
		}
	}
}

// smiComposed computes SMI from MAX, MIN and four EMA calls, the composition
// the C comment says the one-pass loop is bit-identical to.
func smiComposed(t *testing.T, h, l, c []float64, tp, fast, slow, signal int) ([]float64, []float64) {
	t.Helper()
	n := len(c)
	hh, _ := MAX(h, tp)
	ll, _ := MIN(l, tp)
	start := tp - 1
	num := make([]float64, 0, n)
	den := make([]float64, 0, n)
	for i := start; i < n; i++ {
		den = append(den, hh[i]-ll[i])
		num = append(num, c[i]-(hh[i]+ll[i])*0.5)
	}
	trim := func(x []float64, k int) []float64 { return x[k:] }
	sNum, _ := EMA(num, slow)
	sDen, _ := EMA(den, slow)
	k1 := EMALookback(slow)
	fNum, _ := EMA(trim(sNum, k1), fast)
	fDen, _ := EMA(trim(sDen, k1), fast)
	k2 := EMALookback(fast)
	fNum, fDen = trim(fNum, k2), trim(fDen, k2)
	smi := make([]float64, len(fNum))
	for i := range fNum {
		half := 0.5 * fDen[i]
		if half > 0 {
			smi[i] = 100.0 * fNum[i] / half
		}
	}
	sig, _ := EMA(smi, signal)
	k3 := EMALookback(signal)
	lb := start + k1 + k2 + k3
	outA, outB := nanSlice(n), nanSlice(n)
	for i := lb; i < n; i++ {
		outA[i] = smi[i-start-k1-k2]
		outB[i] = sig[i-start-k1-k2]
	}
	return outA, outB
}

func TestSMIKnownValues(t *testing.T) {
	nan := math.NaN()
	// Every bar has range [90, 110] (midpoint 100, den 20) and closes at the
	// high: num = 10, so SMI = 100*10/(0.5*20) = 100, and the signal too.
	n := 8
	h, l, cHi, cLo, cMid := make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
	for i := range n {
		h[i], l[i], cHi[i], cLo[i], cMid[i] = 110, 90, 110, 90, 100
	}
	want := func(v float64) []float64 { return []float64{nan, nan, nan, nan, v, v, v, v} }
	a, b, err := SMI(h, l, cHi, 2, 2, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "close at high", a, want(100), 0)
	assertReal(t, "close at high signal", b, want(100), 0)
	a, b, _ = SMI(h, l, cLo, 2, 2, 2, 2)
	assertReal(t, "close at low", a, want(-100), 0)
	assertReal(t, "close at low signal", b, want(-100), 0)
	a, b, _ = SMI(h, l, cMid, 2, 2, 2, 2)
	assertReal(t, "close at mid", a, want(0), 0)
	assertReal(t, "close at mid signal", b, want(0), 0)

	// Hand-derived: periods all 2 (k = 2/3), lookback 4.
	// H = [10,12,14,16,18,20], L = [8,9,10,11,12,13], C = [9,11,13,15,17,19].
	// From bar 1 on, HH = H[i] and LL = L[i-1], so for bars 1..5
	// den = 4,5,6,7,8 and num = C - (HH+LL)/2 = 1,1.5,2,2.5,3.
	// Slow EMA (bars 2..5): num 5/4,7/4,9/4,11/4; den 9/2,11/2,13/2,15/2.
	// Fast EMA (bars 3..5): num 3/2, 2, 5/2; den 5, 6, 7.
	// SMI (bars 3..5) = 100*num/(den/2): 60, 200/3, 500/7.
	// Signal seeds at bar 4 with (60 + 200/3)/2 = 190/3, then 4330/63.
	H := []float64{10, 12, 14, 16, 18, 20}
	L := []float64{8, 9, 10, 11, 12, 13}
	C := []float64{9, 11, 13, 15, 17, 19}
	numv := []float64{11 - 10, 13 - 11.5, 15 - 13, 17 - 14.5, 19 - 16}
	denv := []float64{12 - 8, 14 - 9, 16 - 10, 18 - 11, 20 - 12}
	k := 2.0 / 3.0
	ema2 := func(x []float64) []float64 { // EMA period 2 with SMA seed, from index 1
		out := make([]float64, len(x))
		out[1] = (x[0] + x[1]) / 2
		for i := 2; i < len(x); i++ {
			out[i] = out[i-1] + k*(x[i]-out[i-1])
		}
		return out[1:]
	}
	fn, fd := ema2(ema2(numv)), ema2(ema2(denv))
	smi := make([]float64, len(fn))
	for i := range fn {
		smi[i] = 100 * fn[i] / (0.5 * fd[i])
	}
	sig := ema2(smi)
	a, b, _ = SMI(H, L, C, 2, 2, 2, 2)
	assertReal(t, "hand SMI", a, []float64{nan, nan, nan, nan, smi[1], smi[2]}, 1e-12)
	assertReal(t, "hand signal", b, []float64{nan, nan, nan, nan, sig[0], sig[1]}, 1e-12)
	if math.Abs(a[4]-200.0/3) > 1e-12 || math.Abs(a[5]-500.0/7) > 1e-12 || math.Abs(b[4]-190.0/3) > 1e-12 || math.Abs(b[5]-4330.0/63) > 1e-12 {
		t.Errorf("hand values: SMI[4]=%.17g SMI[5]=%.17g signal[4]=%.17g signal[5]=%.17g", a[4], a[5], b[4], b[5])
	}

	// Flat bars (H == L): den is 0, so SMI is the neutral 0 (not NaN).
	for _, s := range fixtures(60)[5:7] {
		a, b, _ := SMI(s.H, s.L, s.C, 5, 3, 7, 4)
		for i := SMILookback(5, 3, 7, 4); i < len(a); i++ {
			if a[i] != 0 || b[i] != 0 {
				t.Fatalf("%s: [%d] = %g, %g, want 0", s.Name, i, a[i], b[i])
			}
		}
	}

	// Bounds: SMI is within [-100, 100] for well-formed bars.
	for _, s := range fixtures(400) {
		for _, p := range smiParamSets {
			a, b, _ := SMI(s.H, s.L, s.C, p[0], p[1], p[2], p[3])
			for i := range a {
				if !math.IsNaN(a[i]) && (a[i] < -100-1e-9 || a[i] > 100+1e-9 || b[i] < -100-1e-9 || b[i] > 100+1e-9) {
					t.Fatalf("%s p=%v: [%d] = %g, %g out of [-100,100]", s.Name, p, i, a[i], b[i])
				}
			}
		}
	}

	// Equivalence with MAX/MIN + four EMA calls.
	for _, s := range fixtures(300) {
		for _, p := range smiParamSets {
			a, b, _ := SMI(s.H, s.L, s.C, p[0], p[1], p[2], p[3])
			wa, wb := smiComposed(t, s.H, s.L, s.C, p[0], p[1], p[2], p[3])
			assertReal(t, lbl("composed ", s.Name, p), a, wa, 1e-9)
			assertReal(t, lbl("composed signal ", s.Name, p), b, wb, 1e-9)
		}
	}
}

func TestSMIRangeForm(t *testing.T) {
	s := fixtures(60)[0]
	n := len(s.C)
	out, out2 := make([]float64, n), make([]float64, n)
	if _, _, err := smiRange(-1, 5, s.H, s.L, s.C, 2, 2, 2, 2, out, out2); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := smiRange(5, 4, s.H, s.L, s.C, 2, 2, 2, 2, out, out2); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := smiRange(0, -1, s.H, s.L, s.C, 2, 2, 2, 2, out, out2); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (4): no output, no error.
	beg, nb, err := smiRange(0, 3, s.H, s.L, s.C, 2, 2, 2, 2, out, out2)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := smiRange(0, 30, s.H[:10], s.L, s.C, 2, 2, 2, 2, out, out2); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: %v", err)
	}
	if _, _, err := smiRange(0, 30, s.H, s.L, s.C[:30], 2, 2, 2, 2, out, out2); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: %v", err)
	}
	if _, _, err := smiRange(10, 30, s.H, s.L, s.C, 2, 2, 2, 2, out[:20], out2); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outSMI: %v", err)
	}
	if _, _, err := smiRange(10, 30, s.H, s.L, s.C, 2, 2, 2, 2, out, out2[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outSMISignal: %v", err)
	}
	// The same buffer for both outputs is rejected, as in C.
	if _, _, err := smiRange(0, n-1, s.H, s.L, s.C, 2, 2, 2, 2, out, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("aliased outputs: %v", err)
	}
	// A sub-range re-seeds its EMAs from startIdx-lookback, so it reports
	// startIdx and endIdx-startIdx+1 values (values are checked against C in
	// the oracle tests).
	beg, nb, err = smiRange(20, 40, s.H, s.L, s.C, 5, 3, 7, 4, out, out2)
	if err != nil || beg != 20 || nb != 21 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// A range starting at the lookback equals the full computation there.
	fa, fb, _ := SMI(s.H, s.L, s.C, 5, 3, 7, 4)
	lb := SMILookback(5, 3, 7, 4)
	beg, nb, _ = smiRange(0, n-1, s.H, s.L, s.C, 5, 3, 7, 4, out, out2)
	if beg != lb || nb != n-lb {
		t.Fatalf("full range: beg=%d nb=%d", beg, nb)
	}
	assertReal(t, "full-range SMI", out[:nb], fa[lb:], 0)
	assertReal(t, "full-range signal", out2[:nb], fb[lb:], 0)
}
