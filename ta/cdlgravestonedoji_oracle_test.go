//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlgravestonedojiOracleCompare compares CDLGRAVESTONEDOJI with C on every
// fixture and edge length under the current candle settings, and returns how
// many bars fired.
func cdlgravestonedojiOracleCompare(t *testing.T, tag string) (hits int) {
	t.Helper()
	lb := CDLGRAVESTONEDOJILookback()
	if c := mustLookback(t, "CDLGRAVESTONEDOJI"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLGRAVESTONEDOJI(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLGRAVESTONEDOJI", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLGRAVESTONEDOJI ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v == 100 {
					hits++
				}
			}
		}
	}
	return hits
}

func TestCDLGRAVESTONEDOJIOracle(t *testing.T) {
	if hits := cdlgravestonedojiOracleCompare(t, "default"); hits == 0 {
		t.Fatal("pattern never fired on the fixtures with default settings")
	}
}

func TestCDLGRAVESTONEDOJIOracleConstructed(t *testing.T) {
	// Hand-built gravestone dojis (and near misses) must agree in Go and C.
	o, h, l, c := cdlgravestonedojiBase(10)
	o = append(o, 100, 100, 100.5, 100, 100, 100)
	h = append(h, 105, 105, 105, 105, 101, 101.5)
	l = append(l, 100, 100, 100, 99, 100, 100)
	c = append(c, 100, 101.5, 100, 100, 100, 100)
	got, err := CDLGRAVESTONEDOJI(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CDLGRAVESTONEDOJI", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	want := r.AlignedInt(0, len(o))
	assertInt(t, "constructed", got, want)
	if want[10] != 100 || want[12] != 100 || want[11] != 0 || want[13] != 0 {
		t.Fatalf("C output on the constructed bars unexpected: %v", want)
	}
}

func TestCDLGRAVESTONEDOJIOracleRanges(t *testing.T) {
	for _, s := range fixtures(300) {
		n := len(s.C)
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 9}, {0, 10}, {9, 9}, {10, 10}, {11, 11}, {9, 50}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlgravestonedojiRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLGRAVESTONEDOJI", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLGRAVESTONEDOJI ", s.Name, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

func TestCDLGRAVESTONEDOJIOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLGRAVESTONEDOJI", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlgravestonedojiRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLGRAVESTONEDOJIOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		{{BodyDoji, RangeRealBody, 5, 0.5}},
		{{BodyDoji, RangeShadows, 7, 1.5}},
		{{ShadowVeryShort, RangeShadows, 3, 0.3}},
		{{ShadowVeryShort, RangeRealBody, 20, 0.2}},
		{{BodyDoji, RangeHighLow, 0, 0.1}},
		{{ShadowVeryShort, RangeHighLow, 0, 0.2}},
		// Both periods 0: lookback 0.
		{{BodyDoji, RangeHighLow, 0, 0.1}, {ShadowVeryShort, RangeHighLow, 0, 0.1}},
		{{BodyDoji, RangeShadows, 0, 1.0}, {ShadowVeryShort, RangeShadows, 0, 0.5}},
		// Long periods, generous thresholds (makes the pattern fire on fixtures).
		{{BodyDoji, RangeHighLow, 50, 0.5}, {ShadowVeryShort, RangeHighLow, 30, 0.5}},
		// Unrelated setting changed: no effect.
		{{BodyLong, RangeHighLow, 40, 2.0}},
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
		total += cdlgravestonedojiOracleCompare(t, lbl("settings#", k))
	}
	if total == 0 {
		t.Fatal("pattern never fired on fixtures under any settings")
	}
}
