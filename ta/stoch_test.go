package ta

import (
	"errors"
	"math"
	"testing"
)

type stochParamSet struct {
	fastK, slowK int
	slowKT       MAType
	slowD        int
	slowDT       MAType
}

func TestSTOCHParams(t *testing.T) {
	s := fixtures(60)[0]
	bad := []stochParamSet{
		{0, 3, MATypeSMA, 3, MATypeSMA},
		{100001, 3, MATypeSMA, 3, MATypeSMA},
		{-1, 3, MATypeSMA, 3, MATypeSMA},
		{5, 0, MATypeSMA, 3, MATypeSMA},
		{5, 100001, MATypeSMA, 3, MATypeSMA},
		{5, 3, -1, 3, MATypeSMA},
		{5, 3, MATypeRMA + 1, 3, MATypeSMA},
		{5, 3, MATypeSMA, 0, MATypeSMA},
		{5, 3, MATypeSMA, 100001, MATypeSMA},
		{5, 3, MATypeSMA, 3, -1},
		{5, 3, MATypeSMA, 3, MATypeRMA + 1},
		{math.MinInt32, 3, MATypeSMA, 3, MATypeSMA},
	}
	for _, p := range bad {
		if _, _, err := STOCH(s.H, s.L, s.C, p.fastK, p.slowK, p.slowKT, p.slowD, p.slowDT); !errors.Is(err, ErrBadParam) {
			t.Errorf("%+v: err = %v, want ErrBadParam", p, err)
		}
		if lb := STOCHLookback(p.fastK, p.slowK, p.slowKT, p.slowD, p.slowDT); lb != -1 {
			t.Errorf("STOCHLookback(%+v) = %d, want -1", p, lb)
		}
	}
	good := []stochParamSet{
		{1, 1, MATypeSMA, 1, MATypeSMA},
		{100000, 3, MATypeSMA, 3, MATypeSMA},
		{5, 100000, MATypeSMA, 3, MATypeSMA},
		{5, 3, MATypeSMA, 100000, MATypeSMA},
		{5, 3, MATypeDefault, 3, MATypeDefault},
		{5, 3, MATypeRMA, 3, MATypeRMA},
	}
	for _, p := range good {
		if _, _, err := STOCH(s.H, s.L, s.C, p.fastK, p.slowK, p.slowKT, p.slowD, p.slowDT); err != nil {
			t.Errorf("%+v: unexpected err %v", p, err)
		}
	}
	// MATypeDefault is SMA.
	if a, b := STOCHLookback(5, 3, MATypeDefault, 3, MATypeDefault), STOCHLookback(5, 3, MATypeSMA, 3, MATypeSMA); a != b || a != 8 {
		t.Errorf("default lookback %d, SMA lookback %d, want 8", a, b)
	}
	// Mismatched lengths.
	if _, _, err := STOCH(s.H, s.L[:59], s.C, 5, 3, MATypeSMA, 3, MATypeSMA); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: %v", err)
	}
	if _, _, err := STOCH(s.H, s.L, s.C[:10], 5, 3, MATypeSMA, 3, MATypeSMA); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: %v", err)
	}
	if _, _, err := STOCH(s.H, nil, s.C, 5, 3, MATypeSMA, 3, MATypeSMA); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inLow: %v", err)
	}
	// nil/empty input -> empty outputs.
	k, d, err := STOCH(nil, nil, nil, 5, 3, MATypeSMA, 3, MATypeSMA)
	if err != nil || len(k) != 0 || len(d) != 0 {
		t.Errorf("nil input: k=%v d=%v err=%v", k, d, err)
	}
	k, d, err = STOCH([]float64{}, []float64{}, []float64{}, 5, 3, MATypeSMA, 3, MATypeSMA)
	if err != nil || len(k) != 0 || len(d) != 0 {
		t.Errorf("empty input: k=%v d=%v err=%v", k, d, err)
	}
}

