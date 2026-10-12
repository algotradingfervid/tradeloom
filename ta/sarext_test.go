package ta

import (
	"errors"
	"math"
	"testing"
)

// sarextParams holds the eight SAREXT optional inputs in TA-Lib order.
type sarextParams [8]float64

var sarextDefaults = sarextParams{0, 0, 0.02, 0.02, 0.2, 0.02, 0.02, 0.2}

func sarextCall(h, l []float64, p sarextParams) ([]float64, error) {
	return SAREXT(h, l, p[0], p[1], p[2], p[3], p[4], p[5], p[6], p[7])
}

func sarextLB(p sarextParams) int {
	return SAREXTLookback(p[0], p[1], p[2], p[3], p[4], p[5], p[6], p[7])
}

func sarextRangeCall(start, end int, h, l []float64, p sarextParams, out []float64) (int, int, error) {
	return sarextRange(start, end, h, l, p[0], p[1], p[2], p[3], p[4], p[5], p[6], p[7], out)
}

// sarextParamSets is a spread of parameter sets shared by the Go and oracle tests.
var sarextParamSets = []sarextParams{
	sarextDefaults,
	{0, 0.01, 0.01, 0.01, 0.2, 0.01, 0.01, 0.2},
	{0, 0.15, 0.19, 0.2, 0.4, 0.19, 0.2, 0.4},
	{0, 0, 0.5, 0.5, 0.2, 0.5, 0.5, 0.2}, // init/step above max: clamped
	{0, 0, 0, 0, 0, 0, 0, 0},             // all zero
	{0, 0, 0.02, 0.05, 0.3, 0.04, 0.01, 0.1},
	{0, 0, 1, 1, 1, 1, 1, 1},
	{50, 0, 0.02, 0.02, 0.2, 0.02, 0.02, 0.2},      // forced long
	{-50, 0, 0.02, 0.02, 0.2, 0.02, 0.02, 0.2},     // forced short
	{1e12, 0.05, 0.02, 0.02, 0.2, 0.02, 0.02, 0.2}, // forced long far away
	{-1e12, 0.05, 0.02, 0.02, 0.2, 0.02, 0.02, 0.2},
	{-1e-12, 0, 0.02, 0.02, 0.2, 0.02, 0.02, 0.2},
	{3e37, 3e37, 3e37, 3e37, 3e37, 3e37, 3e37, 3e37}, // every maximum
	{-3e37, 0, 0, 0, 0, 0, 0, 0},
}

func TestSAREXTParams(t *testing.T) {
	s := fixtures(50)[0]
	names := []string{"StartValue", "OffsetOnReverse", "AccInitLong", "AccLong", "AccMaxLong", "AccInitShort", "AccShort", "AccMaxShort"}
	for i := range sarextDefaults {
		lo, hi := 0.0, 3e37
		if i == 0 {
			lo = -3e37
		}
		bad := []float64{math.NaN(), math.Inf(1), math.Inf(-1), 3.0000001e37, math.Nextafter(hi, math.Inf(1))}
		if i == 0 {
			bad = append(bad, -3.0000001e37, math.Nextafter(lo, math.Inf(-1)))
		} else {
			bad = append(bad, -1e-300, -1, math.Nextafter(lo, math.Inf(-1)))
		}
		for _, v := range bad {
			p := sarextDefaults
			p[i] = v
			if _, err := sarextCall(s.H, s.L, p); !errors.Is(err, ErrBadParam) {
				t.Errorf("%s=%g: err = %v, want ErrBadParam", names[i], v, err)
			}
			if lb := sarextLB(p); lb != -1 {
				t.Errorf("%s=%g: lookback %d, want -1", names[i], v, lb)
			}
			if _, _, err := sarextRangeCall(0, 49, s.H, s.L, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
				t.Errorf("range %s=%g: err = %v, want ErrBadParam", names[i], v, err)
			}
		}
		for _, v := range []float64{lo, hi} {
			p := sarextDefaults
			p[i] = v
			if _, err := sarextCall(s.H, s.L, p); err != nil {
				t.Errorf("%s=%g: unexpected err %v", names[i], v, err)
			}
			if lb := sarextLB(p); lb != 1 {
				t.Errorf("%s=%g: lookback %d, want 1", names[i], v, lb)
			}
		}
	}
	if _, err := sarextCall(s.H, s.L[:49], sarextDefaults); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v", err)
	}
	if out, err := sarextCall(nil, nil, sarextDefaults); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v", out, err)
	}
	if out, err := sarextCall([]float64{}, []float64{}, sarextDefaults); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v", out, err)
	}
	if _, _, err := sarextRangeCall(0, 10, s.H[:5], s.L, sarextDefaults, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: %v", err)
	}
	if _, _, err := sarextRangeCall(0, 10, s.H, s.L[:5], sarextDefaults, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: %v", err)
	}
	if _, _, err := sarextRangeCall(0, 10, s.H, s.L, sarextDefaults, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outReal: %v", err)
	}
}

