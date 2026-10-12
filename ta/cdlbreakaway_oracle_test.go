//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlbreakawayOracleCompare compares CDLBREAKAWAY with C on every fixture and
// edge length under the current candle settings, and returns how many bars
// fired.
func cdlbreakawayOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLBREAKAWAYLookback()
	if c := mustLookback(t, "CDLBREAKAWAY"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLBREAKAWAY(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLBREAKAWAY", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLBREAKAWAY ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLBREAKAWAYOracle(t *testing.T) {
	fired := cdlbreakawayOracleCompare(t, "default")
	t.Logf("default: fired %d", fired)
}

func TestCDLBREAKAWAYOracleFires(t *testing.T) {
	// With a lenient BodyLong the fixtures produce the pattern; Go and C must
	// agree on every bar and it must fire at least once.
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	if err := SetCandleSettings(BodyLong, RangeRealBody, 10, 0.1); err != nil {
		t.Fatal(err)
	}
	oracle.SetCandleSettings(int(BodyLong), int(RangeRealBody), 10, 0.1)
	fired := cdlbreakawayOracleCompare(t, "lenient")
	t.Logf("lenient: fired %d", fired)
	if fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
}

func TestCDLBREAKAWAYOracleConstructed(t *testing.T) {
	// The hand-built Breakaways (and their near misses) must agree in Go and C.
	type B = cdlbreakawayBar
	bu, be := cdlbreakawayBull, cdlbreakawayBear
	w := cdlbreakawayWith
	for k, tc := range []struct {
		bars [5]B
		want int
	}{
		{bu, 100},
		{be, -100},
		{w(bu, 0, B{102, 102.5, 99.5, 100}), 0},
		{w(bu, 0, B{102.1, 102.5, 99.5, 100}), 100},
		{w(bu, 1, B{100, 100.5, 95.5, 96}), 0},
		{w(bu, 2, B{95, 97, 94, 96}), 100},
		{w(bu, 2, B{96, 98.5, 94, 95}), 0},
		{w(bu, 3, B{95, 96, 94, 94.5}), 0},
		{w(bu, 4, B{93, 98.5, 92.8, 98}), 0},
		{w(bu, 4, B{93, 100.5, 92.8, 100}), 0},
		{w(bu, 4, B{93, 98.5, 92.8, 98.01}), 100},
		{w(be, 1, B{110, 114.5, 109.5, 114}), 0},
		{w(be, 2, B{115, 116, 113, 114}), -100},
		{w(be, 3, B{115, 117.5, 113, 117}), 0},
		{w(be, 4, B{111, 117.2, 110.8, 111}), 0},
		{w(be, 4, B{117, 117.2, 111.8, 112}), 0},
		{w(be, 4, B{117, 117.2, 109.8, 110}), 0},
		{w(be, 4, B{117, 117.2, 111.8, 111.99}), -100},
	} {
		o, h, l, c := cdlbreakawaySeries(tc.bars)
		got, err := CDLBREAKAWAY(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLBREAKAWAY", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed #", k), got, want)
		if want[14] != tc.want {
			t.Errorf("constructed #%d: C gave %d, want %d", k, want[14], tc.want)
		}
	}
}

func TestCDLBREAKAWAYOracleRanges(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	_ = SetCandleSettings(BodyLong, RangeRealBody, 10, 0.1)
	oracle.SetCandleSettings(int(BodyLong), int(RangeRealBody), 10, 0.1)
	for _, s := range fixtures(300) {
		n := len(s.C)
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 13}, {0, 14}, {13, 13}, {14, 14}, {15, 15}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlbreakawayRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLBREAKAWAY", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLBREAKAWAY ", s.Name, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

func TestCDLBREAKAWAYOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLBREAKAWAY", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlbreakawayRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLBREAKAWAYOracleSettings(t *testing.T) {
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
		fired := cdlbreakawayOracleCompare(t, lbl("settings#", k))
		t.Logf("settings#%d: fired %d", k, fired)
	}
}

func TestCDLBREAKAWAYOraclePlanted(t *testing.T) {
	// Plant bullish and bearish Breakaways (and near misses) into every
	// fixture at several scales, so both branches fire many times on noisy
	// data and Go and C must agree bar for bar.
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	for _, setting := range []float64{1.0, 0.5} {
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		_ = SetCandleSettings(BodyLong, RangeRealBody, 10, setting)
		oracle.SetCandleSettings(int(BodyLong), int(RangeRealBody), 10, setting)
		fired := map[int]int{}
		for _, s := range fixtures(400) {
			o := append([]float64(nil), s.O...)
			h := append([]float64(nil), s.H...)
			l := append([]float64(nil), s.L...)
			c := append([]float64(nil), s.C...)
			for k, at := 0, 20; at+5 <= len(o); k, at = k+1, at+23 {
				base := c[at-1]
				scale := 0.002 * base
				if base == 0 {
					scale = 0.01
				}
				pat := cdlbreakawayBull
				if k%2 == 1 {
					pat = cdlbreakawayBear
				}
				if k%5 == 4 { // near miss: 5th closes exactly at the gap edge
					pat = cdlbreakawayWith(cdlbreakawayBull, 4, cdlbreakawayBar{93, 98.5, 92.8, 98})
				}
				for j, b := range pat {
					o[at+j] = base + (b.o-100)*scale
					h[at+j] = base + (b.h-100)*scale
					l[at+j] = base + (b.l-100)*scale
					c[at+j] = base + (b.c-100)*scale
				}
			}
			got, err := CDLBREAKAWAY(o, h, l, c)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "CDLBREAKAWAY", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
			assertInt(t, lbl("planted ", s.Name, " factor ", setting), got, r.AlignedInt(0, len(o)))
			for _, v := range got {
				fired[v]++
			}
		}
		t.Logf("factor %v: +100 x%d, -100 x%d", setting, fired[100], fired[-100])
		if fired[100] == 0 || fired[-100] == 0 {
			t.Fatalf("factor %v: bullish %d, bearish %d; both must fire", setting, fired[100], fired[-100])
		}
	}
}
