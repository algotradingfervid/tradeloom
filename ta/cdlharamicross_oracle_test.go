//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlharamicrossOracleCompare compares CDLHARAMICROSS with C on every fixture and
// edge length under the current candle settings, and returns how many bars
// fired.
func cdlharamicrossOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLHARAMICROSSLookback()
	if c := mustLookback(t, "CDLHARAMICROSS"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLHARAMICROSS(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLHARAMICROSS", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLHARAMICROSS ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLHARAMICROSSOracle(t *testing.T) {
	if fired := cdlharamicrossOracleCompare(t, "default"); fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
}

func TestCDLHARAMICROSSOracleConstructed(t *testing.T) {
	// The hand-built patterns must fire (or not) identically in Go and C,
	// with both signs and both magnitudes, including the exact-threshold cases.
	type b = cdlharamicrossBar
	white := b{100, 105, 100, 105}
	black := b{105, 105, 100, 100}
	cases := []struct {
		b1, b2 b
		want   int
	}{
		{white, b{102, 103, 101, 102.2}, -100},
		{black, b{102, 103, 101, 102.2}, 100},
		{white, b{104.8, 105.5, 104, 105}, -80},
		{white, b{100, 101, 99.5, 100.3}, -80},
		{black, b{104.6, 105.5, 104, 105}, 80},
		{black, b{100, 100.5, 99.5, 100}, 80},
		{white, b{102, 103, 101, 102.5}, -100},
		{white, b{102, 103, 101, 102.5000001}, 0},
		{white, b{104.8, 106, 104, 105.2}, 0},
		{black, b{99, 99.3, 98, 98.8}, 0},
		{b{100, 102.5, 97.5, 102}, b{101, 101.5, 100.5, 101.2}, 0},
		{b{100, 102.5, 97.5, 102.0000001}, b{101, 101.5, 100.5, 101.2}, -100},
	}
	for k, tc := range cases {
		o, h, l, c := cdlharamicrossSeries(tc.b1, tc.b2)
		got, err := CDLHARAMICROSS(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLHARAMICROSS", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed#", k), got, want)
		if want[11] != tc.want {
			t.Fatalf("case %d: C gave %d at bar 11, want %d", k, want[11], tc.want)
		}
	}
}

func TestCDLHARAMICROSSOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 10}, {0, 11}, {10, 10}, {11, 11}, {12, 12}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdlharamicrossRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLHARAMICROSS", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLHARAMICROSS range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLHARAMICROSSOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLHARAMICROSS", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlharamicrossRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLHARAMICROSSOracleSettings(t *testing.T) {
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
		{{BodyDoji, RangeRealBody, 7, 0.4}},
		{{BodyLong, RangeShadows, 3, 0.3}},
		{{BodyDoji, RangeShadows, 20, 1.5}},
		{{BodyLong, RangeRealBody, 0, 0.5}},
		{{BodyDoji, RangeHighLow, 0, 0.3}},
		// All periods 0: lookback 1.
		{{BodyLong, RangeRealBody, 0, 0.8}, {BodyDoji, RangeHighLow, 0, 0.5}},
		// Long periods.
		{{BodyLong, RangeRealBody, 50, 1.5}, {BodyDoji, RangeShadows, 30, 1.0}},
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
		fired += cdlharamicrossOracleCompare(t, lbl("settings#", k))
	}
	if fired == 0 {
		t.Fatal("pattern never fired under modified settings")
	}
}
