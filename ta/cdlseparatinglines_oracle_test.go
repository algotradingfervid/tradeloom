//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlseparatinglinesOracleCompare compares CDLSEPARATINGLINES with C on every
// fixture and edge length under the current candle settings, and returns how
// many bars fired.
func cdlseparatinglinesOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDLSEPARATINGLINESLookback()
	if c := mustLookback(t, "CDLSEPARATINGLINES"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLSEPARATINGLINES(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLSEPARATINGLINES", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLSEPARATINGLINES ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	return fired
}

func TestCDLSEPARATINGLINESOracle(t *testing.T) {
	// Firing on the fixtures under default settings is not guaranteed (the
	// Equal band is 5% of the average range); firing is proven by
	// TestCDLSEPARATINGLINESOracleConstructed and
	// TestCDLSEPARATINGLINESOracleSettings.
	fired := cdlseparatinglinesOracleCompare(t, "default")
	t.Logf("default settings: %d bars fired", fired)
}

func TestCDLSEPARATINGLINESOracleConstructed(t *testing.T) {
	// The hand-built patterns, including the exact Equal and ShadowVeryShort
	// edges, must agree with C, and fire with both signs.
	pos, neg := 0, 0
	for _, tc := range cdlseparatinglinesCases() {
		o, h, l, c := cdlseparatinglinesSeries(tc.b1, tc.b2)
		got, err := CDLSEPARATINGLINES(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDLSEPARATINGLINES", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, "constructed "+tc.name, got, want)
		if want[11] != tc.want {
			t.Fatalf("%s: C gave %d at bar 11, want %d", tc.name, want[11], tc.want)
		}
		switch want[11] {
		case 100:
			pos++
		case -100:
			neg++
		}
	}
	if pos == 0 || neg == 0 {
		t.Fatalf("constructed cases fired +%d -%d times", pos, neg)
	}
}

func TestCDLSEPARATINGLINESOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 10}, {0, 11}, {10, 10}, {11, 11}, {12, 12}, {10, 50}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdlseparatinglinesRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLSEPARATINGLINES", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLSEPARATINGLINES range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLSEPARATINGLINESOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLSEPARATINGLINES", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlseparatinglinesRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLSEPARATINGLINESOracleSettings(t *testing.T) {
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
		{{ShadowVeryShort, RangeShadows, 3, 0.3}},
		{{ShadowVeryShort, RangeRealBody, 20, 1.0}},
		{{Equal, RangeShadows, 20, 1.5}},
		{{BodyLong, RangeRealBody, 0, 0.5}},
		{{ShadowVeryShort, RangeHighLow, 0, 0.2}},
		{{Equal, RangeHighLow, 0, 0.2}},
		// All periods 0: lookback 1.
		{{BodyLong, RangeRealBody, 0, 0.8}, {Equal, RangeHighLow, 0, 0.5}, {ShadowVeryShort, RangeHighLow, 0, 0.3}},
		// Long periods.
		{{BodyLong, RangeRealBody, 50, 1.5}, {Equal, RangeShadows, 30, 1.0}, {ShadowVeryShort, RangeHighLow, 40, 0.1}},
		// Wide Equal band and lax belt hold so the pattern fires often.
		{{BodyLong, RangeRealBody, 10, 0.5}, {Equal, RangeHighLow, 5, 1.0}, {ShadowVeryShort, RangeHighLow, 10, 1.0}},
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
		fired += cdlseparatinglinesOracleCompare(t, lbl("settings#", k))
	}
	if fired == 0 {
		t.Fatal("pattern never fired under modified settings")
	}
	t.Logf("modified settings: %d bars fired", fired)
}
