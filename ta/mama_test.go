package ta

import (
	"errors"
	"math"
	"testing"
)

func TestMAMAParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	nan := math.NaN()
	bad := [][2]float64{
		{0.0099, 0.05}, {0.991, 0.05}, {0, 0.05}, {-1, 0.05}, {1, 0.05}, {nan, 0.05},
		{0.5, 0.0099}, {0.5, 0.991}, {0.5, 0}, {0.5, 1}, {0.5, nan},
		{math.Inf(1), 0.05}, {0.5, math.Inf(-1)},
	}
	for _, p := range bad {
		if _, _, err := MAMA(in, p[0], p[1]); !errors.Is(err, ErrBadParam) {
			t.Errorf("params %v: err = %v, want ErrBadParam", p, err)
		}
		if lb := MAMALookback(p[0], p[1]); lb != -1 {
			t.Errorf("MAMALookback(%v) = %d, want -1", p, lb)
		}
		if _, _, err := mamaRange(0, 49, in, p[0], p[1], make([]float64, 50), make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range params %v: err = %v, want ErrBadParam", p, err)
		}
	}
	good := [][2]float64{{0.01, 0.01}, {0.99, 0.99}, {0.01, 0.99}, {0.99, 0.01}, {0.5, 0.05}}
	for _, p := range good {
		if _, _, err := MAMA(in, p[0], p[1]); err != nil {
			t.Errorf("params %v: unexpected err %v", p, err)
		}
		if lb := MAMALookback(p[0], p[1]); lb != 32 {
			t.Errorf("MAMALookback(%v) = %d, want 32", p, lb)
		}
	}
	for _, in := range [][]float64{nil, {}} {
		m, f, err := MAMA(in, 0.5, 0.05)
		if err != nil || len(m) != 0 || len(f) != 0 {
			t.Errorf("empty input: m=%v f=%v err=%v, want empty and nil", m, f, err)
		}
	}
	if _, _, err := mamaRange(0, 40, in[:5], 0.5, 0.05, make([]float64, 50), make([]float64, 50)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := mamaRange(0, 40, in, 0.5, 0.05, make([]float64, 40), make([]float64, 50)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outMAMA: err = %v, want ErrBadParam", err)
	}
	if _, _, err := mamaRange(0, 40, in, 0.5, 0.05, make([]float64, 50), make([]float64, 40)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outFAMA: err = %v, want ErrBadParam", err)
	}
	same := make([]float64, 50)
	if _, _, err := mamaRange(0, 40, in, 0.5, 0.05, same, same); !errors.Is(err, ErrBadParam) {
		t.Errorf("outMAMA == outFAMA: err = %v, want ErrBadParam", err)
	}
	// outFAMA is nullable, like in C.
	if _, nb, err := mamaRange(0, 49, in, 0.5, 0.05, make([]float64, 50), nil); err != nil || nb != 50-32 {
		t.Errorf("nil outFAMA: nb=%d err=%v", nb, err)
	}
}

func TestMAMAUnstablePeriodLookback(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstMAMA, 7); err != nil {
		t.Fatal(err)
	}
	if lb := MAMALookback(0.5, 0.05); lb != 39 {
		t.Fatalf("lookback with unstable 7 = %d, want 39", lb)
	}
	in := fixtures(60)[0].Real()
	m, f, err := MAMA(in, 0.5, 0.05)
	if err != nil {
		t.Fatal(err)
	}
	assertLeadingNaN(t, "MAMA unstable", m, 39)
	assertLeadingNaN(t, "FAMA unstable", f, 39)
}

func TestMAMALengths(t *testing.T) {
	for _, p := range [][2]float64{{0.5, 0.05}, {0.01, 0.01}, {0.99, 0.99}, {0.3, 0.1}} {
		lb := MAMALookback(p[0], p[1])
		if lb != 32 {
			t.Fatalf("MAMALookback(%v) = %d", p, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				m, f, err := MAMA(s.Real(), p[0], p[1])
				if err != nil {
					t.Fatalf("%s n=%d p=%v: %v", s.Name, n, p, err)
				}
				if len(m) != n || len(f) != n {
					t.Fatalf("%s n=%d p=%v: len(m)=%d len(f)=%d", s.Name, n, p, len(m), len(f))
				}
				assertLeadingNaN(t, lbl("MAMA ", s.Name, " n=", n, " p=", p), m, lb)
				assertLeadingNaN(t, lbl("FAMA ", s.Name, " n=", n, " p=", p), f, lb)
			}
		}
	}
}

