//go:build oracle

package ta

import (
	"math"
	"math/rand/v2"
	"testing"

	"tradeloom/ta/internal/oracle"
)

var cdlmorningdojistarOraclePens = []float64{0, 0.3, 0.5, 1, 3e37}

// cdlmorningdojistarOracleCompare compares CDLMORNINGDOJISTAR with C on every
// fixture and edge length under the current candle settings, and returns how
// many bars fired.
func cdlmorningdojistarOracleCompare(t *testing.T, tag string, pens []float64) int {
	t.Helper()
	fired := 0
	for _, p := range pens {
		lb := CDLMORNINGDOJISTARLookback(p)
		if c := mustLookback(t, "CDLMORNINGDOJISTAR", p); lb != c {
			t.Fatalf("%s lookback p=%g: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := CDLMORNINGDOJISTAR(s.O, s.H, s.L, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "CDLMORNINGDOJISTAR", s.ohlcv(), p)
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertInt(t, lbl("CDLMORNINGDOJISTAR ", tag, " ", s.Name, " n=", n, " p=", p), got, r.AlignedInt(0, n))
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

func TestCDLMORNINGDOJISTAROracle(t *testing.T) {
	fired := cdlmorningdojistarOracleCompare(t, "default", cdlmorningdojistarOraclePens)
	t.Logf("fired %d times on the fixtures", fired)
}

// TestCDLMORNINGDOJISTAROracleConstructed checks the hand-built star, which
// must fire in both Go and C, and its penetration boundaries.
func TestCDLMORNINGDOJISTAROracleConstructed(t *testing.T) {
	o, h, l, c := cdlmorningdojistarSeries(cdlmorningdojistarStar[:]...)
	for _, p := range []float64{0, 0.3, 0.45, 0.5, 0.55, 1, 3e37} {
		got, err := CDLMORNINGDOJISTAR(o, h, l, c, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLMORNINGDOJISTAR", oracle.Inputs{Open: o, High: h, Low: l, Close: c}, p)
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed p=", p), got, want)
		if p == 0.3 && want[14] != 100 {
			t.Fatalf("C did not fire on the constructed star: %v", want)
		}
	}
}

// TestCDLMORNINGDOJISTAROracleThreshold puts the 3rd close within a few ulps
// of the penetration threshold fma(|body1|, p, c1) for random penetrations, so
// C's single-rounding fma() is compared with math.FMA. It also counts the
// cases where an unfused c1 + |body1|*p would round differently, to show the
// test can tell the two apart.
func TestCDLMORNINGDOJISTAROracleThreshold(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	fired, differs := 0, 0
	for k := 0; k < 400; k++ {
		p := rng.Float64()
		bodyIn := 2 + rng.Float64()*6 // 1st body in (2, 8), well above avg 1
		b := cdlmorningdojistarStar
		b[0].o = b[0].c + bodyIn
		b[0].h = b[0].o + 0.5
		body := math.Abs(b[0].c - b[0].o)
		th := math.FMA(body, p, b[0].c)
		if th != b[0].c+float64(body*p) {
			differs++
		}
		for d := -3; d <= 3; d++ {
			cl := th
			for range cdlmorningdojistarAbs(d) {
				if d < 0 {
					cl = math.Nextafter(cl, math.Inf(-1))
				} else {
					cl = math.Nextafter(cl, math.Inf(1))
				}
			}
			bb := b
			bb[2] = cdlmorningdojistarBar{cl - 2.5, cl + 0.1, cl - 2.6, cl} // white, body 2.5
			o, h, l, c := cdlmorningdojistarSeries(bb[:]...)
			got, err := CDLMORNINGDOJISTAR(o, h, l, c, p)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "CDLMORNINGDOJISTAR", oracle.Inputs{Open: o, High: h, Low: l, Close: c}, p)
			assertInt(t, lbl("threshold k=", k, " d=", d, " p=", p), got, r.AlignedInt(0, len(o)))
			if got[14] != 0 {
				fired++
			}
		}
	}
	if fired == 0 {
		t.Fatal("threshold cases never fired")
	}
	t.Logf("fired %d times; fused and unfused thresholds differ in %d of 400 cases", fired, differs)
}

func cdlmorningdojistarAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func TestCDLMORNINGDOJISTAROracleBadParams(t *testing.T) {
	s := fixtures(30)[0]
	// -4e37 is TA_REAL_DEFAULT, which C substitutes; Go has no sentinel, so
	// it is not compared here.
	for _, p := range []float64{-1, -1e-300, math.NaN(), math.Inf(1), math.Inf(-1), 3.0000001e37, 0, 3e37, 0.3} {
		r := mustCall(t, "CDLMORNINGDOJISTAR", s.ohlcv(), p)
		_, err := CDLMORNINGDOJISTAR(s.O, s.H, s.L, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%g: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		lbErr := CDLMORNINGDOJISTARLookback(p) == -1
		if c := mustLookback(t, "CDLMORNINGDOJISTAR", p); (c == -1) != lbErr {
			t.Errorf("p=%g lookback: C %d, Go %d", p, c, CDLMORNINGDOJISTARLookback(p))
		}
	}
}

func TestCDLMORNINGDOJISTAROracleRanges(t *testing.T) {
	s := fixtures(300)[11] // gaps
	n := len(s.C)
	for _, p := range []float64{0, 0.3} {
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 11}, {0, 12}, {12, 12}, {10, 50}, {13, 13}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlmorningdojistarRange(rg[0], rg[1], s.O, s.H, s.L, s.C, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLMORNINGDOJISTAR", s.ohlcv(), rg[0], rg[1], p)
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%g: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLMORNINGDOJISTAR range ", rg, " p=", p), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
	// Sub-ranges over the constructed star.
	o, h, l, c := cdlmorningdojistarSeries(cdlmorningdojistarStar[:]...)
	for _, rg := range [][2]int{{14, 14}, {13, 14}, {0, 14}, {12, 13}} {
		out := make([]int, len(o))
		beg, nb, _ := cdlmorningdojistarRange(rg[0], rg[1], o, h, l, c, 0.3, out)
		r := mustCallRange(t, "CDLMORNINGDOJISTAR", oracle.Inputs{Open: o, High: h, Low: l, Close: c}, rg[0], rg[1], 0.3)
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("constructed range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("constructed range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLMORNINGDOJISTAROracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLMORNINGDOJISTAR", s.ohlcv(), rg[0], rg[1], 0.3)
		_, _, err := cdlmorningdojistarRange(rg[0], rg[1], s.O, s.H, s.L, s.C, 0.3, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLMORNINGDOJISTAROracleSettings(t *testing.T) {
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
		total += cdlmorningdojistarOracleCompare(t, lbl("settings#", k), []float64{0, 0.3})
	}
	if total == 0 {
		t.Fatal("pattern never fired under modified settings")
	}
	t.Logf("fired %d times under modified settings", total)
}
