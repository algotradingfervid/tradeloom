//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlkickingbylengthOracleCompare compares CDLKICKINGBYLENGTH with C on every
// fixture and edge length under the current candle settings, and returns how
// many bars fired.
func cdlkickingbylengthOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLKICKINGBYLENGTHLookback()
	if c := mustLookback(t, "CDLKICKINGBYLENGTH"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLKICKINGBYLENGTH(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLKICKINGBYLENGTH", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLKICKINGBYLENGTH ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLKICKINGBYLENGTHOracle(t *testing.T) {
	// Default marubozu shadows (10% of the average range) are rarely met by
	// the random fixtures; firing is proven by the constructed cases and by
	// the loosened settings below.
	fired := cdlkickingbylengthOracleCompare(t, "default")
	t.Logf("default settings: %d bars fired", fired)
}

func TestCDLKICKINGBYLENGTHOracleConstructed(t *testing.T) {
	pos, neg := 0, 0
	for _, tc := range cdlkickingbylengthCases() {
		o, h, l, c := cdlkickingbylengthSeries(tc.b1, tc.b2)
		got, err := CDLKICKINGBYLENGTH(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLKICKINGBYLENGTH", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, "constructed "+tc.name, got, want)
		if want[11] != tc.want {
			t.Fatalf("%s: C gave %d at bar 11, want %d", tc.name, want[11], tc.want)
		}
		switch want[11] {
		case 100:
			pos++
		case -100:
			neg++
		}
	}
	if pos == 0 || neg == 0 {
		t.Fatalf("constructed cases fired +%d / -%d", pos, neg)
	}
}

func TestCDLKICKINGBYLENGTHOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 10}, {0, 11}, {10, 10}, {11, 11}, {12, 12}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdlkickingbylengthRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLKICKINGBYLENGTH", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLKICKINGBYLENGTH range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLKICKINGBYLENGTHOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLKICKINGBYLENGTH", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlkickingbylengthRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLKICKINGBYLENGTHOracleSettings(t *testing.T) {
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
		f := cdlkickingbylengthOracleCompare(t, lbl("settings#", k))
		t.Logf("settings#%d: %d bars fired", k, f)
		fired += f
	}
	if fired == 0 {
		t.Fatal("pattern never fired under modified settings")
	}
}
