package ta

import (
	"errors"
	"math"
	"testing"
)

func TestHAParams(t *testing.T) {
	s := fixtures(50)[0]
	if lb := HALookback(); lb != 0 {
		t.Errorf("HALookback() = %d, want 0 by default", lb)
	}
	// Mismatched input lengths, in every position.
	short := s.O[:49]
	for i, in := range [][4][]float64{
		{short, s.H, s.L, s.C},
		{s.O, short, s.L, s.C},
		{s.O, s.H, short, s.C},
		{s.O, s.H, s.L, short},
		{s.O, s.H, s.L, nil},
	} {
		if _, _, _, _, err := HA(in[0], in[1], in[2], in[3]); !errors.Is(err, ErrBadParam) {
			t.Errorf("mismatch #%d: err = %v, want ErrBadParam", i, err)
		}
	}
	// nil / empty input → empty outputs, nil error.
	for _, in := range [][]float64{nil, {}} {
		o, h, l, c, err := HA(in, in, in, in)
		if err != nil || len(o) != 0 || len(h) != 0 || len(l) != 0 || len(c) != 0 {
			t.Errorf("empty input: lens %d %d %d %d err=%v", len(o), len(h), len(l), len(c), err)
		}
	}
	// Short input / output buffers in the range form.
	out := func(n int) []float64 { return make([]float64, n) }
	if _, _, err := haRange(0, 10, s.O[:5], s.H, s.L, s.C, out(20), out(20), out(20), out(20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inOpen: err = %v", err)
	}
	if _, _, err := haRange(0, 10, s.O, s.H, s.L, s.C[:10], out(20), out(20), out(20), out(20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short inClose: err = %v", err)
	}
	if _, _, err := haRange(0, 10, s.O, s.H, s.L, s.C, out(20), out(20), out(10), out(20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short outHALow: err = %v", err)
	}
	if _, _, err := haRange(0, 10, s.O, s.H, s.L, s.C, out(20), out(20), out(20), nil); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil outHAClose: err = %v", err)
	}
	// Output buffers must be pairwise distinct (C rejects equal pointers).
	a, b, c, d := out(50), out(50), out(50), out(50)
	for i, o := range [][4][]float64{
		{a, a, c, d}, {a, b, a, d}, {a, b, c, a}, {a, b, b, d}, {a, b, c, b}, {a, b, c, c},
	} {
		if _, _, err := haRange(0, 49, s.O, s.H, s.L, s.C, o[0], o[1], o[2], o[3]); !errors.Is(err, ErrBadParam) {
			t.Errorf("aliased outputs #%d: err = %v, want ErrBadParam", i, err)
		}
	}
}

func TestHALengths(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	for _, u := range []int{0, 1, 2, 5, 30} {
		SetUnstablePeriod(UnstHA, u)
		lb := HALookback()
		if lb != u {
			t.Fatalf("HALookback() with unstable %d = %d", u, lb)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				o, h, l, c, err := HA(s.O, s.H, s.L, s.C)
				if err != nil {
					t.Fatalf("%s n=%d u=%d: %v", s.Name, n, u, err)
				}
				for k, out := range [][]float64{o, h, l, c} {
					if len(out) != n {
						t.Fatalf("%s n=%d u=%d out#%d: len=%d", s.Name, n, u, k, len(out))
					}
					assertLeadingNaN(t, lbl("HA ", s.Name, " n=", n, " u=", u, " out#", k), out, lb)
				}
			}
		}
	}
}

func TestHAKnownValues(t *testing.T) {
	// Hand-derived from the formula in ha.md:
	// bar0: open (10+11)/2=10.5, close 41/4=10.25, high 12, low 8
	// bar1: open (10.5+10.25)/2=10.375, close 48/4=12, high 14, low 10
	// bar2: open (10.375+12)/2=11.1875, close 45/4=11.25, high 13, low 9
	// bar3: open (11.1875+11.25)/2=11.21875, close 20/4=5,
	//       high = HA open 11.21875 (above the raw high 6), low 4
	oIn := []float64{10, 11, 13, 5}
	hIn := []float64{12, 14, 13, 6}
	lIn := []float64{8, 10, 9, 4}
	cIn := []float64{11, 13, 10, 5}
	o, h, l, c, err := HA(oIn, hIn, lIn, cIn)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "HA open", o, []float64{10.5, 10.375, 11.1875, 11.21875}, 0)
	assertReal(t, "HA high", h, []float64{12, 14, 13, 11.21875}, 0)
	assertReal(t, "HA low", l, []float64{8, 10, 9, 4}, 0)
	assertReal(t, "HA close", c, []float64{10.25, 12, 11.25, 5}, 0)

	// A bar whose HA close falls below the raw low.
	o, h, l, c, _ = HA([]float64{10, 1}, []float64{10, 30}, []float64{10, 29}, []float64{10, 30})
	// bar1: open 10 (below low 29), close 90/4=22.5 (below low 29)
	assertReal(t, "HA low below raw", l, []float64{10, 10}, 0)
	assertReal(t, "HA high above", h, []float64{10, 30}, 0)
	assertReal(t, "HA open2", o, []float64{10, 10}, 0)
	assertReal(t, "HA close2", c, []float64{10, 22.5}, 0)

	// Signed zeros: plain comparisons keep the raw high (+0) and low (-0),
	// while the HA open is -0 and the HA close is +0.
	nz := math.Copysign(0, -1)
	o, h, l, c, _ = HA([]float64{nz}, []float64{0}, []float64{nz}, []float64{nz})
	if !math.Signbit(o[0]) || math.Signbit(h[0]) || !math.Signbit(l[0]) || math.Signbit(c[0]) {
		t.Errorf("signed zeros: signbits open %v high %v low %v close %v, want true false true false",
			math.Signbit(o[0]), math.Signbit(h[0]), math.Signbit(l[0]), math.Signbit(c[0]))
	}

	// Constant input maps to itself.
	k := fixtures(40)[5]
	o, h, l, c, _ = HA(k.O, k.H, k.L, k.C)
	for _, out := range [][]float64{o, h, l, c} {
		for i, v := range out {
			if v != 100 {
				t.Fatalf("constant: [%d]=%g", i, v)
			}
		}
	}
}

