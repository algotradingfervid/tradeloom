package ta

import (
	"errors"
	"math"
	"testing"
)

// macdextAllTypes is every MAType value TA-Lib accepts.
var macdextAllTypes = []MAType{
	MATypeSMA, MATypeEMA, MATypeWMA, MATypeDEMA, MATypeTEMA, MATypeTRIMA, MATypeKAMA,
	MATypeMAMA, MATypeT3, MATypeHMA, MATypeDisabled, MATypeDefault, MATypeZLEMA, MATypeRMA,
}

type macdextParams struct {
	fast   int
	fastT  MAType
	slow   int
	slowT  MAType
	signal int
	sigT   MAType
}

func (p macdextParams) run(in []float64) ([]float64, []float64, []float64, error) {
	return MACDEXT(in, p.fast, p.fastT, p.slow, p.slowT, p.signal, p.sigT)
}

func (p macdextParams) lookback() int {
	return MACDEXTLookback(p.fast, p.fastT, p.slow, p.slowT, p.signal, p.sigT)
}

func TestMACDEXTParams(t *testing.T) {
	in := fixtures(80)[0].Real()
	def := macdextParams{12, MATypeSMA, 26, MATypeSMA, 9, MATypeSMA}
	bad := []macdextParams{}
	for _, v := range []int{1, 0, -1, 100001, math.MinInt32} {
		p := def
		p.fast = v
		bad = append(bad, p)
		p = def
		p.slow = v
		bad = append(bad, p)
	}
	for _, v := range []int{0, -1, 100001} {
		p := def
		p.signal = v
		bad = append(bad, p)
	}
	for _, mt := range []MAType{-1, 14, 100} {
		p := def
		p.fastT = mt
		bad = append(bad, p)
		p = def
		p.slowT = mt
		bad = append(bad, p)
		p = def
		p.sigT = mt
		bad = append(bad, p)
	}
	for _, p := range bad {
		if _, _, _, err := p.run(in); !errors.Is(err, ErrBadParam) {
			t.Errorf("%+v: err = %v, want ErrBadParam", p, err)
		}
		if lb := p.lookback(); lb != -1 {
			t.Errorf("lookback %+v = %d, want -1", p, lb)
		}
		out := make([]float64, len(in))
		if _, _, err := macdextRange(0, len(in)-1, in, p.fast, p.fastT, p.slow, p.slowT, p.signal, p.sigT, out, make([]float64, len(in)), make([]float64, len(in))); !errors.Is(err, ErrBadParam) {
			t.Errorf("range %+v: err = %v, want ErrBadParam", p, err)
		}
	}
	good := []macdextParams{
		{2, MATypeSMA, 2, MATypeSMA, 1, MATypeSMA},
		{100000, MATypeSMA, 100000, MATypeSMA, 100000, MATypeSMA},
		{2, MATypeDefault, 3, MATypeDefault, 1, MATypeDefault},
		{2, MATypeRMA, 100000, MATypeEMA, 100000, MATypeZLEMA},
	}
	for _, p := range good {
		if _, _, _, err := p.run(in); err != nil {
			t.Errorf("%+v: unexpected err %v", p, err)
		}
		if lb := p.lookback(); lb < 0 {
			t.Errorf("lookback %+v = %d", p, lb)
		}
	}
	// MATypeDefault is SMA in every slot.
	if a, b := MACDEXTLookback(5, MATypeDefault, 9, MATypeDefault, 4, MATypeDefault), MACDEXTLookback(5, MATypeSMA, 9, MATypeSMA, 4, MATypeSMA); a != b || a != 11 {
		t.Errorf("default lookback %d, SMA lookback %d, want 11", a, b)
	}
	for _, s := range [][]float64{nil, {}} {
		m, sg, h, err := def.run(s)
		if err != nil || len(m) != 0 || len(sg) != 0 || len(h) != 0 {
			t.Errorf("empty input: %v %v %v %v", m, sg, h, err)
		}
	}
	o := make([]float64, 100)
	if _, _, err := macdextRange(0, 79, in[:50], 12, 0, 26, 0, 9, 0, o, make([]float64, 100), make([]float64, 100)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v", err)
	}
	if _, _, err := macdextRange(0, 79, in, 12, 0, 26, 0, 9, 0, o[:40], make([]float64, 100), make([]float64, 100)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v", err)
	}
	if _, _, err := macdextRange(0, 79, in, 12, 0, 26, 0, 9, 0, o, o, make([]float64, 100)); !errors.Is(err, ErrBadParam) {
		t.Errorf("aliased outputs: err = %v", err)
	}
}

