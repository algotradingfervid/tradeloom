package ta

import (
	"errors"
	"math"
	"testing"
)

func TestHTSINEParams(t *testing.T) {
	// HT_SINE has no optional inputs; only the buffers can be invalid.
	in := fixtures(100)[0].Real()
	if lb := HTSINELookback(); lb != 63 {
		t.Fatalf("HTSINELookback() = %d, want 63", lb)
	}
	for _, in := range [][]float64{nil, {}} {
		s, l, err := HTSINE(in)
		if err != nil || len(s) != 0 || len(l) != 0 {
			t.Errorf("empty input: s=%v l=%v err=%v, want empty and nil", s, l, err)
		}
	}
	if _, _, err := htsineRange(0, 80, in[:50], make([]float64, 100), make([]float64, 100)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := htsineRange(0, 80, in, make([]float64, 80), make([]float64, 100)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outSine: err = %v, want ErrBadParam", err)
	}
	if _, _, err := htsineRange(0, 80, in, make([]float64, 100), make([]float64, 80)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outLeadSine: err = %v, want ErrBadParam", err)
	}
	if _, _, err := htsineRange(0, 80, in, nil, make([]float64, 100)); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil outSine: err = %v, want ErrBadParam", err)
	}
	same := make([]float64, 100)
	if _, _, err := htsineRange(0, 80, in, same, same); !errors.Is(err, ErrBadParam) {
		t.Errorf("outSine == outLeadSine: err = %v, want ErrBadParam", err)
	}
	if _, nb, err := htsineRange(0, 99, in, make([]float64, 100), make([]float64, 100)); err != nil || nb != 100-63 {
		t.Errorf("valid range: nb=%d err=%v", nb, err)
	}
}

func TestHTSINEUnstablePeriodLookback(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstHTSine, 7); err != nil {
		t.Fatal(err)
	}
	if lb := HTSINELookback(); lb != 70 {
		t.Fatalf("lookback with unstable 7 = %d, want 70", lb)
	}
	// Another function's unstable period does not affect HT_SINE.
	if err := SetUnstablePeriod(UnstHTDCPhase, 20); err != nil {
		t.Fatal(err)
	}
	if lb := HTSINELookback(); lb != 70 {
		t.Fatalf("lookback with foreign unstable period = %d, want 70", lb)
	}
	in := fixtures(100)[0].Real()
	s, l, err := HTSINE(in)
	if err != nil {
		t.Fatal(err)
	}
	assertLeadingNaN(t, "sine unstable", s, 70)
	assertLeadingNaN(t, "leadsine unstable", l, 70)
}

func TestHTSINELengths(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	for _, u := range []int{0, 1, 30} {
		if err := SetUnstablePeriod(UnstHTSine, u); err != nil {
			t.Fatal(err)
		}
		lb := HTSINELookback()
		if lb != 63+u {
			t.Fatalf("HTSINELookback() = %d with unstable %d", lb, u)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				sine, lead, err := HTSINE(s.Real())
				if err != nil {
					t.Fatalf("%s n=%d u=%d: %v", s.Name, n, u, err)
				}
				if len(sine) != n || len(lead) != n {
					t.Fatalf("%s n=%d u=%d: len=%d,%d", s.Name, n, u, len(sine), len(lead))
				}
				assertLeadingNaN(t, lbl("HT_SINE sine ", s.Name, " n=", n, " u=", u), sine, lb)
				assertLeadingNaN(t, lbl("HT_SINE lead ", s.Name, " n=", n, " u=", u), lead, lb)
			}
		}
	}
}

// htsineZeroReference derives HT_SINE for an all-zero series by hand. Every
// smoothed price and Hilbert component is 0, so Re = Im = 0 and the period
// recursion reduces to period = 0.2*clamp(period) + 0.8*period with the
// clamp forcing 6 whenever the carried period is below 6. With realPart =
// imagPart = 0 the phase only advances by 90 + 360/smoothPeriod per bar,
// wrapping above 315 degrees.
func htsineZeroReference(n int) (sine, lead []float64) {
	sine, lead = nanSlice(n), nanSlice(n)
	period, smoothPeriod, phase := 0.0, 0.0, 0.0
	deg2Rad := math.Pi / 180
	for today := 37; today < n; today++ {
		prev := period
		p := prev // no update: Im and Re are 0
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
		smoothPeriod = 0.67*smoothPeriod + 0.33*period
		phase += 90
		phase += 360 / smoothPeriod
		if phase > 315 {
			phase -= 360
		}
		if today >= 63 {
			sine[today] = math.Sin(phase * deg2Rad)
			lead[today] = math.Sin((phase + 45) * deg2Rad)
		}
	}
	return sine, lead
}

