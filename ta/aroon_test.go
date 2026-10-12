package ta

import (
	"errors"
	"math"
	"testing"
)

func TestAROONParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, _, err := AROON(s.H, s.L, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := AROONLookback(p); lb != -1 {
			t.Errorf("AROONLookback(%d) = %d, want -1", p, lb)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, _, err := AROON(s.H, s.L, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := AROONLookback(p); lb != p {
			t.Errorf("AROONLookback(%d) = %d, want %d", p, lb, p)
		}
	}
	if _, _, err := AROON(s.H, s.L[:49], 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, _, err := AROON(nil, s.L, 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil high, non-empty low: err = %v, want ErrBadParam", err)
	}
	if d, u, err := AROON(nil, nil, 5); err != nil || len(d) != 0 || len(u) != 0 {
		t.Errorf("nil input: down=%v up=%v err=%v, want empty and nil", d, u, err)
	}
	if d, u, err := AROON([]float64{}, []float64{}, 5); err != nil || len(d) != 0 || len(u) != 0 {
		t.Errorf("empty input: down=%v up=%v err=%v, want empty and nil", d, u, err)
	}
}

func TestAROONLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30} {
		lb := AROONLookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				down, up, err := AROON(s.H, s.L, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(down) != n || len(up) != n {
					t.Fatalf("%s n=%d p=%d: len(down)=%d len(up)=%d", s.Name, n, period, len(down), len(up))
				}
				assertLeadingNaN(t, lbl("AROON down ", s.Name, " n=", n, " p=", period), down, lb)
				assertLeadingNaN(t, lbl("AROON up ", s.Name, " n=", n, " p=", period), up, lb)
			}
		}
	}
}

func TestAROONKnownValues(t *testing.T) {
	nan := math.NaN()
	// Period 2 (factor 50), window of 3 bars.
	//  bar 2: highest high at 1, lowest low at 1           -> up 50, down 50
	//  bar 3: new high at 3; lowest low still at 1 (age 2) -> up 100, down 0
	//  bar 4: high at 3 (age 1); low rescanned over 2..4,
	//         tie 6 == 6 picks the later bar 3 (age 1)     -> up 50, down 50
	high := []float64{1, 3, 2, 5, 4}
	low := []float64{5, 4, 6, 6, 7}
	down, up, err := AROON(high, low, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "AROON up", up, []float64{nan, nan, 50, 100, 50}, 0)
	assertReal(t, "AROON down", down, []float64{nan, nan, 50, 0, 50}, 0)

	// Constant input: ties pick the latest bar, so both are 100.
	c := fixtures(40)[5]
	down, up, _ = AROON(c.H, c.L, 7)
	for i := 7; i < len(up); i++ {
		if up[i] != 100 || down[i] != 100 {
			t.Fatalf("constant: [%d] down=%g up=%g", i, down[i], up[i])
		}
	}

	// Strictly rising: up is 100, down is 0 (lowest low is the oldest bar).
	r := fixtures(60)[7]
	down, up, _ = AROON(r.H, r.L, 10)
	for i := 10; i < len(up); i++ {
		if up[i] != 100 || down[i] != 0 {
			t.Fatalf("up trend: [%d] down=%g up=%g", i, down[i], up[i])
		}
	}

	// Bounds: values are in [0, 100] on every fixture.
	for _, s := range fixtures(300) {
		for _, p := range []int{2, 14, 25} {
			down, up, _ := AROON(s.H, s.L, p)
			for i := p; i < len(up); i++ {
				if up[i] < 0 || up[i] > 100 || down[i] < 0 || down[i] > 100 {
					t.Fatalf("%s p=%d: [%d] down=%g up=%g out of [0,100]", s.Name, p, i, down[i], up[i])
				}
			}
		}
	}
}

func TestAROONRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	down := make([]float64, 30)
	up := make([]float64, 30)
	if _, _, err := aroonRange(-1, 5, s.H, s.L, 3, down, up); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := aroonRange(5, 4, s.H, s.L, 3, down, up); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := aroonRange(0, 10, s.H, s.L, 1, down, up); !errors.Is(err, ErrBadParam) {
		t.Errorf("period 1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := aroonRange(0, 2, s.H, s.L, 3, down, up)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := aroonRange(0, 10, s.H[:5], s.L, 3, down, up); !errors.Is(err, ErrBadParam) {
		t.Errorf("short high: %v", err)
	}
	if _, _, err := aroonRange(0, 10, s.H, s.L[:5], 3, down, up); !errors.Is(err, ErrBadParam) {
		t.Errorf("short low: %v", err)
	}
	if _, _, err := aroonRange(0, 10, s.H, s.L, 3, down[:5], up); !errors.Is(err, ErrBadParam) {
		t.Errorf("short down: %v", err)
	}
	if _, _, err := aroonRange(0, 10, s.H, s.L, 3, down, up[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short up: %v", err)
	}
	// Same buffer for both outputs is rejected, as in C.
	if _, _, err := aroonRange(0, 10, s.H, s.L, 3, down, down); !errors.Is(err, ErrBadParam) {
		t.Errorf("aliased outputs: %v", err)
	}
	// Outputs may alias the inputs.
	wantDown, wantUp, _ := AROON(s.H, s.L, 4)
	h := append([]float64(nil), s.H...)
	l := append([]float64(nil), s.L...)
	beg, nb, _ = aroonRange(0, len(h)-1, h, l, 4, l, h)
	alignReal(l, beg, nb)
	alignReal(h, beg, nb)
	assertReal(t, "in-place down", l, wantDown, 0)
	assertReal(t, "in-place up", h, wantUp, 0)
}
