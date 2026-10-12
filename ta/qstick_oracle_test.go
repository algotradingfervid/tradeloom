//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func qstickOracleIn(s series) oracle.Inputs {
	return oracle.Inputs{Open: s.O, Close: s.C}
}

func TestQSTICKOracle(t *testing.T) {
	for _, p := range []int{1, 2, 3, 4, 10, 14, 30, 200} {
		lb := QSTICKLookback(p)
		if c := mustLookback(t, "QSTICK", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := QSTICK(s.O, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "QSTICK", qstickOracleIn(s), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("QSTICK ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestQSTICKOracleMaxPeriod(t *testing.T) {
	p := 100000
	if lb, c := QSTICKLookback(p), mustLookback(t, "QSTICK", float64(p)); lb != c {
		t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
	}
	for _, n := range []int{1, p - 1, p, p + 1, p + 5} {
		s := fixtures(n)[0]
		got, err := QSTICK(s.O, s.C, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "QSTICK", qstickOracleIn(s), float64(p))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		assertReal(t, lbl("QSTICK max n=", n), got, r.AlignedReal(0, n), relTol)
	}
}

// NaN and Inf in the inputs propagate through the running sum the same way.
func TestQSTICKOracleNonFinite(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	s := series{
		Name: "nonfinite",
		O:    []float64{1, 2, nan, 4, 5, 6, inf, 8, 9, 10, 11, 12},
		C:    []float64{2, 1, 3, 5, 4, 7, 8, inf, 10, 9, 12, 11},
	}
	for _, p := range []int{1, 2, 3} {
		got, err := QSTICK(s.O, s.C, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "QSTICK", qstickOracleIn(s), float64(p))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		assertReal(t, lbl("QSTICK nonfinite p=", p), got, r.AlignedReal(0, len(s.C)), relTol)
	}
}

func TestQSTICKOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{0, -5, 100001, 1, 100000} {
		r := mustCall(t, "QSTICK", qstickOracleIn(s), float64(p))
		_, err := QSTICK(s.O, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		lb, cerr := oracle.Lookback("QSTICK", float64(p))
		golb := QSTICKLookback(p)
		if (cerr != nil || lb == -1) != (golb == -1) {
			t.Errorf("lookback p=%d: C %d/%v, Go %d", p, lb, cerr, golb)
		}
	}
}

func TestQSTICKOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {5, 15}, {100, 119}} {
		for _, p := range []int{1, 10, 30} {
			out := make([]float64, len(s.C))
			beg, nb, err := qstickRange(rg[0], rg[1], s.O, s.C, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "QSTICK", qstickOracleIn(s), rg[0], rg[1], float64(p))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("QSTICK range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