var stochTestSets = []stochParamSet{
	{1, 1, MATypeSMA, 1, MATypeSMA},
	{5, 3, MATypeSMA, 3, MATypeSMA},
	{14, 3, MATypeEMA, 5, MATypeWMA},
	{5, 4, MATypeKAMA, 3, MATypeT3},
	{3, 2, MATypeMAMA, 2, MATypeDisabled},
	{21, 7, MATypeTEMA, 4, MATypeHMA},
}

func TestSTOCHLengths(t *testing.T) {
	for _, p := range stochTestSets {
		lb := STOCHLookback(p.fastK, p.slowK, p.slowKT, p.slowD, p.slowDT)
		want := p.fastK - 1 + MALookback(p.slowK, p.slowKT) + MALookback(p.slowD, p.slowDT)
		if lb != want {
			t.Fatalf("STOCHLookback(%+v) = %d, want %d", p, lb, want)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				k, d, err := STOCH(s.H, s.L, s.C, p.fastK, p.slowK, p.slowKT, p.slowD, p.slowDT)
				if err != nil {
					t.Fatalf("%s n=%d %+v: %v", s.Name, n, p, err)
				}
				if len(k) != n || len(d) != n {
					t.Fatalf("%s n=%d %+v: len(k)=%d len(d)=%d", s.Name, n, p, len(k), len(d))
				}
				assertLeadingNaN(t, lbl("STOCH K ", s.Name, " n=", n, " ", p), k, lb)
				assertLeadingNaN(t, lbl("STOCH D ", s.Name, " n=", n, " ", p), d, lb)
			}
		}
	}
}

func TestSTOCHKnownValues(t *testing.T) {
	nan := math.NaN()
	h := []float64{10, 12, 11}
	l := []float64{8, 9, 7}
	c := []float64{9, 11, 8}
	// FastK(2): bar1 (11-8)/(12-8)*100 = 75, bar2 (8-7)/(12-7)*100 = 20.
	// SlowK period 1 is FastK; SlowD = SMA2 of SlowK = 47.5 at bar 2.
	k, d, err := STOCH(h, l, c, 2, 1, MATypeSMA, 2, MATypeSMA)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "STOCH K", k, []float64{nan, nan, 20}, 1e-12)
	assertReal(t, "STOCH D", d, []float64{nan, nan, 47.5}, 1e-12)

	// SlowK = SMA2 of FastK = 47.5, SlowD period 1 is SlowK.
	k, d, _ = STOCH(h, l, c, 2, 2, MATypeSMA, 1, MATypeSMA)
	assertReal(t, "STOCH K smoothed", k, []float64{nan, nan, 47.5}, 1e-12)
	assertReal(t, "STOCH D identity", d, []float64{nan, nan, 47.5}, 1e-12)

	// Period-1 FastK: the window is the bar itself, so (C-L)/(H-L)*100.
	s := fixtures(80)[0]
	k, d, _ = STOCH(s.H, s.L, s.C, 1, 1, MATypeSMA, 1, MATypeSMA)
	for i := range s.C {
		want := (s.C[i] - s.L[i]) / (s.H[i] - s.L[i]) * 100.0
		if !closeEnough(k[i], want, 1e-12) || k[i] != d[i] {
			t.Fatalf("period 1 [%d]: k=%g d=%g want %g", i, k[i], d[i], want)
		}
	}

	// A flat window yields 0, not NaN.
	cst := fixtures(40)[5]
	k, d, _ = STOCH(cst.H, cst.L, cst.C, 5, 3, MATypeSMA, 3, MATypeSMA)
	for i := 8; i < len(k); i++ {
		if k[i] != 0 || d[i] != 0 {
			t.Fatalf("constant [%d]: k=%g d=%g", i, k[i], d[i])
		}
	}

	// Bounds: with averaging MAs the lines stay in [0, 100].
	for _, mt := range []MAType{MATypeSMA, MATypeEMA, MATypeWMA, MATypeTRIMA} {
		for _, s := range fixtures(300) {
			k, d, _ := STOCH(s.H, s.L, s.C, 14, 3, mt, 3, mt)
			for i := range k {
				if k[i] < -1e-9 || k[i] > 100+1e-9 || d[i] < -1e-9 || d[i] > 100+1e-9 {
					t.Fatalf("%s mt=%d [%d]: k=%g d=%g out of [0,100]", s.Name, mt, i, k[i], d[i])
				}
			}
		}
	}

	// Rising series: the close is near the top of every window.
	up := fixtures(50)[7]
	k, _, _ = STOCH(up.H, up.L, up.C, 5, 3, MATypeSMA, 3, MATypeSMA)
	for i := 8; i < len(k); i++ {
		if k[i] < 80 {
			t.Fatalf("up trend [%d]: k=%g, want >= 80", i, k[i])
		}
	}
}

