//go:build oracle

package ta

import (
	"math"
	"math/rand/v2"
	"testing"

	"tradeloom/ta/internal/oracle"
)

var cdlmatholdOraclePens = []float64{0, 0.2, 0.5, 1, 3e37}

// cdlmatholdOracleCompare compares CDLMATHOLD with C on every fixture and
// edge length under the current candle settings, and returns how many bars
// fired.
func cdlmatholdOracleCompare(t *testing.T, tag string, pens []float64) int {
	t.Helper()
	fired := 0
	for _, p := range pens {
		lb := CDLMATHOLDLookback(p)
		if c := mustLookback(t, "CDLMATHOLD", p); lb != c {
			t.Fatalf("%s lookback p=%g: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := CDLMATHOLD(s.O, s.H, s.L, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "CDLMATHOLD", s.ohlcv(), p)
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertInt(t, lbl("CDLMATHOLD ", tag, " ", s.Name, " n=", n, " p=", p), got, r.AlignedInt(0, n))
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

func TestCDLMATHOLDOracle(t *testing.T) {
	fired := cdlmatholdOracleCompare(t, "default", cdlmatholdOraclePens)
	t.Logf("fired %d times on the fixtures", fired)
}

// TestCDLMATHOLDOracleConstructed checks the hand-built pattern, which must
// fire in both Go and C, and its penetration boundaries.
func TestCDLMATHOLDOracleConstructed(t *testing.T) {
	o, h, l, c := cdlmatholdSeries(cdlmatholdPattern[:]...)
	for _, p := range []float64{0, 0.1, 0.15, 0.2, 0.5, 1, 3e37} {
		got, err := CDLMATHOLD(o, h, l, c, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLMATHOLD", oracle.Inputs{Open: o, High: h, Low: l, Close: c}, p)
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed p=", p), got, want)
		if p == 0.5 && want[18] != 100 {
			t.Fatalf("C did not fire on the constructed pattern: %v", want)
		}
	}
}

// TestCDLMATHOLDOracleThreshold puts the 4th candle's low body edge within a
// few ulps of the penetration threshold 1stClose - |1stBody|*p for random
// penetrations, so the exact rounding of C's unfused threshold is compared.
func TestCDLMATHOLDOracleThreshold(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 11))
	fired, total := 0, 0
	for k := 0; k < 400; k++ {
		p := 0.05 + rng.Float64()*0.4 // threshold inside (102.2, 103.8)
		b := cdlmatholdPattern
		body := 4 + rng.Float64()*0.5
		b[0].o = b[0].c - body
		b[0].l = b[0].o - 0.5
		th := b[0].c - float64(body*p)
		for d := -3; d <= 3; d++ {
			x := th
			for range cdlmatholdAbs(d) {
				if d < 0 {
					x = math.Nextafter(x, math.Inf(-1))
				} else {
					x = math.Nextafter(x, math.Inf(1))
				}
			}
			bb := b
			// 4th: black, close x, body 0.3; 3rd keeps a higher body.
			bb[3] = cdlmatholdBar{x + 0.3, x + 0.4, x - 0.2, x}
			bb[2] = cdlmatholdBar{x + 0.5, x + 0.6, x + 0.1, x + 0.2}
			if bb[2].o >= bb[1].o || cMin(bb[2].o, bb[2].c) >= bb[0].c {
				continue
			}
			bb[4].o = x + 0.1
			o, h, l, c := cdlmatholdSeries(bb[:]...)
			got, err := CDLMATHOLD(o, h, l, c, p)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "CDLMATHOLD", oracle.Inputs{Open: o, High: h, Low: l, Close: c}, p)
			assertInt(t, lbl("threshold k=", k, " d=", d, " p=", p), got, r.AlignedInt(0, len(o)))
			total++
			if got[18] != 0 {
				fired++
			}
		}
	}
	if fired == 0 || fired == total {
		t.Fatalf("threshold cases did not straddle the boundary: fired %d of %d", fired, total)
	}
}

func cdlmatholdAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func TestCDLMATHOLDOracleBadParams(t *testing.T) {
	s := fixtures(30)[0]
	// -4e37 is TA_REAL_DEFAULT, which C substitutes; Go has no sentinel, so
	// it is not compared here.
	for _, p := range []float64{-1, -1e-300, math.NaN(), math.Inf(1), math.Inf(-1), 3.0000001e37, 0, 3e37, 0.5} {
		r := mustCall(t, "CDLMATHOLD", s.ohlcv(), p)
		_, err := CDLMATHOLD(s.O, s.H, s.L, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%g: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		lbErr := CDLMATHOLDLookback(p) == -1
		if c := mustLookback(t, "CDLMATHOLD", p); (c == -1) != lbErr {
			t.Errorf("p=%g lookback: C %d, Go %d", p, c, CDLMATHOLDLookback(p))
		}
	}
}

func TestCDLMATHOLDOracleRanges(t *testing.T) {
	s := fixtures(300)[11] // gaps
	n := len(s.C)
	for _, p := range []float64{0, 0.5} {
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 13}, {0, 14}, {14, 14}, {10, 50}, {15, 15}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlmatholdRange(rg[0], rg[1], s.O, s.H, s.L, s.C, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLMATHOLD", s.ohlcv(), rg[0], rg[1], p)
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%g: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLMATHOLD range ", rg, " p=", p), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
	// A sub-range over the constructed pattern.
	o, h, l, c := cdlmatholdSeries(cdlmatholdPattern[:]...)
	for _, rg := range [][2]int{{18, 18}, {17, 18}, {0, 18}, {14, 17}} {
		out := make([]int, len(o))
		beg, nb, _ := cdlmatholdRange(rg[0], rg[1], o, h, l, c, 0.5, out)
		r := mustCallRange(t, "CDLMATHOLD", oracle.Inputs{Open: o, High: h, Low: l, Close: c}, rg[0], rg[1], 0.5)
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("constructed range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("constructed range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLMATHOLDOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLMATHOLD", s.ohlcv(), rg[0], rg[1], 0.5)
		_, _, err := cdlmatholdRange(rg[0], rg[1], s.O, s.H, s.L, s.C, 0.5, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLMATHOLDOracleSettings(t *testing.T) {
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
		{{BodyLong, RangeRealBody, 10, 0.3}, {BodyShort, RangeRealBody, 10, 3}},
		{{BodyLong, RangeHighLow, 5, 0.2}, {BodyShort, RangeHighLow, 5, 2}},
		{{BodyShort, RangeShadows, 3, 2}},
		{{BodyLong, RangeShadows, 7, 0.5}},
		{{BodyLong, RangeShadows, 0, 2.0}, {BodyShort, RangeRealBody, 0, 3}},
		// Both periods 0: lookback 4.
		{{BodyLong, RangeRealBody, 0, 0.5}, {BodyShort, RangeRealBody, 0, 2}},
		// Long and unequal periods.
		{{BodyLong, RangeRealBody, 50, 0.5}, {BodyShort, RangeHighLow, 3, 1}},
		{{BodyLong, RangeRealBody, 2, 0.5}, {BodyShort, RangeRealBody, 30, 2}},
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
		total += cdlmatholdOracleCompare(t, lbl("settings#", k), []float64{0.5, 1, 3e37})
	}
	t.Logf("fired %d times under modified settings", total)
	if total == 0 {
		t.Fatal("pattern never fired under modified settings")
	}
}
