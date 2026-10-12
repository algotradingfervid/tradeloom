//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlconcealbabyswallOracleCompare compares CDLCONCEALBABYSWALL with C on
// every fixture and edge length (and on injected fixtures) under the current
// candle settings, and returns how many bars fired.
func cdlconcealbabyswallOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLCONCEALBABYSWALLLookback()
	if c := mustLookback(t, "CDLCONCEALBABYSWALL"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLCONCEALBABYSWALL(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLCONCEALBABYSWALL", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLCONCEALBABYSWALL ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	// The pattern is rare in random data, so also compare fixtures with the
	// constructed swallow injected every 50 bars.
	for _, s := range fixtures(1000) {
		o, h, l, c := cdlconcealbabyswallInject(s)
		got, err := CDLCONCEALBABYSWALL(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLCONCEALBABYSWALL", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		assertInt(t, lbl("CDLCONCEALBABYSWALL injected ", tag, " ", s.Name), got, r.AlignedInt(0, len(o)))
		for _, v := range got {
			if v != 0 {
				fired++
			}
		}
	}
	return fired
}

// cdlconcealbabyswallInject returns a copy of s with the four pattern bars of
// cdlconcealbabyswallSeries written every 50 bars, shifted to the previous
// close and scaled so that the pattern's 4-wide bars match the average
// high-low range of the ten bars before it.
func cdlconcealbabyswallInject(s series) (o, h, l, c []float64) {
	o = append([]float64(nil), s.O...)
	h = append([]float64(nil), s.H...)
	l = append([]float64(nil), s.L...)
	c = append([]float64(nil), s.C...)
	po, ph, pl, pc := cdlconcealbabyswallSeries()
	for p := 20; p+4 <= len(c); p += 50 {
		sum := 0.0
		for k := p - 10; k < p; k++ {
			sum += h[k] - l[k]
		}
		scale := sum / 40
		if scale == 0 {
			scale = 1
		}
		base := c[p-1]
		for k := 0; k < 4; k++ {
			o[p+k] = base + (po[10+k]-106)*scale
			h[p+k] = base + (ph[10+k]-106)*scale
			l[p+k] = base + (pl[10+k]-106)*scale
			c[p+k] = base + (pc[10+k]-106)*scale
		}
	}
	return
}

func TestCDLCONCEALBABYSWALLOracle(t *testing.T) {
	fired := cdlconcealbabyswallOracleCompare(t, "default")
	if fired == 0 {
		t.Fatal("pattern never fired with default settings")
	}
	t.Logf("fired %d times on fixtures", fired)
}

func TestCDLCONCEALBABYSWALLOracleConstructed(t *testing.T) {
	// The hand-built swallow must fire in both Go and C, and each single-field
	// variant must agree too.
	mods := []func(o, h, l, c []float64){
		func(o, h, l, c []float64) {},
		func(o, h, l, c []float64) { c[10] = 110 },
		func(o, h, l, c []float64) { c[13] = 103.5 },
		func(o, h, l, c []float64) { l[10] = 105.5 },
		func(o, h, l, c []float64) { h[11] = 106.5 },
		func(o, h, l, c []float64) { o[12] = 102 },
		func(o, h, l, c []float64) { o[12], h[12] = 101.9, 102.1 },
		func(o, h, l, c []float64) { o[12], h[12] = 101.9, 102.5 },
		func(o, h, l, c []float64) { h[12] = 102 },
		func(o, h, l, c []float64) { h[13] = 103 },
		func(o, h, l, c []float64) { l[13] = 99.5 },
	}
	for k, m := range mods {
		o, h, l, c := cdlconcealbabyswallSeries()
		m(o, h, l, c)
		got, err := CDLCONCEALBABYSWALL(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLCONCEALBABYSWALL", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed#", k), got, want)
		if k == 0 && want[13] != 100 {
			t.Fatalf("C did not fire on the constructed swallow: %v", want)
		}
	}
	// Repeated pattern in a longer series (fires many times, also exercises
	// the rolling totals over several windows).
	var o, h, l, c []float64
	for range 8 {
		bo, bh, bl, bc := cdlconcealbabyswallSeries()
		o, h, l, c = append(o, bo...), append(h, bh...), append(l, bl...), append(c, bc...)
	}
	got, _ := CDLCONCEALBABYSWALL(o, h, l, c)
	r := mustCall(t, "CDLCONCEALBABYSWALL", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
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

func TestCDLCONCEALBABYSWALLOracleRanges(t *testing.T) {
	plain := fixtures(300)[0]
	io, ih, il, ic := cdlconcealbabyswallInject(plain)
	injected := series{Name: "injected", O: io, H: ih, L: il, C: ic, V: plain.V}
	for _, s := range []series{plain, injected} {
		n := len(s.C)
		// Injected patterns end at bars 23, 73, 123, ...
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 12}, {0, 13}, {13, 13}, {12, 50}, {14, 14}, {23, 23}, {20, 73}, {74, 123}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlconcealbabyswallRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLCONCEALBABYSWALL", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLCONCEALBABYSWALL range ", s.Name, " ", rg), out[:nb], r.Outputs[0].Int[:nb])
			if s.Name == "injected" && rg == [2]int{23, 23} && out[0] != 100 {
				t.Errorf("injected single bar 23 did not fire: %d", out[0])
			}
		}
	}
}

func TestCDLCONCEALBABYSWALLOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLCONCEALBABYSWALL", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlconcealbabyswallRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLCONCEALBABYSWALLOracleSettings(t *testing.T) {
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
		{{ShadowVeryShort, RangeHighLow, 10, 2.0}}, // generous threshold
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
		total += cdlconcealbabyswallOracleCompare(t, lbl("settings#", k))
	}
	if total == 0 {
		t.Fatal("pattern never fired on any fixture under any setting")
	}
	t.Logf("fired %d times across settings", total)
}
