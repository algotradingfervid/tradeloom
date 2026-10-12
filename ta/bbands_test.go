package ta

import (
	"errors"
	"math"
	"testing"
)

// bbandsAllTypes lists every MAType value the C enum accepts (0..13).
var bbandsAllTypes = []MAType{
	MATypeSMA, MATypeEMA, MATypeWMA, MATypeDEMA, MATypeTEMA, MATypeTRIMA,
	MATypeKAMA, MATypeMAMA, MATypeT3, MATypeHMA, MATypeDisabled,
	MATypeDefault, MATypeZLEMA, MATypeRMA,
}

func TestBBANDSParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, _, _, err := BBANDS(in, p, 2, 2, MATypeSMA); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := BBANDSLookback(p, 2, 2, MATypeSMA); lb != -1 {
			t.Errorf("BBANDSLookback(%d) = %d, want -1", p, lb)
		}
	}
	badReals := []float64{math.NaN(), -3.0000001e37, 3.0000001e37, math.Inf(1), math.Inf(-1)}
	for _, v := range badReals {
		if _, _, _, err := BBANDS(in, 5, v, 2, MATypeSMA); !errors.Is(err, ErrBadParam) {
			t.Errorf("nbDevUp %g: err = %v, want ErrBadParam", v, err)
		}
		if _, _, _, err := BBANDS(in, 5, 2, v, MATypeSMA); !errors.Is(err, ErrBadParam) {
			t.Errorf("nbDevDn %g: err = %v, want ErrBadParam", v, err)
		}
		if lb := BBANDSLookback(5, v, 2, MATypeSMA); lb != -1 {
			t.Errorf("BBANDSLookback up %g = %d, want -1", v, lb)
		}
		if lb := BBANDSLookback(5, 2, v, MATypeSMA); lb != -1 {
			t.Errorf("BBANDSLookback dn %g = %d, want -1", v, lb)
		}
	}
	for _, mt := range []MAType{-1, 14, 100} {
		if _, _, _, err := BBANDS(in, 5, 2, 2, mt); !errors.Is(err, ErrBadParam) {
			t.Errorf("matype %d: err = %v, want ErrBadParam", mt, err)
		}
		if lb := BBANDSLookback(5, 2, 2, mt); lb != -1 {
			t.Errorf("BBANDSLookback matype %d = %d, want -1", mt, lb)
		}
	}
	for _, mt := range bbandsAllTypes {
		for _, p := range []int{2, 3, 100000} {
			for _, d := range [][2]float64{{taRealMin, taRealMax}, {taRealMax, taRealMin}, {2, 2}, {0, -1}} {
				if _, _, _, err := BBANDS(in, p, d[0], d[1], mt); err != nil {
					t.Errorf("p=%d dev=%v mt=%d: unexpected err %v", p, d, mt, err)
				}
				if lb := BBANDSLookback(p, d[0], d[1], mt); lb < p-1 {
					t.Errorf("BBANDSLookback(%d, %v, %d) = %d", p, d, mt, lb)
				}
			}
		}
	}
	if _, _, _, err := BBANDS(in, 5, 2, 2, MATypeSMA); err != nil {
		t.Errorf("default-ish params: %v", err)
	}
	for _, in := range [][]float64{nil, {}} {
		u, m, l, err := BBANDS(in, 5, 2, 2, MATypeEMA)
		if err != nil || len(u) != 0 || len(m) != 0 || len(l) != 0 {
			t.Errorf("empty input: u=%v m=%v l=%v err=%v, want empty and nil", u, m, l, err)
		}
	}
}

func TestBBANDSLengths(t *testing.T) {
	type ps struct {
		p      int
		up, dn float64
		mt     MAType
	}
	var sets []ps
	for _, mt := range bbandsAllTypes {
		sets = append(sets, ps{2, 2, 2, mt}, ps{5, 2, 1.5, mt}, ps{20, 2, 2, mt}, ps{40, 1, 3, mt})
	}
	for _, s0 := range sets {
		lb := BBANDSLookback(s0.p, s0.up, s0.dn, s0.mt)
		mt := s0.mt
		if mt == MATypeDefault {
			mt = MATypeSMA
		}
		if want := cMaxInt(MALookback(s0.p, mt), STDDEVLookback(s0.p, 1)); lb != want {
			t.Fatalf("BBANDSLookback(%v) = %d, want %d", s0, lb, want)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				u, m, l, err := BBANDS(s.Real(), s0.p, s0.up, s0.dn, s0.mt)
				if err != nil {
					t.Fatalf("%s n=%d %v: %v", s.Name, n, s0, err)
				}
				if len(u) != n || len(m) != n || len(l) != n {
					t.Fatalf("%s n=%d %v: lens %d %d %d", s.Name, n, s0, len(u), len(m), len(l))
				}
				tag := lbl(" ", s.Name, " n=", n, " ", s0)
				assertLeadingNaN(t, "BBANDS upper"+tag, u, lb)
				assertLeadingNaN(t, "BBANDS middle"+tag, m, lb)
				assertLeadingNaN(t, "BBANDS lower"+tag, l, lb)
			}
		}
	}
}

