//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdl3linestrikeOracleCompare compares CDL3LINESTRIKE with C on every fixture
// and edge length under the current candle settings, and returns how many
// bars fired.
func cdl3linestrikeOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDL3LINESTRIKELookback()
	if c := mustLookback(t, "CDL3LINESTRIKE"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDL3LINESTRIKE(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDL3LINESTRIKE", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDL3LINESTRIKE ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDL3LINESTRIKEOracle(t *testing.T) {
	if fired := cdl3linestrikeOracleCompare(t, "default"); fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
}

// cdl3linestrikePlanted returns a long random walk with bullish and bearish
// three-line strikes planted at regular intervals, so the recogniser fires
// many times in a realistic context.
func cdl3linestrikePlanted() series {
	s := fixtures(400)[0]
	white := cdl3linestrikeWhite()
	black := cdl3linestrikeBlack()
	plant := func(at int, bars []cdl3linestrikeBar, last cdl3linestrikeBar) {
		base := s.C[at-1] - 100
		for k, b := range append(append([]cdl3linestrikeBar{}, bars...), last) {
			s.O[at+k], s.H[at+k], s.L[at+k], s.C[at+k] = b.o+base, b.h+base, b.l+base, b.c+base
		}
	}
	for at := 20; at+4 < 400; at += 30 {
		if (at/30)%2 == 0 {
			plant(at, white, cdl3linestrikeBar{105, 106, 98, 99})
		} else {
			plant(at, black, cdl3linestrikeBar{97, 104, 96, 103})
		}
	}
	return s
}

func TestCDL3LINESTRIKEOracleConstructed(t *testing.T) {
	cases := []struct {
		name  string
		three []cdl3linestrikeBar
		last  cdl3linestrikeBar
		want  int
	}{
		{"bullish", cdl3linestrikeWhite(), cdl3linestrikeBar{105, 106, 98, 99}, 100},
		{"bearish", cdl3linestrikeBlack(), cdl3linestrikeBar{97, 104, 96, 103}, -100},
	}
	for _, tc := range cases {
		o, h, l, c := cdl3linestrikeSeries(append(tc.three, tc.last)...)
		got, err := CDL3LINESTRIKE(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDL3LINESTRIKE", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, "constructed "+tc.name, got, want)
		if want[8] != tc.want {
			t.Fatalf("%s: C output %v, want %d at [8]", tc.name, want, tc.want)
		}
	}

	// Planted series: both signs fire in Go and C and agree everywhere.
	s := cdl3linestrikePlanted()
	got, err := CDL3LINESTRIKE(s.O, s.H, s.L, s.C)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CDL3LINESTRIKE", s.ohlcv())
	want := r.AlignedInt(0, len(s.C))
	assertInt(t, "planted", got, want)
	pos, neg := 0, 0
	for _, v := range want {
		if v > 0 {
			pos++
		} else if v < 0 {
			neg++
		}
	}
	if pos == 0 || neg == 0 {
		t.Fatalf("planted series: C fired %d bullish, %d bearish; want both > 0", pos, neg)
	}
}

func TestCDL3LINESTRIKEOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(300)[0], cdl3linestrikePlanted()} {
		n := len(s.C)
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 7}, {0, 8}, {7, 7}, {8, 8}, {9, 9}, {10, 50}, {20, 30}, {150, n - 1}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdl3linestrikeRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDL3LINESTRIKE", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDL3LINESTRIKE range ", s.Name, " ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

func TestCDL3LINESTRIKEOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDL3LINESTRIKE", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdl3linestrikeRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDL3LINESTRIKEOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		{{Near, RangeHighLow, 0, 0.2}},
		{{Near, RangeRealBody, 0, 1.0}},
		{{Near, RangeShadows, 3, 0.5}},
		{{Near, RangeRealBody, 10, 2.0}},
		{{Near, RangeHighLow, 1, 0.0}},
		{{Near, RangeHighLow, 40, 3.0}},
		// A setting the pattern does not use: no effect.
		{{BodyLong, RangeHighLow, 30, 2.0}},
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
		cdl3linestrikeOracleCompare(t, lbl("settings#", k))
		s := cdl3linestrikePlanted()
		got, err := CDL3LINESTRIKE(s.O, s.H, s.L, s.C)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDL3LINESTRIKE", s.ohlcv())
		assertInt(t, lbl("planted settings#", k), got, r.AlignedInt(0, len(s.C)))
	}
}
