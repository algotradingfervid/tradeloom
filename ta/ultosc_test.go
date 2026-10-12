package ta

import (
	"errors"
	"math"
	"testing"
)

func TestULTOSCParams(t *testing.T) {
	s := fixtures(50)[0]
	bad := []int{0, -1, 100001, math.MinInt32}
	for _, p := range bad {
		for k := 0; k < 3; k++ {
			ps := [3]int{7, 14, 28}
			ps[k] = p
			if _, err := ULTOSC(s.H, s.L, s.C, ps[0], ps[1], ps[2]); !errors.Is(err, ErrBadParam) {
				t.Errorf("periods %v: err = %v, want ErrBadParam", ps, err)
			}
			if lb := ULTOSCLookback(ps[0], ps[1], ps[2]); lb != -1 {
				t.Errorf("ULTOSCLookback(%v) = %d, want -1", ps, lb)
			}
			out := make([]float64, len(s.C))
			if _, _, err := ultoscRange(0, len(s.C)-1, s.H, s.L, s.C, ps[0], ps[1], ps[2], out); !errors.Is(err, ErrBadParam) {
				t.Errorf("range periods %v: err = %v, want ErrBadParam", ps, err)
			}
		}
	}
	for _, p := range []int{1, 2, 100000} {
		for k := 0; k < 3; k++ {
			ps := [3]int{7, 14, 28}
			ps[k] = p
			if _, err := ULTOSC(s.H, s.L, s.C, ps[0], ps[1], ps[2]); err != nil {
				t.Errorf("periods %v: unexpected err %v", ps, err)
			}
		}
	}
	if _, err := ULTOSC(s.H, s.L[:49], s.C, 7, 14, 28); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v, want ErrBadParam", err)
	}
	if _, err := ULTOSC(s.H, s.L, nil, 7, 14, 28); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil close with non-empty others: err = %v, want ErrBadParam", err)
	}
	if out, err := ULTOSC(nil, nil, nil, 7, 14, 28); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := ULTOSC([]float64{}, []float64{}, []float64{}, 7, 14, 28); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestULTOSCLookbackValues(t *testing.T) {
	for _, c := range []struct{ p1, p2, p3, want int }{
		{7, 14, 28, 28}, {28, 14, 7, 28}, {1, 1, 1, 1}, {1, 100000, 5, 100000}, {3, 9, 2, 9},
	} {
		if lb := ULTOSCLookback(c.p1, c.p2, c.p3); lb != c.want {
			t.Errorf("ULTOSCLookback(%d,%d,%d) = %d, want %d", c.p1, c.p2, c.p3, lb, c.want)
		}
	}
}

func TestULTOSCLengths(t *testing.T) {
	for _, ps := range [][3]int{{1, 1, 1}, {1, 2, 3}, {7, 14, 28}, {28, 7, 14}, {5, 5, 5}, {2, 40, 33}} {
		lb := ULTOSCLookback(ps[0], ps[1], ps[2])
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := ULTOSC(s.H, s.L, s.C, ps[0], ps[1], ps[2])
				if err != nil {
					t.Fatalf("%s n=%d p=%v: %v", s.Name, n, ps, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%v: len(out)=%d", s.Name, n, ps, len(out))
				}
				assertLeadingNaN(t, lbl("ULTOSC ", s.Name, " n=", n, " p=", ps), out, lb)
			}
		}
	}
}

// ultoscNaive is the formula from ultosc.md with fresh sums per bar.
func ultoscNaive(h, l, c []float64, p1, p2, p3 int) []float64 {
	ps := []int{p1, p2, p3}
	// sort ascending
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 3; j++ {
			if ps[j] < ps[i] {
				ps[i], ps[j] = ps[j], ps[i]
			}
		}
	}
	lb := ps[2]
	out := make([]float64, len(c))
	for i := range out {
		out[i] = math.NaN()
		if i < lb {
			continue
		}
		avg := func(p int) float64 {
			var a, b float64
			for k := i - p + 1; k <= i; k++ {
				a += c[k] - math.Min(l[k], c[k-1])
				b += trueRange(h[k], l[k], c[k-1])
			}
			if b > 0 {
				return a / b
			}
			return 0
		}
		out[i] = 100 * (4*avg(ps[0]) + 2*avg(ps[1]) + avg(ps[2])) / 7
	}
	return out
}