func TestMACDEXTLengths(t *testing.T) {
	sets := []macdextParams{
		{12, MATypeSMA, 26, MATypeSMA, 9, MATypeSMA},
		{2, MATypeSMA, 2, MATypeSMA, 1, MATypeSMA},
		{26, MATypeWMA, 12, MATypeTEMA, 9, MATypeKAMA},
		{5, MATypeMAMA, 10, MATypeT3, 3, MATypeHMA},
		{12, MATypeEMA, 26, MATypeEMA, 9, MATypeEMA},
		{7, MATypeZLEMA, 14, MATypeRMA, 5, MATypeTRIMA},
		{4, MATypeDisabled, 8, MATypeDEMA, 2, MATypeDefault},
	}
	for _, p := range sets {
		lb := p.lookback()
		if lb < 0 {
			t.Fatalf("%+v: lookback %d", p, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				m, sg, h, err := p.run(s.Real())
				if err != nil {
					t.Fatalf("%s n=%d %+v: %v", s.Name, n, p, err)
				}
				if len(m) != n || len(sg) != n || len(h) != n {
					t.Fatalf("%s n=%d %+v: lens %d %d %d", s.Name, n, p, len(m), len(sg), len(h))
				}
				l := lbl("MACDEXT ", s.Name, " n=", n, " ", p)
				assertLeadingNaN(t, l+" macd", m, lb)
				assertLeadingNaN(t, l+" signal", sg, lb)
				assertLeadingNaN(t, l+" hist", h, lb)
			}
		}
	}
}

func TestMACDEXTKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{1, 2, 4, 7, 11, 16}
	// SMA(2) - SMA(3): bars 2..5 -> (3-7/3), (5.5-13/3), (9-22/3), (13.5-34/3).
	// Signal SMA(2) of MACD, starting at bar 3.
	m, sg, h, err := MACDEXT(in, 2, MATypeSMA, 3, MATypeSMA, 2, MATypeSMA)
	if err != nil {
		t.Fatal(err)
	}
	macd := []float64{3 - 7.0/3, 5.5 - 13.0/3, 9 - 22.0/3, 13.5 - 34.0/3}
	wantM := []float64{nan, nan, nan, macd[1], macd[2], macd[3]}
	wantS := []float64{nan, nan, nan, (macd[0] + macd[1]) / 2, (macd[1] + macd[2]) / 2, (macd[2] + macd[3]) / 2}
	wantH := []float64{nan, nan, nan, wantM[3] - wantS[3], wantM[4] - wantS[4], wantM[5] - wantS[5]}
	assertReal(t, "MACDEXT macd", m, wantM, 1e-12)
	assertReal(t, "MACDEXT signal", sg, wantS, 1e-12)
	assertReal(t, "MACDEXT hist", h, wantH, 1e-12)

	// Swapping fast and slow (periods and types together) changes nothing.
	m2, sg2, h2, _ := MACDEXT(in, 3, MATypeSMA, 2, MATypeSMA, 2, MATypeSMA)
	assertReal(t, "swap macd", m2, m, 0)
	assertReal(t, "swap signal", sg2, sg, 0)
	assertReal(t, "swap hist", h2, h, 0)

	// Signal period 1: signal == MACD and hist == 0 for every signal type.
	w := fixtures(120)[0].Real()
	for _, st := range macdextAllTypes {
		m, sg, h, err := MACDEXT(w, 5, MATypeWMA, 13, MATypeSMA, 1, st)
		if err != nil {
			t.Fatal(err)
		}
		assertReal(t, lbl("signal1 type ", st), sg, m, 0)
		for i := 12; i < len(h); i++ {
			if h[i] != 0 {
				t.Fatalf("signal1 type %d: hist[%d]=%g", st, i, h[i])
			}
		}
	}

	// Constant input: every MA equals the constant, so MACD/signal/hist are 0.
	c := fixtures(200)[5].Real()
	for _, mt := range []MAType{MATypeSMA, MATypeWMA, MATypeTRIMA, MATypeDisabled} {
		m, sg, h, _ := MACDEXT(c, 4, mt, 9, MATypeSMA, 3, mt)
		lb := MACDEXTLookback(4, mt, 9, MATypeSMA, 3, mt)
		for i := lb; i < len(c); i++ {
			if m[i] != 0 || sg[i] != 0 || h[i] != 0 {
				t.Fatalf("constant type %d: [%d] %g %g %g", mt, i, m[i], sg[i], h[i])
			}
		}
	}

	// All-EMA delegates to MACD.
	wm, ws, wh, _ := MACD(w, 12, 26, 9)
	m, sg, h, _ = MACDEXT(w, 12, MATypeEMA, 26, MATypeEMA, 9, MATypeEMA)
	assertReal(t, "all-EMA macd", m, wm, 0)
	assertReal(t, "all-EMA signal", sg, ws, 0)
	assertReal(t, "all-EMA hist", h, wh, 0)

	// Both sides MAMA: fast and slow are identical, MACD is 0.
	m, _, h, _ = MACDEXT(w, 5, MATypeMAMA, 20, MATypeMAMA, 3, MATypeSMA)
	for i := MACDEXTLookback(5, MATypeMAMA, 20, MATypeMAMA, 3, MATypeSMA); i < len(w); i++ {
		if m[i] != 0 || h[i] != 0 {
			t.Fatalf("MAMA/MAMA: [%d] %g %g", i, m[i], h[i])
		}
	}

	// Hist is always MACD - signal.
	m, sg, h, _ = MACDEXT(w, 6, MATypeT3, 15, MATypeKAMA, 4, MATypeHMA)
	for i := range h {
		if !math.IsNaN(h[i]) && h[i] != m[i]-sg[i] {
			t.Fatalf("hist identity at %d", i)
		}
	}
}

