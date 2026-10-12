package ta

import (
	"errors"
	"math"
	"testing"
)

func TestSTOCHFParams(t *testing.T) {
	s := fixtures(50)[0]
	h, l, c := s.H, s.L, s.C
	for _, p := range []int{0, -1, 100001, math.MinInt32} {
		if _, _, err := STOCHF(h, l, c, p, 3, MATypeSMA); !errors.Is(err, ErrBadParam) {
			t.Errorf("fastK %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := STOCHFLookback(p, 3, MATypeSMA); lb != -1 {
			t.Errorf("STOCHFLookback(%d,3,SMA) = %d, want -1", p, lb)
		}
		if _, _, err := STOCHF(h, l, c, 5, p, MATypeSMA); !errors.Is(err, ErrBadParam) {
			t.Errorf("fastD %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := STOCHFLookback(5, p, MATypeSMA); lb != -1 {
			t.Errorf("STOCHFLookback(5,%d,SMA) = %d, want -1", p, lb)
		}
	}
	for _, mt := range []MAType{-1, 14, 100} {
		if _, _, err := STOCHF(h, l, c, 5, 3, mt); !errors.Is(err, ErrBadParam) {
			t.Errorf("matype %d: err = %v, want ErrBadParam", mt, err)
		}
		if lb := STOCHFLookback(5, 3, mt); lb != -1 {
			t.Errorf("STOCHFLookback(5,3,%d) = %d, want -1", mt, lb)
		}
		if _, _, err := stochfRange(0, 49, h, l, c, 5, 3, mt, make([]float64, 50), make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("stochfRange matype %d: err = %v, want ErrBadParam", mt, err)
		}
	}
	for _, mt := range maAllTypes {
		for _, pk := range []int{1, 2, 100000} {
			for _, pd := range []int{1, 2, 100000} {
				if _, _, err := STOCHF(h, l, c, pk, pd, mt); err != nil {
					t.Errorf("k=%d d=%d mt=%d: unexpected err %v", pk, pd, mt, err)
				}
				if lb := STOCHFLookback(pk, pd, mt); lb < 0 {
					t.Errorf("STOCHFLookback(%d,%d,%d) = %d", pk, pd, mt, lb)
				}
			}
		}
	}
	// Default MAType is SMA.
	if a, b := STOCHFLookback(5, 3, MATypeDefault), STOCHFLookback(5, 3, MATypeSMA); a != b || a != 6 {
		t.Errorf("default matype lookback %d, SMA %d, want 6", a, b)
	}
	// Mismatched lengths.
	if _, _, err := STOCHF(h, l[:49], c, 5, 3, MATypeSMA); !errors.Is(err, ErrBadParam) {
		t.Errorf("short low: err = %v", err)
	}
	if _, _, err := STOCHF(h, l, c[:10], 5, 3, MATypeSMA); !errors.Is(err, ErrBadParam) {
		t.Errorf("short close: err = %v", err)
	}
	if _, _, err := STOCHF(nil, l, c, 5, 3, MATypeSMA); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil high: err = %v", err)
	}
	// nil / empty input.
	if k, d, err := STOCHF(nil, nil, nil, 5, 3, MATypeSMA); err != nil || len(k) != 0 || len(d) != 0 {
		t.Errorf("nil input: k=%v d=%v err=%v", k, d, err)
	}
	if k, d, err := STOCHF([]float64{}, []float64{}, []float64{}, 5, 3, MATypeEMA); err != nil || len(k) != 0 || len(d) != 0 {
		t.Errorf("empty input: k=%v d=%v err=%v", k, d, err)
	}
}

func TestSTOCHFLookbackValues(t *testing.T) {
	for _, pk := range []int{1, 5, 14} {
		for _, pd := range []int{1, 3, 10} {
			for _, mt := range maAllTypes {
				want := pk - 1 + MALookback(pd, mt)
				if got := STOCHFLookback(pk, pd, mt); got != want {
					t.Errorf("STOCHFLookback(%d,%d,%d) = %d, want %d", pk, pd, mt, got, want)
				}
			}
		}
	}
}

type stochfParams struct {
	k, d int
	mt   MAType
}

var stochfParamSets = []stochfParams{
	{1, 1, MATypeSMA}, {5, 3, MATypeSMA}, {5, 3, MATypeDefault}, {14, 3, MATypeEMA},
	{3, 5, MATypeWMA}, {5, 3, MATypeDEMA}, {5, 3, MATypeTEMA}, {5, 3, MATypeTRIMA},
	{5, 3, MATypeKAMA}, {5, 3, MATypeMAMA}, {5, 3, MATypeT3}, {5, 4, MATypeHMA},
	{5, 3, MATypeDisabled}, {5, 3, MATypeZLEMA}, {5, 3, MATypeRMA}, {30, 1, MATypeSMA},
}

func TestSTOCHFLengths(t *testing.T) {
	for _, p := range stochfParamSets {
		lb := STOCHFLookback(p.k, p.d, p.mt)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				k, d, err := STOCHF(s.H, s.L, s.C, p.k, p.d, p.mt)
				if err != nil {
					t.Fatalf("%s n=%d %v: %v", s.Name, n, p, err)
				}
				if len(k) != n || len(d) != n {
					t.Fatalf("%s n=%d %v: len k=%d d=%d", s.Name, n, p, len(k), len(d))
				}
				assertLeadingNaN(t, lbl("STOCHF K ", s.Name, " n=", n, " ", p), k, lb)
				assertLeadingNaN(t, lbl("STOCHF D ", s.Name, " n=", n, " ", p), d, lb)
			}
		}
	}
}

