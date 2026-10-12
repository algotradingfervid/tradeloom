//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlpiercingOracleCompare compares CDLPIERCING with C on every fixture and
// edge length under the current candle settings, and returns how many bars
// fired.
func cdlpiercingOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	fired := 0
	lb := CDLPIERCINGLookback()
	if c := mustLookback(t, "CDLPIERCING"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLPIERCING(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLPIERCING", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLPIERCING ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLPIERCINGOracle(t *testing.T) {
	fired := cdlpiercingOracleCompare(t, "default")
	if fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
	t.Logf("fired %d times on fixtures", fired)
}

func TestCDLPIERCINGOracleConstructed(t *testing.T) {
	mods := []func(o, h, l, c []float64){
		func(o, h, l, c []float64) {},
		func(o, h, l, c []float64) { o[10], c[10] = 100, 104 },
		func(o, h, l, c []float64) { o[10] = 102; c[11] = 101.5 },
		func(o, h, l, c []float64) { o[10] = 102.5; c[11] = 101.5 },
		func(o, h, l, c []float64) { o[10], l[10], o[11], c[11] = 102.5, 100, 99.9, 101.3 },
		func(o, h, l, c []float64) { o[10], l[10], o[11], c[11] = 102.5, 100, 99.9, 102.2 },
		func(o, h, l, c []float64) { o[11], c[11] = 103, 99 },
		func(o, h, l, c []float64) { c[11] = 99 },
		func(o, h, l, c []float64) { o[11] = 99.5 },
		func(o, h, l, c []float64) { c[11] = 104 },
		func(o, h, l, c []float64) { c[11] = 103.99 },
		func(o, h, l, c []float64) { c[11] = 102 },
		func(o, h, l, c []float64) { c[11] = 102.01 },
	}
	for k, m := range mods {
		o, h, l, c := cdlpiercingSeries()
		m(o, h, l, c)
		got, err := CDLPIERCING(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLPIERCING", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed#", k), got, want)
		if k == 0 && want[11] != 100 {
			t.Fatalf("C did not fire on the constructed pattern: %v", want)
		}
	}
	// Repeated pattern in a longer series exercises the rolling totals.
	var o, h, l, c []float64
	for range 8 {
		bo, bh, bl, bc := cdlpiercingSeries()
		o, h, l, c = append(o, bo...), append(h, bh...), append(l, bl...), append(c, bc...)
	}
	got, _ := CDLPIERCING(o, h, l, c)
	r := mustCall(t, "CDLPIERCING", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
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

func TestCDLPIERCINGOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 10}, {0, 11}, {11, 11}, {10, 50}, {12, 12}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdlpiercingRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLPIERCING", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLPIERCING range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLPIERCINGOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLPIERCING", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlpiercingRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLPIERCINGOracleSettings(t *testing.T) {
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
		{{BodyLong, RangeRealBody, 10, 0.2}},
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
		total += cdlpiercingOracleCompare(t, lbl("settings#", k))
	}
	if total == 0 {
		t.Fatal("pattern never fired on any fixture under any setting")
	}
	t.Logf("fired %d times across settings", total)
}
