//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdldojiOracleCompare compares CDLDOJI with C on every fixture and edge
// length under the current candle settings, and returns how many bars fired.
func cdldojiOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLDOJILookback()
	if c := mustLookback(t, "CDLDOJI"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLDOJI(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLDOJI", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLDOJI ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for i, v := range got {
				// Fixtures other than the flat ones must fire somewhere too.
				if v != 0 && s.Name != "constant" && s.Name != "zeros" && i >= lb {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLDOJIOracle(t *testing.T) {
	if fired := cdldojiOracleCompare(t, "default"); fired == 0 {
		t.Fatal("pattern never fired on any non-flat fixture")
	}
}

func TestCDLDOJIOracleConstructed(t *testing.T) {
	// Hand-built doji and near-threshold bodies must agree with C.
	o, h, l, c := cdldojiBase(10)
	for _, b := range [][4]float64{
		{100, 101, 99, 100},     // doji
		{100, 101, 99, 100.375}, // body 0.375 <= 0.4
		{100, 101, 99, 100.5},   // body 0.5 > threshold
		{100, 101, 99, 99.8},    // black, small body
	} {
		o = append(o, b[0])
		h = append(h, b[1])
		l = append(l, b[2])
		c = append(c, b[3])
	}
	got, err := CDLDOJI(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CDLDOJI", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	want := r.AlignedInt(0, len(o))
	assertInt(t, "constructed", got, want)
	if want[10] != 100 || want[11] != 100 {
		t.Fatalf("C did not fire on the constructed doji: %v", want)
	}
}

func TestCDLDOJIOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 9}, {0, 10}, {10, 10}, {9, 50}, {11, 11}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdldojiRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLDOJI", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLDOJI range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLDOJIOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLDOJI", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdldojiRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLDOJIOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		{{BodyDoji, RangeHighLow, 0, 0.1}},
		{{BodyDoji, RangeHighLow, 1, 0.1}},
		{{BodyDoji, RangeHighLow, 5, 0.3}},
		{{BodyDoji, RangeRealBody, 10, 0.5}},
		{{BodyDoji, RangeRealBody, 0, 1.0}}, // every bar fires
		{{BodyDoji, RangeShadows, 7, 0.4}},
		{{BodyDoji, RangeShadows, 0, 0.2}},
		{{BodyDoji, RangeHighLow, 50, 0.05}},
		{{BodyDoji, RangeHighLow, 10, 0}},
		// Unrelated setting changes must not matter.
		{{BodyLong, RangeShadows, 30, 2.0}},
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
		cdldojiOracleCompare(t, lbl("settings#", k))
	}
}
