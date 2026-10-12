//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlbeltholdOracleCompare compares CDLBELTHOLD with C on every fixture and edge
// length under the current candle settings, and returns how many bars fired.
func cdlbeltholdOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLBELTHOLDLookback()
	if c := mustLookback(t, "CDLBELTHOLD"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLBELTHOLD(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLBELTHOLD", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLBELTHOLD ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLBELTHOLDOracle(t *testing.T) {
	fired := cdlbeltholdOracleCompare(t, "default")
	if fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
}

func TestCDLBELTHOLDOracleConstructed(t *testing.T) {
	// Hand-built bullish and bearish belt-holds must fire in both Go and C.
	cases := []struct {
		o, h, l, c float64
		want       int
	}{
		{100, 103.5, 99.9, 103, 100},
		{103, 103.1, 99, 100, -100},
	}
	for _, tc := range cases {
		o, h, l, c := cdlbeltholdBase(10)
		o = append(o, tc.o)
		h = append(h, tc.h)
		l = append(l, tc.l)
		c = append(c, tc.c)
		got, err := CDLBELTHOLD(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLBELTHOLD", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed ", tc.want), got, want)
		if want[10] != tc.want {
			t.Fatalf("C output %v, want %d at bar 10", want, tc.want)
		}
	}
}

func TestCDLBELTHOLDOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 9}, {0, 10}, {9, 9}, {10, 10}, {9, 50}, {11, 11}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdlbeltholdRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLBELTHOLD", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLBELTHOLD range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLBELTHOLDOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLBELTHOLD", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlbeltholdRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLBELTHOLDOracleSettings(t *testing.T) {
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
		{{BodyLong, RangeShadows, 7, 1.5}},
		{{ShadowVeryShort, RangeShadows, 3, 0.3}},
		{{ShadowVeryShort, RangeRealBody, 0, 0.5}},
		{{BodyLong, RangeRealBody, 0, 0.8}},
		{{ShadowVeryShort, RangeHighLow, 20, 0.5}},
		// All periods 0: lookback 0.
		{{BodyLong, RangeRealBody, 0, 0.5}, {ShadowVeryShort, RangeHighLow, 0, 0.1}},
		// Long periods.
		{{BodyLong, RangeRealBody, 50, 1.5}, {ShadowVeryShort, RangeShadows, 30, 1.0}},
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
		cdlbeltholdOracleCompare(t, lbl("settings#", k))
	}
}
