package ta

import (
	"errors"
	"math"
	"testing"
)

type kdjParamSet struct {
	fastK, slowK int
	slowKT       MAType
	slowD        int
	slowDT       MAType
}

func kdjCall(s series, p kdjParamSet) ([]float64, []float64, []float64, error) {
	return KDJ(s.H, s.L, s.C, p.fastK, p.slowK, p.slowKT, p.slowD, p.slowDT)
}

func kdjLB(p kdjParamSet) int {
	return KDJLookback(p.fastK, p.slowK, p.slowKT, p.slowD, p.slowDT)
}

func TestKDJParams(t *testing.T) {
	s := fixtures(60)[0]
	bad := []kdjParamSet{
		{0, 3, MATypeRMA, 3, MATypeRMA},
		{100001, 3, MATypeRMA, 3, MATypeRMA},
		{-1, 3, MATypeRMA, 3, MATypeRMA},
		{math.MinInt32, 3, MATypeRMA, 3, MATypeRMA},
		{9, 0, MATypeRMA, 3, MATypeRMA},
		{9, 100001, MATypeRMA, 3, MATypeRMA},
		{9, 3, -1, 3, MATypeRMA},
		{9, 3, MATypeRMA + 1, 3, MATypeRMA},
		{9, 3, MATypeRMA, 0, MATypeRMA},
		{9, 3, MATypeRMA, 100001, MATypeRMA},
		{9, 3, MATypeRMA, 3, -1},
		{9, 3, MATypeRMA, 3, MATypeRMA + 1},
	}
	for _, p := range bad {
		if _, _, _, err := kdjCall(s, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("%+v: err = %v, want ErrBadParam", p, err)
		}
		if lb := kdjLB(p); lb != -1 {
			t.Errorf("KDJLookback(%+v) = %d, want -1", p, lb)
		}
		k := make([]float64, 60)
		d := make([]float64, 60)
		j := make([]float64, 60)
		if _, _, err := kdjRange(0, 59, s.H, s.L, s.C, p.fastK, p.slowK, p.slowKT, p.slowD, p.slowDT, k, d, j); !errors.Is(err, ErrBadParam) {
			t.Errorf("range %+v: err = %v, want ErrBadParam", p, err)
		}
	}
	good := []kdjParamSet{
		{1, 1, MATypeSMA, 1, MATypeSMA},
		{100000, 3, MATypeRMA, 3, MATypeRMA},
		{9, 100000, MATypeRMA, 3, MATypeRMA},
		{9, 3, MATypeRMA, 100000, MATypeRMA},
		{9, 3, MATypeDefault, 3, MATypeDefault},
		{9, 3, MATypeSMA, 3, MATypeRMA},
	}
	for _, p := range good {
		if _, _, _, err := kdjCall(s, p); err != nil {
			t.Errorf("%+v: unexpected err %v", p, err)
		}
		if lb := kdjLB(p); lb < 0 {
			t.Errorf("KDJLookback(%+v) = %d", p, lb)
		}
	}
	// MATypeDefault is RMA (C substitutes 13), not SMA as in STOCH.
	if a, b := KDJLookback(9, 3, MATypeDefault, 3, MATypeDefault), KDJLookback(9, 3, MATypeRMA, 3, MATypeRMA); a != b || a != 12 {
		t.Errorf("default lookback %d, RMA lookback %d, want 12", a, b)
	}
	// Mismatched lengths and nil components.
	if _, _, _, err := KDJ(s.H, s.L[:59], s.C, 9, 3, MATypeRMA, 3, MATypeRMA); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: %v", err)
	}
	if _, _, _, err := KDJ(s.H, s.L, s.C[:10], 9, 3, MATypeRMA, 3, MATypeRMA); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: %v", err)
	}
	if _, _, _, err := KDJ(nil, s.L, s.C, 9, 3, MATypeRMA, 3, MATypeRMA); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil inHigh: %v", err)
	}
	// nil/empty input -> empty outputs.
	k, d, j, err := KDJ(nil, nil, nil, 9, 3, MATypeRMA, 3, MATypeRMA)
	if err != nil || len(k) != 0 || len(d) != 0 || len(j) != 0 {
		t.Errorf("nil input: k=%v d=%v j=%v err=%v", k, d, j, err)
	}
	k, d, j, err = KDJ([]float64{}, []float64{}, []float64{}, 9, 3, MATypeRMA, 3, MATypeRMA)
	if err != nil || len(k) != 0 || len(d) != 0 || len(j) != 0 {
		t.Errorf("empty input: k=%v d=%v j=%v err=%v", k, d, j, err)
	}
}

