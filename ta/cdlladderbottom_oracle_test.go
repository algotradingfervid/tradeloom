//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlladderbottomOracleCompare compares CDLLADDERBOTTOM with C on every
// fixture and edge length under the current candle settings, plus a repeated
// constructed pattern, and returns how many bars fired on the fixtures.
func cdlladderbottomOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLLADDERBOTTOMLookback()
	if c := mustLookback(t, "CDLLADDERBOTTOM"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLLADDERBOTTOM(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLLADDERBOTTOM", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLLADDERBOTTOM ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	// Repeated constructed pattern (also exercises the rolling total).
	o, h, l, c := cdlladderbottomRepeated(8)
	got, _ := CDLLADDERBOTTOM(o, h, l, c)
	r := mustCall(t, "CDLLADDERBOTTOM", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	assertInt(t, lbl("CDLLADDERBOTTOM ", tag, " repeated"), got, r.AlignedInt(0, len(o)))
	return fired
}

func cdlladderbottomRepeated(k int) (o, h, l, c []float64) {
	for range k {
		bo, bh, bl, bc := cdlladderbottomSeries()
		o, h, l, c = append(o, bo...), append(h, bh...), append(l, bl...), append(c, bc...)
	}
	return
}

func TestCDLLADDERBOTTOMOracle(t *testing.T) {
	fired := cdlladderbottomOracleCompare(t, "default")
	if fired == 0 {
		t.Fatal("pattern never fired on the fixtures")
	}
	t.Logf("fired %d times on fixtures", fired)
}

func TestCDLLADDERBOTTOMOracleConstructed(t *testing.T) {
	mods := []func(o, h, l, c []float64){
		func(o, h, l, c []float64) {},
		func(o, h, l, c []float64) { c[10] = 110 },
		func(o, h, l, c []float64) { o[12] = 108.5 },
		func(o, h, l, c []float64) { c[12] = 106 },
		func(o, h, l, c []float64) { c[13] = 104.5 },
		func(o, h, l, c []float64) { h[13] = 104.8 },
		func(o, h, l, c []float64) { h[13] = 105 },
		func(o, h, l, c []float64) { o[14] = 107 },
		func(o, h, l, c []float64) { o[14] = 104.5 },
		func(o, h, l, c []float64) { c[14] = 106 },
	}
	for k, m := range mods {
		o, h, l, c := cdlladderbottomSeries()
		m(o, h, l, c)
		got, err := CDLLADDERBOTTOM(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLLADDERBOTTOM", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed#", k), got, want)
		if k == 0 && want[14] != 100 {
			t.Fatalf("C did not fire on the constructed ladder bottom: %v", want)
		}
	}
	o, h, l, c := cdlladderbottomRepeated(8)
	r := mustCall(t, "CDLLADDERBOTTOM", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	fired := 0
	for _, v := range r.AlignedInt(0, len(o)) {
		if v != 0 {
			fired++
		}
	}
	if fired < 2 {
		t.Fatalf("repeated pattern fired %d times in C", fired)
	}
}

func TestCDLLADDERBOTTOMOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 13}, {0, 14}, {14, 14}, {13, 50}, {15, 15}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdlladderbottomRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLLADDERBOTTOM", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLLADDERBOTTOM range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
	// Sub-ranges over the repeated constructed pattern.
	o, h, l, c := cdlladderbottomRepeated(4)
	in := oracle.Inputs{Open: o, High: h, Low: l, Close: c}
	for _, rg := range [][2]int{{14, 14}, {10, 29}, {29, 29}, {20, 59}} {
		out := make([]int, len(o))
		beg, nb, err := cdlladderbottomRange(rg[0], rg[1], o, h, l, c, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLLADDERBOTTOM", in, rg[0], rg[1])
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("constructed range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("constructed range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLLADDERBOTTOMOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLLADDERBOTTOM", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlladderbottomRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLLADDERBOTTOMOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		{{ShadowVeryShort, RangeHighLow, 0, 0.1}},
		{{ShadowVeryShort, RangeHighLow, 1, 0.5}},
		{{ShadowVeryShort, RangeRealBody, 5, 0.3}},
		{{ShadowVeryShort, RangeShadows, 3, 1.0}},
		{{ShadowVeryShort, RangeHighLow, 10, 0.0}}, // any upper shadow counts: fires more
		{{ShadowVeryShort, RangeShadows, 0, 3.0}},
		{{ShadowVeryShort, RangeRealBody, 50, 1.5}},
		// An unrelated setting must not matter.
		{{BodyLong, RangeHighLow, 40, 2.0}},
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
		total += cdlladderbottomOracleCompare(t, lbl("settings#", k))
	}
	if total == 0 {
		t.Fatal("pattern never fired on any fixture under any setting")
	}
	t.Logf("fired %d times on fixtures across settings", total)
}
