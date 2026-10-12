//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlkickingOracleCompare compares CDLKICKING with C on every fixture and
// edge length under the current candle settings, and returns how many bars
// fired.
func cdlkickingOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLKICKINGLookback()
	if c := mustLookback(t, "CDLKICKING"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLKICKING(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLKICKING", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLKICKING ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLKICKINGOracle(t *testing.T) {
	// Kicking needs two marubozu and a full candle gap; with default settings
	// the random fixtures rarely (if ever) produce one. Firing is proven by
	// TestCDLKICKINGOracleConstructed (both signs, every boundary) and by
	// TestCDLKICKINGOracleSettings (loosened shadow/body settings).
	fired := cdlkickingOracleCompare(t, "default")
	t.Logf("default settings: %d bars fired", fired)
}

func TestCDLKICKINGOracleConstructed(t *testing.T) {
	signs := map[int]bool{}
	for _, tc := range cdlkickingCases {
		o, h, l, c := cdlkickingSeries(tc.b1, tc.b2)
		got, err := CDLKICKING(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLKICKING", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, "constructed "+tc.name, got, want)
		if want[11] != tc.want {
			t.Fatalf("%s: C gave %d at bar 11, want %d", tc.name, want[11], tc.want)
		}
		signs[want[11]] = true
	}
	if !signs[100] || !signs[-100] {
		t.Fatalf("constructed cases did not fire both signs: %v", signs)
	}
}

func TestCDLKICKINGOracleRanges(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	cdlkickingOracleRanges(t, "default")
	// Loose marubozu definition so the compared sub-ranges contain hits
	// (lookback 6).
	for _, st := range []struct {
		st     CandleSettingType
		factor float64
	}{{BodyLong, 0.1}, {ShadowVeryShort, 5.0}} {
		if err := SetCandleSettings(st.st, RangeHighLow, 5, st.factor); err != nil {
			t.Fatal(err)
		}
		oracle.SetCandleSettings(int(st.st), int(RangeHighLow), 5, st.factor)
	}
	if fired := cdlkickingOracleRanges(t, "loose"); fired == 0 {
		t.Fatal("loose settings: no hits in the compared ranges")
	}
}

func cdlkickingOracleRanges(t *testing.T, tag string) int {
	t.Helper()
	fired := 0
	s := fixtures(300)[11] // "gaps": frequent candle gaps
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 5}, {0, 6}, {5, 5}, {6, 6}, {0, 10}, {0, 11}, {10, 10}, {11, 11}, {12, 12}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdlkickingRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLKICKING", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLKICKING range ", tag, " ", rg), out[:nb], r.Outputs[0].Int[:nb])
		for _, v := range out[:nb] {
			if v != 0 {
				fired++
			}
		}
	}
	return fired
}

func TestCDLKICKINGOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLKICKING", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlkickingRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLKICKINGOracleSettings(t *testing.T) {
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
		{{ShadowVeryShort, RangeRealBody, 7, 0.4}},
		{{BodyLong, RangeShadows, 3, 0.3}},
		{{ShadowVeryShort, RangeShadows, 20, 1.5}},
		{{BodyLong, RangeRealBody, 0, 0.5}},
		{{ShadowVeryShort, RangeHighLow, 0, 0.2}},
		// All periods 0: lookback 1.
		{{BodyLong, RangeRealBody, 0, 0.8}, {ShadowVeryShort, RangeHighLow, 0, 0.5}},
		// Long periods.
		{{BodyLong, RangeRealBody, 50, 1.5}, {ShadowVeryShort, RangeShadows, 30, 1.0}},
		// Loose marubozu definition so the pattern fires on the fixtures.
		{{BodyLong, RangeRealBody, 10, 0.3}, {ShadowVeryShort, RangeHighLow, 10, 2.0}},
		{{BodyLong, RangeHighLow, 5, 0.1}, {ShadowVeryShort, RangeHighLow, 5, 5.0}},
	}
	fired := 0
	for k, set := range sets {
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		for _, ch := range set {
			if err := SetCandleSettings(ch.st, ch.rt, ch.period, ch.factor); err != nil {
				t.Fatal(err)
			}
			oracle.SetCandleSettings(int(ch.st), int(ch.rt), ch.period, ch.factor)
		}
		f := cdlkickingOracleCompare(t, lbl("settings#", k))
		t.Logf("settings#%d: %d bars fired", k, f)
		fired += f
	}
	if fired == 0 {
		t.Fatal("pattern never fired under modified settings")
	}
}
