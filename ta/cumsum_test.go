package ta

import (
	"errors"
	"math"
	"testing"
)

func TestCUMSUMParams(t *testing.T) {
	if lb := CUMSUMLookback(); lb != 0 {
		t.Errorf("CUMSUMLookback() = %d, want 0", lb)
	}
	if out, err := CUMSUM(nil); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := CUMSUM([]float64{}); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	in := fixtures(20)[0].Real()
	if _, _, err := cumsumRange(0, 10, in[:5], make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := cumsumRange(0, 10, in, make([]float64, 10)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := cumsumRange(0, 10, nil, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := cumsumRange(0, 10, in, nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil output buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := cumsumRange(5, 10, in, make([]float64, 6)); err != nil {
		t.Errorf("exact output buffer: err = %v", err)
	}
}

func TestCUMSUMLengths(t *testing.T) {
	lb := CUMSUMLookback()
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			out, err := CUMSUM(s.Real())
			if err != nil {
				t.Fatalf("%s n=%d: %v", s.Name, n, err)
			}
			if len(out) != n {
				t.Fatalf("%s n=%d: len(out)=%d", s.Name, n, len(out))
			}
			// No lookback and finite inputs: no NaN anywhere.
			assertLeadingNaN(t, lbl("CUMSUM ", s.Name, " n=", n), out, lb)
		}
	}
}

func TestCUMSUMKnownValues(t *testing.T) {
	out, err := CUMSUM([]float64{1, 2, 3, 4, 5, 6})
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "CUMSUM(1..6)", out, []float64{1, 3, 6, 10, 15, 21}, 0)

	out, _ = CUMSUM([]float64{5, -2, -3, 0.5, -0.5})
	assertReal(t, "CUMSUM signed", out, []float64{5, 3, 0, 0.5, 0}, 0)

	// A single bar is the identity.
	out, _ = CUMSUM([]float64{-7.25})
	assertReal(t, "CUMSUM single", out, []float64{-7.25}, 0)

	// Constant input c gives (i+1)*c exactly (small integers).
	c := fixtures(40)[5].Real()
	out, _ = CUMSUM(c)
	for i := range out {
		if out[i] != float64(i+1)*100 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}
	// All zeros stay zero.
	out, _ = CUMSUM(fixtures(40)[6].Real())
	for i := range out {
		if out[i] != 0 {
			t.Fatalf("zeros: [%d]=%g", i, out[i])
		}
	}

	// Uncompensated left-to-right accumulation, as C: 1e16 + 1 + 1 stays 1e16.
	out, _ = CUMSUM([]float64{1e16, 1, 1})
	assertReal(t, "CUMSUM uncompensated", out, []float64{1e16, 1e16, 1e16}, 0)

	// NaN poisons everything after it; infinities propagate.
	nan := math.NaN()
	out, _ = CUMSUM([]float64{1, nan, 2})
	assertReal(t, "CUMSUM NaN", out, []float64{1, nan, nan}, 0)
	out, _ = CUMSUM([]float64{1, math.Inf(1), 2, math.Inf(-1)})
	assertReal(t, "CUMSUM Inf", out, []float64{1, math.Inf(1), math.Inf(1), nan}, 0)

	// Property: differences of the output recover the input (exactly for
	// small integers), and the output equals a naive left-to-right sum.
	in := fixtures(500)[0].Real()
	out, _ = CUMSUM(in)
	sum := 0.0
	for i, v := range in {
		sum += v
		if out[i] != sum {
			t.Fatalf("naive sum: [%d] %g vs %g", i, out[i], sum)
		}
	}
	up := fixtures(100)[7].Real() // strictly rising positive: output strictly rising
	out, _ = CUMSUM(up)
	for i := 1; i < len(out); i++ {
		if out[i] <= out[i-1] {
			t.Fatalf("monotone: [%d] %g <= %g", i, out[i], out[i-1])
		}
	}
}

func TestCUMSUMRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := cumsumRange(-1, 5, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := cumsumRange(taMaxIdx+1, taMaxIdx+1, in, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start > max: %v", err)
	}
	if _, _, err := cumsumRange(5, 4, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := cumsumRange(0, -1, in, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Lookback is 0, so no range lies inside it; a single bar yields that bar.
	beg, nb, err := cumsumRange(7, 7, in, out)
	if err != nil || beg != 7 || nb != 1 || out[0] != in[7] {
		t.Errorf("single bar: beg=%d nb=%d err=%v out=%g", beg, nb, err, out[0])
	}
	// The accumulator re-seeds at startIdx (the anchor), as C.
	small := []float64{1, 2, 3, 4, 5, 6, 7, 8}
	o := make([]float64, len(small))
	beg, nb, err = cumsumRange(3, 7, small, o)
	if err != nil || beg != 3 || nb != 5 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	assertReal(t, "anchor re-seed", o[:nb], []float64{4, 9, 15, 22, 30}, 0)
	// In-place computation is allowed.
	buf := append([]float64(nil), in...)
	want, _ := CUMSUM(in)
	beg, nb, _ = cumsumRange(0, len(buf)-1, buf, buf)
	alignReal(buf, beg, nb)
	assertReal(t, "in-place", buf, want, 0)
}
