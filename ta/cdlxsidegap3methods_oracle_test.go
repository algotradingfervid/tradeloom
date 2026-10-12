//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlxsidegap3methodsOracleCompare compares CDLXSIDEGAP3METHODS with C on
// every fixture and edge length under the current candle settings, and
// returns how many bars fired up and down.
func cdlxsidegap3methodsOracleCompare(t *testing.T, tag string) (up, down int) {
	t.Helper()
	lb := CDLXSIDEGAP3METHODSLookback()
	if c := mustLookback(t, "CDLXSIDEGAP3METHODS"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	for _, n := range append(edgeLengths(lb), 1000) {
		for _, s := range fixtures(n) {
			got, err := CDLXSIDEGAP3METHODS(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLXSIDEGAP3METHODS", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLXSIDEGAP3METHODS ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				switch {
				case v > 0:
					up++
				case v < 0:
					down++
				}
			}
		}
	}
	return up, down
}

func TestCDLXSIDEGAP3METHODSOracle(t *testing.T) {
	up, down := cdlxsidegap3methodsOracleCompare(t, "default")
	if up == 0 || down == 0 {
		t.Fatalf("pattern did not fire in both directions: up=%d down=%d", up, down)
	}
}

func TestCDLXSIDEGAP3METHODSOracleConstructed(t *testing.T) {
	// Up at bar 2, down at bar 5, then boundary near-misses (no gap,
	// open at 2nd top, close at 1st bottom, doji third).
	o := []float64{10, 13, 14, 15, 12, 11, 10, 12, 14, 10, 13, 15, 15, 12, 11}
	c := []float64{12, 15, 11, 13, 10, 14, 12, 15, 11, 12, 15, 10, 13, 10, 11}
	h := make([]float64, len(o))
	l := make([]float64, len(o))
	for i := range o {
		h[i] = max(o[i], c[i]) + 1
		l[i] = min(o[i], c[i]) - 1
	}
	got, err := CDLXSIDEGAP3METHODS(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CDLXSIDEGAP3METHODS", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	want := r.AlignedInt(0, len(o))
	assertInt(t, "constructed", got, want)
	if want[2] != 100 || want[5] != -100 {
		t.Fatalf("C did not fire on the constructed patterns: %v", want)
	}
}

func TestCDLXSIDEGAP3METHODSOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {0, 1}, {0, 2}, {1, 1}, {2, 2}, {1, 50}, {3, 3}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdlxsidegap3methodsRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLXSIDEGAP3METHODS", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLXSIDEGAP3METHODS range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLXSIDEGAP3METHODSOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLXSIDEGAP3METHODS", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlxsidegap3methodsRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLXSIDEGAP3METHODSOracleSettings(t *testing.T) {
	// The pattern ignores candle settings; changed settings must not alter
	// the lookback or the output in either Go or C.
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	for _, st := range []CandleSettingType{BodyLong, BodyShort, Near, Equal} {
		if err := SetCandleSettings(st, RangeShadows, 25, 3.0); err != nil {
			t.Fatal(err)
		}
		oracle.SetCandleSettings(int(st), int(RangeShadows), 25, 3.0)
	}
	cdlxsidegap3methodsOracleCompare(t, "settings")
}
