package ta

import (
	"errors"
	"math"
	"testing"
)

func TestHTDCPHASEParams(t *testing.T) {
	// HT_DCPHASE has no optional inputs; the lookback is 63 + unstable period.
	if lb := HTDCPHASELookback(); lb != 63 {
		t.Fatalf("HTDCPHASELookback() = %d, want 63", lb)
	}
	func() {
		defer SetAllUnstablePeriods(0)
		for _, u := range []int{1, 7, 100} {
			if err := SetUnstablePeriod(UnstHTDCPhase, u); err != nil {
				t.Fatal(err)
			}
			if lb := HTDCPHASELookback(); lb != 63+u {
				t.Errorf("unstable %d: lookback %d, want %d", u, lb, 63+u)
			}
		}
		// Another function's unstable period does not affect HT_DCPHASE.
		SetAllUnstablePeriods(0)
		if err := SetUnstablePeriod(UnstHTDCPeriod, 5); err != nil {
			t.Fatal(err)
		}
		if lb := HTDCPHASELookback(); lb != 63 {
			t.Errorf("UnstHTDCPeriod changed HT_DCPHASE lookback to %d", lb)
		}
	}()

	if out, err := HTDCPHASE(nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := HTDCPHASE([]float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	in := fixtures(100)[0].Real()
	if _, _, err := htdcphaseRange(0, 99, in[:50], make([]float64, 100)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := htdcphaseRange(0, 99, in, make([]float64, 99)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := htdcphaseRange(0, 99, nil, make([]float64, 100)); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input buffer: err = %v, want ErrBadParam", err)
	}
}

func TestHTDCPHASELengths(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	for _, u := range []int{0, 1, 20} {
		if err := SetUnstablePeriod(UnstHTDCPhase, u); err != nil {
			t.Fatal(err)
		}
		lb := HTDCPHASELookback()
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := HTDCPHASE(s.Real())
				if err != nil {
					t.Fatalf("%s n=%d u=%d: %v", s.Name, n, u, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d u=%d: len(out)=%d", s.Name, n, u, len(out))
				}
				assertLeadingNaN(t, lbl("HTDCPHASE ", s.Name, " n=", n, " u=", u), out, lb)
			}
		}
	}
}

func TestHTDCPHASEKnownValues(t *testing.T) {
	// An all-zero series: every smoothed price, Hilbert component, Re and Im
	// is 0, so the period never updates from the Hilbert output and
	// realPart = imagPart = 0. Per ht_dcphase.md the phase then only
	// accumulates the +90 offset and the 360/smoothPeriod lag compensation,
	// with smoothPeriod driven by the clamped period recurrence (period
	// clamped to [0.67, 1.5] x previous, then to [6, 50], then smoothed).
	n := 120
	zeros := make([]float64, n)
	out, err := HTDCPHASE(zeros)
	if err != nil {
		t.Fatal(err)
	}
	want := make([]float64, n)
	for i := range want {
		want[i] = math.NaN()
	}
	period, smoothPeriod, phase := 0.0, 0.0, 0.0
	for bar := 37; bar < n; bar++ { // the main loop starts after 3+34 WMA warm-up bars
		prev := period
		p := period
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
		period = math.FMA(0.2, p, 0.8*prev)
		smoothPeriod = math.FMA(0.67, smoothPeriod, 0.33*period)
		phase += 90
		phase += 360 / smoothPeriod
		if phase > 315 {
			phase -= 360
		}
		if bar >= 63 {
			want[bar] = phase
		}
	}
	assertReal(t, "HTDCPHASE zeros", out, want, 1e-12)
	// First iteration by hand: period = 0.2*6 = 1.2, smoothPeriod = 0.33*1.2
	// = 0.396, phase = 90 + 360/0.396 - 360 = 639.0909...; then the series
	// converges to period 6, where 360/6 = 60 per bar.
	if math.Abs(period-6) > 1e-6 || math.Abs(smoothPeriod-6) > 1e-6 {
		t.Errorf("zeros: period %g smoothPeriod %g, want both 6", period, smoothPeriod)
	}

	// A pure sine of period 20 (well inside the [6, 50] band): the phase of
	// a sine advances by roughly 360/20 = 18 degrees per bar once the
	// estimator has locked on; check the wrapped step is near 18 on average.
	sine := make([]float64, 400)
	for i := range sine {
		sine[i] = 100 + 10*math.Sin(2*math.Pi*float64(i)/20)
	}
	out, err = HTDCPHASE(sine)
	if err != nil {
		t.Fatal(err)
	}
	sum, cnt := 0.0, 0
	for i := 200; i < len(out); i++ {
		d := out[i] - out[i-1]
		for d < -180 {
			d += 360
		}
		for d > 180 {
			d -= 360
		}
		sum += d
		cnt++
	}
	if avg := sum / float64(cnt); math.Abs(avg-18) > 2 {
		t.Errorf("sine period 20: mean phase step %g, want ~18", avg)
	}

	// Bounds: once smoothPeriod is established and imagPart != 0, the phase
	// is atan(...) in (-90, 90) + 90 + 360/smoothPeriod (+180), wrapped once
	// by 360 above 315, so it lies in (-45, 315].
	for _, s := range fixtures(600) {
		switch s.Name {
		case "walk", "walk2", "penny", "huge", "gaps", "doji", "negative":
		default:
			continue
		}
		out, _ := HTDCPHASE(s.Real())
		for i := 63; i < len(out); i++ {
			if !(out[i] > -45 && out[i] <= 315) {
				t.Errorf("%s: [%d]=%g outside (-45, 315]", s.Name, i, out[i])
				break
			}
		}
	}

	// Constant input gives the same phase regardless of the level, up to
	// rounding (the transform is linear and the phase is a ratio).
	c1, _ := HTDCPHASE(fixtures(200)[5].Real()) // constant 100
	k := make([]float64, 200)
	for i := range k {
		k[i] = 100
	}
	c2, _ := HTDCPHASE(k)
	assertReal(t, "constant reproducible", c1, c2, 0)
}

func TestHTDCPHASERangeForm(t *testing.T) {
	in := fixtures(200)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := htdcphaseRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := htdcphaseRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := htdcphaseRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	for _, rg := range [][2]int{{0, 0}, {0, 62}, {62, 62}, {10, 40}} {
		beg, nb, err := htdcphaseRange(rg[0], rg[1], in, out)
		if err != nil || beg != 0 || nb != 0 {
			t.Errorf("inside lookback %v: beg=%d nb=%d err=%v", rg, beg, nb, err)
		}
	}
	// First valid bar.
	full, _ := HTDCPHASE(in)
	beg, nb, err := htdcphaseRange(0, 63, in, out)
	if err != nil || beg != 63 || nb != 1 {
		t.Fatalf("first bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "first bar", out[:1], full[63:64], 0)
	// A sub-range starting past the lookback re-warms from startIdx-63, so it
	// is not a slice of the full run, but its length and begin are fixed.
	beg, nb, err = htdcphaseRange(100, 150, in, out)
	if err != nil || beg != 100 || nb != 51 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	beg, nb, _ = htdcphaseRange(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, full, 0)
}
