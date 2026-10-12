//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdltasukigapOracleCompare compares CDLTASUKIGAP with C on every fixture and
// edge length under the current candle settings, and returns how many bars
// fired.
func cdltasukigapOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLTASUKIGAPLookback()
	if c := mustLookback(t, "CDLTASUKIGAP"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLTASUKIGAP(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLTASUKIGAP", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLTASUKIGAP ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLTASUKIGAPOracle(t *testing.T) {
	fired := cdltasukigapOracleCompare(t, "default")
	t.Logf("default: fired %d", fired)
}

func TestCDLTASUKIGAPOracleFires(t *testing.T) {
	// With a lenient Near band the fixtures produce the pattern; Go and C
	// must agree on every bar and it must fire at least once.
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	_ = SetCandleSettings(Near, RangeHighLow, 5, 2.0)
	oracle.SetCandleSettings(int(Near), int(RangeHighLow), 5, 2.0)
	fired := cdltasukigapOracleCompare(t, "lenient")
	t.Logf("lenient: fired %d", fired)
	if fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
}

func TestCDLTASUKIGAPOracleConstructed(t *testing.T) {
	// The hand-built patterns and near misses must agree in Go and C, under
	// default and exact (dyadic) settings.
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	for _, exact := range []bool{false, true} {
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		if exact {
			cdltasukigapExactSettings(t)
			oracle.SetCandleSettings(int(Near), int(RangeHighLow), 5, 0.25)
		}
		for _, tc := range cdltasukigapExactCases() {
			o, h, l, c := cdltasukigapSeries(tc.bars)
			got, err := CDLTASUKIGAP(o, h, l, c)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "CDLTASUKIGAP", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			want := r.AlignedInt(0, len(o))
			assertInt(t, lbl("constructed exact=", exact, " ", tc.name), got, want)
			if exact && want[12] != tc.want {
				t.Errorf("constructed %q: C gave %d, want %d", tc.name, want[12], tc.want)
			}
		}
	}
}

func TestCDLTASUKIGAPOracleRanges(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	_ = SetCandleSettings(Near, RangeHighLow, 5, 2.0)
	oracle.SetCandleSettings(int(Near), int(RangeHighLow), 5, 2.0)
	for _, s := range fixtures(300) {
		n := len(s.C)
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 6}, {0, 7}, {6, 6}, {7, 7}, {8, 8}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdltasukigapRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLTASUKIGAP", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLTASUKIGAP ", s.Name, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

func TestCDLTASUKIGAPOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLTASUKIGAP", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdltasukigapRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLTASUKIGAPOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	for k, chs := range [][]change{
		{{Near, RangeHighLow, 0, 0.2}},
		{{Near, RangeHighLow, 0, 2.0}},
		{{Near, RangeHighLow, 1, 1.0}},
		{{Near, RangeRealBody, 3, 1.5}},
		{{Near, RangeShadows, 7, 3.0}},
		{{Near, RangeShadows, 0, 3.0}},
		{{Near, RangeRealBody, 0, 1.0}},
		{{Near, RangeHighLow, 50, 0.9}},
		// Settings the pattern does not use must not change anything.
		{{Equal, RangeHighLow, 20, 5.0}, {BodyLong, RangeShadows, 0, 3.0}},
	} {
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		for _, ch := range chs {
			if err := SetCandleSettings(ch.st, ch.rt, ch.period, ch.factor); err != nil {
				t.Fatal(err)
			}
			oracle.SetCandleSettings(int(ch.st), int(ch.rt), ch.period, ch.factor)
		}
		fired := cdltasukigapOracleCompare(t, lbl("settings#", k))
		t.Logf("settings#%d: fired %d", k, fired)
	}
}

func TestCDLTASUKIGAPOraclePlanted(t *testing.T) {
	// Plant upside and downside tasuki gaps (and near misses) into every
	// fixture at several scales, so both branches fire many times on noisy
	// data and Go and C must agree bar for bar.
	type B = cdltasukigapBar
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	for _, factor := range []float64{0, 1} { // 0: default settings
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		if factor != 0 {
			cdltasukigapExactSettings(t)
			oracle.SetCandleSettings(int(Near), int(RangeHighLow), 5, 0.25)
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
				pat := cdltasukigapUp
				if k%2 == 1 {
					pat = cdltasukigapDown
				}
				switch k % 6 {
				case 4: // near miss: closes exactly at the open of the gap candle
					pat = cdltasukigapWith(cdltasukigapUp, 2, B{106.5, 107, 103.5, 104})
				case 5: // body a little bigger than the gap candle's
					pat = cdltasukigapWith(cdltasukigapDown, 2, B{103.5, 107.5, 103, 107.3})
				}
				for j, b := range pat {
					o[at+j] = base + (b.o-100)*scale
					h[at+j] = base + (b.h-100)*scale
					l[at+j] = base + (b.l-100)*scale
					c[at+j] = base + (b.c-100)*scale
				}
			}
			got, err := CDLTASUKIGAP(o, h, l, c)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "CDLTASUKIGAP", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
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
