package ta

import (
	"errors"
	"math"
	"testing"
)

func TestHTTRENDLINEParams(t *testing.T) {
	// HT_TRENDLINE has no optional inputs: the lookback is never -1.
	if lb := HTTRENDLINELookback(); lb != 63 {
		t.Fatalf("HTTRENDLINELookback() = %d, want 63", lb)
	}
	for _, in := range [][]float64{nil, {}} {
		out, err := HTTRENDLINE(in)
		if err != nil || len(out) != 0 {
			t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
		}
	}
	in := fixtures(100)[0].Real()
	if _, _, err := httrendlineRange(0, 99, in[:50], make([]float64, 100)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := httrendlineRange(0, 99, in, make([]float64, 99)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := httrendlineRange(10, 99, in, make([]float64, 90)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestHTTRENDLINEUnstablePeriodLookback(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstHTTrendline, 7); err != nil {
		t.Fatal(err)
	}
	if lb := HTTRENDLINELookback(); lb != 70 {
		t.Fatalf("lookback with unstable 7 = %d, want 70", lb)
	}
	// Another function's unstable period must not change it.
	if err := SetUnstablePeriod(UnstHTDCPeriod, 20); err != nil {
		t.Fatal(err)
	}
	if lb := HTTRENDLINELookback(); lb != 70 {
		t.Fatalf("lookback after HT_DCPERIOD unstable = %d, want 70", lb)
	}
	for _, n := range edgeLengths(70) {
		out, err := HTTRENDLINE(fixtures(n)[0].Real())
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != n {
			t.Fatalf("n=%d len(out)=%d", n, len(out))
		}
		assertLeadingNaN(t, lbl("HTTRENDLINE unstable n=", n), out, 70)
	}
}

func TestHTTRENDLINELengths(t *testing.T) {
	lb := HTTRENDLINELookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := HTTRENDLINE(s.Real())
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			assertLeadingNaN(t, lbl("HTTRENDLINE ", s.Name, " n=", n), out, lb)
		}
	}
}

func TestHTTRENDLINEKnownValues(t *testing.T) {
	// A constant series: every average of raw prices is the constant, and
	// (4c+3c+2c+c)/10 == c exactly for c = 100 and c = 0.
	for _, c := range []float64{100, 0} {
		in := constant("c", 200, c).Real()
		out, err := HTTRENDLINE(in)
		if err != nil {
			t.Fatal(err)
		}
		for i := 63; i < len(out); i++ {
			if out[i] != c {
				t.Fatalf("constant %g: [%d]=%.17g", c, i, out[i])
			}
		}
	}

	// A step on the last bar of a flat series: the three previous averages
	// are still 100 and today's is (100*(k-1)+x)/k for the dominant cycle
	// length k = int(smoothPeriod+0.5), which is in [6, 50]. So
	// out = 100 + 0.4*(x-100)/k for some integer k in that range.
	in := constant("c", 150, 100).Real()
	in[149] = 100 + 1200
	out, err := HTTRENDLINE(in)
	if err != nil {
		t.Fatal(err)
	}
	if out[148] != 100 {
		t.Fatalf("bar before step = %.17g, want 100", out[148])
	}
	k := 0.4 * 1200 / (out[149] - 100)
	if math.Abs(k-math.Round(k)) > 1e-9 || k < 6 || k > 50 {
		t.Fatalf("step: out=%.17g implies k=%g, want an integer in [6,50]", out[149], k)
	}
	// Bounds: the output is a convex combination of the last 53 raw prices.
	for _, s := range fixtures(400) {
		in := s.Real()
		out, _ := HTTRENDLINE(in)
		for i := 63; i < len(out); i++ {
			lo, hi := math.Inf(1), math.Inf(-1)
			for j := i - 52; j <= i; j++ {
				lo = math.Min(lo, in[j])
				hi = math.Max(hi, in[j])
			}
			eps := 1e-9 * math.Max(1, math.Max(math.Abs(lo), math.Abs(hi)))
			if out[i] < lo-eps || out[i] > hi+eps {
				t.Fatalf("%s [%d]=%g outside [%g, %g]", s.Name, i, out[i], lo, hi)
			}
		}
	}
}

func TestHTTRENDLINERangeForm(t *testing.T) {
	in := fixtures(200)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := httrendlineRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := httrendlineRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := httrendlineRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	for _, rg := range [][2]int{{0, 0}, {0, 62}, {62, 62}} {
		beg, nb, err := httrendlineRange(rg[0], rg[1], in, out)
		if err != nil || beg != 0 || nb != 0 {
			t.Errorf("inside lookback %v: beg=%d nb=%d err=%v", rg, beg, nb, err)
		}
	}
	// First bar with output.
	beg, nb, err := httrendlineRange(0, 63, in, out)
	if err != nil || beg != 63 || nb != 1 {
		t.Errorf("first bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// A sub-range equals the matching slice of the full computation only
	// when it starts at the lookback (the warm-up restarts at startIdx-63).
	full, _ := HTTRENDLINE(in)
	beg, nb, _ = httrendlineRange(63, 199, in, out)
	assertReal(t, "range from lookback", out[:nb], full[beg:], 0)
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	beg, nb, _ = httrendlineRange(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, full, 0)
}
