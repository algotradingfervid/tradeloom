package ta

import (
	"errors"
	"math"
	"testing"
)

func TestHTPHASORParams(t *testing.T) {
	// HT_PHASOR has no optional inputs: the lookback is always 32 (+unstable).
	if lb := HTPHASORLookback(); lb != 32 {
		t.Fatalf("HTPHASORLookback() = %d, want 32", lb)
	}
	in := fixtures(50)[0].Real()
	for _, in := range [][]float64{nil, {}} {
		i, q, err := HTPHASOR(in)
		if err != nil || len(i) != 0 || len(q) != 0 {
			t.Errorf("empty input: i=%v q=%v err=%v, want empty and nil", i, q, err)
		}
	}
	if _, _, err := htphasorRange(0, 40, in[:5], make([]float64, 50), make([]float64, 50)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := htphasorRange(0, 40, in, make([]float64, 40), make([]float64, 50)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outInPhase: err = %v, want ErrBadParam", err)
	}
	if _, _, err := htphasorRange(0, 40, in, make([]float64, 50), make([]float64, 40)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outQuadrature: err = %v, want ErrBadParam", err)
	}
	if _, _, err := htphasorRange(0, 40, in, nil, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil outInPhase: err = %v, want ErrBadParam", err)
	}
	if _, _, err := htphasorRange(0, 40, in, make([]float64, 50), nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil outQuadrature: err = %v, want ErrBadParam", err)
	}
	same := make([]float64, 50)
	if _, _, err := htphasorRange(0, 40, in, same, same); !errors.Is(err, ErrBadParam) {
		t.Errorf("outInPhase == outQuadrature: err = %v, want ErrBadParam", err)
	}
	// Exactly-sized buffers are fine.
	if beg, nb, err := htphasorRange(0, 49, in, make([]float64, 50), make([]float64, 50)); err != nil || beg != 32 || nb != 18 {
		t.Errorf("exact buffers: beg=%d nb=%d err=%v", beg, nb, err)
	}
}

func TestHTPHASORUnstablePeriodLookback(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstHTPhasor, 7); err != nil {
		t.Fatal(err)
	}
	if lb := HTPHASORLookback(); lb != 39 {
		t.Fatalf("lookback with unstable 7 = %d, want 39", lb)
	}
	in := fixtures(60)[0].Real()
	i, q, err := HTPHASOR(in)
	if err != nil {
		t.Fatal(err)
	}
	assertLeadingNaN(t, "InPhase unstable", i, 39)
	assertLeadingNaN(t, "Quadrature unstable", q, 39)
	// Another function's unstable period does not affect HT_PHASOR.
	SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstHTDCPeriod, 9); err != nil {
		t.Fatal(err)
	}
	if lb := HTPHASORLookback(); lb != 32 {
		t.Fatalf("lookback with other unstable = %d, want 32", lb)
	}
}

func TestHTPHASORLengths(t *testing.T) {
	lb := HTPHASORLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			i, q, err := HTPHASOR(s.Real())
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(i) != n || len(q) != n {
				t.Fatalf("%s n=%d: len(i)=%d len(q)=%d", s.Name, n, len(i), len(q))
			}
			assertLeadingNaN(t, lbl("InPhase ", s.Name, " n=", n), i, lb)
			assertLeadingNaN(t, lbl("Quadrature ", s.Name, " n=", n), q, lb)
		}
	}
}

