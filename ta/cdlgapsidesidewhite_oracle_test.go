//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlgapsidesidewhiteOracleCompare compares CDLGAPSIDESIDEWHITE with C on
// every fixture and edge length under the current candle settings, and
// returns how many bars fired.
func cdlgapsidesidewhiteOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLGAPSIDESIDEWHITELookback()
	if c := mustLookback(t, "CDLGAPSIDESIDEWHITE"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLGAPSIDESIDEWHITE(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLGAPSIDESIDEWHITE", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLGAPSIDESIDEWHITE ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLGAPSIDESIDEWHITEOracle(t *testing.T) {
	fired := cdlgapsidesidewhiteOracleCompare(t, "default")
	t.Logf("default: fired %d", fired)
}

func TestCDLGAPSIDESIDEWHITEOracleFires(t *testing.T) {
	// With lenient Near/Equal bands the fixtures produce the pattern; Go and
	// C must agree on every bar and it must fire at least once.
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	_ = SetCandleSettings(Near, RangeHighLow, 5, 1.0)
	_ = SetCandleSettings(Equal, RangeHighLow, 5, 1.0)
	oracle.SetCandleSettings(int(Near), int(RangeHighLow), 5, 1.0)
	oracle.SetCandleSettings(int(Equal), int(RangeHighLow), 5, 1.0)
	fired := cdlgapsidesidewhiteOracleCompare(t, "lenient")
	t.Logf("lenient: fired %d", fired)
	if fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
}

func TestCDLGAPSIDESIDEWHITEOracleConstructed(t *testing.T) {
	// The hand-built patterns and near misses must agree in Go and C, under
	// default and exact (dyadic) settings.
	type B = cdlgapsidesidewhiteBar
	up, dn := cdlgapsidesidewhiteUp, cdlgapsidesidewhiteDown
	w := cdlgapsidesidewhiteWith
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	for _, exact := range []bool{false, true} {
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		if exact {
			cdlgapsidesidewhiteExactSettings(t)
			oracle.SetCandleSettings(int(Near), int(RangeHighLow), 5, 0.25)
			oracle.SetCandleSettings(int(Equal), int(RangeHighLow), 5, 0.125)
		}
		cases := []struct {
			bars [3]B
			want int // under exact settings
		}{
			{up, 100},
			{dn, -100},
			{w(up, 0, B{102, 103, 99, 100}), 100},
			{w(w(up, 1, B{102, 105.5, 101.5, 105}), 2, B{102.5, 105.5, 102, 105.5}), 0},
			{w(w(up, 1, B{102.5, 106, 102, 105.5}), 2, B{102, 105.5, 101.5, 105}), 0},
			{w(w(dn, 1, B{105, 108.5, 104.5, 108}), 2, B{104.5, 107.5, 104, 107.5}), 0},
			{w(w(dn, 1, B{104, 107.5, 103.5, 107}), 2, B{104.5, 108.5, 104, 108}), 0},
			{w(w(dn, 1, B{104, 107.5, 103.5, 107}), 2, B{104.5, 108.5, 104, 107.75}), -100},
			{w(up, 1, B{108, 108.5, 104.5, 105}), 0},
			{w(up, 2, B{108, 108.5, 104.5, 105}), 0},
			{w(w(up, 1, B{105, 107, 103, 105}), 2, B{105, 107, 103, 105}), 100},
			{w(up, 2, B{105, 109.5, 104.5, 109}), 100},
			{w(up, 2, B{105, 109.5, 104.5, 109.25}), 0},
			{w(up, 2, B{105, 108.5, 104.5, 107}), 100},
			{w(up, 2, B{105, 108.5, 104.5, 106.75}), 0},
			{w(up, 2, B{105.5, 109, 105, 108.5}), 100},
			{w(up, 2, B{105.75, 109.25, 105, 108.75}), 0},
			{w(up, 2, B{104.5, 108, 104, 107.5}), 100},
			{w(up, 2, B{104.25, 108, 104, 107.25}), 0},
			{w(dn, 2, B{101.5, 105, 101, 104.5}), -100},
			{w(dn, 2, B{102.75, 106, 102, 105.75}), 0},
			{w(up, 2, B{105.1, 108.6, 104.6, 108.1}), 100},
			{w(up, 2, B{105.3, 108.8, 104.8, 108.3}), 100}, // rejected only under defaults (band 0.2)
		}
		for k, tc := range cases {
			o, h, l, c := cdlgapsidesidewhiteSeries(tc.bars)
			got, err := CDLGAPSIDESIDEWHITE(o, h, l, c)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "CDLGAPSIDESIDEWHITE", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
			want := r.AlignedInt(0, len(o))
			assertInt(t, lbl("constructed exact=", exact, " #", k), got, want)
			if exact && want[12] != tc.want {
				t.Errorf("constructed #%d: C gave %d, want %d", k, want[12], tc.want)
			}
		}
	}
}

func TestCDLGAPSIDESIDEWHITEOracleRanges(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	_ = SetCandleSettings(Near, RangeHighLow, 5, 1.0)
	_ = SetCandleSettings(Equal, RangeHighLow, 5, 1.0)
	oracle.SetCandleSettings(int(Near), int(RangeHighLow), 5, 1.0)
	oracle.SetCandleSettings(int(Equal), int(RangeHighLow), 5, 1.0)
	for _, s := range fixtures(300) {
		n := len(s.C)
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 6}, {0, 7}, {6, 6}, {7, 7}, {8, 8}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlgapsidesidewhiteRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLGAPSIDESIDEWHITE", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLGAPSIDESIDEWHITE ", s.Name, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

func TestCDLGAPSIDESIDEWHITEOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLGAPSIDESIDEWHITE", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlgapsidesidewhiteRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLGAPSIDESIDEWHITEOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	for k, chs := range [][]change{
		{{Near, RangeHighLow, 0, 0.2}, {Equal, RangeHighLow, 0, 0.05}},
		{{Near, RangeHighLow, 0, 1.0}, {Equal, RangeHighLow, 0, 1.0}},
		{{Near, RangeRealBody, 1, 1.0}, {Equal, RangeRealBody, 3, 0.5}},
		{{Near, RangeShadows, 7, 0.8}, {Equal, RangeShadows, 2, 0.6}},
		{{Near, RangeHighLow, 10, 0.5}},
		{{Equal, RangeHighLow, 12, 0.5}},
		{{Near, RangeShadows, 0, 1.0}, {Equal, RangeRealBody, 0, 1.0}},
		{{Near, RangeRealBody, 50, 0.9}, {Equal, RangeHighLow, 20, 0.7}},
	} {
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		for _, ch := range chs {
			if err := SetCandleSettings(ch.st, ch.rt, ch.period, ch.factor); err != nil {
				t.Fatal(err)
			}
			oracle.SetCandleSettings(int(ch.st), int(ch.rt), ch.period, ch.factor)
		}
		fired := cdlgapsidesidewhiteOracleCompare(t, lbl("settings#", k))
		t.Logf("settings#%d: fired %d", k, fired)
	}
}

