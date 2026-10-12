//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdl3whitesoldiersOracleCompare compares CDL3WHITESOLDIERS with C on every
// fixture and edge length under the current candle settings, and returns how
// many bars fired.
func cdl3whitesoldiersOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDL3WHITESOLDIERSLookback()
	if c := mustLookback(t, "CDL3WHITESOLDIERS"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDL3WHITESOLDIERS(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDL3WHITESOLDIERS", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDL3WHITESOLDIERS ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDL3WHITESOLDIERSOracle(t *testing.T) {
	if fired := cdl3whitesoldiersOracleCompare(t, "default"); fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
}

func TestCDL3WHITESOLDIERSOracleConstructed(t *testing.T) {
	// The hand-built soldiers must fire in both Go and C, also repeated
	// along a longer series.
	var bars []cdl3whitesoldiersBar
	for k := range 8 {
		for _, b := range cdl3whitesoldiersSoldiers {
			d := float64(k) * 0.5
			bars = append(bars, cdl3whitesoldiersBar{b.o + d, b.h + d, b.l + d, b.c + d})
		}
	}
	o, h, l, c := cdl3whitesoldiersSeries(bars...)
	got, err := CDL3WHITESOLDIERS(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CDL3WHITESOLDIERS", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	want := r.AlignedInt(0, len(o))
	assertInt(t, "constructed", got, want)
	if want[12] != 100 {
		t.Fatalf("C did not fire on the constructed soldiers: %v", want)
	}
}

func TestCDL3WHITESOLDIERSOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 11}, {0, 12}, {12, 12}, {11, 50}, {13, 13}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdl3whitesoldiersRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDL3WHITESOLDIERS", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDL3WHITESOLDIERS range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDL3WHITESOLDIERSOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDL3WHITESOLDIERS", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdl3whitesoldiersRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDL3WHITESOLDIERSOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		// Loose upper-shadow limit so the random walks fire.
		{{ShadowVeryShort, RangeHighLow, 10, 1.0}},
		{{ShadowVeryShort, RangeShadows, 3, 2.0}, {BodyShort, RangeHighLow, 5, 0.1}},
		{{ShadowVeryShort, RangeRealBody, 0, 3.0}, {Near, RangeRealBody, 0, 1.0}},
		{{ShadowVeryShort, RangeHighLow, 7, 0.8}, {Far, RangeShadows, 20, 0.5}},
		{{ShadowVeryShort, RangeHighLow, 10, 1.0}, {Near, RangeShadows, 20, 2.0}, {Far, RangeRealBody, 0, 0.3}},
		// All periods 0: lookback 2.
		{{ShadowVeryShort, RangeHighLow, 0, 0.5}, {BodyShort, RangeRealBody, 0, 0.1},
			{Far, RangeHighLow, 0, 0.6}, {Near, RangeHighLow, 0, 0.5}},
		// Long periods.
		{{BodyShort, RangeRealBody, 50, 0.5}, {ShadowVeryShort, RangeHighLow, 30, 1.0}},
	}
	total := 0
	for k, set := range sets {
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		for _, ch := range set {
			if err := SetCandleSettings(ch.st, ch.rt, ch.period, ch.factor); err != nil {
				t.Fatal(err)
			}
			oracle.SetCandleSettings(int(ch.st), int(ch.rt), ch.period, ch.factor)
		}
		total += cdl3whitesoldiersOracleCompare(t, lbl("settings#", k))
	}
	if total == 0 {
		t.Fatal("pattern never fired on any fixture under modified settings")
	}
}
