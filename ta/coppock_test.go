package ta

import (
	"errors"
	"math"
	"testing"
)

func TestCOPPOCKParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	bad := []int{0, -1, 100001, math.MinInt32}
	for _, p := range bad {
		for _, args := range [][3]int{{p, 11, 14}, {10, p, 14}, {10, 11, p}} {
			if _, err := COPPOCK(in, args[0], args[1], args[2]); !errors.Is(err, ErrBadParam) {
				t.Errorf("params %v: err = %v, want ErrBadParam", args, err)
			}
			if lb := COPPOCKLookback(args[0], args[1], args[2]); lb != -1 {
				t.Errorf("COPPOCKLookback%v = %d, want -1", args, lb)
			}
			if _, _, err := coppockRange(0, len(in)-1, in, args[0], args[1], args[2], make([]float64, len(in))); !errors.Is(err, ErrBadParam) {
				t.Errorf("range params %v: err = %v, want ErrBadParam", args, err)
			}
		}
	}
	for _, args := range [][3]int{{1, 1, 1}, {100000, 11, 14}, {10, 100000, 14}, {10, 11, 100000}, {100000, 100000, 100000}, {10, 14, 11}} {
		if _, err := COPPOCK(in, args[0], args[1], args[2]); err != nil {
			t.Errorf("params %v: unexpected err %v", args, err)
		}
		if lb := COPPOCKLookback(args[0], args[1], args[2]); lb < 0 {
			t.Errorf("COPPOCKLookback%v = %d", args, lb)
		}
	}
	if lb := COPPOCKLookback(100000, 100000, 100000); lb != 199999 {
		t.Errorf("max lookback = %d, want 199999", lb)
	}
	if out, err := COPPOCK(nil, 10, 11, 14); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := COPPOCK([]float64{}, 10, 11, 14); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestCOPPOCKLengths(t *testing.T) {
	for _, ps := range [][3]int{{1, 1, 1}, {2, 1, 2}, {10, 11, 14}, {10, 14, 11}, {3, 5, 5}, {60, 2, 3}} {
		w, r1, r2 := ps[0], ps[1], ps[2]
		lb := COPPOCKLookback(w, r1, r2)
		if want := max(r1, r2) + w - 1; lb != want {
			t.Fatalf("COPPOCKLookback%v = %d, want %d", ps, lb, want)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := COPPOCK(s.Real(), w, r1, r2)
				if err != nil {
					t.Fatalf("%s n=%d p=%v: %v", s.Name, n, ps, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%v: len(out)=%d", s.Name, n, ps, len(out))
				}
				assertLeadingNaN(t, lbl("COPPOCK ", s.Name, " n=", n, " p=", ps), out, lb)
			}
		}
	}
}

// coppockNaive is a direct WMA-of-(ROC+ROC) reference for the tests.
func coppockNaive(in []float64, w, r1, r2 int) []float64 {
	roc := func(j, p int) float64 {
		if in[j-p] == 0 {
			return 0
		}
		return (in[j]/in[j-p] - 1) * 100
	}
	lb := max(r1, r2) + w - 1
	out := make([]float64, len(in))
	for k := range out {
		if k < lb {
			out[k] = math.NaN()
			continue
		}
		num, den := 0.0, 0.0
		for m := 1; m <= w; m++ {
			j := k - w + m
			num += float64(m) * (roc(j, r1) + roc(j, r2))
			den += float64(m)
		}
		out[k] = num / den
	}
	return out
}