func TestHAProperties(t *testing.T) {
	for _, s := range fixtures(300) {
		o, h, l, c, err := HA(s.O, s.H, s.L, s.C)
		if err != nil {
			t.Fatal(err)
		}
		for i := range o {
			if h[i] < s.H[i] || h[i] < o[i] || h[i] < c[i] {
				t.Fatalf("%s [%d]: high %g below max(H %g, open %g, close %g)", s.Name, i, h[i], s.H[i], o[i], c[i])
			}
			if l[i] > s.L[i] || l[i] > o[i] || l[i] > c[i] {
				t.Fatalf("%s [%d]: low %g above min(L %g, open %g, close %g)", s.Name, i, l[i], s.L[i], o[i], c[i])
			}
			if want := (s.O[i] + s.H[i] + s.L[i] + s.C[i]) / 4; c[i] != want {
				t.Fatalf("%s [%d]: close %g want %g", s.Name, i, c[i], want)
			}
			if i > 0 && o[i] != (o[i-1]+c[i-1])/2 {
				t.Fatalf("%s [%d]: open %g not midpoint of previous", s.Name, i, o[i])
			}
		}
	}

	// With an unstable period u and startIdx 0 the seed is still bar 0, so
	// the outputs equal the u=0 outputs from bar u onward.
	defer SetAllUnstablePeriods(0)
	s := fixtures(100)[1]
	o0, h0, l0, c0, _ := HA(s.O, s.H, s.L, s.C)
	SetUnstablePeriod(UnstHA, 7)
	o7, h7, l7, c7, _ := HA(s.O, s.H, s.L, s.C)
	for k, pair := range [][2][]float64{{o0, o7}, {h0, h7}, {l0, l7}, {c0, c7}} {
		assertReal(t, lbl("unstable tail out#", k), pair[1][7:], pair[0][7:], 0)
	}
	// Another function's unstable period must not change HA.
	SetAllUnstablePeriods(0)
	SetUnstablePeriod(UnstEMA, 9)
	if lb := HALookback(); lb != 0 {
		t.Errorf("HALookback with UnstEMA set = %d", lb)
	}
}

func TestHARangeForm(t *testing.T) {
	s := fixtures(30)[0]
	out := func() []float64 { return make([]float64, 30) }
	o, h, l, c := out(), out(), out(), out()
	if _, _, err := haRange(-1, 5, s.O, s.H, s.L, s.C, o, h, l, c); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := haRange(5, 4, s.O, s.H, s.L, s.C, o, h, l, c); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := haRange(0, -1, s.O, s.H, s.L, s.C, o, h, l, c); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}

	// Range entirely inside the lookback: no output, no error.
	func() {
		defer SetAllUnstablePeriods(0)
		SetUnstablePeriod(UnstHA, 5)
		beg, nb, err := haRange(0, 4, s.O, s.H, s.L, s.C, o, h, l, c)
		if err != nil || beg != 0 || nb != 0 {
			t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
		}
		// The first output belongs to bar 5 and is seeded from bar 0.
		beg, nb, err = haRange(2, 9, s.O, s.H, s.L, s.C, o, h, l, c)
		if err != nil || beg != 5 || nb != 5 {
			t.Errorf("start inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
		}
	}()

	// A sub-range reseeds at startIdx: its first open is (O+C)/2 of that bar.
	beg, nb, err := haRange(10, 12, s.O, s.H, s.L, s.C, o, h, l, c)
	if err != nil || beg != 10 || nb != 3 {
		t.Fatalf("sub-range: beg=%d nb=%d err=%v", beg, nb, err)
	}
	if o[0] != (s.O[10]+s.C[10])/2 {
		t.Errorf("sub-range seed: open %g want %g", o[0], (s.O[10]+s.C[10])/2)
	}

	// Outputs may alias the inputs (one output per input buffer).
	wo, wh, wl, wc, _ := HA(s.O, s.H, s.L, s.C)
	bo := append([]float64(nil), s.O...)
	bh := append([]float64(nil), s.H...)
	bl := append([]float64(nil), s.L...)
	bc := append([]float64(nil), s.C...)
	// Rotate the mapping so each output overwrites a different input.
	beg, nb, err = haRange(0, 29, bo, bh, bl, bc, bh, bl, bc, bo)
	if err != nil {
		t.Fatal(err)
	}
	alignReal(bh, beg, nb)
	alignReal(bl, beg, nb)
	alignReal(bc, beg, nb)
	alignReal(bo, beg, nb)
	assertReal(t, "in-place open", bh, wo, 0)
	assertReal(t, "in-place high", bl, wh, 0)
	assertReal(t, "in-place low", bc, wl, 0)
	assertReal(t, "in-place close", bo, wc, 0)
}
