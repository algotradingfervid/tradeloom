package ta

import (
	"errors"
	"math"
	"testing"
)

func TestROCPParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, err := ROCP(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("period %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := ROCPLookback(p); lb != -1 {
			t.Errorf("ROCPLookback(%d) = %d, want -1", p, lb)
		}
		if _, _, err := rocpRange(0, 49, in, p, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("range period %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 10, 100000} {
		if _, err := ROCP(in, p); err != nil {
			t.Errorf("period %d: unexpected err %v", p, err)
		}
		if lb := ROCPLookback(p); lb != p {
			t.Errorf("ROCPLookback(%d) = %d, want %d", p, lb, p)
		}
	}
	if out, err := ROCP(nil, 5); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := ROCP([]float64{}, 5); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	// Bad parameter is reported even for empty input.
	if _, err := ROCP(nil, 0); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil input bad period: err = %v", err)
	}
}

func TestROCPLengths(t *testing.T) {
	for _, period := range []int{1, 2, 5, 10, 30} {
		lb := ROCPLookback(period)
		if lb != period {
			t.Fatalf("ROCPLookback(%d) = %d", period, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				out, err := ROCP(s.Real(), period)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, period, err)
				}
				if len(out) != n {
					t.Fatalf("%s n=%d p=%d: len(out)=%d", s.Name, n, period, len(out))
				}
				assertLeadingNaN(t, lbl("ROCP ", s.Name, " n=", n, " p=", period), out, lb)
			}
		}
	}
}

func TestROCPKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{10, 20, 15, 30, 0, 5}
	out, err := ROCP(in, 1)
	if err != nil {
		t.Fatal(err)
	}
	// (20-10)/10=1, (15-20)/20=-0.25, (30-15)/15=1, (0-30)/30=-1, prev 0 -> 0.
	assertReal(t, "ROCP p=1", out, []float64{nan, 1, -0.25, 1, -1, 0}, 0)

	out, _ = ROCP(in, 2)
	// (15-10)/10=0.5, (30-20)/20=0.5, (0-15)/15=-1, (5-30)/30=-25/30.
	assertReal(t, "ROCP p=2", out, []float64{nan, nan, 0.5, 0.5, -1, (5.0 - 30.0) / 30.0}, 0)

	// Constant input has zero rate of change.
	c := fixtures(40)[5].Real()
	out, _ = ROCP(c, 7)
	for i := 7; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("constant: [%d]=%g", i, out[i])
		}
	}
	// All-zero input: division guarded, output 0.
	z := fixtures(40)[6].Real()
	out, _ = ROCP(z, 3)
	for i := 3; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("zeros: [%d]=%g", i, out[i])
		}
	}
	// Strictly rising positive series: strictly positive ROCP; falling: negative.
	up := fixtures(60)[7].Real()
	out, _ = ROCP(up, 5)
	for i := 5; i < len(out); i++ {
		if !(out[i] > 0) {
			t.Fatalf("up: [%d]=%g", i, out[i])
		}
	}
	down := fixtures(60)[8].Real()
	out, _ = ROCP(down, 5)
	for i := 5; i < len(out); i++ {
		if !(out[i] < 0) {
			t.Fatalf("down: [%d]=%g", i, out[i])
		}
	}
	// Period larger than the series: all NaN.
	out, _ = ROCP(in, 6)
	assertLeadingNaN(t, "period==len", out, 6)
}

func TestROCPRangeForm(t *testing.T) {
	in := fixtures(30)[0].Real()
	out := make([]float64, len(in))
	if _, _, err := rocpRange(-1, 5, in, 3, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := rocpRange(5, 4, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := rocpRange(0, -1, in, 3, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := rocpRange(0, 2, in, 3, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Start inside the lookback is moved up to the lookback.
	beg, nb, err = rocpRange(1, 10, in, 3, out)
	if err != nil || beg != 3 || nb != 8 {
		t.Errorf("start in lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := rocpRange(0, 10, in[:5], 3, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v, want ErrBadParam", err)
	}
	if _, _, err := rocpRange(0, 10, in, 3, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v, want ErrBadParam", err)
	}
	// In-place computation is allowed.
	for _, p := range []int{1, 4} {
		buf := append([]float64(nil), in...)
		want, _ := ROCP(in, p)
		beg, nb, _ = rocpRange(0, len(buf)-1, buf, p, buf)
		alignReal(buf, beg, nb)
		assertReal(t, lbl("in-place p=", p), buf, want, 0)
	}
}
