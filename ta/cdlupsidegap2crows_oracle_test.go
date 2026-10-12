//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlupsidegap2crowsOracleCompare compares CDLUPSIDEGAP2CROWS with C on every
// fixture and edge length under the current candle settings, and returns how
// many bars fired.
func cdlupsidegap2crowsOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLUPSIDEGAP2CROWSLookback()
	if c := mustLookback(t, "CDLUPSIDEGAP2CROWS"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLUPSIDEGAP2CROWS(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLUPSIDEGAP2CROWS", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLUPSIDEGAP2CROWS ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

// cdlupsidegap2crowsLenient makes the random fixtures produce the pattern.
func cdlupsidegap2crowsLenient(t *testing.T) {
	t.Helper()
	if err := SetCandleSettings(BodyLong, RangeRealBody, 10, 0.2); err != nil {
		t.Fatal(err)
	}
	if err := SetCandleSettings(BodyShort, RangeRealBody, 10, 3.0); err != nil {
		t.Fatal(err)
	}
	oracle.SetCandleSettings(int(BodyLong), int(RangeRealBody), 10, 0.2)
	oracle.SetCandleSettings(int(BodyShort), int(RangeRealBody), 10, 3.0)
}

func TestCDLUPSIDEGAP2CROWSOracle(t *testing.T) {
	fired := cdlupsidegap2crowsOracleCompare(t, "default")
	t.Logf("default settings: fired %d", fired)
}

func TestCDLUPSIDEGAP2CROWSOracleFires(t *testing.T) {
	// With lenient BodyLong/BodyShort the random fixtures produce the
	// pattern; Go and C must agree on every bar and it must fire.
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	cdlupsidegap2crowsLenient(t)
	if fired := cdlupsidegap2crowsOracleCompare(t, "lenient"); fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
}

func TestCDLUPSIDEGAP2CROWSOracleConstructed(t *testing.T) {
	// The hand-built pattern (and its near misses) must agree in Go and C.
	o, h, l, c := cdlupsidegap2crowsSeries(cdlupsidegap2crowsB1, cdlupsidegap2crowsB2, cdlupsidegap2crowsB3)
	got, err := CDLUPSIDEGAP2CROWS(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CDLUPSIDEGAP2CROWS", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	want := r.AlignedInt(0, len(o))
	assertInt(t, "constructed", got, want)
	if want[12] != -100 {
		t.Fatalf("C did not fire on the constructed pattern: %v", want)
	}
	type B = cdlupsidegap2crowsBar
	b1, b2, b3 := cdlupsidegap2crowsB1, cdlupsidegap2crowsB2, cdlupsidegap2crowsB3
	for k, bars := range [][3]B{
		{{104, 106.5, 103.5, 106}, b2, b3},
		{{103.9, 106.5, 103.5, 106}, b2, b3},
		{{106, 106.5, 105.5, 106}, b2, b3},
		{b1, {107.5, 108.5, 106.8, 107.5}, b3},
		{b1, {110, 110.5, 106.8, 107}, {111, 111.5, 106, 106.5}},
		{b1, {109.25, 109.5, 106.8, 107}, {110, 110.5, 106, 106.5}},
		{b1, {107, 107.5, 105.8, 106}, {108, 108.5, 105.5, 106.1}},
		{b1, b2, {108, 108.5, 106, 106.5}},
		{b1, b2, {109, 109.5, 106.9, 107}},
		{b1, b2, {109, 109.5, 105.5, 106}},
		{b1, b2, {109, 109.5, 105.5, 106.01}},
		{b1, b2, {106.5, 109.5, 106, 109}},
	} {
		o, h, l, c := cdlupsidegap2crowsSeries(bars[0], bars[1], bars[2])
		got, _ := CDLUPSIDEGAP2CROWS(o, h, l, c)
		r := mustCall(t, "CDLUPSIDEGAP2CROWS", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		assertInt(t, lbl("near miss #", k), got, r.AlignedInt(0, len(o)))
	}
}

func TestCDLUPSIDEGAP2CROWSOracleRanges(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	cdlupsidegap2crowsLenient(t)
	fired := 0
	for _, s := range fixtures(300) {
		n := len(s.C)
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 11}, {0, 12}, {11, 11}, {12, 12}, {13, 13}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlupsidegap2crowsRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLUPSIDEGAP2CROWS", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLUPSIDEGAP2CROWS ", s.Name, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
			for _, v := range out[:nb] {
				if v != 0 {
					fired++
				}
			}
		}
	}
	t.Logf("lenient sub-ranges: fired %d", fired)

	// Sub-ranges over the constructed pattern (default settings), so a
	// firing bar is inside, at the start of, and at the end of a range.
	RestoreAllCandleDefaultSettings()
	oracle.RestoreCandleDefaults()
	o, h, l, c := cdlupsidegap2crowsSeries(cdlupsidegap2crowsB1, cdlupsidegap2crowsB2, cdlupsidegap2crowsB3)
	o2, h2, l2, c2 := cdlupsidegap2crowsBase(10)
	o, h, l, c = append(o, o2...), append(h, h2...), append(l, l2...), append(c, c2...)
	in := oracle.Inputs{Open: o, High: h, Low: l, Close: c}
	for _, rg := range [][2]int{{0, 22}, {5, 12}, {12, 12}, {12, 20}, {11, 13}, {13, 22}} {
		out := make([]int, len(o))
		beg, nb, err := cdlupsidegap2crowsRange(rg[0], rg[1], o, h, l, c, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLUPSIDEGAP2CROWS", in, rg[0], rg[1])
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("constructed range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("constructed range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		if rg[0] <= 12 && rg[1] >= 12 && out[12-beg] != -100 {
			t.Fatalf("constructed range %v: bar 12 = %d, want -100", rg, out[12-beg])
		}
	}
}

func TestCDLUPSIDEGAP2CROWSOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLUPSIDEGAP2CROWS", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlupsidegap2crowsRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLUPSIDEGAP2CROWSOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	for k, chs := range [][]change{
		{{BodyLong, RangeRealBody, 0, 1.0}},
		{{BodyLong, RangeRealBody, 0, 0.5}, {BodyShort, RangeRealBody, 0, 1.0}},
		{{BodyLong, RangeRealBody, 1, 0.3}, {BodyShort, RangeRealBody, 1, 2.0}},
		{{BodyLong, RangeRealBody, 5, 0.3}, {BodyShort, RangeRealBody, 20, 2.0}},
		{{BodyLong, RangeRealBody, 20, 0.3}, {BodyShort, RangeRealBody, 5, 2.0}},
		{{BodyLong, RangeHighLow, 10, 0.2}, {BodyShort, RangeHighLow, 10, 1.0}},
		{{BodyLong, RangeHighLow, 0, 0.3}, {BodyShort, RangeShadows, 0, 2.0}},
		{{BodyLong, RangeShadows, 7, 0.5}, {BodyShort, RangeShadows, 3, 3.0}},
		{{BodyShort, RangeRealBody, 0, 0.5}},
		{{BodyLong, RangeRealBody, 50, 0.3}, {BodyShort, RangeRealBody, 2, 3.0}},
		{{BodyLong, RangeRealBody, 2, 0.3}, {BodyShort, RangeRealBody, 50, 3.0}},
	} {
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		for _, ch := range chs {
			if err := SetCandleSettings(ch.st, ch.rt, ch.period, ch.factor); err != nil {
				t.Fatal(err)
			}
			oracle.SetCandleSettings(int(ch.st), int(ch.rt), ch.period, ch.factor)
		}
		fired := cdlupsidegap2crowsOracleCompare(t, lbl("settings#", k))
		t.Logf("settings#%d: fired %d", k, fired)
	}
}
