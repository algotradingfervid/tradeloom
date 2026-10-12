//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdlhikkakeOracleCompare compares CDLHIKKAKE with C on every fixture and
// edge length under the current candle settings, and counts each output value.
func cdlhikkakeOracleCompare(t *testing.T, tag string) map[int]int {
	t.Helper()
	counts := map[int]int{}
	lb := CDLHIKKAKELookback()
	if c := mustLookback(t, "CDLHIKKAKE"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	for _, n := range append(edgeLengths(lb), 3000) {
		for _, s := range fixtures(n) {
			got, err := CDLHIKKAKE(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDLHIKKAKE", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDLHIKKAKE ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				counts[v]++
			}
		}
	}
	return counts
}

func TestCDLHIKKAKEOracle(t *testing.T) {
	counts := cdlhikkakeOracleCompare(t, "default")
	for _, v := range []int{100, -100, 200, -200} {
		if counts[v] == 0 {
			t.Fatalf("pattern value %d never produced: %v", v, counts)
		}
	}
}

func TestCDLHIKKAKEOracleConstructed(t *testing.T) {
	filler := [3]float64{20, 0, 10}
	mother := [3]float64{15, 5, 10}
	inside := [3]float64{14, 6, 10}
	bull := [3]float64{13, 4, 8}
	bear := [3]float64{16, 7, 12}
	quiet := [3]float64{13, 5, 10}
	up := [3]float64{16, 9, 15}
	down := [3]float64{10, 3, 4}
	o, h, l, c := cdlhikkakeBars(filler, filler, mother, inside, bull, up, // 200 at 5 via warm-up
		filler, mother, inside, bull, quiet, quiet, up, // 100 at 9, 200 at 12
		filler, mother, inside, bear, down, // -100 at 16, -200 at 17
		filler, mother, inside, bull, quiet, quiet, quiet, up) // 100 at 21, expired
	got, err := CDLHIKKAKE(o, h, l, c)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "CDLHIKKAKE", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
	want := r.AlignedInt(0, len(o))
	assertInt(t, "constructed", got, want)
	if want[5] != 200 || want[9] != 100 || want[12] != 200 || want[16] != -100 || want[17] != -200 || want[21] != 100 || want[25] != 0 {
		t.Fatalf("C did not produce the expected constructed values: %v", want)
	}
}

func TestCDLHIKKAKEOracleRanges(t *testing.T) {
	for _, s := range fixtures(300) {
		n := len(s.C)
		var ranges [][2]int
		for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {0, 4}, {0, 5}, {4, 4}, {5, 5}, {6, 6}, {1, 50}, {150, 299}, {n - 1, n - 1}} {
			ranges = append(ranges, rg)
		}
		// Every single-bar and short range start exercises the warm-up.
		for st := 5; st < 120; st++ {
			ranges = append(ranges, [2]int{st, st}, [2]int{st, st + 4})
		}
		for _, rg := range ranges {
			out := make([]int, n)
			beg, nb, err := cdlhikkakeRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDLHIKKAKE", s.ohlcv(), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("CDLHIKKAKE range ", s.Name, " ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

func TestCDLHIKKAKEOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDLHIKKAKE", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdlhikkakeRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDLHIKKAKEOracleSettings(t *testing.T) {
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
	cdlhikkakeOracleCompare(t, "settings")
}