func TestCOPPOCKKnownValues(t *testing.T) {
	nan := math.NaN()
	// in = 1..6, w=2, roc1=1, roc2=2. S(j) = (x_j/x_{j-1}-1)*100 + (x_j/x_{j-2}-1)*100:
	// S2 = 50+200 = 250, S3 = 100/3+100 = 400/3, S4 = 25+200/3 = 275/3, S5 = 20+50 = 70 = 210/3.
	// out[k] = (S(k-1) + 2*S(k))/3: 1550/9, 950/9, 695/9.
	in := []float64{1, 2, 3, 4, 5, 6}
	out, err := COPPOCK(in, 2, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "COPPOCK(1..6,2,1,2)", out, []float64{nan, nan, nan, 1550.0 / 9, 950.0 / 9, 695.0 / 9}, 1e-13)

	// Symmetric in the two ROC periods.
	out2, _ := COPPOCK(in, 2, 2, 1)
	assertReal(t, "COPPOCK swapped ROC periods", out2, out, 0)

	// Zero guard: a zero base yields 0 for that term, never Inf.
	out, _ = COPPOCK([]float64{0, 5, 10}, 1, 1, 1)
	assertReal(t, "COPPOCK zero base", out, []float64{nan, 0, 200}, 0)
	out, _ = COPPOCK(fixtures(40)[6].Real(), 3, 2, 4) // all zeros
	for i := 6; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("zeros: [%d]=%g", i, out[i])
		}
	}

	// A constant (non-zero) series has zero momentum.
	out, _ = COPPOCK(fixtures(40)[5].Real(), 4, 3, 5)
	for i := 8; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// A geometric series has constant ROCs, so the curve is constant:
	// ROC(1)=100, ROC(2)=300 on doubling prices.
	geo := make([]float64, 20)
	for i := range geo {
		geo[i] = math.Pow(2, float64(i))
	}
	out, _ = COPPOCK(geo, 5, 1, 2)
	for i := 6; i < len(out); i++ {
		if !closeEnough(out[i], 400, 1e-12) {
			t.Fatalf("geometric: [%d]=%g, want 400", i, out[i])
		}
	}

	// WMA period 1 is plain ROC+ROC (C's identity arm).
	s := fixtures(100)[0].Real()
	out, _ = COPPOCK(s, 1, 11, 14)
	assertReal(t, "COPPOCK w=1", out, coppockNaive(s, 1, 11, 14), 1e-12)

	// Long series across several 8*w re-anchors matches the direct formula.
	for _, ps := range [][3]int{{2, 1, 2}, {3, 5, 2}, {10, 11, 14}, {60, 3, 7}} {
		for _, fx := range fixtures(1000) {
			if fx.Name == "negative" || fx.Name == "zeros" {
				continue // sign changes make ROC terms huge relative to the sum
			}
			out, _ := COPPOCK(fx.Real(), ps[0], ps[1], ps[2])
			assertReal(t, lbl("COPPOCK naive ", fx.Name, " p=", ps), out, coppockNaive(fx.Real(), ps[0], ps[1], ps[2]), 1e-9)
		}
	}
}

func TestCOPPOCKRangeForm(t *testing.T) {
	in := fixtures(60)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := coppockRange(-1, 5, in, 10, 11, 14, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := coppockRange(5, 4, in, 10, 11, 14, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := coppockRange(0, -1, in, 10, 11, 14, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback (23): no output, no error.
	beg, nb, err := coppockRange(0, 22, in, 10, 11, 14, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// First bar after the lookback.
	beg, nb, err = coppockRange(0, 23, in, 10, 11, 14, out)
	if err != nil || beg != 23 || nb != 1 {
		t.Errorf("lookback edge: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Too-short buffers.
	if _, _, err := coppockRange(0, 40, in[:30], 10, 11, 14, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := coppockRange(0, 40, in, 10, 11, 14, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Sub-range values equal the full-series values.
	full, _ := COPPOCK(in, 3, 4, 6)
	beg, nb, err = coppockRange(20, 45, in, 3, 4, 6, out)
	if err != nil || beg != 20 || nb != 26 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[20:46], 1e-9) // recurrence restarts, so not bit-identical
	// In-place computation is allowed.
	for _, ps := range [][3]int{{10, 11, 14}, {1, 3, 3}} {
		buf := append([]float64(nil), in...)
		want, _ := COPPOCK(in, ps[0], ps[1], ps[2])
		beg, nb, _ = coppockRange(0, len(buf)-1, buf, ps[0], ps[1], ps[2], buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place ", ps), buf, want, 0)
	}
}
