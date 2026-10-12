package ta

import (
	"errors"
	"math"
	"math/rand/v2"
	"testing"
)

// mavpPeriodSets returns named per-bar period series of length n: integer
// cycles, fractional values, out-of-window values (below min, above max,
// negative, zero, NaN, ±Inf, huge) and a single constant period.
func mavpPeriodSets(n int) []struct {
	Name string
	P    []float64
} {
	cyc := make([]float64, n)
	frac := make([]float64, n)
	wild := make([]float64, n)
	cons := make([]float64, n)
	r := rand.New(rand.NewPCG(42, 0x3a7f))
	specials := []float64{math.NaN(), math.Inf(1), math.Inf(-1), -5, 0, 0.5, 1, 1e300, -1e300, 3e9, 1.999999}
	for i := range n {
		cyc[i] = float64(2 + (i*7)%29)
		frac[i] = r.Float64() * 40
		if i%3 == 0 {
			wild[i] = specials[(i/3)%len(specials)]
		} else {
			wild[i] = r.Float64()*60 - 10
		}
		cons[i] = 7
	}
	return []struct {
		Name string
		P    []float64
	}{{"cycle", cyc}, {"fractional", frac}, {"wild", wild}, {"const7", cons}}
}

func TestMAVPParams(t *testing.T) {
	in := fixtures(50)[0].Real()
	per := mavpPeriodSets(50)[0].P
	bad := []struct {
		mn, mx int
		mt     MAType
	}{
		{0, 30, MATypeSMA}, {-1, 30, MATypeSMA}, {100001, 100000, MATypeSMA}, {math.MinInt32, 30, MATypeSMA},
		{2, 0, MATypeSMA}, {1, -1, MATypeSMA}, {2, 100001, MATypeSMA},
		{2, 30, -1}, {2, 30, 14}, {2, 30, 100},
		{31, 30, MATypeSMA}, {100000, 1, MATypeEMA}, // inverted window
	}
	for _, c := range bad {
		if _, err := MAVP(in, per, c.mn, c.mx, c.mt); !errors.Is(err, ErrBadParam) {
			t.Errorf("%v: err = %v, want ErrBadParam", c, err)
		}
		if lb := MAVPLookback(c.mn, c.mx, c.mt); lb != -1 {
			t.Errorf("MAVPLookback%v = %d, want -1", c, lb)
		}
		if _, _, err := mavpRange(0, 49, in, per, c.mn, c.mx, c.mt, make([]float64, 50)); !errors.Is(err, ErrBadParam) {
			t.Errorf("mavpRange %v: err = %v, want ErrBadParam", c, err)
		}
	}
	for _, mt := range maAllTypes {
		for _, c := range [][2]int{{1, 1}, {1, 100000}, {100000, 100000}, {2, 30}, {5, 5}} {
			if _, err := MAVP(in, per, c[0], c[1], mt); err != nil {
				t.Errorf("%v mt=%d: unexpected err %v", c, mt, err)
			}
			if lb := MAVPLookback(c[0], c[1], mt); lb != MALookback(c[1], mt) {
				t.Errorf("MAVPLookback(%d,%d,%d) = %d, want %d", c[0], c[1], mt, lb, MALookback(c[1], mt))
			}
		}
	}
	if _, err := MAVP(in, per[:49], 2, 30, MATypeSMA); !errors.Is(err, ErrBadParam) {
		t.Errorf("mismatched lengths: err = %v", err)
	}
	if _, err := MAVP(in, nil, 2, 30, MATypeSMA); !errors.Is(err, ErrBadParam) {
		t.Errorf("nil periods: err = %v", err)
	}
	if out, err := MAVP(nil, nil, 2, 30, MATypeSMA); err != nil || len(out) != 0 {
		t.Errorf("nil input: out=%v err=%v, want empty and nil", out, err)
	}
	if out, err := MAVP([]float64{}, []float64{}, 2, 30, MATypeSMA); err != nil || len(out) != 0 {
		t.Errorf("empty input: out=%v err=%v, want empty and nil", out, err)
	}
	if _, _, err := mavpRange(0, 10, in[:5], per, 2, 3, MATypeSMA, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short input buffer: err = %v", err)
	}
	if _, _, err := mavpRange(0, 10, in, per[:5], 2, 3, MATypeSMA, make([]float64, 20)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short periods buffer: err = %v", err)
	}
	if _, _, err := mavpRange(0, 10, in, per, 2, 3, MATypeSMA, make([]float64, 5)); !errors.Is(err, ErrBadParam) {
		t.Errorf("short output buffer: err = %v", err)
	}
}

func TestMAVPLengths(t *testing.T) {
	for _, mt := range maAllTypes {
		for _, c := range [][2]int{{1, 1}, {2, 5}, {2, 30}} {
			lb := MAVPLookback(c[0], c[1], mt)
			for _, n := range edgeLengths(lb) {
				for _, ps := range mavpPeriodSets(n) {
					for _, s := range fixtures(n) {
						out, err := MAVP(s.Real(), ps.P, c[0], c[1], mt)
						if err != nil {
							t.Fatalf("%s/%s n=%d %v mt=%d: %v", s.Name, ps.Name, n, c, mt, err)
						}
						if len(out) != n {
							t.Fatalf("%s/%s n=%d %v mt=%d: len(out)=%d", s.Name, ps.Name, n, c, mt, len(out))
						}
						assertLeadingNaN(t, lbl("MAVP ", s.Name, "/", ps.Name, " n=", n, " ", c, " mt=", mt), out, lb)
					}
				}
			}
		}
	}
}

