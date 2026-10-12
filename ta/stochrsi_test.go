package ta

import (
	"errors"
	"math"
	"testing"
)

func TestSTOCHRSIParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, _, err := STOCHRSI(in, p, 5, 3, MATypeSMA); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := STOCHRSILookback(p, 5, 3, MATypeSMA); lb != -1 {
			t.Errorf("STOCHRSILookback(%d,5,3,SMA) = %d, want -1", p, lb)
		}
	}
	for _, p := range []int{0, -1, 100001, math.MinInt32} {
		if _, _, err := STOCHRSI(in, 14, p, 3, MATypeSMA); !errors.Is(err, ErrBadParam) {
			t.Errorf("fastK %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := STOCHRSILookback(14, p, 3, MATypeSMA); lb != -1 {
			t.Errorf("STOCHRSILookback(14,%d,3,SMA) = %d, want -1", p, lb)
		}
		if _, _, err := STOCHRSI(in, 14, 5, p, MATypeSMA); !errors.Is(err, ErrBadParam) {
			t.Errorf("fastD %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := STOCHRSILookback(14, 5, p, MATypeSMA); lb != -1 {
			t.Errorf("STOCHRSILookback(14,5,%d,SMA) = %d, want -1", p, lb)
		}
	}
	for _, mt := range []MAType{-1, 14, 100} {
		if _, _, err := STOCHRSI(in, 14, 5, 3, mt); !errors.Is(err, ErrBadParam) {
			t.Errorf("matype %d: err = %v, want ErrBadParam", mt, err)
		}
		if lb := STOCHRSILookback(14, 5, 3, mt); lb != -1 {
			t.Errorf("STOCHRSILookback(14,5,3,%d) = %d, want -1", mt, lb)
		}
		if _, _, err := stochrsiRange(0, 49, in, 14, 5, 3, mt, make([]float64, 50), make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("stochrsiRange matype %d: err = %v, want ErrBadParam", mt, err)
		}
	}
	for _, mt := range maAllTypes {
		for _, pt := range []int{2, 100000} {
			for _, pk := range []int{1, 100000} {
				for _, pd := range []int{1, 100000} {
					if _, _, err := STOCHRSI(in, pt, pk, pd, mt); err != nil {
						t.Errorf("t=%d k=%d d=%d mt=%d: unexpected err %v", pt, pk, pd, mt, err)
					}
					if lb := STOCHRSILookback(pt, pk, pd, mt); lb < 0 {
						t.Errorf("STOCHRSILookback(%d,%d,%d,%d) = %d", pt, pk, pd, mt, lb)
					}
				}
			}
		}
	}
	// Default MAType is SMA: 14 + (5-1) + (3-1) = 20.
	if a, b := STOCHRSILookback(14, 5, 3, MATypeDefault), STOCHRSILookback(14, 5, 3, MATypeSMA); a != b || a != 20 {
		t.Errorf("default matype lookback %d, SMA %d, want 20", a, b)
	}
	// nil / empty input.
	if k, d, err := STOCHRSI(nil, 14, 5, 3, MATypeSMA); err != nil || len(k) != 0 || len(d) != 0 {
		t.Errorf("nil input: k=%v d=%v err=%v", k, d, err)
	}
	if k, d, err := STOCHRSI([]float64{}, 14, 5, 3, MATypeEMA); err != nil || len(k) != 0 || len(d) != 0 {
		t.Errorf("empty input: k=%v d=%v err=%v", k, d, err)
	}
}

func TestSTOCHRSILookbackValues(t *testing.T) {
	for _, pt := range []int{2, 14} {
		for _, pk := range []int{1, 5} {
			for _, pd := range []int{1, 3, 10} {
				for _, mt := range maAllTypes {
					want := RSILookback(pt) + STOCHFLookback(pk, pd, mt)
					if got := STOCHRSILookback(pt, pk, pd, mt); got != want {
						t.Errorf("STOCHRSILookback(%d,%d,%d,%d) = %d, want %d", pt, pk, pd, mt, got, want)
					}
				}
			}
		}
	}
	// The RSI unstable period is part of the lookback.
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstRSI, 7); err != nil {
		t.Fatal(err)
	}
	if got := STOCHRSILookback(14, 5, 3, MATypeSMA); got != 27 {
		t.Errorf("lookback with RSI unstable 7 = %d, want 27", got)
	}
}

