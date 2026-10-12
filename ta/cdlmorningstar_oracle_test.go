//go:build oracle

package ta

import (
	"math"
	"math/rand/v2"
	"testing"

	"tradeloom/ta/internal/oracle"
)

var cdlmorningstarOraclePens = []float64{0, 0.3, 0.5, 1, 3e37}

// cdlmorningstarOracleCompare compares CDLMORNINGSTAR with C on every fixture
// and edge length under the current candle settings, and returns how many
// bars fired.
func cdlmorningstarOracleCompare(t *testing.T, tag string, pens []float64) int {
	t.Helper()
	fired := 0
	for _, p := range pens {
		lb := CDLMORNINGSTARLookback(p)
		if c := mustLookback(t, "CDLMORNINGSTAR", p); lb != c {
			t.Fatalf("%s lookback p=%g: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := CDLMORNINGSTAR(s.O, s.H, s.L, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "CDLMORNINGSTAR", s.ohlcv(), p)
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertInt(t, lbl("CDLMORNINGSTAR ", tag, " ", s.Name, " n=", n, " p=", p), got, r.AlignedInt(0, n))
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

func TestCDLMORNINGSTAROracle(t *testing.T) {
	fired := cdlmorningstarOracleCompare(t, "default", cdlmorningstarOraclePens)
	t.Logf("fired %d times on the fixtures", fired)
	if fired == 0 {
		t.Fatal("pattern never fired on the fixtures with default settings")
	}
}

// TestCDLMORNINGSTAROracleConstructed checks the hand-built pattern, which
// must fire in both Go and C, and its penetration boundaries.
func TestCDLMORNINGSTAROracleConstructed(t *testing.T) {
	o, h, l, c := cdlmorningstarSeries(cdlmorningstarPattern[:]...)
	for _, p := range []float64{0, 0.3, 0.7, 0.75, 0.8, 3e37} {
		got, err := CDLMORNINGSTAR(o, h, l, c, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLMORNINGSTAR", oracle.Inputs{Open: o, High: h, Low: l, Close: c}, p)
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed p=", p), got, want)
		if p == 0.3 && want[14] != 100 {
			t.Fatalf("C did not fire on the constructed pattern: %v", want)
		}
	}
}

// TestCDLMORNINGSTAROracleThreshold puts the 3rd close within a few ulps of
// the penetration threshold for random penetrations, so the exact rounding of
// C's threshold (an explicit fma) is compared.
func TestCDLMORNINGSTAROracleThreshold(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 11))
	fired, total := 0, 0
	for k := 0; k < 400; k++ {
		p := rng.Float64()
		body := 2 + rng.Float64()*6 // 1st body in (2, 8), well above avg 1
		b := cdlmorningstarPattern
		b[0].o = 104 + rng.Float64()
		b[0].c = b[0].o - body
		b[0].h = b[0].o + 0.5
		b[0].l = b[0].c - 0.5
		b[1] = cdlmorningstarBar{b[0].c - 1, b[0].c - 0.8, b[0].c - 2, b[0].c - 1.5}
		// Centre on the unfused threshold: the fused one C computes differs
		// by at most an ulp, so both fall inside the +-3 ulp sweep.
		th := b[0].c + math.Abs(b[0].c-b[0].o)*p
		for d := -3; d <= 3; d++ {
			cl := th
			for j := 0; j < d; j++ {
				cl = math.Nextafter(cl, math.Inf(1))
			}
			for j := 0; j > d; j-- {
				cl = math.Nextafter(cl, math.Inf(-1))
			}
			bb := b
			bb[2] = cdlmorningstarBar{cl - 2.5, cl + 0.1, cl - 2.6, cl}
			o, h, l, c := cdlmorningstarSeries(bb[:]...)
			got, err := CDLMORNINGSTAR(o, h, l, c, p)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "CDLMORNINGSTAR", oracle.Inputs{Open: o, High: h, Low: l, Close: c}, p)
			assertInt(t, lbl("threshold k=", k, " d=", d, " p=", p), got, r.AlignedInt(0, len(o)))
			total++
			if got[14] != 0 {
				fired++
			}
		}
	}
	if fired == 0 || fired == total {
		t.Fatalf("threshold cases fired %d of %d; want a mix", fired, total)
	}
}

func TestCDLMORNINGSTAROracleBadParams(t *testing.T) {
	s := fixtures(30)[0]
	// -4e37 is TA_REAL_DEFAULT, which C substitutes; Go has no sentinel, so
	// it is not compared here.
	for _, p := range []float64{-1, -1e-300, math.NaN(), math.Inf(1), math.Inf(-1), 3.0000001e37, 0, 3e37, 0.3} {
		r := mustCall(t, "CDLMORNINGSTAR", s.ohlcv(), p)
		_, err := CDLMORNINGSTAR(s.O, s.H, s.L, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%g: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		lbErr := CDLMORNINGSTARLookback(p) == -1
		if c := mustLookback(t, "CDLMORNINGSTAR", p); (c == -1) != lbErr {
			t.Errorf("p=%g lookback: C %d, Go %d", p, c, CDLMORNINGSTARLookback(p))
		}
	}
}

func TestCDLMORNINGSTAROracleRanges(t *testing.T) {
	s := fixtures(300)[11] // gaps
	n := len(s.C)
	for _, p := range []float64{0, 0.3} {
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 11}, {0, 12}, {12, 12}, {10, 50}, {13, 13}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlmorningstarRange(rg[0], rg[1], s.O, s.H, s.L, s.C, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLMORNINGSTAR", s.ohlcv(), rg[0], rg[1], p)
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%g: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLMORNINGSTAR range ", rg, " p=", p), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
	// A sub-range over the constructed pattern.
	o, h, l, c := cdlmorningstarSeries(cdlmorningstarPattern[:]...)
	for _, rg := range [][2]int{{14, 14}, {13, 14}, {0, 14}, {12, 13}} {
		out := make([]int, len(o))
		beg, nb, _ := cdlmorningstarRange(rg[0], rg[1], o, h, l, c, 0.3, out)
		r := mustCallRange(t, "CDLMORNINGSTAR", oracle.Inputs{Open: o, High: h, Low: l, Close: c}, rg[0], rg[1], 0.3)
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("constructed range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("constructed range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLMORNINGSTAROracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLMORNINGSTAR", s.ohlcv(), rg[0], rg[1], 0.3)
		_, _, err := cdlmorningstarRange(rg[0], rg[1], s.O, s.H, s.L, s.C, 0.3, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLMORNINGSTAROracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		// Loose thresholds so the pattern fires often on the random fixtures.
		{{BodyLong, RangeRealBody, 10, 0.3}, {BodyShort, RangeRealBody, 10, 1.5}},
		{{BodyShort, RangeHighLow, 5, 0.3}},
		{{BodyLong, RangeHighLow, 7, 0.4}},
		{{BodyShort, RangeShadows, 3, 0.8}},
		{{BodyLong, RangeShadows, 20, 0.5}},
		{{BodyLong, RangeShadows, 0, 2.0}, {BodyShort, RangeRealBody, 0, 0.5}},
		// Both periods 0: lookback 2.
		{{BodyLong, RangeRealBody, 0, 0.5}, {BodyShort, RangeHighLow, 0, 0.6}},
		// Long periods, unequal.
		{{BodyLong, RangeRealBody, 50, 1.5}, {BodyShort, RangeRealBody, 30, 1}},
		{{BodyShort, RangeRealBody, 40, 1}},
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
		total += cdlmorningstarOracleCompare(t, lbl("settings#", k), []float64{0, 0.3})
	}
	if total == 0 {
		t.Fatal("pattern never fired under modified settings")
	}
}
