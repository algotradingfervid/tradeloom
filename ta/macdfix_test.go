package ta

import (
	"errors"
	"math"
	"testing"
)

func TestMACDFIXParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	for _, p := range []int{0, -1, 100001, math.MinInt32, math.MaxInt32} {
		if _, _, _, err := MACDFIX(in, p); !errors.Is(err, ErrBadParam) {
			t.Errorf("signal %d: err = %v, want ErrBadParam", p, err)
		}
		if lb := MACDFIXLookback(p); lb != -1 {
			t.Errorf("MACDFIXLookback(%d) = %d, want -1", p, lb)
		}
		o1, o2, o3 := make([]float64, len(in)), make([]float64, len(in)), make([]float64, len(in))
		if _, _, err := macdfixRange(0, len(in)-1, in, p, o1, o2, o3); !errors.Is(err, ErrBadParam) {
			t.Errorf("range signal %d: err = %v, want ErrBadParam", p, err)
		}
	}
	for _, p := range []int{1, 2, 9, 100000} {
		if _, _, _, err := MACDFIX(in, p); err != nil {
			t.Errorf("signal %d: unexpected err %v", p, err)
		}
	}
	for _, in := range [][]float64{nil, {}} {
		m, s, h, err := MACDFIX(in, 9)
		if err != nil || len(m) != 0 || len(s) != 0 || len(h) != 0 {
			t.Errorf("empty input: %v %v %v err=%v, want empty and nil", m, s, h, err)
		}
	}
}

func TestMACDFIXLookbackValues(t *testing.T) {
	for _, c := range []struct{ sig, want int }{{1, 25}, {2, 26}, {9, 33}, {100000, 100024}} {
		if lb := MACDFIXLookback(c.sig); lb != c.want {
			t.Errorf("MACDFIXLookback(%d) = %d, want %d", c.sig, lb, c.want)
		}
	}
}

func TestMACDFIXLengths(t *testing.T) {
	for _, p := range []int{1, 2, 9, 30} {
		lb := MACDFIXLookback(p)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				m, sg, h, err := MACDFIX(s.Real(), p)
				if err != nil {
					t.Fatalf("%s n=%d p=%d: %v", s.Name, n, p, err)
				}
				if len(m) != n || len(sg) != n || len(h) != n {
					t.Fatalf("%s n=%d p=%d: lens %d %d %d", s.Name, n, p, len(m), len(sg), len(h))
				}
				tag := lbl("MACDFIX ", s.Name, " n=", n, " p=", p)
				assertLeadingNaN(t, tag+" macd", m, lb)
				assertLeadingNaN(t, tag+" signal", sg, lb)
				assertLeadingNaN(t, tag+" hist", h, lb)
			}
		}
	}
}

