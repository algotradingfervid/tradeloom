//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

func TestSAREXTOracle(t *testing.T) {
	for _, p := range sarextParamSets {
		lb := sarextLB(p)
		if c := mustLookback(t, "SAREXT", p[:]...); lb != c {
			t.Fatalf("lookback %v: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := sarextCall(s.H, s.L, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "SAREXT", s.ohlcv(), p[:]...)
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("SAREXT ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestSAREXTOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	// TA_REAL_DEFAULT (-4e37) is not tested: C substitutes the default for it,
	// while Go has no sentinel and rejects it as out of range.
	bad := []float64{math.NaN(), math.Inf(1), math.Inf(-1), 3.0000001e37, -3.0000001e37, -1e-300, -1, 0, 3e37, -3e37, 0.5}
	for i := range sarextDefaults {
		for _, v := range bad {
			p := sarextDefaults
			p[i] = v
			r := mustCall(t, "SAREXT", s.ohlcv(), p[:]...)
			_, err := sarextCall(s.H, s.L, p)
			if (r.RetCode == oracle.BadParam) != (err != nil) {
				t.Errorf("param %d = %g: C retcode %d, Go err %v", i, v, r.RetCode, err)
			}
			if err == nil && r.RetCode != oracle.Success {
				t.Errorf("param %d = %g: C retcode %d", i, v, r.RetCode)
			}
			c, lerr := oracle.Lookback("SAREXT", p[:]...)
			if lerr == nil && c != sarextLB(p) {
				t.Errorf("param %d = %g: lookback go %d c %d", i, v, sarextLB(p), c)
			}
			if lerr != nil && sarextLB(p) != -1 {
				t.Errorf("param %d = %g: C lookback err %v, go %d", i, v, lerr, sarextLB(p))
			}
		}
	}
}

func TestSAREXTOracleRanges(t *testing.T) {
	for _, fi := range []int{0, 1, 9, 11} {
		s := fixtures(120)[fi]
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {1, 1}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {118, 119}} {
			for _, p := range sarextParamSets {
				out := make([]float64, 120)
				beg, nb, err := sarextRangeCall(rg[0], rg[1], s.H, s.L, p, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "SAREXT", s.ohlcv(), rg[0], rg[1], p[:]...)
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%v: go (%d,%d) c (%d,%d)", s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("SAREXT range ", s.Name, rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

// The default start direction comes from MINUS_DM with period 1, which has no
// unstable period; a non-zero MINUS_DM unstable period must not change SAREXT.
func TestSAREXTOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	SetUnstablePeriod(UnstMinusDM, 5)
	oracle.SetUnstablePeriod(int(UnstMinusDM), 5)
	s := fixtures(64)[1]
	got, _ := sarextCall(s.H, s.L, sarextDefaults)
	r := mustCall(t, "SAREXT", s.ohlcv(), sarextDefaults[:]...)
	assertReal(t, "SAREXT unstable", got, r.AlignedReal(0, 64), relTol)
}