func TestCDLGAPSIDESIDEWHITEOraclePlanted(t *testing.T) {
	// Plant up- and down-gap patterns (and near misses) into every fixture at
	// several scales, so both branches fire many times on noisy data and Go
	// and C must agree bar for bar.
	type B = cdlgapsidesidewhiteBar
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	for _, factor := range []float64{0, 1} { // 0: default settings
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		if factor != 0 {
			cdlgapsidesidewhiteExactSettings(t)
			oracle.SetCandleSettings(int(Near), int(RangeHighLow), 5, 0.25)
			oracle.SetCandleSettings(int(Equal), int(RangeHighLow), 5, 0.125)
		}
		fired := map[int]int{}
		for _, s := range fixtures(400) {
			o := append([]float64(nil), s.O...)
			h := append([]float64(nil), s.H...)
			l := append([]float64(nil), s.L...)
			c := append([]float64(nil), s.C...)
			for k, at := 0, 20; at+3 <= len(o); k, at = k+1, at+17 {
				base := c[at-1]
				scale := 0.002 * base
				if base == 0 {
					scale = 0.01
				}
				pat := cdlgapsidesidewhiteUp
				if k%2 == 1 {
					pat = cdlgapsidesidewhiteDown
				}
				switch k % 6 {
				case 4: // near miss: 2nd body touches the 1st
					pat = cdlgapsidesidewhiteWith(cdlgapsidesidewhiteWith(cdlgapsidesidewhiteUp, 1, B{102, 105.5, 101.5, 105}), 2, B{102, 105.5, 101.5, 105})
				case 5: // 3rd opens a little higher
					pat = cdlgapsidesidewhiteWith(cdlgapsidesidewhiteDown, 2, B{102.1, 105.5, 101.5, 105.1})
				}
				for j, b := range pat {
					o[at+j] = base + (b.o-100)*scale
					h[at+j] = base + (b.h-100)*scale
					l[at+j] = base + (b.l-100)*scale
					c[at+j] = base + (b.c-100)*scale
				}
			}
			got, err := CDLGAPSIDESIDEWHITE(o, h, l, c)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "CDLGAPSIDESIDEWHITE", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
			assertInt(t, lbl("planted ", s.Name, " factor ", factor), got, r.AlignedInt(0, len(o)))
			for _, v := range got {
				fired[v]++
			}
		}
		t.Logf("settings %v: +100 x%d, -100 x%d", factor, fired[100], fired[-100])
		if fired[100] == 0 || fired[-100] == 0 {
			t.Fatalf("settings %v: up %d, down %d; both must fire", factor, fired[100], fired[-100])
		}
	}
}
