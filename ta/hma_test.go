package ta

import (
	"errors"
	"math"
	"testing"
)

func TestHMAParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := HMA(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := HMALookback(p); lb != -1 {
			t.Errorf("HMALookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := hmaRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 3, 4, 100000} {
		if _, err := HMA(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
	}
	if out, err := HMA(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := HMA([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestHMALookbackValues(t *testing.T) {
	// lookback = (n-1) + (Integer(sqrt(n))-1).
	cases := map[int]int{1: 0, 2: 1, 3: 2, 4: 4, 8: 8, 9: 10, 15: 16, 16: 18, 20: 22, 100: 108, 100000: 99999 + 315}
	for p, want := range cases {
		if got := HMALookback(p); got != want {
			t.Errorf("HMALookback(%d) = %d, want %d", p, got, want)
		}
	}
}

func TestHMALengths(t *testing.T) {
	for _, period := range []int{1, 2, 3, 4, 5, 9, 20, 50} {
		lb := HMALookback(period)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := HMA(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("HMA ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

// hmaCompose builds HMA from three WMA calls, the formula in hma.md.
func hmaCompose(t *testing.T, in []float64, p int) []float64 {
	t.Helper()
	full, err := WMA(in, p)
	if err != nil {
		t.Fatal(err)
	}
	half, err := WMA(in, p/2)
	if err != nil {
		t.Fatal(err)
	}
	sq := int(math.Sqrt(float64(p)))
	lbFull := p - 1
	diff := make([]float64, len(in)-lbFull)
	for i := range diff {
		diff[i] = 2.0*half[i+lbFull] - full[i+lbFull]
	}
	tmp := make([]float64, len(diff))
	beg, nb, err := wmaRange(0, len(diff)-1, diff, sq, tmp)
	if err != nil {
		t.Fatal(err)
	}
	out := nanSlice(len(in))
	for k := 0; k < nb; k++ {
		out[lbFull+beg+k] = tmp[k]
	}
	return out
}

func TestHMAKnownValues(t *testing.T) {
	nan := math.NaN()
	// p=4: half=2, sqrt=2. Input 1,2,4,8,16.
	// WMA4 at i=3: (1+4+12+32)/10 = 4.9;  i=4: (2+8+24+64)/10 = 9.8
	// WMA2 at i=3: (4+16)/3 = 20/3;        i=4: (8+32)/3 = 40/3
	// diff3 = 40/3-4.9, diff4 = 80/3-9.8
	// HMA[4] = (diff3 + 2*diff4)/3
	in := []float64{1, 2, 4, 8, 16}
	out, err := HMA(in, 4)
	if err != nil {
		t.Fatal(err)
	}
	d3 := 2.0*(20.0/3) - 4.9
	d4 := 2.0*(40.0/3) - 9.8
	assertReal(t, "HMA(p=4)", out, []float64{nan, nan, nan, nan, (d3 + 2*d4) / 3}, 1e-12)

	// p=2 degenerate: HMA = 2*x - WMA2(x). Input 1,2,4: WMA2[1]=5/3, [2]=10/3.
	out, _ = HMA([]float64{1, 2, 4}, 2)
	assertReal(t, "HMA(p=2)", out, []float64{nan, 4 - 5.0/3, 8 - 10.0/3}, 1e-12)

	// Period 1 is the identity.
	src := fixtures(30)[0].Real()
	out, _ = HMA(src, 1)
	assertReal(t, "HMA period 1", out, src, 0)

	// A constant series averages to itself.
	c := fixtures(200)[5].Real()
	for _, p := range []int{2, 3, 4, 9, 20} {
		out, _ = HMA(c, p)
		for i := HMALookback(p); i < len(out); i++ {
			if math.Abs(out[i]-100) > 1e-9 {
				t.Fatalf("constant p=%d: [%d]=%.17g", p, i, out[i])
			}
		}
	}

	// On a straight line a period-m WMA lags by (m-1)/3 bars, so HMA lags by
	// (2*(h-1) - (n-1) + (s-1))/3 bars with h = n/2, s = Integer(sqrt(n)):
	// zero for n = 4, 2/3 for n = 16, 4/3 for n = 20.
	up := make([]float64, 300)
	for i := range up {
		up[i] = 10 + 0.5*float64(i)
	}
	for _, p := range []int{4, 16, 20} {
		h, sq := p/2, int(math.Sqrt(float64(p)))
		lag := float64(2*(h-1)-(p-1)+(sq-1)) / 3
		out, _ = HMA(up, p)
		for i := HMALookback(p); i < len(out); i++ {
			want := 10 + 0.5*(float64(i)-lag)
			if math.Abs(out[i]-want) > 1e-9*want {
				t.Fatalf("linear p=%d: [%d]=%.17g want %.17g", p, i, out[i], want)
			}
		}
	}

	// Matches the three-WMA composition, including across re-anchors.
	for _, p := range []int{2, 3, 4, 5, 9, 16, 20, 63} {
		for _, s := range fixtures(1500) {
			got, _ := HMA(s.Real(), p)
			assertReal(t, lbl("HMA compose ", s.Name, " p=", p), got, hmaCompose(t, s.Real(), p), relTol)
		}
	}
}

func TestHMARangeForm(t *testing.T) {
	in := fixtures(60)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := hmaRange(-1, 5, in, 4, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := hmaRange(5, 4, in, 4, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	for _, p := range []int{2, 4, 20} {
		lb := HMALookback(p)
		beg, nb, err := hmaRange(0, lb-1, in, p, out)
		if err != nil || beg != 0 || nb != 0 {
			t.Errorf("inside lookback p=%d: beg=%d nb=%d err=%v", p, beg, nb, err)
		}
	}
	// Too-short buffers.
	if _, _, err := hmaRange(0, 10, in[:5], 4, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := hmaRange(0, 10, in, 4, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Sub-range values match the full series (rolling sums start from a
	// different anchor, so only to rounding).
	for _, p := range []int{1, 3, 9} {
		full, _ := HMA(in, p)
		beg, nb, err := hmaRange(30, 40, in, p, out)
		if err != nil || beg != 30 || nb != 11 {
			t.Fatalf("sub-range p=%d: beg=%d nb=%d err=%v", p, beg, nb, err)
		}
		assertReal(t, lbl("sub-range p=", p), out[:nb], full[30:41], relTol)
	}
	// In-place computation is allowed.
	for _, p := range []int{1, 2, 3, 4, 9, 25} {
		buf := append([]float64(nil), in...)
		want, _ := HMA(in, p)
		beg, nb, _ := hmaRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