func TestMAMAKnownValues(t *testing.T) {
	// All-zero input: every smoother stays at 0.
	z := make([]float64, 40)
	m, f, err := MAMA(z, 0.5, 0.05)
	if err != nil {
		t.Fatal(err)
	}
	for i := 32; i < 40; i++ {
		if m[i] != 0 || f[i] != 0 {
			t.Fatalf("zeros: [%d] mama=%g fama=%g", i, m[i], f[i])
		}
	}

	// Constant input c: once the Hilbert warm-up has passed (the main loop
	// starts at bar 12; the delayed I1/jI/jQ terms die out by bar 44), the
	// detrender, Q1 and I1 are 0, so the phase is 0, the delta
	// phase is clamped to 1 and alpha is fastLimit. By the formula in mama.md:
	//   MAMA_k = f*c + (1-f)*MAMA_{k-1}
	//   FAMA_k = (f/2)*MAMA_k + (1-f/2)*FAMA_{k-1}
	// so with f = 0.5 the distance to c halves every bar.
	const c = 100.0
	in := make([]float64, 120)
	for i := range in {
		in[i] = c
	}
	for _, fast := range []float64{0.5, 0.2} {
		m, f, err = MAMA(in, fast, 0.05)
		if err != nil {
			t.Fatal(err)
		}
		for k := 44; k < len(in); k++ {
			wm := fast*c + (1-fast)*m[k-1]
			wf := fast/2*wm + (1-fast/2)*f[k-1]
			if !closeEnough(m[k], wm, 1e-13) || !closeEnough(f[k], wf, 1e-13) {
				t.Fatalf("constant f=%g [%d]: mama=%.17g want %.17g, fama=%.17g want %.17g", fast, k, m[k], wm, f[k], wf)
			}
			if m[k] < m[k-1] || m[k] > c || f[k] < f[k-1] || f[k] > m[k]+1e-12 {
				t.Fatalf("constant f=%g [%d]: not converging monotonically to c", fast, k)
			}
		}
	}
	// Hand-derived: f=0.5, the distance to c halves per bar.
	m, _, _ = MAMA(in, 0.5, 0.05)
	if d0, d1 := c-m[45], c-m[46]; d0 <= 0 || math.Abs(d1-d0/2) > 1e-5*d0 {
		t.Errorf("distance did not halve: %g -> %g", d0, d1)
	}
}

func TestMAMABounds(t *testing.T) {
	// MAMA and FAMA are convex combinations of 0 (their seed) and the prices,
	// so they stay inside [min(0, min price), max(0, max price)].
	for _, s := range fixtures(500) {
		in := s.Real()
		lo, hi := 0.0, 0.0
		for _, v := range in {
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
		for _, p := range [][2]float64{{0.5, 0.05}, {0.99, 0.01}} {
			m, f, err := MAMA(in, p[0], p[1])
			if err != nil {
				t.Fatal(err)
			}
			eps := 1e-9 * math.Max(1, math.Max(math.Abs(lo), math.Abs(hi)))
			for i := 32; i < len(in); i++ {
				if m[i] < lo-eps || m[i] > hi+eps || f[i] < lo-eps || f[i] > hi+eps {
					t.Fatalf("%s p=%v [%d]: mama=%g fama=%g outside [%g,%g]", s.Name, p, i, m[i], f[i], lo, hi)
				}
			}
		}
	}
}

func TestMAMARangeForm(t *testing.T) {
	in := fixtures(100)[0].Real()
	om, of := make([]float64, len(in)), make([]float64, len(in))
	if _, _, err := mamaRange(-1, 5, in, 0.5, 0.05, om, of); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := mamaRange(5, 4, in, 0.5, 0.05, om, of); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := mamaRange(0, -1, in, 0.5, 0.05, om, of); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := mamaRange(0, 31, in, 0.5, 0.05, om, of)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Like C, a sub-range starts its warm-up at startIdx-lookback, so
	// [40, 70] equals a full run over in[8:71] (8 is even, so the odd/even
	// bar parity is unchanged).
	wm, wf, _ := MAMA(in[8:71], 0.5, 0.05)
	beg, nb, err = mamaRange(40, 70, in, 0.5, 0.05, om, of)
	if err != nil || beg != 40 || nb != 31 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range MAMA", om[:nb], wm[32:], 0)
	assertReal(t, "sub-range FAMA", of[:nb], wf[32:], 0)
	// Start inside the lookback moves up to the lookback.
	beg, nb, _ = mamaRange(10, 32, in, 0.5, 0.05, om, of)
	if beg != 32 || nb != 1 {
		t.Errorf("start inside lookback: beg=%d nb=%d", beg, nb)
	}
}