func TestULTOSCKnownValues(t *testing.T) {
	nan := math.NaN()
	// Hand-checked: bar 1 has trueLow = min(9, 9) = 9, BP = 11-9 = 2,
	// TR = max(12-9, |9-12|, |9-9|) = 3, so every window gives 2/3 and
	// ULTOSC = 100 * (4+2+1)*(2/3) / 7 = 66.67.
	h := []float64{10, 12}
	l := []float64{8, 9}
	c := []float64{9, 11}
	out, err := ULTOSC(h, l, c, 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "ULTOSC hand", out, []float64{nan, 100.0 * (7 * (2.0 / 3.0)) / 7}, 1e-12)

	// Two-bar windows, periods (1,2,2): bar1 BP=2 TR=3, bar2 (H=13 L=10 C=10,
	// prevC=11): trueLow=10, BP=0, TR=max(3,2,1)=3.
	// avg1 = 0/3 = 0; avg2 = 2/6 twice -> 100*(0 + 2/3 + 1/3)/7 = 100/7.
	h = []float64{10, 12, 13}
	l = []float64{8, 9, 10}
	c = []float64{9, 11, 10}
	out, _ = ULTOSC(h, l, c, 2, 1, 2)
	assertReal(t, "ULTOSC hand 2", out, []float64{nan, nan, 100.0 / 7}, 1e-12)

	// Matches the naive formula on random data, and period order does not matter.
	for _, s := range fixtures(300)[:5] {
		want := ultoscNaive(s.H, s.L, s.C, 7, 14, 28)
		for _, ps := range [][3]int{{7, 14, 28}, {28, 14, 7}, {14, 28, 7}} {
			got, _ := ULTOSC(s.H, s.L, s.C, ps[0], ps[1], ps[2])
			assertReal(t, lbl("ULTOSC naive ", s.Name, ps), got, want, 1e-9)
		}
	}

	// Bounds: valid bars give values in [0, 100].
	for _, s := range fixtures(500) {
		if s.Name == "negative" {
			continue
		}
		out, _ := ULTOSC(s.H, s.L, s.C, 7, 14, 28)
		for i := 28; i < len(out); i++ {
			if out[i] < -1e-9 || out[i] > 100+1e-9 {
				t.Fatalf("%s: [%d]=%g outside [0,100]", s.Name, i, out[i])
			}
		}
	}

	// Constant series: zero true range in every window gives exactly 0.
	for _, s := range []series{constant("c", 60, 100), constant("z", 60, 0)} {
		out, _ := ULTOSC(s.H, s.L, s.C, 7, 14, 28)
		for i := 28; i < len(out); i++ {
			if out[i] != 0 {
				t.Fatalf("%s: [%d]=%g, want 0", s.Name, i, out[i])
			}
		}
	}

	// A series that goes flat after moving: once the longest window is
	// entirely flat the output is exactly 0 (residue is reseeded).
	s := fixtures(100)[0]
	h, l, c = append([]float64(nil), s.H...), append([]float64(nil), s.L...), append([]float64(nil), s.C...)
	for i := 50; i < 100; i++ {
		h[i], l[i], c[i] = c[49], c[49], c[49]
	}
	out, _ = ULTOSC(h, l, c, 3, 5, 10)
	for i := 60; i < 100; i++ {
		if out[i] != 0 {
			t.Fatalf("flat tail: [%d]=%g, want 0", i, out[i])
		}
	}
}

func TestULTOSCRangeForm(t *testing.T) {
	s := fixtures(60)[0]
	out := make([]float64, len(s.C))
	if _, _, err := ultoscRange(-1, 5, s.H, s.L, s.C, 7, 14, 28, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := ultoscRange(5, 4, s.H, s.L, s.C, 7, 14, 28, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := ultoscRange(0, -1, s.H, s.L, s.C, 7, 14, 28, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	beg, nb, err := ultoscRange(0, 27, s.H, s.L, s.C, 7, 14, 28, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if _, _, err := ultoscRange(0, 40, s.H, s.L[:10], s.C, 7, 14, 28, out); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := ultoscRange(0, 40, s.H, s.L, s.C, 7, 14, 28, out[:5]); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Sub-range matches the full computation.
	full, _ := ULTOSC(s.H, s.L, s.C, 7, 14, 28)
	beg, nb, err = ultoscRange(30, 50, s.H, s.L, s.C, 7, 14, 28, out)
	if err != nil || beg != 30 || nb != 21 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[30:51], 1e-12)
	// In-place: output aliases inClose.
	cbuf := append([]float64(nil), s.C...)
	beg, nb, _ = ultoscRange(0, len(cbuf)-1, s.H, s.L, cbuf, 7, 14, 28, cbuf)
	alignReal(cbuf, beg, nb)
	assertReal(t, "in-place", cbuf, full, 0)
}