func TestSTOCHFKnownValues(t *testing.T) {
	nan := math.NaN()
	h := []float64{10, 12, 11, 13, 12}
	l := []float64{8, 9, 7, 10, 11}
	c := []float64{9, 11, 10, 12, 11.5}
	// K(3): bar2 window L 8,9,7 H 10,12,11 -> (10-7)/(12-7)*100 = 60
	//       bar3 window L 9,7,10 H 12,11,13 -> (12-7)/(13-7)*100 = 83.33..
	//       bar4 window L 7,10,11 H 11,13,12 -> (11.5-7)/(13-7)*100 = 75
	k3 := (12.0 - 7) / (13.0 - 7) * 100
	k, d, err := STOCHF(h, l, c, 3, 2, MATypeSMA)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "K(3)", k, []float64{nan, nan, nan, k3, 75}, 1e-12)
	assertReal(t, "D(2,SMA)", d, []float64{nan, nan, nan, (60 + k3) / 2, (k3 + 75) / 2}, 1e-12)

	// K period 1, D period 1: K is the position of the close within the bar,
	// and D (period 1 is the MA identity) equals K.
	s := fixtures(80)[0]
	k, d, _ = STOCHF(s.H, s.L, s.C, 1, 1, MATypeEMA)
	for i := range k {
		want := (s.C[i] - s.L[i]) / (s.H[i] - s.L[i]) * 100
		if !closeEnough(k[i], want, 1e-12) || k[i] != d[i] {
			t.Fatalf("period 1 [%d]: k=%g d=%g want %g", i, k[i], d[i], want)
		}
	}

	// A flat window gives K = 0 (and so D = 0).
	cs := fixtures(40)[5]
	k, d, _ = STOCHF(cs.H, cs.L, cs.C, 5, 3, MATypeSMA)
	for i := 6; i < len(k); i++ {
		if k[i] != 0 || d[i] != 0 {
			t.Fatalf("constant [%d]: k=%g d=%g", i, k[i], d[i])
		}
	}

	// Bounds: with low <= close <= high, K and an SMA of K lie in [0, 100].
	for _, s := range fixtures(300) {
		k, d, _ := STOCHF(s.H, s.L, s.C, 14, 3, MATypeSMA)
		for i := 15; i < len(k); i++ {
			if k[i] < 0 || k[i] > 100 || d[i] < -1e-9 || d[i] > 100+1e-9 {
				t.Fatalf("%s [%d]: k=%g d=%g out of [0,100]", s.Name, i, k[i], d[i])
			}
		}
	}

	// D is the MA of K. K is only emitted from bar 11 (the full lookback), so
	// a windowed MA of the emitted K matches D from bar 14 on (up to rounding:
	// WMA keeps running sums, so a different start changes the last ulps).
	s = fixtures(200)[1]
	k, d, _ = STOCHF(s.H, s.L, s.C, 9, 4, MATypeWMA)
	want, _ := MA(k[11:], 4, MATypeWMA)
	assertReal(t, "D = WMA(K)", d[14:], want[3:], 1e-12)
}

func TestSTOCHFRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	h, l, c := s.H, s.L, s.C
	k := make([]float64, len(c))
	d := make([]float64, len(c))
	if _, _, err := stochfRange(-1, 5, h, l, c, 5, 3, MATypeSMA, k, d); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := stochfRange(5, 4, h, l, c, 5, 3, MATypeSMA, k, d); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := stochfRange(0, -1, h, l, c, 5, 3, MATypeSMA, k, d); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (6): no output, no error.
	beg, nb, err := stochfRange(0, 5, h, l, c, 5, 3, MATypeSMA, k, d)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := stochfRange(0, 20, h, l[:10], c, 5, 3, MATypeSMA, k, d); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := stochfRange(0, 20, h, l, c, 5, 3, MATypeSMA, k[:5], d); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outFastK: %v", err)
	}
	if _, _, err := stochfRange(0, 20, h, l, c, 5, 3, MATypeSMA, k, d[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outFastD: %v", err)
	}
	// outFastK == outFastD is rejected, like C.
	if _, _, err := stochfRange(0, 20, h, l, c, 5, 3, MATypeSMA, k, k); !errors.Is(err, ErrBadParam) {
		t.Errorf("same output buffer: %v", err)
	}
	// Exact-size output buffers for a sub-range work.
	wantK, wantD, _ := STOCHF(h, l, c, 5, 3, MATypeSMA)
	k2, d2 := make([]float64, 11), make([]float64, 11)
	beg, nb, err = stochfRange(10, 20, h, l, c, 5, 3, MATypeSMA, k2, d2)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range K", k2, wantK[10:21], 0)
	assertReal(t, "sub-range D", d2, wantD[10:21], 0)
	// outFastK may alias each price input (it then doubles as the scratch).
	for which := range 3 {
		hh := append([]float64(nil), h...)
		ll := append([]float64(nil), l...)
		cc := append([]float64(nil), c...)
		alias := [][]float64{hh, ll, cc}[which]
		dd := make([]float64, len(c))
		beg, nb, err := stochfRange(0, len(c)-1, hh, ll, cc, 5, 3, MATypeEMA, alias, dd)
		if err != nil {
			t.Fatal(err)
		}
		wk, wd, _ := STOCHF(h, l, c, 5, 3, MATypeEMA)
		kk := append([]float64(nil), alias...)
		alignReal(kk, beg, nb)
		alignReal(dd, beg, nb)
		assertReal(t, lbl("in-place K alias ", which), kk, wk, 0)
		assertReal(t, lbl("in-place D alias ", which), dd, wd, 0)
	}
}
