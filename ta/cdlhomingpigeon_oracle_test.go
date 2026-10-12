//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlhomingpigeonOracleCompare compares CDLHOMINGPIGEON with C on every
// fixture and edge length under the current candle settings, and returns how
// many bars fired.
func cdlhomingpigeonOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLHOMINGPIGEONLookback()
	if c := mustLookback(t, "CDLHOMINGPIGEON"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLHOMINGPIGEON(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLHOMINGPIGEON", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLHOMINGPIGEON ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLHOMINGPIGEONOracle(t *testing.T) {
	if fired := cdlhomingpigeonOracleCompare(t, "default"); fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
}

func TestCDLHOMINGPIGEONOracleConstructed(t *testing.T) {
	// The hand-built Homing Pigeon (and its near misses) must agree in Go and C.
	o, h, l, c := cdlhomingpigeonSeries(cdlhomingpigeonB1, cdlhomingpigeonB2)
	got, err := CDLHOMINGPIGEON(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CDLHOMINGPIGEON", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	want := r.AlignedInt(0, len(o))
	assertInt(t, "constructed", got, want)
	if want[11] != 100 {
		t.Fatalf("C did not fire on the constructed Homing Pigeon: %v", want)
	}
	type B = cdlhomingpigeonBar
	b1 := cdlhomingpigeonB1
	for k, bars := range [][2]B{
		{{104, 104.5, 101.5, 102}, {103.5, 103.8, 102.2, 102.5}},
		{{104.25, 104.5, 101.5, 102}, {103.5, 103.8, 102.2, 102.5}},
		{{100, 107.5, 99.5, 107}, cdlhomingpigeonB2},
		{b1, {103, 105.5, 102.5, 105}},
		{b1, {104, 105.5, 102.5, 104}},
		{b1, {104.5, 105, 101.5, 102}},
		{b1, {104.75, 105, 101.5, 102}},
		{b1, {107, 107.5, 104.5, 105}},
		{b1, {106.9, 107.5, 104.5, 105}},
		{b1, {102, 102.5, 99.5, 100}},
		{b1, {102, 102.5, 99.5, 100.5}},
	} {
		o, h, l, c := cdlhomingpigeonSeries(bars[0], bars[1])
		got, _ := CDLHOMINGPIGEON(o, h, l, c)
		r := mustCall(t, "CDLHOMINGPIGEON", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		assertInt(t, lbl("near miss #", k), got, r.AlignedInt(0, len(o)))
	}
}

func TestCDLHOMINGPIGEONOracleRanges(t *testing.T) {
	for _, s := range fixtures(300) {
		n := len(s.C)
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 10}, {0, 11}, {10, 10}, {11, 11}, {12, 12}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlhomingpigeonRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLHOMINGPIGEON", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLHOMINGPIGEON ", s.Name, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

func TestCDLHOMINGPIGEONOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLHOMINGPIGEON", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlhomingpigeonRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLHOMINGPIGEONOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	for k, chs := range [][]change{
		{{BodyLong, RangeRealBody, 0, 1.0}, {BodyShort, RangeRealBody, 0, 1.0}},
		{{BodyLong, RangeRealBody, 0, 0.5}, {BodyShort, RangeRealBody, 0, 1.0}},
		{{BodyLong, RangeRealBody, 1, 1.0}},
		{{BodyShort, RangeRealBody, 1, 1.0}},
		{{BodyLong, RangeRealBody, 5, 0.3}, {BodyShort, RangeRealBody, 20, 2.0}},
		{{BodyLong, RangeHighLow, 10, 0.4}, {BodyShort, RangeHighLow, 3, 0.5}},
		{{BodyLong, RangeHighLow, 0, 0.3}, {BodyShort, RangeShadows, 0, 1.0}},
		{{BodyLong, RangeShadows, 7, 0.5}, {BodyShort, RangeShadows, 12, 1.0}},
		{{BodyLong, RangeShadows, 0, 1.0}},
		{{BodyLong, RangeRealBody, 50, 0.5}},
		{{BodyShort, RangeRealBody, 50, 1.0}},
		{{BodyLong, RangeRealBody, 3, 1.0}, {BodyShort, RangeRealBody, 30, 1.0}},
	} {
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		for _, ch := range chs {
			if err := SetCandleSettings(ch.st, ch.rt, ch.period, ch.factor); err != nil {
				t.Fatal(err)
			}
			oracle.SetCandleSettings(int(ch.st), int(ch.rt), ch.period, ch.factor)
		}
		fired := cdlhomingpigeonOracleCompare(t, lbl("settings#", k))
		t.Logf("settings#%d: fired %d", k, fired)
	}
}
