package ta

import (
	"errors"
	"math"
	"testing"
)

func TestMACDParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	bad := [][3]int{
		{1, 26, 9}, {100001, 26, 9}, {0, 26, 9}, {math.MinInt32, 26, 9},
		{12, 1, 9}, {12, 100001, 9}, {12, -1, 9},
		{12, 26, 0}, {12, 26, 100001}, {12, 26, -3},
	}
	for _, p := range bad {
		if _, _, _, err := MACD(in, p[0], p[1], p[2]); !errors.Is(err, ErrBadParam) {
			t.Errorf("params %v: err = %v, want ErrBadParam", p, err)
		}
		if lb := MACDLookback(p[0], p[1], p[2]); lb != -1 {
			t.Errorf("MACDLookback(%v) = %d, want -1", p, lb)
		}
		out := make([]float64, len(in))
		if _, _, err := macdRange(0, len(in)-1, in, p[0], p[1], p[2], out, make([]float64, len(in)), make([]float64, len(in))); !errors.Is(err, ErrBadParam) {
			t.Errorf("range params %v: err = %v, want ErrBadParam", p, err)
		}
	}
	good := [][3]int{{2, 2, 1}, {100000, 100000, 100000}, {2, 100000, 1}, {12, 26, 9}, {26, 12, 9}}
	for _, p := range good {
		if _, _, _, err := MACD(in, p[0], p[1], p[2]); err != nil {
			t.Errorf("params %v: unexpected err %v", p, err)
		}
		if lb := MACDLookback(p[0], p[1], p[2]); lb < 0 {
			t.Errorf("MACDLookback(%v) = %d", p, lb)
		}
	}
	for _, in := range [][]float64{nil, {}} {
		m, s, h, err := MACD(in, 12, 26, 9)
		if err != nil || len(m) != 0 || len(s) != 0 || len(h) != 0 {
			t.Errorf("empty input: %v %v %v err=%v, want empty and nil", m, s, h, err)
		}
	}
}

func TestMACDLookbackValues(t *testing.T) {
	cases := []struct{ f, s, sig, want int }{
		{12, 26, 9, 33}, {26, 12, 9, 33}, {2, 2, 1, 1}, {2, 3, 1, 2}, {2, 3, 2, 3},
		{100000, 2, 100000, 199998},
	}
	for _, c := range cases {
		if lb := MACDLookback(c.f, c.s, c.sig); lb != c.want {
			t.Errorf("MACDLookback(%d,%d,%d) = %d, want %d", c.f, c.s, c.sig, lb, c.want)
		}
	}
}

func TestMACDLengths(t *testing.T) {
	for _, p := range [][3]int{{12, 26, 9}, {2, 2, 1}, {3, 2, 1}, {5, 10, 4}, {26, 12, 9}} {
		lb := MACDLookback(p[0], p[1], p[2])
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				m, sg, h, err := MACD(s.Real(), p[0], p[1], p[2])
				if err != nil {
					t.Fatalf("%s n=%d p=%v: %v", s.Name, n, p, err)
				}
				if len(m) != n || len(sg) != n || len(h) != n {
					t.Fatalf("%s n=%d p=%v: lens %d %d %d", s.Name, n, p, len(m), len(sg), len(h))
				}
				tag := lbl("MACD ", s.Name, " n=", n, " p=", p)
				assertLeadingNaN(t, tag+" macd", m, lb)
				assertLeadingNaN(t, tag+" signal", sg, lb)
				assertLeadingNaN(t, tag+" hist", h, lb)
			}
		}
	}
}

