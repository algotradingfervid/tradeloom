package ta

import (
	"errors"
	"math"
	"testing"
)

func TestRVIParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	bad := [][2]int{{0, 10}, {-1, 10}, {100001, 10}, {math.MinInt32, 10},
		{14, 1}, {14, 0}, {14, -1}, {14, 100001}, {0, 1}}
	for _, p := range bad {
		if _, err := RVI(in, p[0], p[1]); !errors.Is(err, ErrBadParam) {
			t.Errorf("params %v: err = %v, want ErrBadParam", p, err)
		}
		if lb := RVILookback(p[0], p[1]); lb != -1 {
			t.Errorf("RVILookback(%v) = %d, want -1", p, lb)
		}
		if _, _, err := rviRange(0, 49, in, p[0], p[1], make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range params %v: err = %v, want ErrBadParam", p, err)
		}
	}
	good := [][2]int{{1, 2}, {2, 2}, {14, 10}, {100000, 2}, {1, 100000}, {100000, 100000}}
	for _, p := range good {
		if _, err := RVI(in, p[0], p[1]); err != nil {
			t.Errorf("params %v: unexpected err %v", p, err)
		}
		if lb := RVILookback(p[0], p[1]); lb != p[1]-1+p[0]-1 {
			t.Errorf("RVILookback(%v) = %d", p, lb)
		}
	}
	if out, err := RVI(nil, 14, 10); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := RVI([]float64{}, 14, 10); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestRVILengths(t *testing.T) {
	for _, p := range [][2]int{{1, 2}, {2, 2}, {14, 10}, {5, 20}, {30, 3}} {
		lb := RVILookback(p[0], p[1])
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := RVI(s.Real(), p[0], p[1])
				if err != nil {
					t.Fatalf("%s n=%d p=%v: %v", s.Name, n, p, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%v: len(out)=%d", s.Name, n, p, len(out))
				}
				label := lbl("RVI ", s.Name, " n=", n, " p=", p)
				assertLeadingNaN(t, label, out, lb)
				// Bounds: RVI is in [0, 100].
				for i := lb; i < n; i++ {
					if out[i] < 0 || out[i] > 100 {
						t.Fatalf("%s: [%d]=%g out of [0,100]", label, i, out[i])
					}
				}
			}
		}
	}
}

func TestRVIKnownValues(t *testing.T) {
	nan := math.NaN()
	// Time period 1 has no memory: 100 on an up bar, 0 on a down bar, 50 on
	// a tie (both legs zero). StdDev period 2 gives sigma = |a-b|/2.
	out, err := RVI([]float64{1, 2, 4, 3, 3}, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "RVI(p=1,sd=2)", out, []float64{nan, 100, 100, 0, 50}, 0)

	// Period 2, stddev 2 on {1,2,4,3}: sigmas 0.5 (up), 1 (up), 0.5 (down).
	// Seed up=(0.5+1)/2=0.75, dn=0 -> 100. Next: up=0.375, dn=0.25 -> 60.
	out, err = RVI([]float64{1, 2, 4, 3}, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "RVI(p=2,sd=2)", out, []float64{nan, nan, 100, 60}, 1e-12)

	// A constant series has only ties: neutral 50 everywhere.
	c := fixtures(60)[5].Real()
	out, _ = RVI(c, 14, 10)
	for i := RVILookback(14, 10); i < len(out); i++ {
		if out[i] != 50 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}
	// Strictly rising / falling series: 100 / 0.
	up := fixtures(60)[7].Real()
	out, _ = RVI(up, 14, 10)
	for i := RVILookback(14, 10); i < len(out); i++ {
		if out[i] != 100 {
			t.Fatalf("up: [%d]=%g", i, out[i])
		}
	}
	dn := fixtures(60)[8].Real()
	out, _ = RVI(dn, 14, 10)
	for i := RVILookback(14, 10); i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("down: [%d]=%g", i, out[i])
		}
	}
}

func TestRVIUnstablePeriod(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	in := fixtures(100)[0].Real()
	base, _ := RVI(in, 14, 10)
	if err := SetUnstablePeriod(UnstRVI, 5); err != nil {
		t.Fatal(err)
	}
	if lb := RVILookback(14, 10); lb != 22+5 {
		t.Fatalf("lookback with unstable 5 = %d", lb)
	}
	out, _ := RVI(in, 14, 10)
	assertLeadingNaN(t, "RVI unst=5", out, 27)
	SetAllUnstablePeriods(0)
	// Another function's unstable period must not leak into RVI.
	SetUnstablePeriod(UnstRSI, 7)
	out, _ = RVI(in, 14, 10)
	assertReal(t, "RVI rsi-unst", out, base, 0)
}

func TestRVIRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := rviRange(-1, 5, in, 3, 2, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := rviRange(5, 4, in, 3, 2, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	// Range entirely inside the lookback (lb = 3): no output, no error.
	beg, nb, err := rviRange(0, 2, in, 3, 2, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if _, _, err := rviRange(0, 10, in[:5], 3, 2, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := rviRange(0, 10, in, 3, 2, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Full range through the range form matches the public form.
	want, _ := RVI(in, 3, 2)
	beg, nb, err = rviRange(0, len(in)-1, in, 3, 2, out)
	if err != nil || beg != 3 || nb != len(in)-3 {
		t.Fatalf("full range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	alignReal(out, beg, nb)
	assertReal(t, "range form", out, want, 0)
}
