package ta

import (
	"errors"
	"math"
	"testing"
)

func TestWILLRParams(t *testing.T) {
	s := fixtures(50)[0]
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := WILLR(s.H, s.L, s.C, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := WILLRLookback(p); lb != -1 {
			t.Errorf("WILLRLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := willrRange(0, 49, s.H, s.L, s.C, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 14, 100000} {
		if _, err := WILLR(s.H, s.L, s.C, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := WILLRLookback(p); lb != p-1 {
			t.Errorf("WILLRLookback(%d) = %d", p, lb)
		}
	}
	if _, err := WILLR(s.H, s.L[:49], s.C, 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, err := WILLR(s.H, s.L, nil, 14); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil close with non-empty others: err = %v, want ErrBadParam", err)
	}
	if out, err := WILLR(nil, nil, nil, 14); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := WILLR([]float64{}, []float64{}, []float64{}, 14); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestWILLRLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 14, 30} {
		lb := WILLRLookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := WILLR(s.H, s.L, s.C, period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("WILLR ", s.Name, " n=", n, " p=", period), out, lb)
				for i := lb; i < n; i++ {
					if out[i] < -100 || out[i] > 0 {
						t.Fatalf("%s n=%d p=%d: [%d]=%g outside [-100,0]", s.Name, n, period, i, out[i])
					}
				}
			}
		}
	}
}

// willrNaive is the formula from willr.md, computed directly per bar.
func willrNaive(h, l, c []float64, p int) []float64 {
	out := make([]float64, len(c))
	for i := range out {
		if i < p-1 {
			out[i] = math.NaN()
			continue
		}
		hh, ll := h[i-p+1], l[i-p+1]
		for j := i - p + 2; j <= i; j++ {
			hh = cMax(hh, h[j])
			ll = cMin(ll, l[j])
		}
		if isZeroScaled(hh-ll, math.Abs(hh)+math.Abs(ll)) {
			out[i] = 0
			continue
		}
		v := (hh - c[i]) / (hh - ll) * -100
		out[i] = cMin(0, cMax(-100, v))
	}
	return out
}

func TestWILLRKnownValues(t *testing.T) {
	nan := math.NaN()
	h := []float64{10, 12, 11, 14, 13}
	l := []float64{8, 9, 7, 10, 11}
	c := []float64{9, 11, 8, 14, 11}
	// p=3: bar2 HH=12 LL=7 -> (12-8)/5*-100 = -80
	//      bar3 HH=14 LL=7 -> (14-14)/7 = 0 (close at the high)
	//      bar4 HH=14 LL=7 -> (14-11)/7*-100 = -42.857142857142854
	out, err := WILLR(h, l, c, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "WILLR hand", out, []float64{nan, nan, -80, 0, (14.0 - 11.0) / 7.0 * -100}, 1e-15)

	// Close on the period low is exactly -100.
	out, _ = WILLR([]float64{5, 6}, []float64{1, 2}, []float64{3, 1}, 2)
	assertReal(t, "WILLR at low", out, []float64{nan, -100}, 0)

	// Constant series: flat range -> 0.
	s := fixtures(40)[5]
	out, _ = WILLR(s.H, s.L, s.C, 7)
	for i := 6; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}

	// Close outside its bar is clamped to [-100, 0].
	out, _ = WILLR([]float64{5, 6}, []float64{1, 2}, []float64{3, 10}, 2)
	assertReal(t, "WILLR clamp above", out, []float64{nan, 0}, 0)
	out, _ = WILLR([]float64{5, 6}, []float64{1, 2}, []float64{3, -4}, 2)
	assertReal(t, "WILLR clamp below", out, []float64{nan, -100}, 0)

	// The block scan agrees with the naive formula on every fixture.
	for _, p := range []int{2, 3, 7, 14, 50} {
		for _, n := range []int{p - 1, p, p + 1, 2 * p, 2*p + 1, 3*p - 1, 257} {
			for _, s := range fixtures(n) {
				out, _ := WILLR(s.H, s.L, s.C, p)
				assertReal(t, lbl("WILLR naive ", s.Name, " n=", n, " p=", p), out, willrNaive(s.H, s.L, s.C, p), 1e-12)
			}
		}
	}
}

func TestWILLRRangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := make([]float64, 30)
	if _, _, err := willrRange(-1, 5, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := willrRange(5, 4, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := willrRange(0, -1, s.H, s.L, s.C, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	beg, nb, err := willrRange(0, 1, s.H, s.L, s.C, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if _, _, err := willrRange(0, 10, s.H, s.L[:5], s.C, 3, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := willrRange(0, 10, s.H, s.L, s.C, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Sub-ranges match the full series.
	full, _ := WILLR(s.H, s.L, s.C, 4)
	for _, rg := range [][2]int{{0, 29}, {3, 3}, {5, 17}, {29, 29}, {0, 3}} {
		o := make([]float64, 30)
		beg, nb, err := willrRange(rg[0], rg[1], s.H, s.L, s.C, 4, o)
		if err != nil {
			t.Fatal(err)
		}
		assertReal(t, lbl("WILLR sub ", rg), o[:nb], full[beg:beg+nb], 0)
	}
	// In-place: output aliasing the close input.
	buf := append([]float64(nil), s.C...)
	beg, nb, _ = willrRange(0, len(buf)-1, s.H, s.L, buf, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place close", buf, full, 0)
	// In-place: output aliasing the high input.
	buf = append([]float64(nil), s.H...)
	beg, nb, _ = willrRange(0, len(buf)-1, buf, s.L, s.C, 4, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place high", buf, full, 0)
}
