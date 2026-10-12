package ta

import (
	"errors"
	"math"
	"testing"
)

func TestERParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{1, 0, -1, 100001, math.MinInt32} {
		if _, err := ER(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := ERLookback(p); lb != -1 {
			t.Errorf("ERLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := erRange(0, len(in)-1, in, p, make([]float64, len(in))); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{2, 3, 10, 100000} {
		if _, err := ER(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := ERLookback(p); lb != p {
			t.Errorf("ERLookback(%d) = %d, want %d", p, lb, p)
		}
	}
	if out, err := ER(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := ER([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
}

func TestERLengths(t *testing.T) {
	for _, period := range []int{2, 3, 5, 10, 30} {
		lb := ERLookback(period)
		if lb != period {
			t.Fatalf("ERLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := ER(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("ER ", s.Name, " n=", n, " p=", period), out, lb)
				// Bounds: ER is a hard [0, 1].
				for i := lb; i < n; i++ {
					if !(out[i] >= 0 && out[i] <= 1) {
						t.Fatalf("%s n=%d p=%d: out[%d]=%g outside [0,1]", s.Name, n, period, i, out[i])
					}
				}
			}
		}
	}
}

func TestERKnownValues(t *testing.T) {
	nan := math.NaN()
	// P=2. t2: |3-1|/(1+1)=1; t3: |2-2|/(1+1)=0; t4: |4-3|/(1+2)=1/3;
	// t5: |4-2|/(2+0)=1; t6: flat window 0/0 -> 1.
	in := []float64{1, 2, 3, 2, 4, 4, 4}
	out, err := ER(in, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "ER known", out, []float64{nan, nan, 1, 0, 1.0 / 3, 1, 1}, 1e-15)

	// Down moves: |3-5|/2 = 1; |2-3|/(2+1) = 1/3.
	out, _ = ER([]float64{5, 4, 3, 1, 2}, 2)
	assertReal(t, "ER down", out, []float64{nan, nan, 1, 1, 1.0 / 3}, 1e-15)

	// Constant series: flat window reads as perfectly efficient (1.0).
	c := fixtures(40)[5].Real()
	out, _ = ER(c, 7)
	for i := 7; i < len(out); i++ {
		if out[i] != 1 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}
	// Strictly monotone trends are perfectly efficient.
	for _, s := range []series{fixtures(60)[7], fixtures(60)[8]} {
		out, _ = ER(s.Real(), 10)
		for i := 10; i < len(out); i++ {
			if out[i] != 1 {
				t.Fatalf("%s: [%d]=%.17g, want 1", s.Name, i, out[i])
			}
		}
	}
	// Zigzag with even period: the net move is zero.
	z := []float64{1, 2, 1, 2, 1, 2, 1}
	out, _ = ER(z, 2)
	assertReal(t, "ER zigzag", out, []float64{nan, nan, 0, 0, 0, 0, 0}, 0)

	// Flat stretch after movement: nullRun purge makes it exactly 1.
	f := []float64{1.1, 2.7, 0.3, 5.9, 5.9, 5.9, 5.9}
	out, _ = ER(f, 3)
	if out[6] != 1 {
		t.Errorf("flat after movement: %.17g, want 1", out[6])
	}
}

func TestERRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := erRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := erRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := erRange(0, 2, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := erRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := erRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// Sub-range starts later and matches the full computation.
	full, _ := ER(in, 4)
	beg, nb, err = erRange(10, 20, in, 4, out)
	if err != nil || beg != 10 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "sub-range", out[:nb], full[10:21], 1e-12)
	// In-place computation is allowed.
	for _, p := range []int{2, 4} {
		buf := append([]float64(nil), in...)
		want, _ := ER(in, p)
		beg, nb, _ = erRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