func TestMACDKnownValues(t *testing.T) {
	nan := math.NaN()
	// Hand-checked: fast 2 (k=2/3), slow 3 (k=1/2). Both EMAs are seeded at
	// bar 2 with 0; at bar 3 fast = 2/3*3 = 2, slow = 1/2*3 = 1.5, MACD = 0.5.
	in := []float64{0, 0, 0, 3}
	m, s, h, err := MACD(in, 2, 3, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "MACD sig1 macd", m, []float64{nan, nan, 0, 0.5}, 1e-15)
	assertReal(t, "MACD sig1 signal", s, []float64{nan, nan, 0, 0.5}, 1e-15)
	assertReal(t, "MACD sig1 hist", h, []float64{nan, nan, 0, 0}, 0)
	// Signal 2: seeded with the mean of the MACD values at bars 2 and 3.
	m, s, h, _ = MACD(in, 2, 3, 2)
	assertReal(t, "MACD sig2 macd", m, []float64{nan, nan, nan, 0.5}, 1e-15)
	assertReal(t, "MACD sig2 signal", s, []float64{nan, nan, nan, 0.25}, 1e-15)
	assertReal(t, "MACD sig2 hist", h, []float64{nan, nan, nan, 0.25}, 1e-15)

	// Linear input: an EMA of period p lags by (p-1)/2, so MACD = (slow-fast)/2.
	lin := make([]float64, 80)
	for i := range lin {
		lin[i] = float64(i)
	}
	m, s, h, _ = MACD(lin, 4, 10, 5)
	for i := MACDLookback(4, 10, 5); i < len(lin); i++ {
		if !closeEnough(m[i], 3, 1e-12) || !closeEnough(s[i], 3, 1e-12) || !closeEnough(h[i], 0, 1e-12) {
			t.Fatalf("linear [%d]: %g %g %g", i, m[i], s[i], h[i])
		}
	}

	// Constant input: MACD, signal and histogram are exactly 0.
	c := fixtures(60)[5].Real()
	m, s, h, _ = MACD(c, 12, 26, 9)
	for i := 33; i < len(c); i++ {
		if m[i] != 0 || s[i] != 0 || h[i] != 0 {
			t.Fatalf("constant [%d]: %g %g %g", i, m[i], s[i], h[i])
		}
	}

	// Swapped periods give identical output; hist = macd - signal; signal 1
	// is the MACD line itself.
	w := fixtures(200)[0].Real()
	m1, s1, h1, _ := MACD(w, 12, 26, 9)
	m2, s2, h2, _ := MACD(w, 26, 12, 9)
	assertReal(t, "swap macd", m2, m1, 0)
	assertReal(t, "swap signal", s2, s1, 0)
	assertReal(t, "swap hist", h2, h1, 0)
	for i := 33; i < len(w); i++ {
		if h1[i] != m1[i]-s1[i] {
			t.Fatalf("hist [%d] %g != %g-%g", i, h1[i], m1[i], s1[i])
		}
	}
	m, s, h, _ = MACD(w, 12, 26, 1)
	for i := 25; i < len(w); i++ {
		if s[i] != m[i] || h[i] != 0 {
			t.Fatalf("signal 1 [%d]: macd %g signal %g hist %g", i, m[i], s[i], h[i])
		}
	}
	// The MACD line equals EMA(fast) - EMA(slow), where the fast EMA's seed
	// window is the tail of the slow one (both seeds end on bar 25).
	ef, _ := EMA(w[14:], 12)
	es, _ := EMA(w, 26)
	for i := 25; i < len(w); i++ {
		if !closeEnough(m[i], ef[i-14]-es[i], 1e-12) {
			t.Fatalf("ema diff [%d]: %g vs %g", i, m[i], ef[i-14]-es[i])
		}
	}
}

func TestMACDRangeForm(t *testing.T) {
	in := fixtures(60)[0].Real()
	n := len(in)
	o1, o2, o3 := make([]float64, n), make([]float64, n), make([]float64, n)
	if _, _, err := macdRange(-1, 5, in, 12, 26, 9, o1, o2, o3); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := macdRange(5, 4, in, 12, 26, 9, o1, o2, o3); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := macdRange(0, -1, in, 12, 26, 9, o1, o2, o3); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := macdRange(0, 32, in, 12, 26, 9, o1, o2, o3)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Short buffers.
	if _, _, err := macdRange(0, 50, in[:40], 12, 26, 9, o1, o2, o3); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input: %v", err)
	}
	for k := range 3 {
		bufs := [][]float64{o1, o2, o3}
		bufs[k] = make([]float64, 10)
		if _, _, err := macdRange(0, 50, in, 12, 26, 9, bufs[0], bufs[1], bufs[2]); !errors.Is(err, ErrBadParam) {
			t.Errorf("short output %d: %v", k, err)
		}
	}
	// Aliased outputs are rejected, like C.
	if _, _, err := macdRange(0, n-1, in, 12, 26, 9, o1, o1, o3); !errors.Is(err, ErrBadParam) {
		t.Errorf("aliased outputs: %v", err)
	}
	if _, _, err := macdRange(0, n-1, in, 12, 26, 9, o1, o2, o2); !errors.Is(err, ErrBadParam) {
		t.Errorf("aliased outputs: %v", err)
	}
	// An output may share inReal (in place).
	wm, ws, wh, _ := MACD(in, 12, 26, 9)
	buf := append([]float64(nil), in...)
	beg, nb, err = macdRange(0, n-1, buf, 12, 26, 9, buf, o2, o3)
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
	beg, nb, err = macdRange(40, 50, in, 12, 26, 9, o1, o2, o3)
	if err != nil || beg != 40 || nb != 11 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	sm, ss, sh, _ := MACD(in[7:51], 12, 26, 9)
	assertReal(t, "sub-range macd", o1[:nb], sm[33:], 0)
	assertReal(t, "sub-range signal", o2[:nb], ss[33:], 0)
	assertReal(t, "sub-range hist", o3[:nb], sh[33:], 0)
}
