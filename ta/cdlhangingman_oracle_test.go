//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlhangingmanOracleCompare compares CDLHANGINGMAN with C on every fixture and edge
// length under the current candle settings, and returns how many bars fired.
func cdlhangingmanOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLHANGINGMANLookback()
	if c := mustLookback(t, "CDLHANGINGMAN"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLHANGINGMAN(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLHANGINGMAN", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLHANGINGMAN ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLHANGINGMANOracle(t *testing.T) {
	if fired := cdlhangingmanOracleCompare(t, "default"); fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
}

func TestCDLHANGINGMANOracleConstructed(t *testing.T) {
	// The hand-built hanging man must fire in both Go and C.
	o, h, l, c := cdlhangingmanBase(11)
	o = append(o, 102.5)
	h = append(h, 103.1)
	l = append(l, 101)
	c = append(c, 103)
	got, err := CDLHANGINGMAN(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CDLHANGINGMAN", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	want := r.AlignedInt(0, len(o))
	assertInt(t, "constructed", got, want)
	if want[11] != -100 {
		t.Fatalf("C did not fire on the constructed hanging man: %v", want)
	}
}

func TestCDLHANGINGMANOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 10}, {0, 11}, {11, 11}, {10, 50}, {12, 12}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdlhangingmanRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLHANGINGMAN", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLHANGINGMAN range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLHANGINGMANOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLHANGINGMAN", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlhangingmanRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLHANGINGMANOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		{{BodyShort, RangeHighLow, 5, 0.5}},
		{{ShadowLong, RangeHighLow, 7, 0.4}},
		{{ShadowVeryShort, RangeShadows, 3, 0.3}},
		{{Near, RangeRealBody, 0, 1.0}},
		{{Near, RangeShadows, 20, 0.5}},
		{{BodyShort, RangeShadows, 0, 2.0}, {ShadowVeryShort, RangeRealBody, 0, 0.5}},
		// All periods 0: lookback 1.
		{{BodyShort, RangeRealBody, 0, 1.0}, {ShadowLong, RangeRealBody, 0, 1.0},
			{ShadowVeryShort, RangeHighLow, 0, 0.1}, {Near, RangeHighLow, 0, 0.2}},
		// Long periods.
		{{BodyShort, RangeRealBody, 50, 1.5}, {ShadowLong, RangeShadows, 30, 1.0}},
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
		cdlhangingmanOracleCompare(t, lbl("settings#", k))
	}
}