// htphasorReference is an independent, unoptimised rendering of the formula
// in ht_phasor.md (Ehlers' Hilbert transform with explicit history arrays
// instead of TA-Lib's odd/even circular buffers):
//
//	smooth_t     = (4p_t + 3p_{t-1} + 2p_{t-2} + p_{t-3}) / 10
//	HT(x)_t      = (a x_t + b x_{t-2} - b x_{t-4} - a x_{t-6}) * (0.075 period_{t-1} + 0.54)
//	detrender    = HT(smooth), Q1 = HT(detrender), I1 = detrender_{t-3}
//	jI = HT(I1), jQ = HT(Q1); I2, Q2 smoothed 0.2/0.8; Re, Im; period clamped.
//
// History before the first Hilbert bar (bar 12 for a full run) counts as 0,
// matching TA-Lib's zero-initialised buffers.
func htphasorReference(p []float64) (inPhase, quad []float64) {
	const a, b = 0.0962, 0.5769
	n := len(p)
	inPhase, quad = nanSlice(n), nanSlice(n)
	smooth := make([]float64, n)
	det := make([]float64, n)
	q1 := make([]float64, n)
	i1 := make([]float64, n)
	ji := make([]float64, n)
	jq := make([]float64, n)
	at := func(x []float64, k int) float64 {
		if k < 12 {
			return 0
		}
		return x[k]
	}
	ht := func(x []float64, k int, adj float64) float64 {
		return (a*at(x, k) + b*at(x, k-2) - b*at(x, k-4) - a*at(x, k-6)) * adj
	}
	period, re, im, prevI2, prevQ2 := 0.0, 0.0, 0.0, 0.0, 0.0
	for t := 12; t < n; t++ {
		adj := 0.075*period + 0.54
		smooth[t] = (4*p[t] + 3*p[t-1] + 2*p[t-2] + p[t-3]) / 10
		det[t] = ht(smooth, t, adj)
		q1[t] = ht(det, t, adj)
		i1[t] = at(det, t-3)
		ji[t] = ht(i1, t, adj)
		jq[t] = ht(q1, t, adj)
		if t >= 32 {
			inPhase[t], quad[t] = i1[t], q1[t]
		}
		q2 := 0.2*(q1[t]+ji[t]) + 0.8*prevQ2
		i2 := 0.2*(i1[t]-jq[t]) + 0.8*prevI2
		re = 0.2*(i2*prevI2+q2*prevQ2) + 0.8*re
		im = 0.2*(i2*prevQ2-q2*prevI2) + 0.8*im
		prevQ2, prevI2 = q2, i2
		prev := period
		if im != 0 && re != 0 {
			period = 360 / (math.Atan(im/re) * 180 / math.Pi)
		}
		period = math.Min(math.Max(period, 0.67*prev), 1.5*prev)
		period = math.Min(math.Max(period, 6), 50)
		period = 0.2*period + 0.8*prev
	}
	return inPhase, quad
}

func TestHTPHASORKnownValues(t *testing.T) {
	// All-zero input: every Hilbert stage stays exactly 0.
	z := make([]float64, 40)
	i, q, err := HTPHASOR(z)
	if err != nil {
		t.Fatal(err)
	}
	for k := 32; k < 40; k++ {
		if i[k] != 0 || q[k] != 0 {
			t.Fatalf("zeros: [%d] i=%g q=%g", k, i[k], q[k])
		}
	}

	// Constant input: the Hilbert FIR (a, 0, b, 0, -b, 0, -a) sums to 0, so
	// once its 7-tap history is full the detrender is 0, then Q1 (another
	// 7 taps) and the 3-bar-delayed I1 vanish too: from bar 12+6+6 = 24 for
	// Q1 and 12+6+3 = 21 for I1 on, both are exactly 0 (the WMA smoother is
	// c from bar 12 on, and the a/b products are bit-identical).
	c := make([]float64, 100)
	for k := range c {
		c[k] = 100
	}
	i, q, _ = HTPHASOR(c)
	for k := 32; k < len(c); k++ {
		if i[k] != 0 || q[k] != 0 {
			t.Fatalf("constant: [%d] i=%g q=%g, want 0", k, i[k], q[k])
		}
	}

	// Hand-derived: a unit impulse at bar 40 on zeros. Bars 12..39 are all
	// zero, so Re = Im = 0 and the period only follows its clamp:
	//   period_k = 0.2*6 + 0.8*period_{k-1}, period_11 = 0
	// (28 bars, 12..39). At bar 40 the WMA smoother is 4/10 = 0.4 and every
	// Hilbert history is 0, so with adj = 0.075*period_39 + 0.54:
	//   detrender_40 = a*0.4*adj,  Q1_40 = a*detrender_40*adj,
	// and InPhase (the detrender delayed 3 bars) at bar 43 is detrender_40.
	imp := make([]float64, 60)
	imp[40] = 1
	i, q, _ = HTPHASOR(imp)
	const a = 0.0962
	per := 0.0
	for k := 12; k < 40; k++ {
		per = 0.2*6 + 0.8*per
	}
	adj := 0.075*per + 0.54
	d40 := a * 0.4 * adj
	if want := a * d40 * adj; !closeEnough(q[40], want, 1e-14) {
		t.Errorf("impulse Q1[40] = %.17g, want %.17g", q[40], want)
	}
	if !closeEnough(i[43], d40, 1e-14) {
		t.Errorf("impulse InPhase[43] = %.17g, want %.17g", i[43], d40)
	}
	if i[40] != 0 || i[41] != 0 || i[42] != 0 {
		t.Errorf("impulse: InPhase moved before bar 43: %g %g %g", i[40], i[41], i[42])
	}
	for k := 32; k < 40; k++ {
		if i[k] != 0 || q[k] != 0 {
			t.Fatalf("impulse: [%d] i=%g q=%g before the impulse", k, i[k], q[k])
		}
	}

	// Every output agrees with the unoptimised formula on realistic data.
	// Only noisy fixtures: on flat, trending or zigzag series Im is pure
	// rounding noise, so atan(Im/Re) and hence the period depend on the
	// summation order and the two renderings legitimately diverge (the
	// oracle test covers those against C bit-for-bit order).
	for _, s := range fixtures(400) {
		switch s.Name {
		case "constant", "zeros", "up", "down", "zigzag":
			continue
		}
		gi, gq, err := HTPHASOR(s.Real())
		if err != nil {
			t.Fatal(err)
		}
		wi, wq := htphasorReference(s.Real())
		// Summation order differs from TA-Lib's, so allow rounding noise
		// relative to the input's scale (the outputs are linear in it).
		scale := 0.0
		for _, v := range s.Real() {
			scale = math.Max(scale, math.Abs(v))
		}
		tol := 1e-9 * math.Max(scale, 1e-300)
		for k := range gi {
			if math.IsNaN(wi[k]) != math.IsNaN(gi[k]) || math.Abs(gi[k]-wi[k]) > tol || math.Abs(gq[k]-wq[k]) > tol {
				t.Fatalf("%s [%d]: go (%g,%g) reference (%g,%g)", s.Name, k, gi[k], gq[k], wi[k], wq[k])
			}
		}
	}
}

