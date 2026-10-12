//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlclosingmarubozuOracleCompare compares CDLCLOSINGMARUBOZU with C on every
// fixture and edge length under the current candle settings, and returns how
// many bars fired white (+100) and black (-100).
func cdlclosingmarubozuOracleCompare(t *testing.T, tag string) (white, black int) {
	t.Helper()
	lb := CDLCLOSINGMARUBOZULookback()
	if c := mustLookback(t, "CDLCLOSINGMARUBOZU"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDLCLOSINGMARUBOZU(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLCLOSINGMARUBOZU", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLCLOSINGMARUBOZU ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				switch v {
				case 100:
					white++
				case -100:
					black++
				}
			}
		}
	}
	return white, black
}

func TestCDLCLOSINGMARUBOZUOracle(t *testing.T) {
	white, black := cdlclosingmarubozuOracleCompare(t, "default")
	if white == 0 || black == 0 {
		t.Fatalf("pattern fired white %d, black %d times; want both > 0", white, black)
	}
}

func TestCDLCLOSINGMARUBOZUOracleConstructed(t *testing.T) {
	// Hand-built white and black closing marubozu must fire in Go and C.
	o, h, l, c := cdlclosingmarubozuBase(10)
	o = append(o, 100, 103, 100)
	h = append(h, 103.25, 104, 102)
	l = append(l, 99, 99.75, 99)
	c = append(c, 103, 100, 102)
	got, err := CDLCLOSINGMARUBOZU(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CDLCLOSINGMARUBOZU", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	want := r.AlignedInt(0, len(o))
	assertInt(t, "constructed", got, want)
	if want[10] != 100 || want[11] != -100 {
		t.Fatalf("C did not fire on the constructed bars: %v", want)
	}
}

func TestCDLCLOSINGMARUBOZUOracleRanges(t *testing.T) {
	for _, s := range fixtures(300) {
		n := len(s.C)
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 9}, {0, 10}, {9, 9}, {10, 10}, {11, 11}, {9, 50}, {150, 299}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdlclosingmarubozuRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLCLOSINGMARUBOZU", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLCLOSINGMARUBOZU ", s.Name, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

func TestCDLCLOSINGMARUBOZUOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLCLOSINGMARUBOZU", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlclosingmarubozuRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLCLOSINGMARUBOZUOracleSettings(t *testing.T) {
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
		{{BodyLong, RangeShadows, 7, 1.5}},
		{{ShadowVeryShort, RangeShadows, 3, 0.3}},
		{{ShadowVeryShort, RangeRealBody, 20, 0.2}},
		{{BodyLong, RangeRealBody, 0, 0.5}},
		{{ShadowVeryShort, RangeHighLow, 0, 0.2}},
		// Both periods 0: lookback 0.
		{{BodyLong, RangeRealBody, 0, 0.6}, {ShadowVeryShort, RangeHighLow, 0, 0.1}},
		{{BodyLong, RangeShadows, 0, 1.0}, {ShadowVeryShort, RangeShadows, 0, 0.5}},
		// Long periods.
		{{BodyLong, RangeRealBody, 50, 1.0}, {ShadowVeryShort, RangeHighLow, 30, 0.2}},
		// Unrelated setting changed: no effect.
		{{BodyShort, RangeHighLow, 40, 2.0}},
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
		cdlclosingmarubozuOracleCompare(t, lbl("settings#", k))
	}
}