func TestMAVPKnownValues(t *testing.T) {
	nan := math.NaN()
	in := []float64{1, 2, 3, 4, 5, 6}
	// Lookback = SMA(3) lookback = 2. Bar 2: SMA3=2; bar 3: period 1 -> 4;
	// bar 4: SMA2=(4+5)/2; bar 5: 2.9 truncates to 2 -> (5+6)/2.
	out, err := MAVP(in, []float64{9, 9, 3, 1, 2, 2.9}, 1, 3, MATypeSMA)
	if err != nil {
		t.Fatal(err)
	}
	assertReal(t, "MAVP hand", out, []float64{nan, nan, 2, 4, 4.5, 5.5}, 0)

	// Clamping: NaN and below-min periods use min (2), above-max use max (3).
	out, _ = MAVP(in, []float64{0, 0, nan, 0.5, 100, math.Inf(1)}, 2, 3, MATypeDefault)
	assertReal(t, "MAVP clamp", out, []float64{nan, nan, 2.5, 3.5, 4, 5}, 0)

	// A constant period equals MA with that period, for every type.
	w := fixtures(300)[0].Real()
	for _, mt := range maAllTypes {
		for _, p := range []int{1, 2, 7, 14} {
			per := make([]float64, len(w))
			for i := range per {
				per[i] = float64(p)
			}
			got, err := MAVP(w, per, 1, p, mt)
			if err != nil {
				t.Fatal(err)
			}
			want, _ := MA(w, p, mt)
			assertReal(t, lbl("MAVP const p=", p, " mt=", mt), got, want, 0)
		}
	}

	// Each bar equals MA at its clamped period computed over the full range
	// for window MAs (the sliding SMA seed starts at the lookback bar in both).
	per := mavpPeriodSets(300)[1].P
	got, _ := MAVP(w, per, 2, 30, MATypeWMA)
	lb := MAVPLookback(2, 30, MATypeWMA)
	for i := lb; i < len(w); i++ {
		p := int(per[i])
		if p < 2 {
			p = 2
		}
		if p > 30 {
			p = 30
		}
		ref, _ := WMA(w[i-p+1:i+1], p)
		if math.Abs(got[i]-ref[p-1]) > 1e-9*math.Abs(ref[p-1]) {
			t.Fatalf("WMA bar %d p=%d: %g vs %g", i, p, got[i], ref[p-1])
		}
	}

	// Constant input averages to itself.
	c := fixtures(200)[5].Real()
	for _, mt := range []MAType{MATypeSMA, MATypeEMA, MATypeWMA, MATypeTRIMA, MATypeRMA} {
		out, _ = MAVP(c, mavpPeriodSets(200)[2].P, 2, 20, mt)
		for i := MAVPLookback(2, 20, mt); i < len(out); i++ {
			if math.Abs(out[i]-100) > 1e-9 {
				t.Fatalf("constant mt=%d: [%d]=%g", mt, i, out[i])
			}
		}
	}

	// Period-1 window is the identity.
	out, _ = MAVP(w, per, 1, 1, MATypeEMA)
	assertReal(t, "MAVP 1..1 identity", out, w, 0)
}

func TestMAVPRangeForm(t *testing.T) {
	in := fixtures(60)[0].Real()
	per := mavpPeriodSets(60)[0].P
	out := make([]float64, len(in))
	if _, _, err := mavpRange(-1, 5, in, per, 2, 5, MATypeSMA, out); !errors.Is(err, ErrOutOfRangeStartIndex) {
		t.Errorf("start -1: %v", err)
	}
	if _, _, err := mavpRange(5, 4, in, per, 2, 5, MATypeSMA, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end < start: %v", err)
	}
	if _, _, err := mavpRange(0, -1, in, per, 2, 5, MATypeSMA, out); !errors.Is(err, ErrOutOfRangeEndIndex) {
		t.Errorf("end -1: %v", err)
	}
	// Range entirely inside the lookback: no output, no error.
	beg, nb, err := mavpRange(0, 3, in, per, 2, 5, MATypeSMA, out)
	if err != nil || beg != 0 || nb != 0 {
		t.Errorf("inside lookback: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// Clamped begin.
	beg, nb, err = mavpRange(0, 10, in, per, 2, 5, MATypeSMA, out)
	if err != nil || beg != 4 || nb != 7 {
		t.Errorf("clamped: beg=%d nb=%d err=%v", beg, nb, err)
	}
	// In-place computation is allowed (multi-period and single-period paths).
	for _, ps := range [][]float64{per, mavpPeriodSets(60)[3].P} {
		buf := append([]float64(nil), in...)
		want, _ := MAVP(in, ps, 2, 30, MATypeEMA)
		beg, nb, _ = mavpRange(0, len(buf)-1, buf, ps, 2, 30, MATypeEMA, buf)
		alignReal(buf, beg, nb)
		assertReal(t, "in-place", buf, want, 0)
	}
}