func TestSTOCHRangeForm(t *testing.T) {
	s := fixtures(40)[0]
	k := make([]float64, 40)
	d := make([]float64, 40)
	if _, _, err := stochRange(-1, 5, s.H, s.L, s.C, 5, 3, MATypeSMA, 3, MATypeSMA, k, d); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := stochRange(5, 4, s.H, s.L, s.C, 5, 3, MATypeSMA, 3, MATypeSMA, k, d); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := stochRange(0, -1, s.H, s.L, s.C, 5, 3, MATypeSMA, 3, MATypeSMA, k, d); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (8): no output, no error.
	beg, nb, err := stochRange(0, 7, s.H, s.L, s.C, 5, 3, MATypeSMA, 3, MATypeSMA, k, d)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := stochRange(0, 30, s.H[:20], s.L, s.C, 5, 3, MATypeSMA, 3, MATypeSMA, k, d); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: %v", err)
	}
	if _, _, err := stochRange(0, 30, s.H, s.L, s.C, 5, 3, MATypeSMA, 3, MATypeSMA, k[:5], d); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outSlowK: %v", err)
	}
	if _, _, err := stochRange(0, 30, s.H, s.L, s.C, 5, 3, MATypeSMA, 3, MATypeSMA, k, d[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outSlowD: %v", err)
	}
	// Bad parameter in the range form.
	if _, _, err := stochRange(0, 30, s.H, s.L, s.C, 0, 3, MATypeSMA, 3, MATypeSMA, k, d); !errors.Is(err, ErrBadParam) {
		t.Errorf("bad fastK: %v", err)
	}
	// outSlowK == outSlowD is rejected like C.
	if _, _, err := stochRange(0, 39, s.H, s.L, s.C, 5, 3, MATypeSMA, 3, MATypeSMA, k, k); !errors.Is(err, ErrBadParam) {
		t.Errorf("same output buffer: %v", err)
	}

	wantK, wantD, _ := STOCH(s.H, s.L, s.C, 5, 3, MATypeSMA, 3, MATypeSMA)
	// Exact-size output buffers (endIdx-startIdx+1) are enough.
	k2 := make([]float64, 21)
	d2 := make([]float64, 21)
	beg, nb, err = stochRange(10, 30, s.H, s.L, s.C, 5, 3, MATypeSMA, 3, MATypeSMA, k2, d2)
	if err != nil || beg != 10 || nb != 21 {
		t.Fatalf("exact buffers: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// A later start re-seeds the SMA running sums, so values may differ from
	// the full-series run in the last ulp (C behaves the same way).
	assertReal(t, "exact K", k2, wantK[10:31], relTol)
	assertReal(t, "exact D", d2, wantD[10:31], relTol)

	// Outputs may alias the price inputs.
	h := append([]float64(nil), s.H...)
	l := append([]float64(nil), s.L...)
	c := append([]float64(nil), s.C...)
	beg, nb, _ = stochRange(0, 39, h, l, c, 5, 3, MATypeSMA, 3, MATypeSMA, h, c)
	alignReal(h, beg, nb)
	alignReal(c, beg, nb)
	assertReal(t, "aliased K", h, wantK, 0)
	assertReal(t, "aliased D", c, wantD, 0)
}