func TestHTSINEKnownValues(t *testing.T) {
	// All-zero input: the phase advances by a hand-derivable amount.
	n := 120
	zeros := make([]float64, n)
	s, l, err := HTSINE(zeros)
	if err != nil {
		t.Fatal(err)
	}
	ws, wl := htsineZeroReference(n)
	assertReal(t, "HT_SINE zeros sine", s, ws, 1e-9)
	assertReal(t, "HT_SINE zeros lead", l, wl, 1e-9)

	// Properties on every fixture: both outputs lie in [-1, 1] and the lead
	// sine is the sine advanced by 45 degrees, so with s = sin(p) and
	// l = sin(p+45°) = (sin p + cos p)/sqrt2, (sqrt2*l - s)^2 + s^2 = 1.
	for _, f := range fixtures(400) {
		s, l, err := HTSINE(f.Real())
		if err != nil {
			t.Fatal(err)
		}
		for i := 63; i < len(s); i++ {
			if s[i] < -1 || s[i] > 1 || l[i] < -1 || l[i] > 1 {
				t.Fatalf("%s [%d]: sine %g lead %g out of [-1,1]", f.Name, i, s[i], l[i])
			}
			c := math.Sqrt2*l[i] - s[i]
			if r := c*c + s[i]*s[i]; math.Abs(r-1) > 1e-9 {
				t.Fatalf("%s [%d]: sine/lead inconsistent, cos^2+sin^2=%g", f.Name, i, r)
			}
		}
	}

	// Scale invariance: the phase depends only on the shape of the series,
	// so multiplying the input by a positive constant leaves it unchanged.
	w := fixtures(300)[0].Real()
	scaled := make([]float64, len(w))
	for i, v := range w {
		scaled[i] = v * 4
	}
	s1, l1, _ := HTSINE(w)
	s2, l2, _ := HTSINE(scaled)
	assertReal(t, "scale sine", s2, s1, 1e-9)
	assertReal(t, "scale lead", l2, l1, 1e-9)

	// A constant series gives the same result at every price level.
	c1, _, _ := HTSINE(fixtures(200)[5].Real())
	c2 := make([]float64, 200)
	for i := range c2 {
		c2[i] = 100
	}
	c3, _, _ := HTSINE(c2)
	assertReal(t, "constant", c1, c3, 0)
}

func TestHTSINERangeForm(t *testing.T) {
	in := fixtures(200)[0].Real()
	o1, o2 := make([]float64, len(in)), make([]float64, len(in))
	if _, _, err := htsineRange(-1, 5, in, o1, o2); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := htsineRange(5, 4, in, o1, o2); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := htsineRange(0, -1, in, o1, o2); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	for _, rg := range [][2]int{{0, 0}, {0, 62}, {10, 62}, {62, 62}} {
		beg, nb, err := htsineRange(rg[0], rg[1], in, o1, o2)
		if err != nil || beg != 0 || nb != 0 {
			t.Errorf("inside lookback %v: beg=%d nb=%d err=%v", rg, beg, nb, err)
		}
	}
	// A sub-range equals the matching slice of the full computation.
	ws, wl, _ := HTSINE(in)
	for _, rg := range [][2]int{{63, 63}, {0, 63}, {70, 120}, {199, 199}, {30, 199}} {
		beg, nb, err := htsineRange(rg[0], rg[1], in, o1, o2)
		if err != nil {
			t.Fatal(err)
		}
		wantBeg := max(rg[0], 63)
		if beg != wantBeg || nb != rg[1]-wantBeg+1 {
			t.Fatalf("range %v: beg=%d nb=%d", rg, beg, nb)
		}
		// The window restarts from bar beg-63, so values differ from the
		// full run (the filter is not stateless); only check they are sane.
		for i := 0; i < nb; i++ {
			if math.IsNaN(o1[i]) || math.IsNaN(o2[i]) {
				t.Fatalf("range %v: NaN at %d", rg, i)
			}
		}
		if rg[0] <= 63 {
			assertReal(t, lbl("range sine ", rg), o1[:nb], ws[beg:beg+nb], 0)
			assertReal(t, lbl("range lead ", rg), o2[:nb], wl[beg:beg+nb], 0)
		}
	}
}
