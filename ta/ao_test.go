package ta

import (
	"errors"
	"math"
	"testing"
)

func TestAOParams(t *testing.T) {
	s := fixtures(50)[0]
	bad := []int{1, 0, -1, 100001, math.MinInt32}
	for _, p := range bad {
		if _, err := AO(s.H, s.L, p, 34); !errors.Is(err, ErrBadParam) {
			t.Errorf("fast %d: err = %v, want ErrBadParam", p, err)
		}
		if _, err := AO(s.H, s.L, 5, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("slow %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := AOLookback(p, 34); lb != -1 {
			t.Errorf("AOLookback(%d,34) = %d, want -1", p, lb)
		}
		if lb := AOLookback(5, p); lb != -1 {
			t.Errorf("AOLookback(5,%d) = %d, want -1", p, lb)
		}
	}
	for _, pp := range [][2]int{{2, 2}, {2, 100000}, {100000, 2}, {100000, 100000}, {5, 34}} {
		if _, err := AO(s.H, s.L, pp[0], pp[1]); err != nil {
			t.Errorf("periods %v: unexpected err %v", pp, err)
		}
		if lb := AOLookback(pp[0], pp[1]); lb != max(pp[0], pp[1])-1 {
			t.Errorf("AOLookback%v = %d", pp, lb)
		}
	}
	if _, err := AO(s.H, s.L[:49], 5, 34); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if out, err := AO(nil, nil, 5, 34); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := AO([]float64{}, []float64{}, 5, 34); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestAOLengths(t *testing.T) {
	for _, pp := range [][2]int{{2, 2}, {2, 3}, {5, 34}, {34, 5}, {10, 30}} {
		lb := AOLookback(pp[0], pp[1])
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := AO(s.H, s.L, pp[0], pp[1])
				if err != nil {
					t.Fatalf("%s n=%d p=%v: %v", s.Name, n, pp, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%v: len(out)=%d", s.Name, n, pp, len(out))
				}
				assertLeadingNaN(t, lbl("AO ", s.Name, " n=", n, " p=", pp), out, lb)
			}
		}
	}
}

func TestAOKnownValues(t *testing.T) {
	// median = (2m + 0)/2 = m exactly: 1, 2, 4, 8, 16.
	med := []float64{1, 2, 4, 8, 16}
	hi := make([]float64, len(med))
	lo := make([]float64, len(med))
	for i, m := range med {
		hi[i] = 2 * m
	}
	out, err := AO(hi, lo, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	// i=2: 3 - 7/3; i=3: 6 - 14/3; i=4: 12 - 28/3.
	want := []float64{nan, nan, 3 - 7.0/3, 6 - 14.0/3, 12 - 28.0/3}
	assertReal(t, "AO(2,3)", out, want, 1e-15)

	// Inverted pair is not swapped: AO(slow, fast) == -AO(fast, slow).
	inv, _ := AO(hi, lo, 3, 2)
	for i := 2; i < len(inv); i++ {
		if inv[i] != -out[i] {
			t.Errorf("inverted [%d] = %g, want %g", i, inv[i], -out[i])
		}
	}

	// Equal periods give exactly zero.
	s := fixtures(80)[0]
	eq, _ := AO(s.H, s.L, 7, 7)
	for i := 6; i < len(eq); i++ {
		if eq[i] != 0 {
			t.Fatalf("equal periods [%d] = %g", i, eq[i])
		}
	}

	// Constant series: both averages agree, so AO is 0.
	c := fixtures(60)[5]
	out, _ = AO(c.H, c.L, 5, 34)
	for i := 33; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Composition: SMA(median, fast) - SMA(median, slow), with each SMA run
	// from the same clamped startIdx, is bit-identical (as ta_AO.c states).
	for _, pp := range [][2]int{{5, 34}, {34, 5}, {3, 10}} {
		s := fixtures(200)[1]
		n := len(s.H)
		medp := make([]float64, n)
		for i := range medp {
			medp[i] = (s.H[i] + s.L[i]) / 2.0
		}
		lb := AOLookback(pp[0], pp[1])
		f := make([]float64, n)
		sl := make([]float64, n)
		_, nbf, _ := smaRange(lb, n-1, medp, pp[0], f)
		_, nbs, _ := smaRange(lb, n-1, medp, pp[1], sl)
		got, _ := AO(s.H, s.L, pp[0], pp[1])
		if nbf != n-lb || nbs != n-lb {
			t.Fatalf("sma nb %d %d", nbf, nbs)
		}
		for k := 0; k < n-lb; k++ {
			if w := f[k] - sl[k]; got[lb+k] != w {
				t.Fatalf("composite p=%v [%d] got %.17g want %.17g", pp, lb+k, got[lb+k], w)
			}
		}
	}

	// Sign follows trend: rising prices -> positive AO, falling -> negative.
	up := fixtures(60)[7]
	out, _ = AO(up.H, up.L, 5, 34)
	for i := 33; i < len(out); i++ {
		if !(out[i] > 0) {
			t.Fatalf("up trend [%d]=%g", i, out[i])
		}
	}
	dn := fixtures(60)[8]
	out, _ = AO(dn.H, dn.L, 5, 34)
	for i := 33; i < len(out); i++ {
		if !(out[i] < 0) {
			t.Fatalf("down trend [%d]=%g", i, out[i])
		}
	}
}

func TestAORangeForm(t *testing.T) {
	s := fixtures(60)[0]
	out := make([]float64, len(s.H))
	if _, _, err := aoRange(-1, 5, s.H, s.L, 5, 10, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := aoRange(5, 4, s.H, s.L, 5, 10, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := aoRange(0, -1, s.H, s.L, 5, 10, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	if _, _, err := aoRange(0, 5, s.H, s.L, 1, 10, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("bad fast: %v", err)
	}
	if _, _, err := aoRange(0, 5, s.H, s.L, 5, 100001, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("bad slow: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := aoRange(0, 8, s.H, s.L, 5, 10, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := aoRange(0, 30, s.H[:20], s.L, 5, 10, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: %v", err)
	}
	if _, _, err := aoRange(0, 30, s.H, s.L[:20], 5, 10, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: %v", err)
	}
	if _, _, err := aoRange(0, 30, s.H, s.L, 5, 10, out[:30]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outReal: %v", err)
	}
	// Sub-range values match the full computation (up to accumulation order).
	full, _ := AO(s.H, s.L, 5, 10)
	beg, nb, err = aoRange(20, 40, s.H, s.L, 5, 10, out)
	if err != nil || beg != 20 || nb != 21 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[20:41], 1e-12)
	// In-place: outReal aliasing inHigh, and aliasing inLow.
	for _, which := range []string{"high", "low"} {
		h := append([]float64(nil), s.H...)
		l := append([]float64(nil), s.L...)
		buf := h
		if which == "low" {
			buf = l
		}
		beg, nb, _ = aoRange(0, len(buf)-1, h, l, 5, 10, buf)
		alignReal(buf, beg, nb)
		assertReal(t, "in-place "+which, buf, full, 0)
	}
}
