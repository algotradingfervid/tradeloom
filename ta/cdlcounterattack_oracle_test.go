//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlcounterattackOracleCompare compares CDLCOUNTERATTACK with C on every
// fixture and edge length under the current candle settings, and returns how
// many bars fired.
func cdlcounterattackOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLCOUNTERATTACKLookback()
	if c := mustLookback(t, "CDLCOUNTERATTACK"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLCOUNTERATTACK(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLCOUNTERATTACK", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLCOUNTERATTACK ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLCOUNTERATTACKOracle(t *testing.T) {
	// With default settings the Equal band (5% of the average range) is too
	// narrow for the random fixtures to hit, so the default comparison is
	// all zeros in both Go and C. Firing is proven by
	// TestCDLCOUNTERATTACKOracleConstructed (both signs, band edges) and by
	// TestCDLCOUNTERATTACKOracleSettings (wide Equal band on the fixtures).
	fired := cdlcounterattackOracleCompare(t, "default")
	t.Logf("default settings: %d bars fired", fired)
}

func TestCDLCOUNTERATTACKOracleConstructed(t *testing.T) {
	// The hand-built patterns, including the exact Equal band edges, must
	// agree with C, and fire with both signs.
	eq := cdlcounterattackEq()
	up, dn := 100+eq, 100-eq
	type b = cdlcounterattackBar
	cases := []struct {
		b1, b2 b
		want   int
	}{
		{b{106, 100}, b{95, 100}, 100},
		{b{100, 106}, b{111, 106}, -100},
		{b{106, 100}, b{95, up}, 100},
		{b{106, 100}, b{95, dn}, 100},
		{b{106, 100}, b{95, math.Nextafter(up, math.Inf(1))}, 0},
		{b{106, 100}, b{95, math.Nextafter(dn, math.Inf(-1))}, 0},
		{b{94, 100}, b{105, up}, -100},
		{b{112, 100}, b{97, 100}, 0},
		{b{112, 100}, b{96.5, 100}, 100},
		{b{102, 100}, b{95, 100}, 0},
	}
	for k, tc := range cases {
		o, h, l, c := cdlcounterattackSeries(tc.b1, tc.b2)
		got, err := CDLCOUNTERATTACK(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLCOUNTERATTACK", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed#", k), got, want)
		if want[11] != tc.want {
			t.Fatalf("case %d: C gave %d at bar 11, want %d", k, want[11], tc.want)
		}
	}
}

func TestCDLCOUNTERATTACKOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 10}, {0, 11}, {10, 10}, {11, 11}, {12, 12}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdlcounterattackRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLCOUNTERATTACK", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLCOUNTERATTACK range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLCOUNTERATTACKOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLCOUNTERATTACK", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlcounterattackRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLCOUNTERATTACKOracleSettings(t *testing.T) {
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
		{{Equal, RangeRealBody, 7, 0.4}},
		{{BodyLong, RangeShadows, 3, 0.3}},
		{{Equal, RangeShadows, 20, 1.5}},
		{{BodyLong, RangeRealBody, 0, 0.5}},
		{{Equal, RangeHighLow, 0, 0.2}},
		// All periods 0: lookback 1.
		{{BodyLong, RangeRealBody, 0, 0.8}, {Equal, RangeHighLow, 0, 0.5}},
		// Long periods.
		{{BodyLong, RangeRealBody, 50, 1.5}, {Equal, RangeShadows, 30, 1.0}},
		// Wide Equal band so the pattern fires often.
		{{BodyLong, RangeRealBody, 10, 0.5}, {Equal, RangeHighLow, 5, 1.0}},
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
		fired += cdlcounterattackOracleCompare(t, lbl("settings#", k))
	}
	if fired == 0 {
		t.Fatal("pattern never fired under modified settings")
	}
}
