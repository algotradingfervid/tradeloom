//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlstalledpatternOracleCompare compares CDLSTALLEDPATTERN with C on every
// fixture and edge length under the current candle settings, and returns how
// many bars fired.
func cdlstalledpatternOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLSTALLEDPATTERNLookback()
	if c := mustLookback(t, "CDLSTALLEDPATTERN"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLSTALLEDPATTERN(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLSTALLEDPATTERN", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLSTALLEDPATTERN ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLSTALLEDPATTERNOracle(t *testing.T) {
	fired := cdlstalledpatternOracleCompare(t, "default")
	t.Logf("default settings: fired %d times", fired)
	if fired == 0 {
		t.Fatal("pattern never fired on any fixture with default settings")
	}
}

func TestCDLSTALLEDPATTERNOracleConstructed(t *testing.T) {
	// The hand-built cases must agree with C, and C must give the derived answer.
	firedC := 0
	for _, tc := range cdlstalledpatternCases {
		o, h, l, c := cdlstalledpatternSeries([3]cdlstalledpatternBar{tc.a, tc.b, tc.c})
		got, err := CDLSTALLEDPATTERN(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLSTALLEDPATTERN", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, "constructed "+tc.name, got, want)
		if want[14] != tc.want {
			t.Fatalf("constructed %s: C gave %d, want %d", tc.name, want[14], tc.want)
		}
		if want[14] != 0 {
			firedC++
		}
	}
	if firedC == 0 {
		t.Fatal("C never fired on the constructed patterns")
	}
}

func TestCDLSTALLEDPATTERNOracleRanges(t *testing.T) {
	for _, s := range fixtures(300) {
		n := len(s.C)
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 11}, {0, 12}, {12, 12}, {11, 50}, {13, 13}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlstalledpatternRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLSTALLEDPATTERN", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLSTALLEDPATTERN ", s.Name, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

func TestCDLSTALLEDPATTERNOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLSTALLEDPATTERN", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlstalledpatternRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLSTALLEDPATTERNOracleSettings(t *testing.T) {
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
		{{BodyShort, RangeShadows, 7, 2.0}},
		{{ShadowVeryShort, RangeRealBody, 3, 0.3}},
		{{Near, RangeRealBody, 0, 1.0}},
		{{Near, RangeShadows, 20, 0.5}},
		{{BodyShort, RangeRealBody, 0, 1.0}},
		{{BodyLong, RangeShadows, 0, 0.5}, {ShadowVeryShort, RangeHighLow, 0, 2.0}},
		// All periods 0: lookback 2.
		{{BodyLong, RangeRealBody, 0, 1.0}, {BodyShort, RangeRealBody, 0, 1.0},
			{ShadowVeryShort, RangeShadows, 0, 1.0}, {Near, RangeHighLow, 0, 0.2}},
		// Long periods.
		{{BodyLong, RangeRealBody, 50, 1.5}, {BodyShort, RangeShadows, 30, 1.0}},
		// Permissive: makes the pattern fire often on fixtures.
		{{BodyLong, RangeRealBody, 10, 0.1}, {BodyShort, RangeRealBody, 10, 3.0},
			{ShadowVeryShort, RangeHighLow, 10, 2.0}, {Near, RangeHighLow, 5, 2.0}},
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
		f := cdlstalledpatternOracleCompare(t, lbl("settings#", k))
		t.Logf("settings#%d: fired %d times", k, f)
		total += f
	}
	if total == 0 {
		t.Fatal("pattern never fired on any fixture under any settings")
	}
}
