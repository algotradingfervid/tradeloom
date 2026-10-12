//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlrisefall3methodsOracleCompare compares CDLRISEFALL3METHODS with C on
// every fixture and edge length under the current candle settings, and
// returns how many bars fired.
func cdlrisefall3methodsOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLRISEFALL3METHODSLookback()
	if c := mustLookback(t, "CDLRISEFALL3METHODS"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLRISEFALL3METHODS(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLRISEFALL3METHODS", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLRISEFALL3METHODS ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLRISEFALL3METHODSOracle(t *testing.T) {
	// The random fixtures rarely produce this five-bar pattern; the planted
	// series below guarantees both signs fire.
	cdlrisefall3methodsOracleCompare(t, "default")
}

// cdlrisefall3methodsPlanted returns a long random walk with rising and
// falling three methods planted at regular intervals, so the recogniser
// fires many times in a realistic context.
func cdlrisefall3methodsPlanted() series {
	s := fixtures(400)[0]
	rising := cdlrisefall3methodsRising()
	falling := cdlrisefall3methodsMirror(rising)
	plant := func(at int, bars []cdlrisefall3methodsBar) {
		base := s.C[at-1] - bars[0].o
		for k, b := range bars {
			s.O[at+k], s.H[at+k], s.L[at+k], s.C[at+k] = b.o+base, b.h+base, b.l+base, b.c+base
		}
	}
	for at := 20; at+5 < 400; at += 30 {
		if (at/30)%2 == 0 {
			plant(at, rising)
		} else {
			plant(at, falling)
		}
	}
	return s
}

func TestCDLRISEFALL3METHODSOracleConstructed(t *testing.T) {
	rising := cdlrisefall3methodsRising()
	cases := []struct {
		name string
		bars []cdlrisefall3methodsBar
		want int
	}{
		{"rising", rising, 100},
		{"falling", cdlrisefall3methodsMirror(rising), -100},
	}
	for _, tc := range cases {
		o, h, l, c := cdlrisefall3methodsSeries(tc.bars...)
		got, err := CDLRISEFALL3METHODS(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLRISEFALL3METHODS", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, "constructed "+tc.name, got, want)
		if want[len(o)-1] != tc.want {
			t.Fatalf("%s: C output %v, want %d at the end", tc.name, want, tc.want)
		}
	}

	// Planted series: both signs fire in Go and C and agree everywhere.
	s := cdlrisefall3methodsPlanted()
	got, err := CDLRISEFALL3METHODS(s.O, s.H, s.L, s.C)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CDLRISEFALL3METHODS", s.ohlcv())
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
		t.Fatalf("planted series: C fired %d rising, %d falling; want both > 0", pos, neg)
	}
}

func TestCDLRISEFALL3METHODSOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(300)[0], cdlrisefall3methodsPlanted()} {
		n := len(s.C)
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {4, 4}, {0, 13}, {0, 14}, {13, 13}, {14, 14}, {15, 15}, {10, 50}, {20, 30}, {24, 24}, {150, n - 1}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlrisefall3methodsRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLRISEFALL3METHODS", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLRISEFALL3METHODS range ", s.Name, " ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

func TestCDLRISEFALL3METHODSOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLRISEFALL3METHODS", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlrisefall3methodsRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLRISEFALL3METHODSOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		{{BodyLong, RangeRealBody, 0, 0.5}, {BodyShort, RangeRealBody, 0, 2.0}},
		{{BodyLong, RangeRealBody, 0, 1.0}, {BodyShort, RangeRealBody, 0, 1.0}},
		{{BodyLong, RangeHighLow, 5, 0.3}, {BodyShort, RangeHighLow, 20, 0.5}},
		{{BodyLong, RangeShadows, 3, 0.5}, {BodyShort, RangeShadows, 1, 2.0}},
		{{BodyLong, RangeRealBody, 1, 0.0}, {BodyShort, RangeRealBody, 40, 3.0}},
		{{BodyLong, RangeRealBody, 40, 0.5}},
		{{BodyShort, RangeHighLow, 2, 0.4}},
		// A setting the pattern does not use: no effect.
		{{Near, RangeHighLow, 30, 2.0}},
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
		cdlrisefall3methodsOracleCompare(t, lbl("settings#", k))
		s := cdlrisefall3methodsPlanted()
		got, err := CDLRISEFALL3METHODS(s.O, s.H, s.L, s.C)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLRISEFALL3METHODS", s.ohlcv())
		assertInt(t, lbl("planted settings#", k), got, r.AlignedInt(0, len(s.C)))
	}
}
