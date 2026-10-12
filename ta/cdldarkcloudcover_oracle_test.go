//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

var cdldarkcloudcoverPens = []float64{0, 0.1, 0.3, 0.5, 0.8, 1, 2, taRealMax}

// cdldarkcloudcoverOracleCompare compares CDLDARKCLOUDCOVER with C on every
// fixture, edge length and penetration under the current candle settings,
// and returns how many bars fired.
func cdldarkcloudcoverOracleCompare(t *testing.T, tag string, pens []float64) int {
	t.Helper()
	fired := 0
	for _, p := range pens {
		lb := CDLDARKCLOUDCOVERLookback(p)
		if c := mustLookback(t, "CDLDARKCLOUDCOVER", p); lb != c {
			t.Fatalf("%s lookback p=%g: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := CDLDARKCLOUDCOVER(s.O, s.H, s.L, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "CDLDARKCLOUDCOVER", s.ohlcv(), p)
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertInt(t, lbl("CDLDARKCLOUDCOVER ", tag, " ", s.Name, " n=", n, " p=", p), got, r.AlignedInt(0, n))
				for _, v := range got {
					if v != 0 {
						fired++
					}
				}
			}
		}
	}
	return fired
}

func TestCDLDARKCLOUDCOVEROracle(t *testing.T) {
	fired := cdldarkcloudcoverOracleCompare(t, "default", cdldarkcloudcoverPens)
	if fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
	t.Logf("fired %d times on fixtures", fired)
}

func TestCDLDARKCLOUDCOVEROracleBadParams(t *testing.T) {
	in := fixtures(30)[0].ohlcv()
	s := fixtures(30)[0]
	// -4e37 (TA_REAL_DEFAULT) is omitted: C substitutes the default 0.5 for
	// that sentinel, while Go has no sentinel for reals and rejects it.
	for _, p := range []float64{-1, -1e-300, math.Nextafter(taRealMax, math.Inf(1)), 1e38, math.Inf(1), math.NaN(), 0, taRealMax, 0.5} {
		r := mustCall(t, "CDLDARKCLOUDCOVER", in, p)
		_, err := CDLDARKCLOUDCOVER(s.O, s.H, s.L, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%g: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		cl, err := oracle.Lookback("CDLDARKCLOUDCOVER", p)
		if err == nil && cl != CDLDARKCLOUDCOVERLookback(p) {
			t.Errorf("p=%g: C lookback %d, Go %d", p, cl, CDLDARKCLOUDCOVERLookback(p))
		}
	}
}

func TestCDLDARKCLOUDCOVEROracleConstructed(t *testing.T) {
	type tc struct {
		pen float64
		m   func(o, h, l, c []float64)
	}
	cases := []tc{
		{0.5, func(o, h, l, c []float64) {}},
		{0.5, func(o, h, l, c []float64) { c[10] = 102; c[11] = 100.5 }},
		{0.5, func(o, h, l, c []float64) { o[11] = 105 }},
		{0.5, func(o, h, l, c []float64) { c[11] = 100 }},
		{0.5, func(o, h, l, c []float64) { c[11] = 102 }},
		{0.5, func(o, h, l, c []float64) { c[11] = 101.99 }},
		{0, func(o, h, l, c []float64) { c[11] = 103.9 }},
		{0, func(o, h, l, c []float64) { c[11] = 104 }},
		{0.25, func(o, h, l, c []float64) { c[11] = 103 }},
		{1, func(o, h, l, c []float64) { c[11] = 100.01 }},
	}
	for k, cs := range cases {
		o, h, l, c := cdldarkcloudcoverSeries()
		cs.m(o, h, l, c)
		got, err := CDLDARKCLOUDCOVER(o, h, l, c, cs.pen)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLDARKCLOUDCOVER", oracle.Inputs{Open: o, High: h, Low: l, Close: c}, cs.pen)
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed#", k), got, want)
		if k == 0 && want[11] != -100 {
			t.Fatalf("C did not fire on the constructed pattern: %v", want)
		}
	}
	// Repeated pattern in a longer series exercises the rolling total.
	var o, h, l, c []float64
	for range 8 {
		bo, bh, bl, bc := cdldarkcloudcoverSeries()
		o, h, l, c = append(o, bo...), append(h, bh...), append(l, bl...), append(c, bc...)
	}
	got, _ := CDLDARKCLOUDCOVER(o, h, l, c, 0.5)
	r := mustCall(t, "CDLDARKCLOUDCOVER", oracle.Inputs{Open: o, High: h, Low: l, Close: c}, 0.5)
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

func TestCDLDARKCLOUDCOVEROracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, p := range []float64{0, 0.5} {
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 10}, {0, 11}, {11, 11}, {10, 50}, {12, 12}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdldarkcloudcoverRange(rg[0], rg[1], s.O, s.H, s.L, s.C, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLDARKCLOUDCOVER", s.ohlcv(), rg[0], rg[1], p)
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%g: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLDARKCLOUDCOVER range ", rg, " p=", p), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

func TestCDLDARKCLOUDCOVEROracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLDARKCLOUDCOVER", s.ohlcv(), rg[0], rg[1], 0.5)
		_, _, err := cdldarkcloudcoverRange(rg[0], rg[1], s.O, s.H, s.L, s.C, 0.5, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLDARKCLOUDCOVEROracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		{{BodyLong, RangeRealBody, 0, 1.0}},
		{{BodyLong, RangeRealBody, 0, 0.5}},
		{{BodyLong, RangeRealBody, 1, 0.5}},
		{{BodyLong, RangeHighLow, 5, 0.3}},
		{{BodyLong, RangeShadows, 3, 1.0}},
		{{BodyLong, RangeHighLow, 10, 0.1}}, // generous threshold: fires more
		{{BodyLong, RangeShadows, 0, 3.0}},
		{{BodyLong, RangeRealBody, 50, 1.5}},
		// An unrelated setting must not matter.
		{{ShadowVeryShort, RangeHighLow, 40, 2.0}},
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
		total += cdldarkcloudcoverOracleCompare(t, lbl("settings#", k), []float64{0, 0.5, 1})
	}
	if total == 0 {
		t.Fatal("pattern never fired on any fixture under any setting")
	}
	t.Logf("fired %d times across settings", total)
}
