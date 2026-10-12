//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlinneckOracleCompare compares CDLINNECK with C on every fixture and edge
// length under the current candle settings, and returns how many bars fired.
func cdlinneckOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLINNECKLookback()
	if c := mustLookback(t, "CDLINNECK"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLINNECK(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLINNECK", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLINNECK ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLINNECKOracle(t *testing.T) {
	// With default settings the fixtures (gaps, walks) produce some in-neck
	// bars; require at least one so the default comparison is not all zeros.
	// Band edges are covered by TestCDLINNECKOracleConstructed.
	fired := cdlinneckOracleCompare(t, "default")
	if fired == 0 {
		t.Fatal("pattern never fired with default settings")
	}
	t.Logf("default settings: %d bars fired", fired)
}

func TestCDLINNECKOracleConstructed(t *testing.T) {
	// The hand-built patterns, including the exact band and prior-low edges,
	// must agree with C.
	fired := 0
	for _, tc := range cdlinneckCases() {
		o, h, l, c := cdlinneckSeries(tc.b1, tc.b2)
		got, err := CDLINNECK(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLINNECK", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed ", tc.name), got, want)
		if want[11] != tc.want {
			t.Fatalf("%s: C gave %d at bar 11, want %d", tc.name, want[11], tc.want)
		}
		if want[11] != 0 {
			fired++
		}
	}
	if fired == 0 {
		t.Fatal("no constructed case fired in C")
	}
}

func TestCDLINNECKOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 10}, {0, 11}, {10, 10}, {11, 11}, {12, 12}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdlinneckRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLINNECK", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLINNECK range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLINNECKOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLINNECK", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlinneckRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLINNECKOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		{{BodyLong, RangeHighLow, 5, 0.5}},
		{{Equal, RangeRealBody, 7, 0.4}},
		{{BodyLong, RangeShadows, 3, 0.3}},
		{{Equal, RangeShadows, 20, 1.5}},
		{{BodyLong, RangeRealBody, 0, 0.5}},
		{{Equal, RangeHighLow, 0, 0.2}},
		// All periods 0: lookback 1.
		{{BodyLong, RangeRealBody, 0, 0.8}, {Equal, RangeHighLow, 0, 0.5}},
		// Long periods.
		{{BodyLong, RangeRealBody, 50, 1.5}, {Equal, RangeShadows, 30, 1.0}},
		// Wide Equal band and short BodyLong so the pattern fires often.
		{{BodyLong, RangeRealBody, 10, 0.5}, {Equal, RangeHighLow, 5, 1.0}},
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
		fired += cdlinneckOracleCompare(t, lbl("settings#", k))
	}
	if fired == 0 {
		t.Fatal("pattern never fired under modified settings")
	}
	t.Logf("modified settings: %d bars fired", fired)
}
