//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdldojistarOracleCompare compares CDLDOJISTAR with C on every fixture and
// edge length under the current candle settings, and returns how many bars
// fired.
func cdldojistarOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLDOJISTARLookback()
	if c := mustLookback(t, "CDLDOJISTAR"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLDOJISTAR(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLDOJISTAR", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLDOJISTAR ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLDOJISTAROracle(t *testing.T) {
	if fired := cdldojistarOracleCompare(t, "default"); fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
}

func TestCDLDOJISTAROracleConstructed(t *testing.T) {
	// The hand-built patterns must fire (or not) identically in Go and C,
	// with both signs, including the exact-threshold cases.
	type b = cdldojistarBar
	white := b{100, 105, 100, 105}
	black := b{105, 105, 100, 100}
	cases := []struct {
		b1, b2 b
		want   int
	}{
		{white, b{106, 107, 105.8, 106.2}, -100},
		{black, b{99, 99.3, 98, 98.8}, 100},
		{white, b{106, 107, 105.8, 106.5}, -100},
		{white, b{106, 107, 105.8, 106.5000001}, 0},
		{white, b{105, 106, 104.5, 105.2}, 0},
		{black, b{106, 107, 105.8, 106.2}, 0},
		{b{100, 102.5, 97.5, 102}, b{103, 104, 102.8, 103.2}, 0},
		{b{100, 102.5, 97.5, 102.0000001}, b{103, 104, 102.8, 103.2}, -100},
	}
	for k, tc := range cases {
		o, h, l, c := cdldojistarSeries(tc.b1, tc.b2)
		got, err := CDLDOJISTAR(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLDOJISTAR", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed#", k), got, want)
		if want[11] != tc.want {
			t.Fatalf("case %d: C gave %d at bar 11, want %d", k, want[11], tc.want)
		}
	}
}

func TestCDLDOJISTAROracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 10}, {0, 11}, {10, 10}, {11, 11}, {12, 12}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdldojistarRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLDOJISTAR", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLDOJISTAR range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLDOJISTAROracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLDOJISTAR", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdldojistarRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLDOJISTAROracleSettings(t *testing.T) {
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
		fired += cdldojistarOracleCompare(t, lbl("settings#", k))
	}
	if fired == 0 {
		t.Fatal("pattern never fired under modified settings")
	}
}