func TestBBANDSKnownValues(t *testing.T) {
	// Window {1,2,3}: mean 2, population variance 2/3.
	// Window {2,3,4}: mean 3, same variance.
	in := []float64{1, 2, 3, 4}
	sd := math.Sqrt(2.0 / 3.0)
	nan := math.NaN()
	for _, mt := range []MAType{MATypeSMA, MATypeDefault, MATypeWMA} {
		u, m, l, err := BBANDS(in, 3, 2, 1, mt)
		if err != nil {
			t.Fatal(err)
		}
		mid := []float64{nan, nan, 2, 3}
		if mt == MATypeWMA {
			// WMA(1,2,3) = (1+4+9)/6, WMA(2,3,4) = (2+6+12)/6.
			mid = []float64{nan, nan, 14.0 / 6, 20.0 / 6}
		}
		assertReal(t, lbl("middle mt=", mt), m, mid, 1e-14)
		assertReal(t, lbl("upper mt=", mt), u, []float64{nan, nan, mid[2] + 2*sd, mid[3] + 2*sd}, 1e-14)
		assertReal(t, lbl("lower mt=", mt), l, []float64{nan, nan, mid[2] - sd, mid[3] - sd}, 1e-14)
	}

	// Constant input: zero-width bands, for every MA type, at the constant
	// (except MAMA, whose adaptive smoothing only converges towards it).
	c := fixtures(120)[5].Real()
	for _, mt := range bbandsAllTypes {
		u, m, l, _ := BBANDS(c, 7, 2, 2, mt)
		lb := BBANDSLookback(7, 2, 2, mt)
		for i := lb; i < len(c); i++ {
			if (mt != MATypeMAMA && math.Abs(m[i]-100) > 1e-9) || u[i] != m[i] || l[i] != m[i] {
				t.Fatalf("constant mt=%d [%d]: %g %g %g", mt, i, u[i], m[i], l[i])
			}
		}
	}

	// Bounds: with non-negative multipliers lower <= middle <= upper, and
	// the SMA middle band equals SMA.
	s := fixtures(300)[0].Real()
	u, m, l, _ := BBANDS(s, 20, 2, 2, MATypeSMA)
	sma, _ := SMA(s, 20)
	assertReal(t, "middle == SMA", m, sma, 0)
	for i := 19; i < len(s); i++ {
		if !(l[i] <= m[i] && m[i] <= u[i]) {
			t.Fatalf("bounds [%d]: %g %g %g", i, l[i], m[i], u[i])
		}
	}
	// Zero multipliers collapse the bands onto the middle.
	u, m, l, _ = BBANDS(s, 20, 0, 0, MATypeEMA)
	for i := 19; i < len(s); i++ {
		if u[i] != m[i] || l[i] != m[i] {
			t.Fatalf("zero dev [%d]: %g %g %g", i, u[i], m[i], l[i])
		}
	}
}

func TestBBANDSRangeForm(t *testing.T) {
	in := fixtures(60)[0].Real()
	n := len(in)
	u, m, l := make([]float64, n), make([]float64, n), make([]float64, n)
	if _, _, err := bbandsRange(-1, 5, in, 3, 2, 2, MATypeSMA, u, m, l); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := bbandsRange(5, 4, in, 3, 2, 2, MATypeSMA, u, m, l); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	for _, mt := range bbandsAllTypes {
		lb := BBANDSLookback(5, 2, 2, mt)
		beg, nb, err := bbandsRange(0, lb-1, in, 5, 2, 2, mt, u, m, l)
		if err != nil || beg != 0 || nb != 0 {
			t.Errorf("inside lookback mt=%d: beg=%d nb=%d err=%v", mt, beg, nb, err)
		}
	}
	// Short buffers.
	if _, _, err := bbandsRange(0, 10, in[:5], 3, 2, 2, MATypeSMA, u, m, l); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := bbandsRange(0, 10, in, 3, 2, 2, MATypeSMA, u, m[:5], l); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// Two outputs sharing a buffer is rejected, as in C.
	if _, _, err := bbandsRange(0, 10, in, 3, 2, 2, MATypeSMA, u, u, l); !errors.Is(err, ErrBadParam) {
		t.Errorf("aliased outputs: %v", err)
	}
	// Input sharing a buffer with one output is allowed, on every path.
	for _, mt := range []MAType{MATypeSMA, MATypeEMA} {
		wu, wm, wl, _ := BBANDS(in, 4, 2, 1.5, mt)
		for which := 0; which < 3; which++ {
			buf := append([]float64(nil), in...)
			outs := [3][]float64{make([]float64, n), make([]float64, n), make([]float64, n)}
			outs[which] = buf
			beg, nb, err := bbandsRange(0, n-1, buf, 4, 2, 1.5, mt, outs[0], outs[1], outs[2])
			if err != nil {
				t.Fatalf("in-place mt=%d which=%d: %v", mt, which, err)
			}
			for k := range outs {
				alignReal(outs[k], beg, nb)
			}
			tag := lbl(" mt=", mt, " which=", which)
			assertReal(t, "in-place upper"+tag, outs[0], wu, 0)
			assertReal(t, "in-place middle"+tag, outs[1], wm, 0)
			assertReal(t, "in-place lower"+tag, outs[2], wl, 0)
		}
	}
}
