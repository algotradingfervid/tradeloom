package ta

import (
	"errors"
	"math"
	"testing"
)

func TestTRIXParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := TRIX(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := TRIXLookback(p); lb != -1 {
			t.Errorf("TRIXLookback(%d) = %d, want -1", p, lb)
		}
		out := make([]float64, len(in))
		if _, _, err := trixRange(0, len(in)-1, in, p, out); !errors.Is(err, ErrBadParam) {
			t.Errorf("trixRange period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 30, 100000} {
		if _, err := TRIX(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb, want := TRIXLookback(p), 3*(p-1)+1; lb != want {
			t.Errorf("TRIXLookback(%d) = %d, want %d", p, lb, want)
		}
	}
	if out, err := TRIX(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := TRIX([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	out, err := TRIX(in, 100000)
	if err != nil || len(out) != len(in) {
		t.Fatalf("p=100000: len=%d err=%v", len(out), err)
	}
	assertLeadingNaN(t, "TRIX p=100000", out, len(in))
}

func TestTRIXLengths(t *testing.T) {
	for _, period := range []int{1, 2, 3, 5, 30} {
		lb := TRIXLookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := TRIX(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("TRIX ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestTRIXKnownValues(t *testing.T) {
	nan := math.NaN()
	// Period 1: k = 1, every EMA is the input itself, so TRIX is the
	// 1-day percent ROC of the input.
	out, err := TRIX([]float64{1, 2, 4, 5, 4}, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "TRIX p=1", out, []float64{nan, 100, 100, 25, -20}, 1e-12)

	// Period 2 by hand, k = 2/3, lookback 4.
	// in = 2,4,8,8,8,8.
	// E1: seed (2+4)/2=3 at bar1; bar2: 3+(8-3)*2/3=19/3; bar3: 19/3+(8-19/3)*2/3=67/9; bar4: 67/9+(8-67/9)*2/3=211/27.
	// E2: seed (3+19/3)/2=14/3 at bar2; bar3: 14/3+(67/9-14/3)*2/3=176/27; bar4: 176/27+(211/27-176/27)*2/3=598/81.
	// E3: seed (14/3+176/27)/2=151/27 at bar3; bar4: 151/27+(598/81-151/27)*2/3=2555/243.. computed below.
	e1 := []float64{3, 19.0 / 3}
	e1 = append(e1, e1[1]+(8-e1[1])*2/3)
	e1 = append(e1, e1[2]+(8-e1[2])*2/3)
	e1 = append(e1, e1[3]+(8-e1[3])*2/3) // bar5
	e2 := []float64{(e1[0] + e1[1]) / 2}
	for _, v := range e1[2:] {
		e2 = append(e2, e2[len(e2)-1]+(v-e2[len(e2)-1])*2/3)
	}
	e3 := []float64{(e2[0] + e2[1]) / 2}
	for _, v := range e2[2:] {
		e3 = append(e3, e3[len(e3)-1]+(v-e3[len(e3)-1])*2/3)
	}
	// e3 holds bars 3,4,5.
	want := []float64{nan, nan, nan, nan, (e3[1]/e3[0] - 1) * 100, (e3[2]/e3[1] - 1) * 100}
	out, _ = TRIX([]float64{2, 4, 8, 8, 8, 8}, 2)
	assertReal(t, "TRIX p=2 hand", out, want, 1e-12)

	// Composition: TRIX == 100*(E3/E3[-1]-1) with E3 = EMA(EMA(EMA(x))).
	for _, p := range []int{2, 5, 14} {
		x := fixtures(200)[0].Real()
		lbE := EMALookback(p)
		a, _ := EMA(x, p)
		b, _ := EMA(a[lbE:], p)
		c, _ := EMA(b[lbE:], p)
		got, _ := TRIX(x, p)
		lb := TRIXLookback(p)
		for i := lb; i < len(x); i++ {
			j := i - 2*lbE
			w := (c[j]/c[j-1] - 1) * 100
			if !closeEnough(got[i], w, 1e-9) {
				t.Fatalf("composition p=%d [%d]: got %g want %g", p, i, got[i], w)
			}
		}
	}

	// Constant input: zero momentum. All-zero input: the zero-divisor rule gives 0.
	for _, s := range []series{fixtures(80)[5], fixtures(80)[6]} {
		out, _ = TRIX(s.Real(), 5)
		for i := TRIXLookback(5); i < len(out); i++ {
			if out[i] != 0 {
				t.Fatalf("%s: [%d]=%g, want 0", s.Name, i, out[i])
			}
		}
	}
	// Strictly rising positive series: positive TRIX.
	out, _ = TRIX(fixtures(80)[7].Real(), 5)
	for i := TRIXLookback(5); i < len(out); i++ {
		if !(out[i] > 0) {
			t.Fatalf("up: [%d]=%g, want > 0", i, out[i])
		}
	}
}

func TestTRIXRangeForm(t *testing.T) {
	in := fixtures(60)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := trixRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := trixRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := trixRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Lookback for p=3 is 7: range entirely inside it.
	beg, nb, err := trixRange(0, 6, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if _, _, err := trixRange(0, 50, in[:20], 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	if _, _, err := trixRange(0, 50, in, 3, out[:10]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output: %v", err)
	}
	// A range starting at or before the lookback seeds at bar 0, so it
	// matches the full series. (Later starts re-seed the EMAs at
	// startIdx-lookback, as in C; the oracle tests cover those.)
	full, _ := TRIX(in, 3)
	beg, nb, err = trixRange(2, 40, in, 3, out)
	if err != nil || beg != 7 || nb != 34 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[7:41], 0)
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	beg, nb, _ = trixRange(0, len(buf)-1, buf, 4, buf)
	alignReal(buf, beg, nb)
	want, _ := TRIX(in, 4)
	assertReal(t, "in-place", buf, want, 0)
}