func TestSAREXTLengths(t *testing.T) {
	for _, p := range sarextParamSets {
		lb := sarextLB(p)
		if lb != 1 {
			t.Fatalf("lookback %v = %d", p, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := sarextCall(s.H, s.L, p)
				if err != nil {
					t.Fatalf("%s n=%d p=%v: %v", s.Name, n, p, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%v: len(out)=%d", s.Name, n, p, len(out))
				}
				if p[2] < 1e30 { // the all-maximum set legitimately overflows to Inf/NaN
					assertLeadingNaN(t, lbl("SAREXT ", s.Name, " n=", n, " p=", p), out, lb)
				}
			}
		}
	}
}

func TestSAREXTKnownValues(t *testing.T) {
	nan := math.NaN()
	// Rising bars: -DM(bar0,bar1) = 0 -> long. ep = H1 = 11, sar = L0 = 9.
	// Bar 2: output 9.04 (= 9 + 0.02*(11-9)); new high 12 -> af 0.04.
	h, l := []float64{10, 11, 12, 13}, []float64{9, 10, 11, 12}
	out, err := sarextCall(h, l, sarextDefaults)
	if err != nil {
		t.Fatal(err)
	}
	s3 := math.FMA(0.04, 12-9.04, 9.04)
	assertReal(t, "rising", out, []float64{nan, 9, 9.04, s3}, 1e-15)

	// Falling bars: -DM = 1 -> short, reported as negative values.
	h, l = []float64{12, 11, 10}, []float64{11, 10, 9}
	out, _ = sarextCall(h, l, sarextDefaults)
	assertReal(t, "falling", out, []float64{nan, -12, -11.96}, 1e-15)

	// Reversal from long to short: bar 2's low 8 penetrates sar 9.04; the
	// new sar is the ep (11), negated. With a 10% offset it is 12.1.
	h, l = []float64{10, 11, 9}, []float64{9, 10, 8}
	out, _ = sarextCall(h, l, sarextDefaults)
	assertReal(t, "reverse", out, []float64{nan, 9, -11}, 1e-15)
	p := sarextDefaults
	p[1] = 0.1
	out, _ = sarextCall(h, l, p)
	assertReal(t, "reverse+offset", out, []float64{nan, 9, -12.1}, 1e-12)

	// Forced start values.
	p = sarextDefaults
	p[0] = 5
	out, _ = sarextCall([]float64{10, 11}, []float64{9, 10}, p)
	assertReal(t, "start long", out, []float64{nan, 5}, 0)
	p[0] = -20
	out, _ = sarextCall([]float64{10, 11}, []float64{9, 10}, p)
	assertReal(t, "start short", out, []float64{nan, -20}, 0)

	// Sign property: on a strict up trend SAREXT stays positive and below
	// the low; on a strict down trend it stays negative with |sar| above the high.
	up := fixtures(100)[7]
	out, _ = sarextCall(up.H, up.L, sarextDefaults)
	for i := 1; i < len(out); i++ {
		if !(out[i] > 0 && out[i] <= up.L[i]) {
			t.Fatalf("up trend [%d] = %g (low %g)", i, out[i], up.L[i])
		}
	}
	down := fixtures(100)[8]
	out, _ = sarextCall(down.H, down.L, sarextDefaults)
	for i := 1; i < len(out); i++ {
		if !(out[i] < 0 && -out[i] >= down.H[i]) {
			t.Fatalf("down trend [%d] = %g (high %g)", i, out[i], down.H[i])
		}
	}

	// Constant bars: tie in -DM -> long; sar is the constant low.
	c := fixtures(30)[5]
	out, _ = sarextCall(c.H, c.L, sarextDefaults)
	for i := 1; i < len(out); i++ {
		if math.Abs(out[i]) != 100 {
			t.Fatalf("constant [%d] = %g", i, out[i])
		}
	}
}

func TestSAREXTRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := sarextRangeCall(-1, 5, s.H, s.L, sarextDefaults, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := sarextRangeCall(5, 4, s.H, s.L, sarextDefaults, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := sarextRangeCall(0, -1, s.H, s.L, sarextDefaults, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	beg, nb, err := sarextRangeCall(0, 0, s.H, s.L, sarextDefaults, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	beg, nb, err = sarextRangeCall(0, 29, s.H, s.L, sarextDefaults, out)
	if err != nil || beg != 1 || nb != 29 {
		t.Errorf("full: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// A sub-range restarts the SAR at startIdx using bar startIdx-1, so it
	// equals the full computation over the series starting at startIdx-1.
	beg, nb, err = sarextRangeCall(10, 12, s.H, s.L, sarextDefaults, out)
	if err != nil || beg != 10 || nb != 3 {
		t.Errorf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	want, _ := sarextCall(s.H[9:13], s.L[9:13], sarextDefaults)
	assertReal(t, "sub-range", out[:3], want[1:], 0)
}
