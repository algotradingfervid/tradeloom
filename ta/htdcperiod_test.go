package ta

import (
	"errors"
	"math"
	"testing"
)

func TestHTDCPERIODParams(t *testing.T) {
	// HT_DCPERIOD has no optional inputs: the lookback is always valid.
	if lb := HTDCPERIODLookback(); lb != 32 {
		t.Fatalf("HTDCPERIODLookback() = %d, want 32", lb)
	}
	for _, in := range [][]float64{nil, {}} {
		out, err := HTDCPERIOD(in)
		if err != nil || len(out) != 0 {
			t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
		}
	}
	in := fixtures(50)[0].Real()
	if _, _, err := htdcperiodRange(0, 40, in[:5], make([]float64, 50)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := htdcperiodRange(0, 40, in, make([]float64, 40)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := htdcperiodRange(0, 40, nil, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := htdcperiodRange(0, 40, in, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil output buffer: err = %v, want ErrBadParam", err)
	}
}

func TestHTDCPERIODUnstablePeriodLookback(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstHTDCPeriod, 7); err != nil {
		t.Fatal(err)
	}
	if lb := HTDCPERIODLookback(); lb != 39 {
		t.Fatalf("lookback with unstable 7 = %d, want 39", lb)
	}
	in := fixtures(60)[0].Real()
	out, err := HTDCPERIOD(in)
	if err != nil {
		t.Fatal(err)
	}
	assertLeadingNaN(t, "HTDCPERIOD unstable", out, 39)
	// Another function's unstable period does not affect HT_DCPERIOD.
	SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstMAMA, 9); err != nil {
		t.Fatal(err)
	}
	if lb := HTDCPERIODLookback(); lb != 32 {
		t.Fatalf("lookback with UnstMAMA set = %d, want 32", lb)
	}
}

func TestHTDCPERIODLengths(t *testing.T) {
	for _, unst := range []int{0, 5} {
		func() {
			defer SetAllUnstablePeriods(0)
			if err := SetUnstablePeriod(UnstHTDCPeriod, unst); err != nil {
				t.Fatal(err)
			}
			lb := HTDCPERIODLookback()
			if lb != 32+unst {
				t.Fatalf("HTDCPERIODLookback() = %d with unstable %d", lb, unst)
			}
			for _, n := range edgeLengths(lb) {
				for _, s := range fixtures(n) {
					out, err := HTDCPERIOD(s.Real())
					if err != nil {
						t.Fatalf("%s n=%d: %v", s.Name, n, err)
					}
					if len(out) != n {
						t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
					}
					assertLeadingNaN(t, lbl("HTDCPERIOD ", s.Name, " n=", n, " unst=", unst), out, lb)
				}
			}
		}()
	}
}

// htdcperiodFlatExpected derives HT_DCPERIOD for an input on which the
// Hilbert terms stay exactly 0 (all zeros), from the formula in
// ht_dcperiod.md: Re = Im = 0, so the raw period is never recomputed; it is
// only clamped to [0.67, 1.5] x previous and to [6, 50], then smoothed:
//
//	period_k = 0.2*6 + 0.8*period_{k-1}   (period_{-1} = 0)
//	smooth_k = 0.33*period_k + 0.67*smooth_{k-1}
//
// The main loop starts at bar 12 (12 bars warm the WMA price smoother).
func htdcperiodFlatExpected(n int) []float64 {
	out := nanSlice(n)
	period, smooth := 0.0, 0.0
	for k := 12; k < n; k++ {
		prev := period
		p := prev
		if p > 1.5*prev {
			p = 1.5 * prev
		}
		if p < 0.67*prev {
			p = 0.67 * prev
		}
		if p < 6 {
			p = 6
		} else if p > 50 {
			p = 50
		}
		period = 0.2*p + 0.8*prev
		smooth = 0.67*smooth + 0.33*period
		if k >= 32 {
			out[k] = smooth
		}
	}
	return out
}

func TestHTDCPERIODKnownValues(t *testing.T) {
	// Hand-checked first steps: bar 12 period 1.2, smooth 0.396; bar 13
	// period 2.16, smooth 0.67*0.396+0.33*2.16 = 0.97812.
	z := make([]float64, 34)
	exp := htdcperiodFlatExpected(len(z))
	got, err := HTDCPERIOD(z)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "zeros", got, exp, 1e-13)
	// The recurrence converges to 6.
	z = make([]float64, 400)
	got, _ = HTDCPERIOD(z)
	if math.Abs(got[399]-6) > 1e-9 {
		t.Errorf("zeros: long-run period %.17g, want 6", got[399])
	}
	assertReal(t, "zeros long", got, htdcperiodFlatExpected(400), 1e-13)

	// A pure sine of period P: the dominant cycle tends toward P.
	for _, p := range []float64{15, 20, 30} {
		in := make([]float64, 600)
		for i := range in {
			in[i] = 100 + 10*math.Sin(2*math.Pi*float64(i)/p)
		}
		out, err := HTDCPERIOD(in)
		if err != nil {
			t.Fatal(err)
		}
		if v := out[599]; math.Abs(v-p) > 0.15*p {
			t.Errorf("sine period %g: measured %g", p, v)
		}
	}
}

func TestHTDCPERIODBounds(t *testing.T) {
	// period is clamped to [6, 50] before being blended with the previous
	// (non-negative) period, and smoothPeriod is a convex blend of those, so
	// every output lies in (0, 50].
	for _, s := range fixtures(1000) {
		out, err := HTDCPERIOD(s.Real())
		if err != nil {
			t.Fatal(err)
		}
		for i := 32; i < len(out); i++ {
			if !(out[i] > 0 && out[i] <= 50) {
				t.Fatalf("%s [%d] = %g outside (0, 50]", s.Name, i, out[i])
			}
		}
	}
	// The cycle period does not depend on the price scale: every Hilbert
	// term is linear in the price, so Im/Re is unchanged when the input is
	// multiplied by a power of two (an exact operation in floating point).
	for _, s := range []series{fixtures(300)[0], fixtures(300)[9], fixtures(300)[13]} {
		in := s.Real()
		base, _ := HTDCPERIOD(in)
		for _, f := range []float64{4, 0.125} {
			sc := make([]float64, len(in))
			for i, v := range in {
				sc[i] = v * f
			}
			out, _ := HTDCPERIOD(sc)
			assertReal(t, lbl("scaled ", s.Name, " x", f), out, base, 0)
		}
	}
}

func TestHTDCPERIODRangeForm(t *testing.T) {
	in := fixtures(100)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := htdcperiodRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := htdcperiodRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := htdcperiodRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := htdcperiodRange(0, 31, in, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Like C, a sub-range starts its warm-up at startIdx-lookback, so
	// [40, 70] equals a full run over in[8:71] (8 is even, so the odd/even
	// bar parity is unchanged).
	want, _ := HTDCPERIOD(in[8:71])
	beg, nb, err = htdcperiodRange(40, 70, in, out)
	if err != nil || beg != 40 || nb != 31 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], want[32:], 0)
	// Start inside the lookback moves up to the lookback.
	beg, nb, _ = htdcperiodRange(10, 32, in, out)
	if beg != 32 || nb != 1 {
		t.Errorf("start inside lookback: beg=%d nb=%d", beg, nb)
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	full, _ := HTDCPERIOD(in)
	beg, nb, _ = htdcperiodRange(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, full, 0)
}
