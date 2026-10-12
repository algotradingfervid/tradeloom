//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlidentical3crowsOracleCompare compares CDLIDENTICAL3CROWS with C on every
// fixture and edge length under the current candle settings, and returns how
// many bars fired.
func cdlidentical3crowsOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLIDENTICAL3CROWSLookback()
	if c := mustLookback(t, "CDLIDENTICAL3CROWS"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLIDENTICAL3CROWS(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLIDENTICAL3CROWS", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLIDENTICAL3CROWS ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLIDENTICAL3CROWSOracle(t *testing.T) {
	fired := cdlidentical3crowsOracleCompare(t, "default")
	t.Logf("fired %d times on fixtures", fired)
}

func TestCDLIDENTICAL3CROWSOracleConstructed(t *testing.T) {
	// The hand-built crows must fire in both Go and C, and each single-field
	// variant must agree too.
	mods := []func(o, h, l, c []float64){
		func(o, h, l, c []float64) {},
		func(o, h, l, c []float64) { c[10] = 104 },
		func(o, h, l, c []float64) { o[11] = 102.15 },
		func(o, h, l, c []float64) { o[11] = 101.75 },
		func(o, h, l, c []float64) { o[12] = 99.83 },
		func(o, h, l, c []float64) { o[12] = 100.19 },
		func(o, h, l, c []float64) { o[12], c[12], l[12] = 100.15, 100.05, 99.95 },
		func(o, h, l, c []float64) { l[11] = 99.5 },
		func(o, h, l, c []float64) { l[10], l[11], l[12] = 102, 100, 98 },
	}
	for k, m := range mods {
		o, h, l, c := cdlidentical3crowsSeries()
		m(o, h, l, c)
		got, err := CDLIDENTICAL3CROWS(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLIDENTICAL3CROWS", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed#", k), got, want)
		if k == 0 && want[12] != -100 {
			t.Fatalf("C did not fire on the constructed crows: %v", want)
		}
	}
	// Repeated pattern in a longer series (fires many times, also exercises
	// the rolling totals over several windows).
	var o, h, l, c []float64
	for range 8 {
		bo, bh, bl, bc := cdlidentical3crowsSeries()
		o, h, l, c = append(o, bo...), append(h, bh...), append(l, bl...), append(c, bc...)
	}
	got, _ := CDLIDENTICAL3CROWS(o, h, l, c)
	r := mustCall(t, "CDLIDENTICAL3CROWS", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	want := r.AlignedInt(0, len(o))
	assertInt(t, "repeated", got, want)
	fired := 0
	for _, v := range want {
		if v != 0 {
			fired++
		}
	}
	if fired == 0 {
		t.Fatal("repeated pattern never fired in C")
	}
}

func TestCDLIDENTICAL3CROWSOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 11}, {0, 12}, {12, 12}, {11, 50}, {13, 13}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdlidentical3crowsRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLIDENTICAL3CROWS", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLIDENTICAL3CROWS range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLIDENTICAL3CROWSOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLIDENTICAL3CROWS", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlidentical3crowsRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLIDENTICAL3CROWSOracleSettings(t *testing.T) {
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
		{{ShadowVeryShort, RangeHighLow, 10, 2.0}, {Equal, RangeHighLow, 5, 0.5}}, // generous: fires more
		{{Equal, RangeHighLow, 0, 0.05}},
		{{Equal, RangeRealBody, 1, 0.5}},
		{{Equal, RangeShadows, 20, 1.0}}, // Equal period > ShadowVeryShort period
		{{Equal, RangeHighLow, 30, 3.0}, {ShadowVeryShort, RangeShadows, 0, 3.0}},
		{{ShadowVeryShort, RangeHighLow, 0, 0.5}, {Equal, RangeHighLow, 0, 0.5}}, // lookback 2
		{{ShadowVeryShort, RangeRealBody, 50, 1.5}, {Equal, RangeHighLow, 7, 1.0}},
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
		total += cdlidentical3crowsOracleCompare(t, lbl("settings#", k))
	}
	if total == 0 {
		t.Fatal("pattern never fired on any fixture under any setting")
	}
	t.Logf("fired %d times across settings", total)
}
