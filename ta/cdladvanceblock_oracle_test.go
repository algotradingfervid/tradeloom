//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdladvanceblockOracleCompare compares CDLADVANCEBLOCK with C on every
// fixture and edge length under the current candle settings, and returns how
// many bars fired.
func cdladvanceblockOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLADVANCEBLOCKLookback()
	if c := mustLookback(t, "CDLADVANCEBLOCK"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLADVANCEBLOCK(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLADVANCEBLOCK", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLADVANCEBLOCK ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLADVANCEBLOCKOracle(t *testing.T) {
	fired := cdladvanceblockOracleCompare(t, "default")
	t.Logf("default settings: fired %d times", fired)
	if fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
}

func TestCDLADVANCEBLOCKOracleConstructed(t *testing.T) {
	// The hand-built patterns (one per weakening branch) must agree with C,
	// and C must fire on each.
	a := cdladvanceblockBar{100, 104.2, 99.8, 104}
	cases := []struct {
		b, c cdladvanceblockBar
		want int
	}{
		{cdladvanceblockBar{103, 106.2, 102.9, 106}, cdladvanceblockBar{105, 108, 104.9, 107}, -100},
		{cdladvanceblockBar{103, 106.2, 102.9, 106}, cdladvanceblockBar{105, 107.1, 104.9, 107}, 0},
		{cdladvanceblockBar{103, 104.6, 102.9, 104.5}, cdladvanceblockBar{104, 105.85, 103.9, 105.8}, -100},
		{cdladvanceblockBar{103, 106.1, 102.9, 106}, cdladvanceblockBar{105.5, 106.55, 105.4, 106.5}, -100},
		{cdladvanceblockBar{103, 107.2, 102.9, 107}, cdladvanceblockBar{105, 111.5, 104.9, 108}, -100},
		{cdladvanceblockBar{103, 107.2, 102.9, 107}, cdladvanceblockBar{105, 110.9, 104.9, 108}, 0},
	}
	for k, tc := range cases {
		o, h, l, c := cdladvanceblockSeries([3]cdladvanceblockBar{a, tc.b, tc.c})
		got, err := CDLADVANCEBLOCK(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLADVANCEBLOCK", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed#", k), got, want)
		if want[14] != tc.want {
			t.Fatalf("constructed#%d: C gave %d, want %d", k, want[14], tc.want)
		}
	}
}

func TestCDLADVANCEBLOCKOracleRanges(t *testing.T) {
	for _, s := range fixtures(300) {
		n := len(s.C)
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 11}, {0, 12}, {12, 12}, {11, 50}, {13, 13}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdladvanceblockRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLADVANCEBLOCK", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLADVANCEBLOCK ", s.Name, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

func TestCDLADVANCEBLOCKOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLADVANCEBLOCK", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdladvanceblockRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLADVANCEBLOCKOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		{{BodyLong, RangeHighLow, 5, 0.5}},
		{{ShadowLong, RangeShadows, 7, 0.4}},
		{{ShadowShort, RangeHighLow, 3, 0.3}},
		{{Near, RangeRealBody, 0, 1.0}},
		{{Near, RangeShadows, 20, 0.5}},
		{{Far, RangeRealBody, 0, 0.3}},
		{{Far, RangeShadows, 15, 1.0}},
		{{BodyLong, RangeShadows, 0, 0.5}, {ShadowShort, RangeRealBody, 0, 2.0}},
		// All periods 0: lookback 2.
		{{BodyLong, RangeRealBody, 0, 1.0}, {ShadowLong, RangeRealBody, 0, 1.0},
			{ShadowShort, RangeShadows, 0, 1.0}, {Near, RangeHighLow, 0, 0.2}, {Far, RangeHighLow, 0, 0.6}},
		// Long periods.
		{{BodyLong, RangeRealBody, 50, 1.5}, {ShadowLong, RangeShadows, 30, 1.0}},
		// Permissive: makes the pattern fire often on fixtures.
		{{BodyLong, RangeRealBody, 10, 0.1}, {ShadowShort, RangeShadows, 10, 5.0}, {Near, RangeHighLow, 5, 2.0}},
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
		f := cdladvanceblockOracleCompare(t, lbl("settings#", k))
		t.Logf("settings#%d: fired %d times", k, f)
		total += f
	}
	if total == 0 {
		t.Fatal("pattern never fired under any settings")
	}
}