type stochrsiParams struct {
	t, k, d int
	mt      MAType
}

var stochrsiParamSets = []stochrsiParams{
	{2, 1, 1, MATypeSMA}, {14, 5, 3, MATypeSMA}, {14, 5, 3, MATypeDefault}, {14, 14, 3, MATypeEMA},
	{5, 3, 5, MATypeWMA}, {14, 5, 3, MATypeDEMA}, {14, 5, 3, MATypeTEMA}, {14, 5, 3, MATypeTRIMA},
	{14, 5, 3, MATypeKAMA}, {14, 5, 3, MATypeMAMA}, {14, 5, 3, MATypeT3}, {14, 5, 4, MATypeHMA},
	{14, 5, 3, MATypeDisabled}, {14, 5, 3, MATypeZLEMA}, {14, 5, 3, MATypeRMA}, {30, 30, 1, MATypeSMA},
}

func TestSTOCHRSILengths(t *testing.T) {
	for _, p := range stochrsiParamSets {
		lb := STOCHRSILookback(p.t, p.k, p.d, p.mt)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				k, d, err := STOCHRSI(s.Real(), p.t, p.k, p.d, p.mt)
				if err != nil {
					t.Fatalf("%s n=%d %v: %v", s.Name, n, p, err)
				}
				if len(k) != n || len(d) != n {
					t.Fatalf("%s n=%d %v: len k=%d d=%d", s.Name, n, p, len(k), len(d))
				}
				assertLeadingNaN(t, lbl("STOCHRSI K ", s.Name, " n=", n, " ", p), k, lb)
				assertLeadingNaN(t, lbl("STOCHRSI D ", s.Name, " n=", n, " ", p), d, lb)
			}
		}
	}
}

func TestSTOCHRSIKnownValues(t *testing.T) {
	nan := math.NaN()
	// RSI(2) of 0,1,0,1,0,1,1 (Wilder): diffs +1,-1,+1,-1,+1,0.
	//   [2] avgGain .5 avgLoss .5 -> 50
	//   [3] gain .75 loss .25 -> 75
	//   [4] gain .375 loss .625 -> 37.5
	//   [5] gain .6875 loss .3125 -> 68.75
	//   [6] gain .34375 loss .15625 -> 68.75
	// K(3): [4] {50,75,37.5} -> 0; [5] {75,37.5,68.75} -> 31.25/37.5*100;
	//       [6] {37.5,68.75,68.75} -> 100.
	// D(2, SMA) of K. Lookback = 2 + 2 + 1 = 5.
	in := []float64{0, 1, 0, 1, 0, 1, 1}
	k5 := (68.75 - 37.5) / (75 - 37.5) * 100
	k, d, err := STOCHRSI(in, 2, 3, 2, MATypeSMA)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "K", k, []float64{nan, nan, nan, nan, nan, k5, 100}, 1e-12)
	assertReal(t, "D", d, []float64{nan, nan, nan, nan, nan, (0 + k5) / 2, (k5 + 100) / 2}, 1e-12)

	// STOCHRSI is STOCHF applied to the RSI series (high = low = close = RSI)
	// from the RSI's first value on. RSI may fuse multiply-adds where
	// STOCHRSI's private RSI copy does not, so compare at 1e-9; MAMA is left
	// out because its phase logic amplifies those last-ulp differences.
	s := fixtures(300)[0]
	for _, p := range stochrsiParamSets {
		if p.mt == MATypeMAMA {
			continue
		}
		k, d, _ := STOCHRSI(s.Real(), p.t, p.k, p.d, p.mt)
		rsi, _ := RSI(s.Real(), p.t)
		lbR := RSILookback(p.t)
		r := rsi[lbR:]
		wk, wd, _ := STOCHF(r, r, r, p.k, p.d, p.mt)
		assertReal(t, lbl("K = STOCHF(RSI) ", p), k[lbR:], wk, 1e-9)
		assertReal(t, lbl("D = STOCHF(RSI) ", p), d[lbR:], wd, 1e-9)
	}

	// K period 1: every window is flat, so K = 0 (and an SMA of it, D = 0).
	k, d, _ = STOCHRSI(s.Real(), 14, 1, 3, MATypeSMA)
	for i := 16; i < len(k); i++ {
		if k[i] != 0 || d[i] != 0 {
			t.Fatalf("fastK 1 [%d]: k=%g d=%g", i, k[i], d[i])
		}
	}

	// Constant and monotonic inputs give a constant RSI: K = 0 everywhere.
	for _, i := range []int{5, 6, 7, 8} {
		c := fixtures(60)[i]
		k, d, _ = STOCHRSI(c.Real(), 14, 5, 3, MATypeSMA)
		for j := 20; j < len(k); j++ {
			if k[j] != 0 || d[j] != 0 {
				t.Fatalf("%s [%d]: k=%g d=%g", c.Name, j, k[j], d[j])
			}
		}
	}

	// Bounds: K and an SMA of K lie in [0, 100].
	for _, s := range fixtures(300) {
		k, d, _ := STOCHRSI(s.Real(), 14, 5, 3, MATypeSMA)
		for i := 20; i < len(k); i++ {
			if k[i] < 0 || k[i] > 100+1e-9 || d[i] < -1e-9 || d[i] > 100+1e-9 {
				t.Fatalf("%s [%d]: k=%g d=%g out of [0,100]", s.Name, i, k[i], d[i])
			}
		}
	}
}

