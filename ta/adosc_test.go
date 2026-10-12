package ta

import (
	"errors"
	"math"
	"testing"
)

func TestADOSCParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := ADOSC(s.H, s.L, s.C, s.V, p, 10); !errors.Is(err, ErrBadParam) {
			t.Errorf("fast %d: err = %v, want ErrBadParam", p, err)
		}
		if _, err := ADOSC(s.H, s.L, s.C, s.V, 3, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("slow %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := ADOSCLookback(p, 10); lb != -1 {
			t.Errorf("ADOSCLookback(%d,10) = %d, want -1", p, lb)
		}
		if lb := ADOSCLookback(3, p); lb != -1 {
			t.Errorf("ADOSCLookback(3,%d) = %d, want -1", p, lb)
		}
		out := make([]float64, 50)
		if _, _, err := adoscRange(0, 49, s.H, s.L, s.C, s.V, p, 10, out); !errors.Is(err, ErrBadParam) {
			t.Errorf("range fast %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, pp := range [][2]int{{2, 2}, {2, 100000}, {100000, 2}, {100000, 100000}, {3, 10}} {
		if _, err := ADOSC(s.H, s.L, s.C, s.V, pp[0], pp[1]); err != nil {
			t.Errorf("periods %v: unexpected err %v", pp, err)
		}
		if lb := ADOSCLookback(pp[0], pp[1]); lb != max(pp[0], pp[1])-1 {
			t.Errorf("ADOSCLookback%v = %d", pp, lb)
		}
	}
	for i := range 4 {
		ins := [][]float64{s.H, s.L, s.C, s.V}
		ins[i] = ins[i][:49]
		if _, err := ADOSC(ins[0], ins[1], ins[2], ins[3], 3, 10); !errors.Is(err, ErrBadParam) {
			t.Errorf("mismatched length of input %d: err = %v, want ErrBadParam", i, err)
		}
	}
	if out, err := ADOSC(nil, nil, nil, nil, 3, 10); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	e := []float64{}
	if out, err := ADOSC(e, e, e, e, 3, 10); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestADOSCLengths(t *testing.T) {
	for _, pp := range [][2]int{{2, 2}, {2, 3}, {3, 10}, {10, 3}, {12, 26}} {
		lb := ADOSCLookback(pp[0], pp[1])
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := ADOSC(s.H, s.L, s.C, s.V, pp[0], pp[1])
				if err != nil {
					t.Fatalf("%s n=%d p=%v: %v", s.Name, n, pp, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%v: len(out)=%d", s.Name, n, pp, len(out))
				}
				assertLeadingNaN(t, lbl("ADOSC ", s.Name, " n=", n, " p=", pp), out, lb)
			}
		}
	}
}

func TestADOSCLookbackUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstEMA, 7); err != nil {
		t.Fatal(err)
	}
	if lb := ADOSCLookback(3, 10); lb != 9+7 {
		t.Errorf("ADOSCLookback(3,10) with UnstEMA=7 = %d, want 16", lb)
	}
	s := fixtures(40)[0]
	out, _ := ADOSC(s.H, s.L, s.C, s.V, 3, 10)
	assertLeadingNaN(t, "ADOSC unstable", out, 16)
}

func TestADOSCKnownValues(t *testing.T) {
	// AD: bar0 (10-0-0)/10*1 = 1; bar1 (5-0-5)/10 = 0 -> 1; bar2 (0-0-10)/10 = -1 -> 0.
	// fastk = 2/3, slowk = 1/2, both EMAs seeded with AD[0] = 1.
	// bar1: fast = 1, slow = 1. bar2: fast = 1/3, slow = 1/2. ADOSC = -1/6.
	h := []float64{10, 10, 10}
	l := []float64{0, 0, 0}
	c := []float64{10, 5, 0}
	v := []float64{1, 1, 1}
	out, err := ADOSC(h, l, c, v, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	assertReal(t, "ADOSC(2,3)", out, []float64{nan, nan, -1.0 / 6}, 1e-15)

	// Swapping the periods negates the output exactly.
	s := fixtures(200)[0]
	a, _ := ADOSC(s.H, s.L, s.C, s.V, 3, 10)
	b, _ := ADOSC(s.H, s.L, s.C, s.V, 10, 3)
	for i := range a {
		if !(math.IsNaN(a[i]) && math.IsNaN(b[i])) && a[i] != -b[i] {
			t.Fatalf("swap: [%d] %g vs %g", i, a[i], b[i])
		}
	}

	// Equal periods: both EMAs are identical, so the output is exactly 0.
	out, _ = ADOSC(s.H, s.L, s.C, s.V, 5, 5)
	for i := 4; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("equal periods: [%d]=%g", i, out[i])
		}
	}

	// Zero range (high == low) never moves AD, so a constant series gives 0;
	// zero volume likewise.
	for _, fx := range []series{fixtures(40)[5], fixtures(40)[12]} {
		out, _ = ADOSC(fx.H, fx.L, fx.C, fx.V, 3, 10)
		for i := 9; i < len(out); i++ {
			if out[i] != 0 {
				t.Fatalf("%s: [%d]=%g", fx.Name, i, out[i])
			}
		}
	}
}

func TestADOSCRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := adoscRange(-1, 5, s.H, s.L, s.C, s.V, 3, 10, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := adoscRange(5, 4, s.H, s.L, s.C, s.V, 3, 10, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	beg, nb, err := adoscRange(0, 8, s.H, s.L, s.C, s.V, 3, 10, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if _, _, err := adoscRange(0, 20, s.H, s.L, s.C[:10], s.V, 3, 10, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := adoscRange(0, 20, s.H, s.L, s.C, s.V, 3, 10, out[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// ADOSC is path dependent: a range starting at startIdx restarts the A/D
	// line at startIdx-lookback, so it equals the full computation over the
	// inputs from bar startIdx-lookback (3) onwards.
	full, _ := ADOSC(s.H[3:21], s.L[3:21], s.C[3:21], s.V[3:21], 3, 10)
	beg, nb, err = adoscRange(12, 20, s.H, s.L, s.C, s.V, 3, 10, out)
	if err != nil || beg != 12 || nb != 9 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[9:18], 0)
}
