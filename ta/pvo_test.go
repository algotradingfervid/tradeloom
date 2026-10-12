package ta

import (
	"errors"
	"math"
	"testing"
)

func TestPVOParams(t *testing.T) {
	in := fixtures(50)[0].V
	bad := []struct {
		fast, slow int
		mt         MAType
	}{
		{1, 26, MATypeEMA}, {0, 26, MATypeEMA}, {-1, 26, MATypeEMA}, {100001, 26, MATypeEMA}, {math.MinInt32, 26, MATypeEMA},
		{12, 1, MATypeEMA}, {12, 0, MATypeEMA}, {12, 100001, MATypeEMA}, {12, math.MaxInt32, MATypeEMA},
		{12, 26, -1}, {12, 26, 14}, {12, 26, 99},
	}
	for _, c := range bad {
		if _, err := PVO(in, c.fast, c.slow, c.mt); !errors.Is(err, ErrBadParam) {
			t.Errorf("fast=%d slow=%d mt=%d: err = %v, want ErrBadParam", c.fast, c.slow, c.mt, err)
		}
		if lb := PVOLookback(c.fast, c.slow, c.mt); lb != -1 {
			t.Errorf("PVOLookback(%d,%d,%d) = %d, want -1", c.fast, c.slow, c.mt, lb)
		}
		if _, _, err := pvoRange(0, len(in)-1, in, c.fast, c.slow, c.mt, make([]float64, len(in))); !errors.Is(err, ErrBadParam) {
			t.Errorf("range fast=%d slow=%d mt=%d: err = %v, want ErrBadParam", c.fast, c.slow, c.mt, err)
		}
	}
	good := []struct {
		fast, slow int
		mt         MAType
	}{
		{2, 2, MATypeSMA}, {2, 100000, MATypeEMA}, {100000, 2, MATypeEMA}, {100000, 100000, MATypeSMA},
		{12, 26, MATypeDefault}, {12, 26, MATypeRMA}, {12, 26, MATypeDisabled},
	}
	for _, c := range good {
		if _, err := PVO(in, c.fast, c.slow, c.mt); err != nil {
			t.Errorf("fast=%d slow=%d mt=%d: unexpected err %v", c.fast, c.slow, c.mt, err)
		}
		if lb := PVOLookback(c.fast, c.slow, c.mt); lb < 0 {
			t.Errorf("PVOLookback(%d,%d,%d) = %d", c.fast, c.slow, c.mt, lb)
		}
	}
	for _, mt := range maAllTypes {
		if _, err := PVO(in, 3, 7, mt); err != nil {
			t.Errorf("mt=%d: unexpected err %v", mt, err)
		}
	}
	// MATypeDefault means EMA for PVO.
	if a, b := PVOLookback(12, 26, MATypeDefault), PVOLookback(12, 26, MATypeEMA); a != b || a != 25 {
		t.Errorf("default lookback %d, EMA lookback %d, want 25", a, b)
	}
	if out, err := PVO(nil, 12, 26, MATypeEMA); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := PVO([]float64{}, 12, 26, MATypeEMA); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := pvoRange(0, 10, in[:5], 2, 3, MATypeSMA, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := pvoRange(0, 10, in, 2, 3, MATypeSMA, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
}

func TestPVOLengths(t *testing.T) {
	params := []struct {
		fast, slow int
		mt         MAType
	}{
		{12, 26, MATypeEMA}, {2, 3, MATypeSMA}, {26, 12, MATypeEMA}, {5, 10, MATypeWMA},
		{4, 9, MATypeT3}, {3, 30, MATypeKAMA}, {5, 8, MATypeMAMA}, {2, 2, MATypeDisabled},
		{3, 14, MATypeTRIMA}, {3, 14, MATypeHMA}, {3, 14, MATypeZLEMA}, {3, 14, MATypeRMA},
		{3, 14, MATypeDEMA}, {3, 14, MATypeTEMA},
	}
	for _, p := range params {
		lb := PVOLookback(p.fast, p.slow, p.mt)
		if want := MALookback(max(p.fast, p.slow), p.mt); lb != want {
			t.Fatalf("PVOLookback(%d,%d,%d) = %d, want %d", p.fast, p.slow, p.mt, lb, want)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := PVO(s.V, p.fast, p.slow, p.mt)
				if err != nil {
					t.Fatalf("%s n=%d %v: %v", s.Name, n, p, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d %v: len(out)=%d", s.Name, n, p, len(out))
				}
				assertLeadingNaN(t, lbl("PVO ", s.Name, " n=", n, " ", p), out, lb)
			}
		}
	}
}

func TestPVOKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{1, 2, 3, 4, 5}
	// SMA(2) vs SMA(3): bar 2 fast 2.5 slow 2 -> 25%; bar 3 3.5/3; bar 4 4.5/4.
	out, err := PVO(in, 2, 3, MATypeSMA)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "PVO SMA(2,3)", out, []float64{nan, nan, 25, (3.5 - 3) / 3 * 100, 12.5}, 1e-12)

	// Fast and slow are swapped when slow < fast.
	sw, _ := PVO(in, 3, 2, MATypeSMA)
	assertReal(t, "PVO swapped", sw, out, 0)

	// EMA default: hand-check against EMA directly.
	v := fixtures(80)[0].V
	fast, _ := EMA(v, 12)
	slow, _ := EMA(v, 26)
	got, _ := PVO(v, 12, 26, MATypeDefault)
	for i := 25; i < len(v); i++ {
		want := (fast[i] - slow[i]) / slow[i] * 100
		if !closeEnough(got[i], want, 1e-12) {
			t.Fatalf("EMA [%d]: got %g want %g", i, got[i], want)
		}
	}

	// Constant volume: zero oscillator. Zero volume: slow MA zero -> 0.
	for _, s := range []series{constant("c", 60, 100), constant("z", 60, 0)} {
		out, _ := PVO(s.V, 5, 12, MATypeEMA)
		for i := 11; i < len(out); i++ {
			if out[i] != 0 {
				t.Fatalf("%s: [%d]=%g, want 0", s.Name, i, out[i])
			}
		}
	}

	// Equal periods, MATypeDisabled and MAMA (which ignores the period) give 0.
	for _, c := range []struct {
		fast, slow int
		mt         MAType
	}{{7, 7, MATypeEMA}, {3, 9, MATypeDisabled}, {3, 9, MATypeMAMA}} {
		out, _ := PVO(v, c.fast, c.slow, c.mt)
		lb := PVOLookback(c.fast, c.slow, c.mt)
		for i := lb; i < len(out); i++ {
			if out[i] != 0 {
				t.Fatalf("%v: [%d]=%g, want 0", c, i, out[i])
			}
		}
	}

	// Non-negative volume: the fast SMA is >= 0, so PVO >= -100.
	for _, s := range fixtures(300) {
		out, _ := PVO(s.V, 4, 20, MATypeSMA)
		for i := 19; i < len(out); i++ {
			if out[i] < -100 {
				t.Fatalf("%s: [%d]=%g < -100", s.Name, i, out[i])
			}
		}
	}
}

func TestPVORangeForm(t *testing.T) {
	in := fixtures(60)[0].V
	out := make([]float64, len(in))
	if _, _, err := pvoRange(-1, 5, in, 2, 3, MATypeSMA, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := pvoRange(5, 4, in, 2, 3, MATypeSMA, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := pvoRange(0, -1, in, 2, 3, MATypeSMA, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := pvoRange(0, 24, in, 12, 26, MATypeEMA, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// A range starting inside the lookback begins at the lookback.
	beg, nb, err = pvoRange(3, 40, in, 12, 26, MATypeEMA, out)
	if err != nil || beg != 25 || nb != 16 {
		t.Errorf("start inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Sub-range values match the full series for SMA (no history dependence).
	full, _ := PVO(in, 3, 8, MATypeSMA)
	beg, nb, err = pvoRange(20, 30, in, 3, 8, MATypeSMA, out)
	if err != nil || beg != 20 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[20:31], 0)
}