func TestMACDFIXKnownValues(t *testing.T) {
	// Hand-checked: 26 zeros seed both EMAs with 0 at bar 25. At bar 26 the
	// input is 100: fast = 0.15*100 = 15, slow = 0.075*100 = 7.5, MACD = 7.5.
	in := make([]float64, 27)
	in[26] = 100
	m, s, h, err := MACDFIX(in, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !closeEnough(m[25], 0, 0) || !closeEnough(m[26], 7.5, 1e-14) || s[26] != m[26] || h[26] != 0 || !math.IsNaN(m[24]) {
		t.Errorf("signal 1: macd %v signal %v hist %v", m[24:], s[24:], h[24:])
	}
	// Signal 2: seeded with the mean of the MACD values at bars 25 and 26.
	m, s, h, _ = MACDFIX(in, 2)
	if !math.IsNaN(m[25]) || !closeEnough(m[26], 7.5, 1e-14) || !closeEnough(s[26], 3.75, 1e-14) || !closeEnough(h[26], 3.75, 1e-14) {
		t.Errorf("signal 2: macd %v signal %v hist %v", m[25:], s[25:], h[25:])
	}

	// Linear input: an EMA with factor k lags by (1-k)/k, so the MACD line
	// converges to 0.925/0.075 - 0.85/0.15 = 20/3.
	lin := make([]float64, 600)
	for i := range lin {
		lin[i] = float64(i)
	}
	m, s, h, _ = MACDFIX(lin, 5)
	last := len(lin) - 1
	if !closeEnough(m[last], 20.0/3, 1e-9) || !closeEnough(s[last], 20.0/3, 1e-9) || !closeEnough(h[last], 0, 1e-9) {
		t.Errorf("linear: %g %g %g", m[last], s[last], h[last])
	}

	// Constant input: MACD, signal and histogram are exactly 0.
	c := fixtures(60)[5].Real()
	m, s, h, _ = MACDFIX(c, 9)
	for i := 33; i < len(c); i++ {
		if m[i] != 0 || s[i] != 0 || h[i] != 0 {
			t.Fatalf("constant [%d]: %g %g %g", i, m[i], s[i], h[i])
		}
	}

	// hist = macd - signal; signal 1 is the MACD line itself.
	w := fixtures(200)[0].Real()
	m, s, h, _ = MACDFIX(w, 9)
	for i := 33; i < len(w); i++ {
		if h[i] != m[i]-s[i] {
			t.Fatalf("hist [%d] %g != %g-%g", i, h[i], m[i], s[i])
		}
	}
	m1, s1, h1, _ := MACDFIX(w, 1)
	for i := 25; i < len(w); i++ {
		if s1[i] != m1[i] || h1[i] != 0 {
			t.Fatalf("signal 1 [%d]: macd %g signal %g hist %g", i, m1[i], s1[i], h1[i])
		}
	}
	// The MACD line does not depend on the signal period.
	assertReal(t, "macd line vs signal 1", m[33:], m1[33:], 0)
}

func TestMACDFIXRangeForm(t *testing.T) {
	in := fixtures(60)[0].Real()
	n := len(in)
	o1, o2, o3 := make([]float64, n), make([]float64, n), make([]float64, n)
	if _, _, err := macdfixRange(-1, 5, in, 9, o1, o2, o3); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := macdfixRange(5, 4, in, 9, o1, o2, o3); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := macdfixRange(0, -1, in, 9, o1, o2, o3); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := macdfixRange(0, 32, in, 9, o1, o2, o3)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := macdfixRange(0, 50, in[:40], 9, o1, o2, o3); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	for k := range 3 {
		bufs := [][]float64{o1, o2, o3}
		bufs[k] = make([]float64, 10)
		if _, _, err := macdfixRange(0, 50, in, 9, bufs[0], bufs[1], bufs[2]); !errors.Is(err, ErrBadParam) {
			t.Errorf("short output %d: %v", k, err)
		}
	}
	// Aliased outputs are rejected, like C.
	if _, _, err := macdfixRange(0, n-1, in, 9, o1, o1, o3); !errors.Is(err, ErrBadParam) {
		t.Errorf("aliased outputs: %v", err)
	}
	if _, _, err := macdfixRange(0, n-1, in, 9, o1, o2, o2); !errors.Is(err, ErrBadParam) {
		t.Errorf("aliased outputs: %v", err)
	}
	if _, _, err := macdfixRange(0, n-1, in, 9, o3, o2, o3); !errors.Is(err, ErrBadParam) {
		t.Errorf("aliased outputs: %v", err)
	}
	// An output may share inReal (in place).
	wm, ws, wh, _ := MACDFIX(in, 9)
	buf := append([]float64(nil), in...)
	beg, nb, err = macdfixRange(0, n-1, buf, 9, buf, o2, o3)
	if err != nil {
		t.Fatal(err)
	}
	alignReal(buf, beg, nb)
	alignReal(o2, beg, nb)
	alignReal(o3, beg, nb)
	assertReal(t, "in-place macd", buf, wm, 0)
	assertReal(t, "in-place signal", o2, ws, 0)
	assertReal(t, "in-place hist", o3, wh, 0)
	// A sub-range seeds its EMAs at startIdx-lookback (like C), so it equals
	// the full computation over in[startIdx-33 : endIdx+1].
	beg, nb, err = macdfixRange(40, 50, in, 9, o1, o2, o3)
	if err != nil || beg != 40 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	sm, ss, sh, _ := MACDFIX(in[7:51], 9)
	assertReal(t, "sub-range macd", o1[:nb], sm[33:], 0)
	assertReal(t, "sub-range signal", o2[:nb], ss[33:], 0)
	assertReal(t, "sub-range hist", o3[:nb], sh[33:], 0)
}
