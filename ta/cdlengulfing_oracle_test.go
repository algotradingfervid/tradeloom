//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlengulfingOracleCompare compares CDLENGULFING with C on every fixture and
// edge length under the current candle settings, and counts each output value.
func cdlengulfingOracleCompare(t *testing.T, tag string) map[int]int {
	t.Helper()
	counts := map[int]int{}
	lb := CDLENGULFINGLookback()
	if c := mustLookback(t, "CDLENGULFING"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	for _, n := range append(edgeLengths(lb), 1000) {
		for _, s := range fixtures(n) {
			got, err := CDLENGULFING(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLENGULFING", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLENGULFING ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				counts[v]++
			}
		}
	}
	return counts
}

func TestCDLENGULFINGOracle(t *testing.T) {
	counts := cdlengulfingOracleCompare(t, "default")
	if counts[100] == 0 || counts[-100] == 0 {
		t.Fatalf("pattern did not fire in both directions: %v", counts)
	}
}

func TestCDLENGULFINGOracleConstructed(t *testing.T) {
	// Pairs (bar i-1, bar i) at i = 2,4,6,8,10,12 hit +100, -100, +80, -80,
	// both-ends-equal (0) and doji-first (-100); fillers keep pairs apart.
	o := []float64{50, 10, 8.5, 9, 10.5, 10, 9, 9, 10, 10, 9, 10, 10.5}
	c := []float64{50, 9, 10.5, 10, 8.5, 9, 10.5, 10, 8.5, 9, 10, 10, 9.5}
	h := make([]float64, len(o))
	l := make([]float64, len(o))
	for i := range o {
		h[i] = max(o[i], c[i]) + 1
		l[i] = min(o[i], c[i]) - 1
	}
	got, err := CDLENGULFING(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CDLENGULFING", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	want := r.AlignedInt(0, len(o))
	assertInt(t, "constructed", got, want)
	if want[2] != 100 || want[4] != -100 || want[6] != 80 || want[8] != -80 || want[10] != 0 || want[12] != -100 {
		t.Fatalf("C did not produce the expected constructed values: %v", want)
	}
}

func TestCDLENGULFINGOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {0, 1}, {0, 2}, {1, 1}, {2, 2}, {1, 50}, {3, 3}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdlengulfingRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDLENGULFING", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDLENGULFING range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDLENGULFINGOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLENGULFING", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlengulfingRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLENGULFINGOracleSettings(t *testing.T) {
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
	cdlengulfingOracleCompare(t, "settings")
}