var kdjTestSets = []kdjParamSet{
	{1, 1, MATypeSMA, 1, MATypeSMA},
	{9, 3, MATypeRMA, 3, MATypeRMA},
	{9, 3, MATypeDefault, 3, MATypeDefault},
	{5, 3, MATypeSMA, 3, MATypeSMA},
	{14, 3, MATypeEMA, 5, MATypeWMA},
	{5, 4, MATypeKAMA, 3, MATypeT3},
	{3, 2, MATypeMAMA, 2, MATypeDisabled},
	{21, 7, MATypeTEMA, 4, MATypeHMA},
}

func TestKDJLengths(t *testing.T) {
	for _, p := range kdjTestSets {
		lb := kdjLB(p)
		kt, dt := p.slowKT, p.slowDT
		if kt == MATypeDefault {
			kt = MATypeRMA
		}
		if dt == MATypeDefault {
			dt = MATypeRMA
		}
		want := p.fastK - 1 + MALookback(p.slowK, kt) + MALookback(p.slowD, dt)
		if lb != want {
			t.Fatalf("KDJLookback(%+v) = %d, want %d", p, lb, want)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				k, d, j, err := kdjCall(s, p)
				if err != nil {
					t.Fatalf("%s n=%d %+v: %v", s.Name, n, p, err)
				}
				if len(k) != n || len(d) != n || len(j) != n {
					t.Fatalf("%s n=%d %+v: len k=%d d=%d j=%d", s.Name, n, p, len(k), len(d), len(j))
				}
				assertLeadingNaN(t, lbl("KDJ K ", s.Name, " n=", n, " ", p), k, lb)
				assertLeadingNaN(t, lbl("KDJ D ", s.Name, " n=", n, " ", p), d, lb)
				assertLeadingNaN(t, lbl("KDJ J ", s.Name, " n=", n, " ", p), j, lb)
			}
		}
	}
}

func TestKDJKnownValues(t *testing.T) {
	nan := math.NaN()
	h := []float64{10, 12, 11, 13}
	l := []float64{8, 9, 7, 10}
	c := []float64{9, 11, 10, 12}
	// RSV(3): bar2 100*(10-7)/(12-7) = 60, bar3 100*(12-7)/(13-7) = 250/3.
	// SlowK period 1 is RSV, D = SMA2(K) = (60+250/3)/2 at bar 3,
	// J = 3K - 2D = 2K - 60 = 500/3 - 60.
	k, d, j, err := KDJ(h, l, c, 3, 1, MATypeSMA, 2, MATypeSMA)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "KDJ K", k, []float64{nan, nan, nan, 250.0 / 3}, 1e-12)
	assertReal(t, "KDJ D", d, []float64{nan, nan, nan, (60 + 250.0/3) / 2}, 1e-12)
	assertReal(t, "KDJ J", j, []float64{nan, nan, nan, 500.0/3 - 60}, 1e-12)

	// Default RMA smoothing with period 2 seeds with the simple average of
	// the first two RSV values: K(bar3) = (60+250/3)/2; D period 1 is K.
	k, d, j, _ = KDJ(h, l, c, 3, 2, MATypeDefault, 1, MATypeDefault)
	wantK := (60 + 250.0/3) / 2
	assertReal(t, "KDJ RMA K", k, []float64{nan, nan, nan, wantK}, 1e-12)
	assertReal(t, "KDJ RMA D", d, []float64{nan, nan, nan, wantK}, 1e-12)
	assertReal(t, "KDJ RMA J", j, []float64{nan, nan, nan, wantK}, 1e-12)

	// K and D equal STOCH with the same (explicit) MA types, and J = 3K - 2D.
	for _, s := range fixtures(200) {
		k, d, j, _ := KDJ(s.H, s.L, s.C, 9, 3, MATypeDefault, 3, MATypeDefault)
		sk, sd, _ := STOCH(s.H, s.L, s.C, 9, 3, MATypeRMA, 3, MATypeRMA)
		assertReal(t, lbl("KDJ K vs STOCH ", s.Name), k, sk, 0)
		assertReal(t, lbl("KDJ D vs STOCH ", s.Name), d, sd, 0)
		for i := range j {
			if math.IsNaN(k[i]) {
				continue
			}
			want := float64(3*k[i]) - float64(2*d[i])
			if j[i] != want {
				t.Fatalf("%s J[%d] = %.17g, want %.17g", s.Name, i, j[i], want)
			}
		}
	}

	// Period-1 identity: with all periods 1, K = D = J = RSV of the bar.
	s := fixtures(80)[0]
	k, d, j, _ = KDJ(s.H, s.L, s.C, 1, 1, MATypeSMA, 1, MATypeSMA)
	for i := range s.C {
		want := (s.C[i] - s.L[i]) / (s.H[i] - s.L[i]) * 100.0
		if !closeEnough(k[i], want, 1e-12) || k[i] != d[i] || !closeEnough(j[i], k[i], 1e-12) {
			t.Fatalf("period 1 [%d]: k=%g d=%g j=%g want %g", i, k[i], d[i], j[i], want)
		}
	}

	// A flat window yields 0 for all three lines, not NaN.
	cst := fixtures(40)[5]
	lb := KDJLookback(9, 3, MATypeRMA, 3, MATypeRMA)
	k, d, j, _ = KDJ(cst.H, cst.L, cst.C, 9, 3, MATypeRMA, 3, MATypeRMA)
	for i := lb; i < len(k); i++ {
		if k[i] != 0 || d[i] != 0 || j[i] != 0 {
			t.Fatalf("constant [%d]: k=%g d=%g j=%g", i, k[i], d[i], j[i])
		}
	}

	// Bounds: K and D stay in [0, 100] with averaging MAs; J is unbounded but
	// within [-200, 300] since it is 3K - 2D.
	jOutside := false
	for _, mt := range []MAType{MATypeSMA, MATypeEMA, MATypeWMA, MATypeRMA} {
		for _, s := range fixtures(300) {
			k, d, j, _ := KDJ(s.H, s.L, s.C, 9, 3, mt, 3, mt)
			for i := range k {
				if math.IsNaN(k[i]) {
					continue
				}
				if k[i] < -1e-9 || k[i] > 100+1e-9 || d[i] < -1e-9 || d[i] > 100+1e-9 {
					t.Fatalf("%s mt=%d [%d]: k=%g d=%g out of [0,100]", s.Name, mt, i, k[i], d[i])
				}
				if j[i] < -200-1e-9 || j[i] > 300+1e-9 {
					t.Fatalf("%s mt=%d [%d]: j=%g out of [-200,300]", s.Name, mt, i, j[i])
				}
				if j[i] < 0 || j[i] > 100 {
					jOutside = true
				}
			}
		}
	}
	if !jOutside {
		t.Errorf("J never left [0,100]; it is meant to be unclamped")
	}

	// Rising series: the close is near the top of every window.
	up := fixtures(50)[7]
	k, _, _, _ = KDJ(up.H, up.L, up.C, 9, 3, MATypeRMA, 3, MATypeRMA)
	for i := lb; i < len(k); i++ {
		if k[i] < 80 {
			t.Fatalf("up trend [%d]: k=%g, want >= 80", i, k[i])
		}
	}
}

