package ta

import (
	"errors"
	"math"
	"testing"
)

func TestACParams(t *testing.T) {
	s := fixtures(50)[0]
	bad := []int{1, 0, -1, 100001, math.MinInt32, math.MaxInt32}
	for _, p := range bad {
		for k, args := range [][3]int{{p, 34, 5}, {5, p, 5}, {5, 34, p}} {
			if _, err := AC(s.H, s.L, args[0], args[1], args[2]); !errors.Is(err, ErrBadParam) {
				t.Errorf("param #%d = %d: err = %v, want ErrBadParam", k, p, err)
			}
			if lb := ACLookback(args[0], args[1], args[2]); lb != -1 {
				t.Errorf("ACLookback%v = %d, want -1", args, lb)
			}
			out := make([]float64, len(s.H))
			if _, _, err := acRange(0, len(s.H)-1, s.H, s.L, args[0], args[1], args[2], out); !errors.Is(err, ErrBadParam) {
				t.Errorf("acRange%v: err = %v, want ErrBadParam", args, err)
			}
		}
	}
	for _, pp := range [][3]int{
		{2, 2, 2}, {2, 100000, 2}, {100000, 2, 2}, {2, 2, 100000},
		{100000, 100000, 100000}, {5, 34, 5}, {34, 5, 5},
	} {
		if _, err := AC(s.H, s.L, pp[0], pp[1], pp[2]); err != nil {
			t.Errorf("periods %v: unexpected err %v", pp, err)
		}
		want := max(pp[0], pp[1]) - 1 + pp[2] - 1
		if lb := ACLookback(pp[0], pp[1], pp[2]); lb != want {
			t.Errorf("ACLookback%v = %d, want %d", pp, lb, want)
		}
	}
	if lb := ACLookback(5, 34, 5); lb != 37 {
		t.Errorf("default lookback = %d, want 37", lb)
	}
	if _, err := AC(s.H, s.L[:49], 5, 34, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, err := AC(s.H, nil, 5, 34, 5); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil low: err = %v, want ErrBadParam", err)
	}
	if out, err := AC(nil, nil, 5, 34, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := AC([]float64{}, []float64{}, 5, 34, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestACLengths(t *testing.T) {
	for _, pp := range [][3]int{{2, 2, 2}, {2, 3, 2}, {5, 34, 5}, {34, 5, 5}, {10, 30, 40}, {3, 7, 33}} {
		lb := ACLookback(pp[0], pp[1], pp[2])
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := AC(s.H, s.L, pp[0], pp[1], pp[2])
				if err != nil {
					t.Fatalf("%s n=%d p=%v: %v", s.Name, n, pp, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%v: len(out)=%d", s.Name, n, pp, len(out))
				}
				assertLeadingNaN(t, lbl("AC ", s.Name, " n=", n, " p=", pp), out, lb)
			}
		}
	}
}

func TestACKnownValues(t *testing.T) {
	// median = (2m + 0)/2 = m exactly: 1, 2, 4, 8, 16, 32.
	med := []float64{1, 2, 4, 8, 16, 32}
	hi := make([]float64, len(med))
	lo := make([]float64, len(med))
	for i, m := range med {
		hi[i] = 2 * m
	}
	out, err := AC(hi, lo, 2, 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	// AO(2,3): [2]=2/3, [3]=4/3, [4]=8/3, [5]=16/3.
	// AC = AO - mean(AO over 2): [3]=4/3-1=1/3, [4]=8/3-2=2/3, [5]=16/3-4=4/3.
	want := []float64{nan, nan, nan, 1.0 / 3, 2.0 / 3, 4.0 / 3}
	assertReal(t, "AC(2,3,2)", out, want, 1e-15)

	// Inverted fast/slow is not swapped: AC(slow, fast) == -AC(fast, slow).
	inv, _ := AC(hi, lo, 3, 2, 2)
	for i := 3; i < len(inv); i++ {
		if inv[i] != -out[i] {
			t.Errorf("inverted [%d] = %g, want %g", i, inv[i], -out[i])
		}
	}

	// Equal fast and slow: AO is exactly 0, so AC is exactly 0.
	s := fixtures(80)[0]
	eq, _ := AC(s.H, s.L, 7, 7, 4)
	for i := ACLookback(7, 7, 4); i < len(eq); i++ {
		if eq[i] != 0 {
			t.Fatalf("equal periods [%d] = %g", i, eq[i])
		}
	}

	// Constant series: AO is 0, so AC is 0.
	c := fixtures(60)[5]
	out, _ = AC(c.H, c.L, 5, 34, 5)
	for i := 37; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Linear trend: AO is constant (up to rounding), so AC is ~0.
	up := fixtures(100)[7]
	out, _ = AC(up.H, up.L, 5, 34, 5)
	for i := 37; i < len(out); i++ {
		if math.Abs(out[i]) > 1e-9 {
			t.Fatalf("linear trend: [%d]=%g", i, out[i])
		}
	}

	// Composition: AO from aoRange, then SMA(AO, signal) from smaRange, both
	// started at the AC oscillator start, then AO - SMA is bit-identical
	// (as ta_AC.c states).
	for _, pp := range [][3]int{{5, 34, 5}, {34, 5, 5}, {3, 10, 7}, {2, 2, 2}, {4, 9, 40}} {
		for _, s := range []series{fixtures(200)[1], fixtures(200)[4], fixtures(200)[9]} {
			n := len(s.H)
			lb := ACLookback(pp[0], pp[1], pp[2])
			oscStart := lb - (pp[2] - 1)
			ao := make([]float64, n)
			begAO, nbAO, err := aoRange(oscStart, n-1, s.H, s.L, pp[0], pp[1], ao)
			if err != nil || begAO != oscStart {
				t.Fatalf("aoRange beg=%d err=%v", begAO, err)
			}
			sig := make([]float64, n)
			begS, nbS, err := smaRange(0, nbAO-1, ao, pp[2], sig)
			if err != nil || begS != pp[2]-1 || nbS != n-lb {
				t.Fatalf("smaRange beg=%d nb=%d err=%v", begS, nbS, err)
			}
			got, _ := AC(s.H, s.L, pp[0], pp[1], pp[2])
			for k := 0; k < nbS; k++ {
				if w := ao[begS+k] - sig[k]; got[lb+k] != w {
					t.Fatalf("composite %s p=%v [%d] got %.17g want %.17g", s.Name, pp, lb+k, got[lb+k], w)
				}
			}
		}
	}

	// Signal ring larger than any small buffer (C switches to heap at > 32).
	z := fixtures(300)[0]
	big, _ := AC(z.H, z.L, 5, 34, 100)
	if math.IsNaN(big[len(big)-1]) {
		t.Fatal("signal 100: last value NaN")
	}
}

func TestACRangeForm(t *testing.T) {
	s := fixtures(60)[0]
	out := make([]float64, len(s.H))
	if _, _, err := acRange(-1, 5, s.H, s.L, 5, 10, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := acRange(5, 4, s.H, s.L, 5, 10, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := acRange(0, -1, s.H, s.L, 5, 10, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (lb = 9 + 2 = 11): no output, no error.
	beg, nb, err := acRange(0, 10, s.H, s.L, 5, 10, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	beg, nb, err = acRange(0, 11, s.H, s.L, 5, 10, 3, out)
	if err != nil || beg != 11 || nb != 1 {
		t.Errorf("first bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := acRange(0, 30, s.H[:20], s.L, 5, 10, 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inHigh: %v", err)
	}
	if _, _, err := acRange(0, 30, s.H, s.L[:20], 5, 10, 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inLow: %v", err)
	}
	if _, _, err := acRange(0, 30, s.H, s.L, 5, 10, 3, out[:30]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outReal: %v", err)
	}
	// Sub-range values match the full computation (up to accumulation order).
	full, _ := AC(s.H, s.L, 5, 10, 3)
	beg, nb, err = acRange(20, 40, s.H, s.L, 5, 10, 3, out)
	if err != nil || beg != 20 || nb != 21 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[20:41], 1e-12)
	// Single bar at the end of the series.
	beg, nb, err = acRange(59, 59, s.H, s.L, 5, 10, 3, out)
	if err != nil || beg != 59 || nb != 1 {
		t.Fatalf("last bar: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "last bar", out[:1], full[59:], 1e-12)
	// In-place: outReal aliasing inHigh, and aliasing inLow.
	for _, which := range []string{"high", "low"} {
		h := append([]float64(nil), s.H...)
		l := append([]float64(nil), s.L...)
		buf := h
		if which == "low" {
			buf = l
		}
		beg, nb, _ = acRange(0, len(buf)-1, h, l, 5, 10, 3, buf)
		alignReal(buf, beg, nb)
		assertReal(t, "in-place "+which, buf, full, 0)
	}
}
