//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdllonglineOracleCompare compares CDLLONGLINE with C on every fixture and edge
// length under the current candle settings, and returns how many bars fired.
func cdllonglineOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLLONGLINELookback()
	if c := mustLookback(t, "CDLLONGLINE"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLLONGLINE(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLLONGLINE", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLLONGLINE ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for i, v := range got {
				// Fixtures other than the flat ones must fire somewhere too.
				if v != 0 && s.Name != "constant" && s.Name != "zeros" && i >= lb {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLLONGLINEOracle(t *testing.T) {
	if fired := cdllonglineOracleCompare(t, "default"); fired == 0 {
		t.Fatal("pattern never fired on any non-flat fixture")
	}
}

func TestCDLLONGLINEOracleConstructed(t *testing.T) {
	// Hand-built long lines and near-threshold bars must agree with C.
	o, h, l, c := cdllonglineBase(10)
	for _, b := range [][4]float64{
		{100, 104.5, 99.5, 104},       // white long line
		{104, 104.5, 99.5, 100},       // black long line
		{100, 105, 99.5, 104},         // upper shadow at the threshold
		{100, 102.5, 99.5, 102},       // body at the threshold
		{100, 104, 100, 104},          // marubozu
		{100.1, 100.4, 99.95, 100.35}, // inexact decimals near the thresholds
	} {
		o = append(o, b[0])
		h = append(h, b[1])
		l = append(l, b[2])
		c = append(c, b[3])
	}
	got, err := CDLLONGLINE(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CDLLONGLINE", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	want := r.AlignedInt(0, len(o))
	assertInt(t, "constructed", got, want)
	if want[10] != 100 || want[11] != -100 {
		t.Fatalf("C did not fire on the constructed long lines: %v", want)
	}
}

func TestCDLLONGLINEOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 9}, {0, 10}, {10, 10}, {9, 50}, {11, 11}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdllonglineRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLLONGLINE", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLLONGLINE range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLLONGLINEOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLLONGLINE", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdllonglineRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLLONGLINEOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		{{BodyLong, RangeRealBody, 0, 1.0}}, // body > body: never fires
		{{BodyLong, RangeHighLow, 0, 0.5}},
		{{BodyLong, RangeHighLow, 1, 1.0}},
		{{BodyLong, RangeShadows, 5, 1.5}},
		{{BodyLong, RangeRealBody, 50, 2.0}},
		{{ShadowShort, RangeHighLow, 10, 0.2}},
		{{ShadowShort, RangeRealBody, 3, 1.0}},
		{{ShadowShort, RangeShadows, 0, 3.0}},
		{{ShadowShort, RangeShadows, 20, 1.0}}, // shadow period > body period
		{{BodyLong, RangeHighLow, 7, 0.3}, {ShadowShort, RangeHighLow, 7, 0.5}},
		{{BodyLong, RangeHighLow, 0, 0.4}, {ShadowShort, RangeHighLow, 0, 0.3}},
		{{BodyLong, RangeRealBody, 10, 0}},
		// Unrelated setting changes must not matter.
		{{BodyShort, RangeShadows, 30, 2.0}},
		{{BodyDoji, RangeHighLow, 40, 0.5}},
	}
	for k, set := range sets {
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		for _, ch := range set {
			if err := SetCandleSettings(ch.st, ch.rt, ch.period, ch.factor); err != nil {
				t.Fatal(err)
			}
			oracle.SetCandleSettings(int(ch.st), int(ch.rt), ch.period, ch.factor)
		}
		cdllonglineOracleCompare(t, lbl("settings#", k))
	}
}
