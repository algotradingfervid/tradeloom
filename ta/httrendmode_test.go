package ta

import (
	"errors"
	"testing"
)

func TestHTTRENDMODEParams(t *testing.T) {
	if lb := HTTRENDMODELookback(); lb != 63 {
		t.Fatalf("HTTRENDMODELookback() = %d, want 63", lb)
	}
	for _, in := range [][]float64{nil, {}} {
		out, err := HTTRENDMODE(in)
		if err != nil || len(out) != 0 {
			t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
		}
	}
	in := fixtures(80)[0].Real()
	if _, _, err := httrendmodeRange(0, 70, in[:5], make([]int, 80)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := httrendmodeRange(0, 70, in, make([]int, 70)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, nb, err := httrendmodeRange(0, 70, in, make([]int, 71)); err != nil || nb != 70-63+1 {
		t.Errorf("exact output buffer: nb=%d err=%v", nb, err)
	}
}

func TestHTTRENDMODEUnstablePeriodLookback(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	if err := SetUnstablePeriod(UnstHTTrendMode, 7); err != nil {
		t.Fatal(err)
	}
	if lb := HTTRENDMODELookback(); lb != 70 {
		t.Fatalf("lookback with unstable 7 = %d, want 70", lb)
	}
	out, err := HTTRENDMODE(fixtures(100)[0].Real())
	if err != nil {
		t.Fatal(err)
	}
	httrendmodeAssertShape(t, "unstable 7", out, 70)
}

// httrendmodeAssertShape checks the lb leading bars are 0 and every value is
// 0 or 1. Zeros after the lookback are legitimate (cycle mode).
func httrendmodeAssertShape(t *testing.T, label string, out []int, lb int) {
	t.Helper()
	for i, v := range out {
		if i < lb && v != 0 {
			t.Fatalf("%s: [%d]=%d inside lookback %d", label, i, v, lb)
		}
		if v != 0 && v != 1 {
			t.Fatalf("%s: [%d]=%d, want 0 or 1", label, i, v)
		}
	}
}

func TestHTTRENDMODELengths(t *testing.T) {
	lb := HTTRENDMODELookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := HTTRENDMODE(s.Real())
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			httrendmodeAssertShape(t, lbl("HTTRENDMODE ", s.Name, " n=", n), out, lb)
		}
	}
}

func TestHTTRENDMODEKnownValues(t *testing.T) {
	// A step from 100 to 200 at bar 63 (the first output bar): the smoothed
	// price is the 4-3-2-1 WMA 0.4*200 + 0.6*100 = 140, while the trendline
	// is (2*100 + 4*avg + 3*100 + 100)/10 with avg the raw mean over the
	// dominant cycle (at most 200), so trendline <= 140 and, for any cycle of
	// 6+ bars, about 104-107: a deviation far above 1.5%, which forces trend.
	in := make([]float64, 64)
	for i := range in {
		in[i] = 100
	}
	in[63] = 200
	out, err := HTTRENDMODE(in)
	if err != nil {
		t.Fatal(err)
	}
	if out[63] != 1 {
		t.Errorf("step: out[63]=%d, want 1", out[63])
	}

	// A steady exponential trend keeps the smoothed price well away from the
	// lagging trendline, so every late bar is in trend mode.
	in = make([]float64, 300)
	v := 100.0
	for i := range in {
		in[i] = v
		v *= 1.02
	}
	out, _ = HTTRENDMODE(in)
	for i := 150; i < len(out); i++ {
		if out[i] != 1 {
			t.Fatalf("exponential trend: out[%d]=%d, want 1", i, out[i])
		}
	}

	// The classifier must actually switch on realistic data.
	seen := [2]bool{}
	out, _ = HTTRENDMODE(fixtures(1000)[0].Real())
	for _, x := range out[HTTRENDMODELookback():] {
		seen[x] = true
	}
	if !seen[0] || !seen[1] {
		t.Errorf("walk n=1000: seen 0=%v 1=%v, want both", seen[0], seen[1])
	}
}

func TestHTTRENDMODERangeForm(t *testing.T) {
	in := fixtures(100)[0].Real()
	out := make([]int, len(in))
	if _, _, err := httrendmodeRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := httrendmodeRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := httrendmodeRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	beg, nb, err := httrendmodeRange(0, 62, in, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// The full range equals the public form.
	want, _ := HTTRENDMODE(in)
	beg, nb, err = httrendmodeRange(0, len(in)-1, in, out)
	if err != nil || beg != 63 || nb != len(in)-63 {
		t.Fatalf("full range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	alignInt(out, beg, nb)
	assertInt(t, "range vs public", out, want)
}
