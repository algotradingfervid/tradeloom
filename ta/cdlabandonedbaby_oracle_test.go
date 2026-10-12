//go:build oracle

package ta

import (
	"math"
	"math/rand/v2"
	"testing"

	"tradeloom/ta/internal/oracle"
)

var cdlabandonedbabyOraclePens = []float64{0, 0.3, 0.5, 1, 3e37}

// cdlabandonedbabyOracleCompare compares CDLABANDONEDBABY with C on every
// fixture and edge length under the current candle settings, and returns how
// many bars fired.
func cdlabandonedbabyOracleCompare(t *testing.T, tag string, pens []float64) int {
	t.Helper()
	fired := 0
	for _, p := range pens {
		lb := CDLABANDONEDBABYLookback(p)
		if c := mustLookback(t, "CDLABANDONEDBABY", p); lb != c {
			t.Fatalf("%s lookback p=%g: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := CDLABANDONEDBABY(s.O, s.H, s.L, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "CDLABANDONEDBABY", s.ohlcv(), p)
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertInt(t, lbl("CDLABANDONEDBABY ", tag, " ", s.Name, " n=", n, " p=", p), got, r.AlignedInt(0, n))
				for _, v := range got {
					if v != 0 {
						fired++
					}
				}
			}
		}
	}
	return fired
}

func TestCDLABANDONEDBABYOracle(t *testing.T) {
	fired := cdlabandonedbabyOracleCompare(t, "default", cdlabandonedbabyOraclePens)
	t.Logf("fired %d times on the fixtures", fired)
}

// TestCDLABANDONEDBABYOracleConstructed checks the hand-built top and bottom,
// which must fire in both Go and C, and their penetration boundaries.
func TestCDLABANDONEDBABYOracleConstructed(t *testing.T) {
	for _, tc := range []struct {
		name string
		bars [3]cdlabandonedbabyBar
		want int
	}{{"top", cdlabandonedbabyTop, -100}, {"bottom", cdlabandonedbabyBottom, 100}} {
		o, h, l, c := cdlabandonedbabySeries(tc.bars[:]...)
		for _, p := range []float64{0, 0.3, 0.7, 0.75, 0.8, 3e37} {
			got, err := CDLABANDONEDBABY(o, h, l, c, p)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "CDLABANDONEDBABY", oracle.Inputs{Open: o, High: h, Low: l, Close: c}, p)
			want := r.AlignedInt(0, len(o))
			assertInt(t, lbl("constructed ", tc.name, " p=", p), got, want)
			if p == 0.3 && want[14] != tc.want {
				t.Fatalf("C did not fire on the constructed %s: %v", tc.name, want)
			}
		}
	}
}

// TestCDLABANDONEDBABYOracleThreshold puts the 3rd close within a few ulps of
// the penetration threshold for random penetrations, so the exact rounding of
// C's unfused (top) and fused (bottom) threshold is compared.
func TestCDLABANDONEDBABYOracleThreshold(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	fired := [2]int{}
	for k := 0; k < 400; k++ {
		p := rng.Float64()
		body := 2 + rng.Float64()*6 // 1st body in (2, 8), well above avg 1
		for side, base := range [2][3]cdlabandonedbabyBar{cdlabandonedbabyTop, cdlabandonedbabyBottom} {
			b := base
			if side == 0 {
				b[0].c = b[0].o + body
				b[0].h = b[0].c + 0.5
				b[1] = cdlabandonedbabyBar{b[0].h + 2, b[0].h + 2.2, b[0].h + 1.8, b[0].h + 2}
				th := b[0].c - body*p
				b[2].c = th
			} else {
				b[0].c = b[0].o - body
				b[0].l = b[0].c - 0.5
				b[1] = cdlabandonedbabyBar{b[0].l - 2, b[0].l - 1.8, b[0].l - 2.2, b[0].l - 2}
				th := math.FMA(body, p, b[0].c)
				b[2].c = th
			}
			for d := -3; d <= 3; d++ {
				bb := b
				cl := bb[2].c
				for range cdlabandonedbabyAbs(d) {
					if d < 0 {
						cl = math.Nextafter(cl, math.Inf(-1))
					} else {
						cl = math.Nextafter(cl, math.Inf(1))
					}
				}
				bb[2].c = cl
				if side == 0 { // black 3rd, below the doji
					bb[2].o = cl + 2.5
					bb[2].h = bb[2].o + 0.1
					bb[2].l = cl - 0.1
					if bb[2].h >= bb[1].l {
						continue
					}
				} else { // white 3rd, above the doji
					bb[2].o = cl - 2.5
					bb[2].l = bb[2].o - 0.1
					bb[2].h = cl + 0.1
					if bb[2].l <= bb[1].h {
						continue
					}
				}
				o, h, l, c := cdlabandonedbabySeries(bb[:]...)
				got, err := CDLABANDONEDBABY(o, h, l, c, p)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCall(t, "CDLABANDONEDBABY", oracle.Inputs{Open: o, High: h, Low: l, Close: c}, p)
				assertInt(t, lbl("threshold k=", k, " side=", side, " d=", d, " p=", p), got, r.AlignedInt(0, len(o)))
				if got[14] != 0 {
					fired[side]++
				}
			}
		}
	}
	if fired[0] == 0 || fired[1] == 0 {
		t.Fatalf("threshold cases never fired: %v", fired)
	}
}

func cdlabandonedbabyAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func TestCDLABANDONEDBABYOracleBadParams(t *testing.T) {
	s := fixtures(30)[0]
	// -4e37 is TA_REAL_DEFAULT, which C substitutes; Go has no sentinel, so
	// it is not compared here.
	for _, p := range []float64{-1, -1e-300, math.NaN(), math.Inf(1), math.Inf(-1), 3.0000001e37, 0, 3e37, 0.3} {
		r := mustCall(t, "CDLABANDONEDBABY", s.ohlcv(), p)
		_, err := CDLABANDONEDBABY(s.O, s.H, s.L, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%g: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		lbErr := CDLABANDONEDBABYLookback(p) == -1
		if c := mustLookback(t, "CDLABANDONEDBABY", p); (c == -1) != lbErr {
			t.Errorf("p=%g lookback: C %d, Go %d", p, c, CDLABANDONEDBABYLookback(p))
		}
	}
}

func TestCDLABANDONEDBABYOracleRanges(t *testing.T) {
	s := fixtures(300)[11] // gaps
	n := len(s.C)
	for _, p := range []float64{0, 0.3} {
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 11}, {0, 12}, {12, 12}, {10, 50}, {13, 13}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlabandonedbabyRange(rg[0], rg[1], s.O, s.H, s.L, s.C, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLABANDONEDBABY", s.ohlcv(), rg[0], rg[1], p)
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%g: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLABANDONEDBABY range ", rg, " p=", p), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
	// A sub-range over a constructed pattern.
	o, h, l, c := cdlabandonedbabySeries(cdlabandonedbabyBottom[:]...)
	for _, rg := range [][2]int{{14, 14}, {13, 14}, {0, 14}, {12, 13}} {
		out := make([]int, len(o))
		beg, nb, _ := cdlabandonedbabyRange(rg[0], rg[1], o, h, l, c, 0.3, out)
		r := mustCallRange(t, "CDLABANDONEDBABY", oracle.Inputs{Open: o, High: h, Low: l, Close: c}, rg[0], rg[1], 0.3)
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("constructed range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("constructed range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLABANDONEDBABYOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLABANDONEDBABY", s.ohlcv(), rg[0], rg[1], 0.3)
		_, _, err := cdlabandonedbabyRange(rg[0], rg[1], s.O, s.H, s.L, s.C, 0.3, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLABANDONEDBABYOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		// Loose thresholds so the pattern fires on the random fixtures.
		{{BodyLong, RangeRealBody, 10, 0.3}, {BodyDoji, RangeHighLow, 10, 0.5}, {BodyShort, RangeRealBody, 10, 0.3}},
		{{BodyDoji, RangeHighLow, 5, 0.3}},
		{{BodyLong, RangeHighLow, 7, 0.4}},
		{{BodyShort, RangeShadows, 3, 0.3}},
		{{BodyDoji, RangeShadows, 20, 0.5}},
		{{BodyLong, RangeShadows, 0, 2.0}, {BodyShort, RangeRealBody, 0, 0.5}},
		// All periods 0: lookback 2.
		{{BodyLong, RangeRealBody, 0, 0.5}, {BodyDoji, RangeHighLow, 0, 0.3}, {BodyShort, RangeRealBody, 0, 0.5}},
		// Long periods.
		{{BodyLong, RangeRealBody, 50, 1.5}, {BodyDoji, RangeHighLow, 30, 0.1}},
	}
	total := 0
	for k, set := range sets {
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		for _, ch := range set {
			if err := SetCandleSettings(ch.st, ch.rt, ch.period, ch.factor); err != nil {
				t.Fatal(err)
			}
			oracle.SetCandleSettings(int(ch.st), int(ch.rt), ch.period, ch.factor)
		}
		total += cdlabandonedbabyOracleCompare(t, lbl("settings#", k), []float64{0, 0.3})
	}
	if total == 0 {
		t.Fatal("pattern never fired under modified settings")
	}
}
