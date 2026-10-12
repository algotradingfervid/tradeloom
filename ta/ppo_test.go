package ta

import (
	"errors"
	"math"
	"testing"
)

func TestPPOParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := PPO(in, p, 26, MATypeEMA); !errors.Is(err, ErrBadParam) {
			t.Errorf("fast %d: err = %v, want ErrBadParam", p, err)
		}
		if _, err := PPO(in, 12, p, MATypeEMA); !errors.Is(err, ErrBadParam) {
			t.Errorf("slow %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := PPOLookback(p, 26, MATypeEMA); lb != -1 {
			t.Errorf("PPOLookback(fast=%d) = %d, want -1", p, lb)
		}
		if lb := PPOLookback(12, p, MATypeEMA); lb != -1 {
			t.Errorf("PPOLookback(slow=%d) = %d, want -1", p, lb)
		}
		if _, _, err := ppoRange(0, 49, in, p, 26, MATypeEMA, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("ppoRange fast %d: err = %v, want ErrBadParam", p, err)
		}
		if _, _, err := ppoRange(0, 49, in, 12, p, MATypeEMA, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("ppoRange slow %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, mt := range []MAType{-1, 14, 100} {
		if _, err := PPO(in, 12, 26, mt); !errors.Is(err, ErrBadParam) {
			t.Errorf("matype %d: err = %v, want ErrBadParam", mt, err)
		}
		if lb := PPOLookback(12, 26, mt); lb != -1 {
			t.Errorf("PPOLookback(matype %d) = %d, want -1", mt, lb)
		}
		if _, _, err := ppoRange(0, 49, in, 12, 26, mt, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("ppoRange matype %d: err = %v, want ErrBadParam", mt, err)
		}
	}
	for _, mt := range maAllTypes {
		for _, pp := range [][2]int{{2, 2}, {2, 100000}, {100000, 2}, {100000, 100000}, {12, 26}} {
			if _, err := PPO(in, pp[0], pp[1], mt); err != nil {
				t.Errorf("periods %v matype %d: unexpected err %v", pp, mt, err)
			}
			if lb := PPOLookback(pp[0], pp[1], mt); lb < 0 {
				t.Errorf("PPOLookback(%v, %d) = %d", pp, mt, lb)
			}
		}
	}
	// MATypeDefault is EMA.
	if a, b := PPOLookback(12, 26, MATypeDefault), PPOLookback(12, 26, MATypeEMA); a != b {
		t.Errorf("default matype lookback %d, EMA %d", a, b)
	}
	d, _ := PPO(in, 3, 7, MATypeDefault)
	e, _ := PPO(in, 3, 7, MATypeEMA)
	assertReal(t, "default matype", d, e, 0)

	if out, err := PPO(nil, 12, 26, MATypeEMA); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := PPO([]float64{}, 12, 26, MATypeEMA); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := ppoRange(0, 10, in[:5], 2, 3, MATypeSMA, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := ppoRange(0, 10, in, 2, 3, MATypeSMA, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
}

func TestPPOLengths(t *testing.T) {
	sets := []struct {
		fast, slow int
		mt         MAType
	}{
		{2, 3, MATypeSMA}, {12, 26, MATypeEMA}, {26, 12, MATypeEMA}, {5, 10, MATypeWMA},
		{3, 8, MATypeT3}, {4, 9, MATypeMAMA}, {2, 2, MATypeDisabled}, {6, 14, MATypeKAMA},
		{3, 7, MATypeDEMA}, {3, 7, MATypeTEMA}, {4, 11, MATypeTRIMA}, {4, 9, MATypeHMA},
		{4, 9, MATypeZLEMA}, {4, 9, MATypeRMA}, {12, 26, MATypeDefault},
	}
	for _, ps := range sets {
		lb := PPOLookback(ps.fast, ps.slow, ps.mt)
		mt := ps.mt
		if mt == MATypeDefault {
			mt = MATypeEMA
		}
		if want := MALookback(max(ps.fast, ps.slow), mt); lb != want {
			t.Fatalf("PPOLookback(%d,%d,%d) = %d, want %d", ps.fast, ps.slow, ps.mt, lb, want)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := PPO(s.Real(), ps.fast, ps.slow, ps.mt)
				if err != nil {
					t.Fatalf("%s n=%d %v: %v", s.Name, n, ps, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d %v: len(out)=%d", s.Name, n, ps, len(out))
				}
				assertLeadingNaN(t, lbl("PPO ", s.Name, " n=", n, " ", ps), out, lb)
			}
		}
	}
}

func TestPPOKnownValues(t *testing.T) {
	in := []float64{1, 2, 3, 4, 5, 6}
	nan := math.NaN()
	// fast SMA(2) = 2.5, 3.5, 4.5, 5.5; slow SMA(3) = 2, 3, 4, 5.
	want := []float64{nan, nan, 25, 0.5 / 3 * 100, 12.5, 10}
	out, err := PPO(in, 2, 3, MATypeSMA)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "PPO(1..6,2,3,SMA)", out, want, 1e-12)

	// Fast > slow is swapped.
	out, _ = PPO(in, 3, 2, MATypeSMA)
	assertReal(t, "PPO swapped", out, want, 1e-12)

	// Equal periods: fast == slow, so PPO is zero.
	out, _ = PPO(fixtures(60)[0].Real(), 5, 5, MATypeEMA)
	for i := 4; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("equal periods: [%d]=%g", i, out[i])
		}
	}

	// MATypeDisabled: both MAs are the identity, so PPO is zero with no lookback.
	out, _ = PPO(in, 2, 5, MATypeDisabled)
	assertReal(t, "disabled", out, make([]float64, len(in)), 0)

	// Constant input: zero; all-zero input: slow MA is zero, so 0.
	for _, idx := range []int{5, 6} {
		c := fixtures(40)[idx].Real()
		out, _ = PPO(c, 3, 7, MATypeSMA)
		for i := 6; i < len(out); i++ {
			if out[i] != 0 {
				t.Fatalf("fixture %d: [%d]=%g", idx, i, out[i])
			}
		}
	}

	// Rising series: fast MA above slow MA, so PPO > 0; falling: < 0.
	up, _ := PPO(fixtures(80)[7].Real(), 5, 20, MATypeEMA)
	down, _ := PPO(fixtures(80)[8].Real(), 5, 20, MATypeEMA)
	for i := 19; i < 80; i++ {
		if !(up[i] > 0) || !(down[i] < 0) {
			t.Fatalf("trend sign at %d: up %g down %g", i, up[i], down[i])
		}
	}
}

func TestPPORangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := ppoRange(-1, 5, in, 2, 3, MATypeSMA, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := ppoRange(5, 4, in, 2, 3, MATypeSMA, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := ppoRange(0, 1, in, 2, 3, MATypeSMA, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range matches the full series.
	full, _ := PPO(in, 3, 6, MATypeSMA)
	beg, nb, err = ppoRange(10, 20, in, 3, 6, MATypeSMA, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// SMA running sums start at a different bar, so last-ulp differences are expected.
	assertReal(t, "sub-range", out[:nb], full[10:21], relTol)
}
