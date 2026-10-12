//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlrickshawmanOracleCompare compares CDLRICKSHAWMAN with C on every
// fixture and edge length under the current candle settings, and returns how
// many bars fired.
func cdlrickshawmanOracleCompare(t *testing.T, tag string) (fired int) {
	t.Helper()
	lb := CDLRICKSHAWMANLookback()
	if c := mustLookback(t, "CDLRICKSHAWMAN"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLRICKSHAWMAN(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLRICKSHAWMAN", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLRICKSHAWMAN ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v == 100 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLRICKSHAWMANOracle(t *testing.T) {
	if fired := cdlrickshawmanOracleCompare(t, "default"); fired == 0 {
		t.Fatal("pattern never fired on the fixtures")
	}
}

func TestCDLRICKSHAWMANOracleConstructed(t *testing.T) {
	// Hand-built bars on the exact boundaries (BodyDoji avg 1, Near avg 2)
	// must agree with C, and C must fire where the formula says so.
	bars := []struct {
		b    [4]float64 // o, h, l, c
		want int
	}{
		{[4]float64{100, 105, 95, 100}, 100},     // classic
		{[4]float64{100, 104, 92, 100}, 100},     // min == mid + near
		{[4]float64{100, 104, 91, 100}, 0},       // min just above mid + near
		{[4]float64{100, 108, 96, 100}, 100},     // max == mid - near
		{[4]float64{100, 109, 96, 100}, 0},       // max just below mid - near
		{[4]float64{100, 103, 97, 101}, 100},     // body == doji avg
		{[4]float64{100, 102, 97, 101}, 0},       // upper 1 == body 1
		{[4]float64{101, 102.5, 98.5, 100}, 100}, // black, shadows 1.5 > body 1
		{[4]float64{100, 103, 97, 101.5}, 0},     // body too big
		{[4]float64{100, 100, 90, 100}, 0},       // dragonfly: no upper shadow
		{[4]float64{100, 100, 100, 100}, 0},      // four price
	}
	for k, tc := range bars {
		o, h, l, c := cdlrickshawmanBase(10)
		o = append(o, tc.b[0])
		h = append(h, tc.b[1])
		l = append(l, tc.b[2])
		c = append(c, tc.b[3])
		got, err := CDLRICKSHAWMAN(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLRICKSHAWMAN", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed#", k), got, want)
		if want[10] != tc.want {
			t.Errorf("constructed#%d: C gave %d, formula says %d", k, want[10], tc.want)
		}
	}
}

func TestCDLRICKSHAWMANOracleRanges(t *testing.T) {
	for _, s := range fixtures(300) {
		n := len(s.C)
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 9}, {0, 10}, {9, 9}, {10, 10}, {11, 11}, {9, 50}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlrickshawmanRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLRICKSHAWMAN", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLRICKSHAWMAN ", s.Name, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

func TestCDLRICKSHAWMANOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLRICKSHAWMAN", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlrickshawmanRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLRICKSHAWMANOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		{{BodyDoji, RangeRealBody, 5, 0.5}},
		{{BodyDoji, RangeShadows, 7, 1.5}},
		{{ShadowLong, RangeShadows, 3, 0.3}},
		{{ShadowLong, RangeHighLow, 10, 0.25}},
		{{ShadowLong, RangeRealBody, 20, 1.0}},
		{{Near, RangeHighLow, 15, 0.5}},
		{{Near, RangeShadows, 4, 1.0}},
		{{Near, RangeRealBody, 0, 0.3}},
		{{BodyDoji, RangeHighLow, 0, 0.1}},
		{{ShadowLong, RangeHighLow, 0, 0.2}},
		// All periods 0: lookback 0.
		{{BodyDoji, RangeHighLow, 0, 0.1}, {ShadowLong, RangeRealBody, 0, 1.0}, {Near, RangeHighLow, 0, 0.2}},
		{{BodyDoji, RangeHighLow, 0, 0.3}, {ShadowLong, RangeHighLow, 0, 0.1}, {Near, RangeHighLow, 0, 0.5}},
		{{BodyDoji, RangeShadows, 0, 1.0}, {ShadowLong, RangeShadows, 0, 0.5}, {Near, RangeShadows, 0, 1.0}},
		// Long periods.
		{{BodyDoji, RangeHighLow, 50, 0.2}, {ShadowLong, RangeHighLow, 30, 0.3}, {Near, RangeHighLow, 40, 0.3}},
		// Unrelated setting changed: no effect.
		{{BodyLong, RangeHighLow, 40, 2.0}},
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
		cdlrickshawmanOracleCompare(t, lbl("settings#", k))
	}
}
