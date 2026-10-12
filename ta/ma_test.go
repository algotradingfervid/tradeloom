package ta

import (
	"errors"
	"math"
	"testing"
)

// maAllTypes lists every MAType value the C enum accepts (0..13).
var maAllTypes = []MAType{
	MATypeSMA, MATypeEMA, MATypeWMA, MATypeDEMA, MATypeTEMA, MATypeTRIMA,
	MATypeKAMA, MATypeMAMA, MATypeT3, MATypeHMA, MATypeDisabled,
	MATypeDefault, MATypeZLEMA, MATypeRMA,
}

func TestMAParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32} {
		if _, err := MA(in, p, MATypeSMA); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := MALookback(p, MATypeSMA); lb != -1 {
			t.Errorf("MALookback(%d) = %d, want -1", p, lb)
		}
	}
	for _, mt := range []MAType{-1, 14, 100} {
		if _, err := MA(in, 5, mt); !errors.Is(err, ErrBadParam) {
			t.Errorf("matype %d: err = %v, want ErrBadParam", mt, err)
		}
		if lb := MALookback(5, mt); lb != -1 {
			t.Errorf("MALookback(5, %d) = %d, want -1", mt, lb)
		}
		if _, _, err := maRange(0, 49, in, 5, mt, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("maRange matype %d: err = %v, want ErrBadParam", mt, err)
		}
	}
	for _, mt := range maAllTypes {
		for _, p := range []int{1, 2, 100000} {
			if _, err := MA(in, p, mt); err != nil {
				t.Errorf("period %d matype %d: unexpected err %v", p, mt, err)
			}
			if lb := MALookback(p, mt); lb < 0 {
				t.Errorf("MALookback(%d, %d) = %d", p, mt, lb)
			}
		}
	}
	if out, err := MA(nil, 5, MATypeEMA); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := MA([]float64{}, 5, MATypeEMA); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := maRange(0, 10, in[:5], 3, MATypeSMA, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := maRange(0, 10, in, 3, MATypeSMA, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
}

func TestMALookbackValues(t *testing.T) {
	for _, p := range []int{1, 2, 5, 30} {
		want := map[MAType]int{
			MATypeSMA: smaLookback(p), MATypeEMA: emaLookback(p), MATypeWMA: wmaLookback(p),
			MATypeDEMA: demaLookback(p), MATypeTEMA: temaLookback(p), MATypeTRIMA: trimaLookback(p),
			MATypeKAMA: kamaLookback(p), MATypeMAMA: mamaLookback(0.5, 0.05), MATypeT3: t3Lookback(p, 0.7),
			MATypeHMA: hmaLookback(p), MATypeDisabled: 0, MATypeDefault: smaLookback(p),
			MATypeZLEMA: zlemaLookback(p), MATypeRMA: rmaLookback(p),
		}
		for _, mt := range maAllTypes {
			w := want[mt]
			if p == 1 {
				w = 0
			}
			if got := MALookback(p, mt); got != w {
				t.Errorf("MALookback(%d, %d) = %d, want %d", p, mt, got, w)
			}
		}
	}
}

func TestMALengths(t *testing.T) {
	for _, mt := range maAllTypes {
		for _, period := range []int{1, 2, 5, 30} {
			lb := MALookback(period, mt)
			for _, n := range edgeLengths(lb) {
				for _, s := range fixtures(n) {
					out, err := MA(s.Real(), period, mt)
					if err != nil {
						t.Fatalf("%s n=%d p=%d mt=%d: %v", s.Name, n, period, mt, err)
					}
					if len(out) != n {
						t.Fatalf("%s n=%d p=%d mt=%d: len(out)=%d", s.Name, n, period, mt, len(out))
					}
					assertLeadingNaN(t, lbl("MA ", s.Name, " n=", n, " p=", period, " mt=", mt), out, lb)
				}
			}
		}
	}
}

func TestMAKnownValues(t *testing.T) {
	in := []float64{1, 2, 3, 4, 5, 6}
	nan := math.NaN()
	out, err := MA(in, 3, MATypeSMA)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "MA SMA(1..6,3)", out, []float64{nan, nan, 2, 3, 4, 5}, 0)

	// MATypeDefault is SMA.
	out, _ = MA(in, 3, MATypeDefault)
	assertReal(t, "MA default(1..6,3)", out, []float64{nan, nan, 2, 3, 4, 5}, 0)

	// WMA(1..6, 3): (1*a + 2*b + 3*c) / 6.
	out, _ = MA(in, 3, MATypeWMA)
	assertReal(t, "MA WMA(1..6,3)", out, []float64{nan, nan, 14.0 / 6, 20.0 / 6, 26.0 / 6, 32.0 / 6}, 1e-12)

	// Period 1 is the identity for every type; MATypeDisabled is the identity
	// for every period.
	for _, mt := range maAllTypes {
		out, _ = MA(in, 1, mt)
		assertReal(t, lbl("MA period 1 mt=", mt), out, in, 0)
	}
	for _, p := range []int{2, 30, 100000} {
		out, _ = MA(in, p, MATypeDisabled)
		assertReal(t, lbl("MA disabled p=", p), out, in, 0)
	}

	// Each type forwards to its function (MAMA: 0.5/0.05 MAMA line; T3: 0.7).
	w := fixtures(300)[0].Real()
	for _, p := range []int{2, 5, 14} {
		check := func(mt MAType, want []float64, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
			got, err := MA(w, p, mt)
			if err != nil {
				t.Fatal(err)
			}
			assertReal(t, lbl("MA forward mt=", mt, " p=", p), got, want, 0)
		}
		r, err := SMA(w, p)
		check(MATypeSMA, r, err)
		r, err = EMA(w, p)
		check(MATypeEMA, r, err)
		r, err = WMA(w, p)
		check(MATypeWMA, r, err)
		r, err = DEMA(w, p)
		check(MATypeDEMA, r, err)
		r, err = TEMA(w, p)
		check(MATypeTEMA, r, err)
		r, err = TRIMA(w, p)
		check(MATypeTRIMA, r, err)
		r, err = KAMA(w, p)
		check(MATypeKAMA, r, err)
		r, _, err = MAMA(w, 0.5, 0.05)
		check(MATypeMAMA, r, err)
		r, err = T3(w, p, 0.7)
		check(MATypeT3, r, err)
		r, err = HMA(w, p)
		check(MATypeHMA, r, err)
		r, err = ZLEMA(w, p)
		check(MATypeZLEMA, r, err)
		r, err = RMA(w, p)
		check(MATypeRMA, r, err)
	}

	// A constant series averages to itself for the plain averages.
	c := fixtures(200)[5].Real()
	for _, mt := range []MAType{MATypeSMA, MATypeEMA, MATypeWMA, MATypeTRIMA, MATypeRMA} {
		out, _ = MA(c, 7, mt)
		lb := MALookback(7, mt)
		for i := lb; i < len(out); i++ {
			if math.Abs(out[i]-100) > 1e-9 {
				t.Fatalf("constant mt=%d: [%d]=%g", mt, i, out[i])
			}
		}
	}
}

func TestMAUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	SetUnstablePeriod(UnstEMA, 5)
	if got, want := MALookback(10, MATypeEMA), 14; got != want {
		t.Errorf("EMA lookback with unstable 5: %d, want %d", got, want)
	}
	in := fixtures(100)[0].Real()
	got, _ := MA(in, 10, MATypeEMA)
	want, _ := EMA(in, 10)
	assertReal(t, "MA EMA unstable", got, want, 0)
}

func TestMARangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := maRange(-1, 5, in, 3, MATypeSMA, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := maRange(5, 4, in, 3, MATypeSMA, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	for _, mt := range maAllTypes {
		if mt == MATypeDisabled {
			continue
		}
		beg, nb, err := maRange(0, 1, in, 3, mt, out)
		if err != nil || beg != 0 || nb != 0 {
			t.Errorf("inside lookback mt=%d: beg=%d nb=%d err=%v", mt, beg, nb, err)
		}
	}
	// Identity path honours startIdx.
	beg, nb, err := maRange(4, 9, in, 7, MATypeDisabled, out)
	if err != nil || beg != 4 || nb != 6 {
		t.Fatalf("disabled range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "disabled range", out[:nb], in[4:10], 0)
	// Start inside the lookback, end beyond it: clamped begin.
	beg, nb, err = maRange(0, 10, in, 4, MATypeSMA, out)
	if err != nil || beg != 3 || nb != 8 {
		t.Fatalf("clamped: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := MA(in, 4, MATypeSMA)
	beg, nb, _ = maRange(0, len(buf)-1, buf, 4, MATypeSMA, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
