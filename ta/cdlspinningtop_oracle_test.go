//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlspinningtopOracleCompare compares CDLSPINNINGTOP with C on every fixture and edge
// length under the current candle settings, and returns how many bars fired.
func cdlspinningtopOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLSPINNINGTOPLookback()
	if c := mustLookback(t, "CDLSPINNINGTOP"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLSPINNINGTOP(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLSPINNINGTOP", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLSPINNINGTOP ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
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

func TestCDLSPINNINGTOPOracle(t *testing.T) {
	if fired := cdlspinningtopOracleCompare(t, "default"); fired == 0 {
		t.Fatal("pattern never fired on any non-flat fixture")
	}
}

func TestCDLSPINNINGTOPOracleConstructed(t *testing.T) {
	// Hand-built spinning tops and near-threshold bars must agree with C.
	o, h, l, c := cdlspinningtopBase(10)
	for _, b := range [][4]float64{
		{100, 103, 98, 101},          // white spinning top
		{101, 103, 98, 100},          // black spinning top
		{100, 102, 98, 101},          // upper shadow equals body
		{100, 103, 99, 101},          // lower shadow equals body
		{100, 105, 97, 102},          // body equal to the average
		{100, 101, 99, 100},          // doji with shadows
		{100.1, 100.4, 99.75, 100.2}, // inexact decimals near the thresholds
		{0.3, 0.7, 0.0, 0.4},         // inexact decimals: shadows vs body
	} {
		o = append(o, b[0])
		h = append(h, b[1])
		l = append(l, b[2])
		c = append(c, b[3])
	}
	got, err := CDLSPINNINGTOP(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CDLSPINNINGTOP", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	want := r.AlignedInt(0, len(o))
	assertInt(t, "constructed", got, want)
	if want[10] != 100 || want[11] != -100 {
		t.Fatalf("C did not fire on the constructed spinning tops: %v", want)
	}
}

func TestCDLSPINNINGTOPOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 9}, {0, 10}, {10, 10}, {9, 50}, {11, 11}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdlspinningtopRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLSPINNINGTOP", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLSPINNINGTOP range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLSPINNINGTOPOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLSPINNINGTOP", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlspinningtopRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLSPINNINGTOPOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		{{BodyShort, RangeRealBody, 0, 1.0}}, // body < body: never fires
		{{BodyShort, RangeHighLow, 0, 0.5}},
		{{BodyShort, RangeHighLow, 1, 1.0}},
		{{BodyShort, RangeShadows, 5, 1.5}},
		{{BodyShort, RangeRealBody, 50, 2.0}},
		{{BodyShort, RangeHighLow, 7, 0.3}},
		{{BodyShort, RangeRealBody, 10, 0}},
		{{BodyShort, RangeShadows, 0, 0.25}},
		{{BodyShort, RangeRealBody, 1, 1.0}},
		// Unrelated setting changes must not matter.
		{{BodyLong, RangeShadows, 30, 2.0}},
		{{ShadowVeryLong, RangeHighLow, 20, 0.2}},
		{{BodyDoji, RangeHighLow, 40, 0.5}},
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
		cdlspinningtopOracleCompare(t, lbl("settings#", k))
	}
}
