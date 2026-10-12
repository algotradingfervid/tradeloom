//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdltristarOracleCompare compares CDLTRISTAR with C on every fixture and
// edge length under the current candle settings, and returns how many bars
// fired.
func cdltristarOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLTRISTARLookback()
	if c := mustLookback(t, "CDLTRISTAR"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLTRISTAR(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLTRISTAR", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLTRISTAR ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLTRISTAROracle(t *testing.T) {
	if fired := cdltristarOracleCompare(t, "default"); fired == 0 {
		t.Fatal("pattern never fired on any fixture")
	}
}

func TestCDLTRISTAROracleConstructed(t *testing.T) {
	// The hand-built patterns must fire (or not) identically in Go and C,
	// with both signs, including the exact-threshold cases.
	pos, neg := 0, 0
	for _, tc := range cdltristarCases {
		o, h, l, c := cdltristarSeries(tc.b1, tc.b2, tc.b3)
		got, err := CDLTRISTAR(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLTRISTAR", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed ", tc.name), got, want)
		if want[12] != tc.want {
			t.Fatalf("%s: C gave %d at bar 12, want %d", tc.name, want[12], tc.want)
		}
		switch want[12] {
		case 100:
			pos++
		case -100:
			neg++
		}
	}
	if pos == 0 || neg == 0 {
		t.Fatalf("constructed cases fired +100 %d times, -100 %d times", pos, neg)
	}
}

// TestCDLTRISTAROracleLongFires tiles the bullish and bearish patterns with background
// bars so a long series fires many times, in Go and C alike.
func TestCDLTRISTAROracleLongFires(t *testing.T) {
	var o, h, l, c []float64
	for k := 0; k < 20; k++ {
		tc := cdltristarCases[k%2]
		so, sh, sl, sc := cdltristarSeries(tc.b1, tc.b2, tc.b3)
		o, h, l, c = append(o, so...), append(h, sh...), append(l, sl...), append(c, sc...)
	}
	got, err := CDLTRISTAR(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CDLTRISTAR", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	assertInt(t, "long tiled", got, r.AlignedInt(0, len(o)))
	fired := 0
	for _, v := range got {
		if v != 0 {
			fired++
		}
	}
	if fired < 20 {
		t.Fatalf("tiled series fired %d times, want >= 20", fired)
	}
}

func TestCDLTRISTAROracleRanges(t *testing.T) {
	s := fixtures(300)[10] // doji fixture
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 11}, {0, 12}, {11, 11}, {12, 12}, {13, 13}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdltristarRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLTRISTAR", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLTRISTAR range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLTRISTAROracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLTRISTAR", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdltristarRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLTRISTAROracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		{{BodyDoji, RangeRealBody, 7, 0.4}},
		{{BodyDoji, RangeShadows, 20, 1.5}},
		{{BodyDoji, RangeHighLow, 0, 0.3}},
		{{BodyDoji, RangeRealBody, 0, 1.0}},
		{{BodyDoji, RangeHighLow, 1, 0.5}},
		{{BodyDoji, RangeHighLow, 50, 2.0}},
		// An unrelated setting must not matter.
		{{BodyLong, RangeShadows, 3, 0.3}},
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
		fired += cdltristarOracleCompare(t, lbl("settings#", k))
	}
	if fired == 0 {
		t.Fatal("pattern never fired under modified settings")
	}
}
