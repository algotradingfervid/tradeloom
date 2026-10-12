//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlmatchinglowOracleCompare compares CDLMATCHINGLOW with C on every fixture
// and edge length under the current candle settings, and returns how many
// bars fired.
func cdlmatchinglowOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLMATCHINGLOWLookback()
	if c := mustLookback(t, "CDLMATCHINGLOW"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLMATCHINGLOW(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLMATCHINGLOW", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLMATCHINGLOW ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLMATCHINGLOWOracle(t *testing.T) {
	fired := cdlmatchinglowOracleCompare(t, "default")
	if fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
	t.Logf("default: fired %d", fired)
}

func TestCDLMATCHINGLOWOracleConstructed(t *testing.T) {
	type B = cdlmatchinglowBar
	b1, b2 := cdlmatchinglowB1, cdlmatchinglowB2
	o, h, l, c := cdlmatchinglowSeries(b1, b2)
	r := mustCall(t, "CDLMATCHINGLOW", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	if want := r.AlignedInt(0, len(o)); want[11] != 100 {
		t.Fatalf("C did not fire on the constructed Matching Low: %v", want)
	}
	for k, bars := range [][2]B{
		{b1, b2},
		{b1, {104, 104.5, 99.5, 100.25}},
		{b1, {104, 104.5, 99.5, 100.26}},
		{b1, {104, 104.5, 99.5, 99.75}},
		{b1, {104, 104.5, 99.5, 99.74}},
		{{100, 106, 99.5, 105}, b2},
		{{100, 106, 99.5, 100}, b2},
		{b1, {99.9, 104.5, 99.5, 100}},
		{b1, {100, 104.5, 99.5, 100}},
		{{100.5, 100.5, 100, 100}, {140, 150, 50, 100.1}},
		{{150, 200, 0, 100}, {104, 104.5, 99.5, 100.26}},
	} {
		o, h, l, c := cdlmatchinglowSeries(bars[0], bars[1])
		got, _ := CDLMATCHINGLOW(o, h, l, c)
		r := mustCall(t, "CDLMATCHINGLOW", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		assertInt(t, lbl("constructed #", k), got, r.AlignedInt(0, len(o)))
	}
}

func TestCDLMATCHINGLOWOracleRanges(t *testing.T) {
	for _, s := range fixtures(300) {
		n := len(s.C)
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 5}, {0, 6}, {5, 5}, {6, 6}, {7, 7}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlmatchinglowRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLMATCHINGLOW", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLMATCHINGLOW ", s.Name, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

func TestCDLMATCHINGLOWOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLMATCHINGLOW", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlmatchinglowRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLMATCHINGLOWOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	for k, chs := range [][]change{
		{{Equal, RangeHighLow, 0, 0.05}},
		{{Equal, RangeHighLow, 0, 0}},
		{{Equal, RangeHighLow, 1, 0.2}},
		{{Equal, RangeHighLow, 20, 0.1}},
		{{Equal, RangeHighLow, 50, 0.3}},
		{{Equal, RangeRealBody, 5, 0.2}},
		{{Equal, RangeRealBody, 0, 0.5}},
		{{Equal, RangeShadows, 5, 0.3}},
		{{Equal, RangeShadows, 0, 1.0}},
		{{BodyLong, RangeRealBody, 40, 1.0}, {Near, RangeHighLow, 30, 0.5}}, // unrelated settings
	} {
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		for _, ch := range chs {
			if err := SetCandleSettings(ch.st, ch.rt, ch.period, ch.factor); err != nil {
				t.Fatal(err)
			}
			oracle.SetCandleSettings(int(ch.st), int(ch.rt), ch.period, ch.factor)
		}
		fired := cdlmatchinglowOracleCompare(t, lbl("settings#", k))
		t.Logf("settings#%d: fired %d", k, fired)
	}
}
