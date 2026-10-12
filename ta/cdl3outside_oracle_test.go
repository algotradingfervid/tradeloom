//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdl3outsideOracleCompare compares CDL3OUTSIDE with C on every fixture and
// edge length under the current candle settings, and returns how many bars
// fired up and down.
func cdl3outsideOracleCompare(t *testing.T, tag string) (up, down int) {
	t.Helper()
	lb := CDL3OUTSIDELookback()
	if c := mustLookback(t, "CDL3OUTSIDE"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	for _, n := range append(edgeLengths(lb), 1000) {
		for _, s := range fixtures(n) {
			got, err := CDL3OUTSIDE(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDL3OUTSIDE", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDL3OUTSIDE ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
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

func TestCDL3OUTSIDEOracle(t *testing.T) {
	up, down := cdl3outsideOracleCompare(t, "default")
	if up == 0 || down == 0 {
		t.Fatalf("pattern did not fire in both directions: up=%d down=%d", up, down)
	}
}

func TestCDL3OUTSIDEOracleConstructed(t *testing.T) {
	// Up at bar 3, down at bar 6, plus boundary near-misses.
	o := []float64{50, 10, 8.5, 10.5, 9, 10.5, 8.5, 10, 9, 10, 10, 10.5, 9.5}
	c := []float64{50, 9, 10.5, 11, 10, 8.5, 8, 9, 10, 10.5, 10, 9.5, 9}
	h := make([]float64, len(o))
	l := make([]float64, len(o))
	for i := range o {
		h[i] = max(o[i], c[i]) + 1
		l[i] = min(o[i], c[i]) - 1
	}
	got, err := CDL3OUTSIDE(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CDL3OUTSIDE", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	want := r.AlignedInt(0, len(o))
	assertInt(t, "constructed", got, want)
	if want[3] != 100 || want[6] != -100 {
		t.Fatalf("C did not fire on the constructed patterns: %v", want)
	}
}

func TestCDL3OUTSIDEOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {0, 2}, {0, 3}, {2, 2}, {3, 3}, {1, 50}, {4, 4}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdl3outsideRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDL3OUTSIDE", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDL3OUTSIDE range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDL3OUTSIDEOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDL3OUTSIDE", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdl3outsideRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDL3OUTSIDEOracleSettings(t *testing.T) {
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
	cdl3outsideOracleCompare(t, "settings")
}
