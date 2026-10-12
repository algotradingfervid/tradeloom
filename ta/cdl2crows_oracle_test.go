//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdl2crowsOracleCompare compares CDL2CROWS with C on every fixture and edge
// length under the current candle settings, and returns how many bars fired.
func cdl2crowsOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDL2CROWSLookback()
	if c := mustLookback(t, "CDL2CROWS"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDL2CROWS(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDL2CROWS", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDL2CROWS ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDL2CROWSOracle(t *testing.T) {
	if fired := cdl2crowsOracleCompare(t, "default"); fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
}

func TestCDL2CROWSOracleFires(t *testing.T) {
	// With a lenient BodyLong the random fixtures produce the pattern; Go
	// and C must agree on every bar and it must fire at least once.
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	if err := SetCandleSettings(BodyLong, RangeRealBody, 10, 0.2); err != nil {
		t.Fatal(err)
	}
	oracle.SetCandleSettings(int(BodyLong), int(RangeRealBody), 10, 0.2)
	if fired := cdl2crowsOracleCompare(t, "lenient"); fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
}

func TestCDL2CROWSOracleConstructed(t *testing.T) {
	// The hand-built Two Crows (and its near misses) must agree in Go and C.
	o, h, l, c := cdl2crowsSeries(cdl2crowsB1, cdl2crowsB2, cdl2crowsB3)
	got, err := CDL2CROWS(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CDL2CROWS", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	want := r.AlignedInt(0, len(o))
	assertInt(t, "constructed", got, want)
	if want[12] != -100 {
		t.Fatalf("C did not fire on the constructed Two Crows: %v", want)
	}
	type B = cdl2crowsBar
	for k, bars := range [][3]B{
		{{104, 106.5, 103.5, 106}, cdl2crowsB2, {108, 108.2, 104.5, 105}},
		{{103.9, 106.5, 103.5, 106}, cdl2crowsB2, {108, 108.2, 104.5, 105}},
		{cdl2crowsB1, {109, 109.5, 105.8, 106}, cdl2crowsB3},
		{cdl2crowsB1, cdl2crowsB2, {109, 109.2, 102.5, 103}},
		{cdl2crowsB1, cdl2crowsB2, {107, 107.2, 102.5, 103}},
		{cdl2crowsB1, cdl2crowsB2, {108, 108.2, 99.5, 100}},
		{cdl2crowsB1, cdl2crowsB2, {108, 108.2, 105.5, 106}},
		{cdl2crowsB1, cdl2crowsB2, {108, 108.2, 102.5, 108}},
	} {
		o, h, l, c := cdl2crowsSeries(bars[0], bars[1], bars[2])
		got, _ := CDL2CROWS(o, h, l, c)
		r := mustCall(t, "CDL2CROWS", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		assertInt(t, lbl("near miss #", k), got, r.AlignedInt(0, len(o)))
	}
}

func TestCDL2CROWSOracleRanges(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	_ = SetCandleSettings(BodyLong, RangeRealBody, 10, 0.2)
	oracle.SetCandleSettings(int(BodyLong), int(RangeRealBody), 10, 0.2)
	for _, s := range fixtures(300) {
		n := len(s.C)
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 11}, {0, 12}, {11, 11}, {12, 12}, {13, 13}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdl2crowsRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDL2CROWS", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDL2CROWS ", s.Name, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

func TestCDL2CROWSOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDL2CROWS", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdl2crowsRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDL2CROWSOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		rt     RangeType
		period int
		factor float64
	}
	for k, ch := range []change{
		{RangeRealBody, 0, 1.0},
		{RangeRealBody, 0, 0.5},
		{RangeRealBody, 1, 1.0},
		{RangeRealBody, 5, 0.3},
		{RangeHighLow, 10, 0.4},
		{RangeHighLow, 0, 0.3},
		{RangeShadows, 7, 0.5},
		{RangeShadows, 0, 1.0},
		{RangeRealBody, 50, 0.5},
	} {
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		if err := SetCandleSettings(BodyLong, ch.rt, ch.period, ch.factor); err != nil {
			t.Fatal(err)
		}
		oracle.SetCandleSettings(int(BodyLong), int(ch.rt), ch.period, ch.factor)
		fired := cdl2crowsOracleCompare(t, lbl("settings#", k))
		t.Logf("settings#%d: fired %d", k, fired)
	}
}