func TestSTOCHRSIRangeForm(t *testing.T) {
	in := fixtures(60)[0].Real()
	k := make([]float64, len(in))
	d := make([]float64, len(in))
	if _, _, err := stochrsiRange(-1, 5, in, 14, 5, 3, MATypeSMA, k, d); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := stochrsiRange(5, 4, in, 14, 5, 3, MATypeSMA, k, d); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := stochrsiRange(0, -1, in, 14, 5, 3, MATypeSMA, k, d); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (20): no output, no error.
	beg, nb, err := stochrsiRange(0, 19, in, 14, 5, 3, MATypeSMA, k, d)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := stochrsiRange(0, 40, in[:30], 14, 5, 3, MATypeSMA, k, d); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := stochrsiRange(0, 40, in, 14, 5, 3, MATypeSMA, k[:5], d); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outFastK: %v", err)
	}
	if _, _, err := stochrsiRange(0, 40, in, 14, 5, 3, MATypeSMA, k, d[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outFastD: %v", err)
	}
	// outFastK == outFastD is rejected, like C.
	if _, _, err := stochrsiRange(0, 40, in, 14, 5, 3, MATypeSMA, k, k); !errors.Is(err, ErrBadParam) {
		t.Errorf("same output buffer: %v", err)
	}
	// Exact-size output buffers for a sub-range work. The RSI restarts its
	// Wilder smoothing at the range start (like C), so values differ from the
	// full-series run; compare with a run on the same range into big buffers.
	k2, d2 := make([]float64, 11), make([]float64, 11)
	beg, nb, err = stochrsiRange(30, 40, in, 14, 5, 3, MATypeSMA, k2, d2)
	if err != nil || beg != 30 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	kb, db := make([]float64, 60), make([]float64, 60)
	beg, nb, err = stochrsiRange(30, 40, in, 14, 5, 3, MATypeSMA, kb, db)
	if err != nil || beg != 30 || nb != 11 {
		t.Fatalf("sub-range big buffers: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range K", k2, kb[:nb], 0)
	assertReal(t, "sub-range D", d2, db[:nb], 0)
	// A range ending at the last bar and starting at the lookback equals the
	// full-series result (same RSI seed).
	k3, d3 := make([]float64, 50), make([]float64, 50)
	beg, nb, err = stochrsiRange(10, 59, in, 14, 5, 3, MATypeSMA, k3, d3)
	if err != nil || beg != 20 || nb != 40 {
		t.Fatalf("sub-range in lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	wantK, wantD, _ := STOCHRSI(in, 14, 5, 3, MATypeSMA)
	assertReal(t, "sub-range-lb K", k3[:nb], wantK[20:], 0)
	assertReal(t, "sub-range-lb D", d3[:nb], wantD[20:], 0)
	// outFastK may alias inReal.
	buf := append([]float64(nil), in...)
	dd := make([]float64, len(in))
	beg, nb, err = stochrsiRange(0, len(buf)-1, buf, 14, 5, 3, MATypeEMA, buf, dd)
	if err != nil {
		t.Fatal(err)
	}
	wk, wd, _ := STOCHRSI(in, 14, 5, 3, MATypeEMA)
	alignReal(buf, beg, nb)
	alignReal(dd, beg, nb)
	assertReal(t, "in-place K", buf, wk, 0)
	assertReal(t, "in-place D", dd, wd, 0)
}