func TestKDJRangeForm(t *testing.T) {
	s := fixtures(40)[0]
	k := make([]float64, 40)
	d := make([]float64, 40)
	j := make([]float64, 40)
	call := func(start, end int, h []float64, k, d, j []float64) (int, int, error) {
		return kdjRange(start, end, h, s.L, s.C, 5, 3, MATypeSMA, 3, MATypeSMA, k, d, j)
	}
	if _, _, err := call(-1, 5, s.H, k, d, j); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := call(5, 4, s.H, k, d, j); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := call(0, -1, s.H, k, d, j); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (8): no output, no error.
	beg, nb, err := call(0, 7, s.H, k, d, j)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	beg, nb, err = call(7, 7, s.H, k, d, j)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("single bar inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Single bar right at the lookback.
	beg, nb, err = call(8, 8, s.H, k, d, j)
	if err != nil || beg != 8 || nb != 1 {
		t.Errorf("single bar at lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := call(0, 30, s.H[:20], k, d, j); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: %v", err)
	}
	if _, _, err := call(0, 30, s.H, k[:5], d, j); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outK: %v", err)
	}
	if _, _, err := call(0, 30, s.H, k, d[:5], j); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outD: %v", err)
	}
	if _, _, err := call(0, 30, s.H, k, d, j[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outJ: %v", err)
	}
	// Aliased outputs are rejected, as in C.
	if _, _, err := call(0, 30, s.H, k, k, j); !errors.Is(err, ErrBadParam) {
		t.Errorf("outK == outD: %v", err)
	}
	if _, _, err := call(0, 30, s.H, k, d, k); !errors.Is(err, ErrBadParam) {
		t.Errorf("outK == outJ: %v", err)
	}
	if _, _, err := call(0, 30, s.H, k, d, d); !errors.Is(err, ErrBadParam) {
		t.Errorf("outD == outJ: %v", err)
	}
	// A sub-range matches the full computation.
	fk, fd, fj, _ := KDJ(s.H, s.L, s.C, 5, 3, MATypeSMA, 3, MATypeSMA)
	beg, nb, err = call(3, 30, s.H, k, d, j)
	if err != nil || beg != 8 || nb != 23 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range K", k[:nb], fk[beg:beg+nb], 1e-12)
	assertReal(t, "sub-range D", d[:nb], fd[beg:beg+nb], 1e-12)
	assertReal(t, "sub-range J", j[:nb], fj[beg:beg+nb], 1e-12)
}