func TestHTPHASORScaleInvariance(t *testing.T) {
	// The transform is linear in the price for a fixed period sequence, and
	// the period depends only on ratios (Im/Re), so scaling the input by a
	// power of two scales both outputs exactly.
	in := fixtures(300)[0].Real()
	i1, q1, _ := HTPHASOR(in)
	sc := make([]float64, len(in))
	for k, v := range in {
		sc[k] = v * 4
	}
	i2, q2, _ := HTPHASOR(sc)
	for k := 32; k < len(in); k++ {
		if i2[k] != 4*i1[k] || q2[k] != 4*q1[k] {
			t.Fatalf("[%d]: scaled (%g,%g) vs 4*(%g,%g)", k, i2[k], q2[k], i1[k], q1[k])
		}
	}
}

func TestHTPHASORRangeForm(t *testing.T) {
	in := fixtures(100)[0].Real()
	oi, oq := make([]float64, len(in)), make([]float64, len(in))
	if _, _, err := htphasorRange(-1, 5, in, oi, oq); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := htphasorRange(5, 4, in, oi, oq); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := htphasorRange(0, -1, in, oi, oq); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := htphasorRange(0, 31, in, oi, oq)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Like C, a sub-range starts its warm-up at startIdx-lookback, so
	// [40, 70] equals a full run over in[8:71] (8 is even, so the odd/even
	// bar parity is unchanged).
	wi, wq, _ := HTPHASOR(in[8:71])
	beg, nb, err = htphasorRange(40, 70, in, oi, oq)
	if err != nil || beg != 40 || nb != 31 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range InPhase", oi[:nb], wi[32:], 0)
	assertReal(t, "sub-range Quadrature", oq[:nb], wq[32:], 0)
	// Start inside the lookback moves up to the lookback.
	beg, nb, _ = htphasorRange(10, 32, in, oi, oq)
	if beg != 32 || nb != 1 {
		t.Errorf("start inside lookback: beg=%d nb=%d", beg, nb)
	}
	// Single bar at the end of the series.
	beg, nb, _ = htphasorRange(99, 99, in, oi, oq)
	if beg != 99 || nb != 1 {
		t.Errorf("last bar: beg=%d nb=%d", beg, nb)
	}
}
