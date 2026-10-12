//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlunique3riverOracleCompare compares CDLUNIQUE3RIVER with C on every
// fixture and edge length under the current candle settings, and returns how
// many bars fired.
func cdlunique3riverOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLUNIQUE3RIVERLookback()
	if c := mustLookback(t, "CDLUNIQUE3RIVER"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLUNIQUE3RIVER(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLUNIQUE3RIVER", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLUNIQUE3RIVER ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLUNIQUE3RIVEROracle(t *testing.T) {
	fired := cdlunique3riverOracleCompare(t, "default")
	t.Logf("default: fired %d", fired)
	if fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
}

func TestCDLUNIQUE3RIVEROracleConstructed(t *testing.T) {
	// The hand-built Unique 3 River and every near miss must agree in Go and C.
	for _, tc := range cdlunique3riverCases {
		o, h, l, c := cdlunique3riverSeries(tc.b1, tc.b2, tc.b3)
		got, err := CDLUNIQUE3RIVER(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLUNIQUE3RIVER", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed ", tc.name), got, want)
		if want[12] != tc.want {
			t.Errorf("%s: C gives %d at bar 12, test expects %d", tc.name, want[12], tc.want)
		}
	}
}

func TestCDLUNIQUE3RIVEROracleRanges(t *testing.T) {
	for _, s := range fixtures(300) {
		n := len(s.C)
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 11}, {0, 12}, {11, 11}, {12, 12}, {13, 13}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlunique3riverRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLUNIQUE3RIVER", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLUNIQUE3RIVER ", s.Name, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

func TestCDLUNIQUE3RIVEROracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLUNIQUE3RIVER", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlunique3riverRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLUNIQUE3RIVEROracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	total := 0
	for k, chs := range [][]change{
		{{BodyLong, RangeRealBody, 0, 1.0}, {BodyShort, RangeRealBody, 0, 1.0}},
		{{BodyLong, RangeRealBody, 0, 0.5}, {BodyShort, RangeRealBody, 0, 2.0}},
		{{BodyLong, RangeRealBody, 1, 1.0}},
		{{BodyShort, RangeRealBody, 1, 1.0}},
		{{BodyLong, RangeRealBody, 5, 0.3}, {BodyShort, RangeRealBody, 20, 2.0}},
		{{BodyLong, RangeHighLow, 10, 0.4}, {BodyShort, RangeHighLow, 3, 0.5}},
		{{BodyLong, RangeHighLow, 0, 0.3}, {BodyShort, RangeShadows, 0, 1.0}},
		{{BodyLong, RangeShadows, 7, 0.5}, {BodyShort, RangeShadows, 12, 1.0}},
		{{BodyLong, RangeShadows, 0, 1.0}},
		{{BodyLong, RangeRealBody, 50, 0.5}},
		{{BodyShort, RangeRealBody, 50, 1.0}},
		{{BodyLong, RangeRealBody, 3, 0.2}, {BodyShort, RangeRealBody, 30, 3.0}},
	} {
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		for _, ch := range chs {
			if err := SetCandleSettings(ch.st, ch.rt, ch.period, ch.factor); err != nil {
				t.Fatal(err)
			}
			oracle.SetCandleSettings(int(ch.st), int(ch.rt), ch.period, ch.factor)
		}
		fired := cdlunique3riverOracleCompare(t, lbl("settings#", k))
		t.Logf("settings#%d: fired %d", k, fired)
		total += fired
	}
	if total == 0 {
		t.Fatal("pattern never fired on any fixture under any settings")
	}
}