func TestMACDEXTRangeForm(t *testing.T) {
	in := fixtures(60)[0].Real()
	n := len(in)
	o1, o2, o3 := make([]float64, n), make([]float64, n), make([]float64, n)
	if _, _, err := macdextRange(-1, 5, in, 3, 0, 5, 0, 2, 0, o1, o2, o3); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := macdextRange(5, 4, in, 3, 0, 5, 0, 2, 0, o1, o2, o3); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := macdextRange(0, -1, in, 3, 0, 5, 0, 2, 0, o1, o2, o3); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Lookback is 4+1 = 5: [0, 4] is entirely inside it.
	beg, nb, err := macdextRange(0, 4, in, 3, 0, 5, 0, 2, 0, o1, o2, o3)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Exactly-sized output buffers work for a sub-range.
	wm, ws, wh, _ := MACDEXT(in, 3, MATypeWMA, 5, MATypeSMA, 4, MATypeTRIMA)
	e1, e2, e3 := make([]float64, 11), make([]float64, 11), make([]float64, 11)
	beg, nb, err = macdextRange(30, 40, in, 3, MATypeWMA, 5, MATypeSMA, 4, MATypeTRIMA, e1, e2, e3)
	if err != nil || beg != 30 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// SMA's running sum starts at a different bar for a sub-range, so the
	// values agree only to rounding (C behaves the same way).
	assertReal(t, "sub macd", e1, wm[30:41], 1e-9)
	assertReal(t, "sub signal", e2, ws[30:41], 1e-9)
	assertReal(t, "sub hist", e3, wh[30:41], 1e-9)
	// outMACD may be the input buffer.
	buf := append([]float64(nil), in...)
	beg, nb, err = macdextRange(0, n-1, buf, 3, MATypeWMA, 5, MATypeSMA, 4, MATypeTRIMA, buf, o2, o3)
	if err != nil {
		t.Fatal(err)
	}
	alignReal(buf, beg, nb)
	assertReal(t, "in-place macd", buf, wm, 0)
}
