//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlhikkakemodOracleCompare compares CDLHIKKAKEMOD with C on every fixture
// and edge length under the current candle settings, and returns how many
// bars fired with each value.
func cdlhikkakemodOracleCompare(t *testing.T, tag string) map[int]int {
	t.Helper()
	lb := CDLHIKKAKEMODLookback()
	if c := mustLookback(t, "CDLHIKKAKEMOD"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := map[int]int{}
	lengths := append(edgeLengths(lb), 1000)
	for _, n := range lengths {
		for _, s := range fixtures(n) {
			got, err := CDLHIKKAKEMOD(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLHIKKAKEMOD", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLHIKKAKEMOD ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired[v]++
				}
			}
		}
	}
	return fired
}

func TestCDLHIKKAKEMODOracle(t *testing.T) {
	fired := cdlhikkakemodOracleCompare(t, "default")
	t.Logf("fired: %v", fired)
	if len(fired) == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
}

func TestCDLHIKKAKEMODOracleConstructed(t *testing.T) {
	sec := cdlhikkakemodSecond
	w := cdlhikkakemodWide
	first, third := cdlhikkakemodFirst, cdlhikkakemodThird
	bull4, bear4 := cdlhikkakemodBull4, cdlhikkakemodBear4
	cases := []struct {
		bars []cdlhikkakemodBar
		want map[int]int
	}{
		{[]cdlhikkakemodBar{first, sec(92.5), third, bull4, w(106.5)}, map[int]int{13: 100, 14: 200}},
		{[]cdlhikkakemodBar{first, sec(107.5), third, bear4, w(100), w(100), w(93.5)}, map[int]int{13: -100, 16: -200}},
		{[]cdlhikkakemodBar{first, sec(92 + cdlhikkakemodNearAv), third, bull4}, map[int]int{13: 100}},
		{[]cdlhikkakemodBar{first, sec(108 - cdlhikkakemodNearAv), third, bear4}, map[int]int{13: -100}},
		{[]cdlhikkakemodBar{first, sec(92.5), third, bull4, w(100), w(100), w(100), w(106.5)}, map[int]int{13: 100}},
		{cdlhikkakemodOverwrite, map[int]int{13: -100, 16: 100, 17: 200}},
	}
	for k, tc := range cases {
		o, h, l, c := cdlhikkakemodSeries(tc.bars...)
		got, err := CDLHIKKAKEMOD(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		in := oracle.Inputs{Open: o, High: h, Low: l, Close: c}
		r := mustCall(t, "CDLHIKKAKEMOD", in)
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed#", k), got, want)
		exp := make([]int, len(o))
		for i, v := range tc.want {
			exp[i] = v
		}
		assertInt(t, lbl("constructed#", k, " C vs expected"), want, exp)
		// Every sub-range ending at the last bar (confirmation replay).
		for start := 0; start < len(o); start++ {
			out := make([]int, len(o))
			beg, nb, err := cdlhikkakemodRange(start, len(o)-1, o, h, l, c, out)
			if err != nil {
				t.Fatal(err)
			}
			rr := mustCallRange(t, "CDLHIKKAKEMOD", in, start, len(o)-1)
			if beg != rr.BegIdx || nb != rr.NbElement {
				t.Fatalf("constructed#%d start %d: go (%d,%d) c (%d,%d)", k, start, beg, nb, rr.BegIdx, rr.NbElement)
			}
			assertInt(t, lbl("constructed#", k, " start ", start), out[:nb], rr.Outputs[0].Int[:nb])
		}
	}
}

func TestCDLHIKKAKEMODOracleRanges(t *testing.T) {
	for _, s := range fixtures(300) {
		n := len(s.C)
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 9}, {0, 10}, {9, 9}, {10, 10}, {11, 11}, {12, 14}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlhikkakemodRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLHIKKAKEMOD", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLHIKKAKEMOD ", s.Name, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

func TestCDLHIKKAKEMODOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLHIKKAKEMOD", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlhikkakemodRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLHIKKAKEMODOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		{{Near, RangeHighLow, 0, 0.2}}, // lookback 6, own range
		{{Near, RangeHighLow, 1, 0.5}}, // lookback 6
		{{Near, RangeRealBody, 3, 1.0}},
		{{Near, RangeShadows, 7, 0.8}},
		{{Near, RangeShadows, 0, 1.0}},
		{{Near, RangeRealBody, 0, 3.0}},
		{{Near, RangeHighLow, 30, 2.0}},
		{{Near, RangeHighLow, 5, 0}},      // close exactly on the extreme
		{{BodyLong, RangeHighLow, 50, 1}}, // unrelated setting: no effect
	}
	fired := 0
	for k, set := range sets {
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		for _, ch := range set {
			if err := SetCandleSettings(ch.st, ch.rt, ch.period, ch.factor); err != nil {
				t.Fatal(err)
			}
			oracle.SetCandleSettings(int(ch.st), int(ch.rt), ch.period, ch.factor)
		}
		for _, v := range cdlhikkakemodOracleCompare(t, lbl("settings#", k)) {
			fired += v
		}
		// Sub-ranges under the modified settings too.
		s := fixtures(200)[0]
		for _, rg := range [][2]int{{0, 0}, {5, 5}, {6, 6}, {7, 40}, {100, 199}} {
			out := make([]int, 200)
			beg, nb, err := cdlhikkakemodRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLHIKKAKEMOD", s.ohlcv(), rg[0], rg[1])
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("settings#%d range %v: go (%d,%d) c (%d,%d)", k, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("settings#", k, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
	if fired == 0 {
		t.Fatal("pattern never fired under modified settings")
	}
}
