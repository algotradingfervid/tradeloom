//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlsticksandwichOracleCompare compares CDLSTICKSANDWICH with C on every
// fixture and edge length under the current candle settings, and returns how
// many bars fired.
func cdlsticksandwichOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLSTICKSANDWICHLookback()
	if c := mustLookback(t, "CDLSTICKSANDWICH"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLSTICKSANDWICH(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLSTICKSANDWICH", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLSTICKSANDWICH ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLSTICKSANDWICHOracle(t *testing.T) {
	fired := cdlsticksandwichOracleCompare(t, "default")
	if fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
	t.Logf("default: fired %d", fired)
}

func TestCDLSTICKSANDWICHOracleConstructed(t *testing.T) {
	type B = cdlsticksandwichBar
	b1, b2, b3 := cdlsticksandwichB1, cdlsticksandwichB2, cdlsticksandwichB3
	o, h, l, c := cdlsticksandwichSeries(b1, b2, b3)
	r := mustCall(t, "CDLSTICKSANDWICH", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	if want := r.AlignedInt(0, len(o)); want[12] != 100 {
		t.Fatalf("C did not fire on the constructed Stick Sandwich: %v", want)
	}
	for k, bars := range [][]B{
		{b1, b2, b3},
		{b1, b2, {104, 104.5, 99.5, 100.25}},
		{b1, b2, {104, 104.5, 99.5, 100.26}},
		{b1, b2, {104, 104.5, 99.5, 99.75}},
		{b1, b2, {104, 104.5, 99.5, 99.74}},
		{{99.5, 106, 99, 100}, b2, b3},
		{{100, 106, 99.5, 100}, b2, b3},
		{b1, {103, 104, 100.5, 101}, b3},
		{b1, {102, 104, 100.5, 102}, b3},
		{b1, {101, 104, 100, 103}, b3},
		{b1, {101, 104, 100.01, 103}, b3},
		{b1, {101, 104, 99, 103}, b3},
		{b1, b2, {99.9, 104.5, 99.5, 100}},
		{b1, b2, {100, 104.5, 99.5, 100}},
		{{100.5, 100.5, 100, 100}, {101, 180, 100.5, 101}, {140, 150, 50, 100.1}},
		{{150, 200, 0, 100}, b2, {104, 104.5, 99.5, 100.26}},
		{b1, b2, b3, b2, b3},
	} {
		o, h, l, c := cdlsticksandwichSeries(bars...)
		got, _ := CDLSTICKSANDWICH(o, h, l, c)
		r := mustCall(t, "CDLSTICKSANDWICH", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		assertInt(t, lbl("constructed #", k), got, r.AlignedInt(0, len(o)))
	}
}

func TestCDLSTICKSANDWICHOracleRanges(t *testing.T) {
	for _, s := range fixtures(300) {
		n := len(s.C)
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 6}, {0, 7}, {6, 6}, {7, 7}, {8, 8}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlsticksandwichRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLSTICKSANDWICH", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLSTICKSANDWICH ", s.Name, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
	// Sub-ranges over a series with chained firing bars.
	o, h, l, c := cdlsticksandwichSeries(cdlsticksandwichB1, cdlsticksandwichB2, cdlsticksandwichB3, cdlsticksandwichB2, cdlsticksandwichB3)
	in := oracle.Inputs{Open: o, High: h, Low: l, Close: c}
	for _, rg := range [][2]int{{0, 14}, {12, 12}, {12, 14}, {13, 14}, {14, 14}, {8, 12}} {
		out := make([]int, len(o))
		beg, nb, err := cdlsticksandwichRange(rg[0], rg[1], o, h, l, c, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLSTICKSANDWICH", in, rg[0], rg[1])
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("constructed range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("constructed range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLSTICKSANDWICHOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLSTICKSANDWICH", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlsticksandwichRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLSTICKSANDWICHOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	total := 0
	for k, chs := range [][]change{
		{{Equal, RangeHighLow, 0, 0.05}},
		{{Equal, RangeHighLow, 0, 0}},
		{{Equal, RangeHighLow, 1, 0.2}},
		{{Equal, RangeHighLow, 20, 0.1}},
		{{Equal, RangeHighLow, 50, 0.3}},
		{{Equal, RangeHighLow, 5, 2.0}},
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
		fired := cdlsticksandwichOracleCompare(t, lbl("settings#", k))
		total += fired
		t.Logf("settings#%d: fired %d", k, fired)
	}
	if total == 0 {
		t.Fatal("pattern never fired on any fixture under any setting")
	}
}
