//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestIMIOracle(t *testing.T) {
	for _, p := range []int{2, 3, 5, 14, 30, 200, 1000} {
		lb := IMILookback(p)
		if c := mustLookback(t, "IMI", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := IMI(s.O, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "IMI", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("IMI ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
	// Maximum period: lookback only (a full series would be 100000 bars of
	// O(n*p) work in both languages), plus one series just past it.
	if lb, c := IMILookback(100000), mustLookback(t, "IMI", 100000); lb != c {
		t.Fatalf("lookback p=100000: go %d c %d", lb, c)
	}
	s := fixtures(100001)[0]
	got, err := IMI(s.O, s.C, 100000)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "IMI", s.ohlcv(), 100000)
	assertReal(t, "IMI p=100000", got, r.AlignedReal(0, len(s.C)), relTol)
}

func TestIMIOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{1, 0, -5, 100001, 2, 100000} {
		r := mustCall(t, "IMI", s.ohlcv(), float64(p))
		_, err := IMI(s.O, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "IMI", float64(p)); c != IMILookback(p) {
			t.Errorf("p=%d lookback: C %d, Go %d", p, c, IMILookback(p))
		}
	}
}

func TestIMIOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[10]} {
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {5, 12}} {
			for _, p := range []int{2, 10} {
				out := make([]float64, len(s.C))
				beg, nb, err := imiRange(rg[0], rg[1], s.O, s.C, p, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "IMI", s.ohlcv(), rg[0], rg[1], float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%d: go (%d,%d) c (%d,%d)", s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("IMI range ", s.Name, rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

// IMI has no unstable period (TA-Lib #14): a non-zero setting must change
// nothing in either library.
func TestIMIOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	if err := SetAllUnstablePeriods(7); err != nil {
		t.Fatal(err)
	}
	for id := 0; id < unstCount; id++ {
		oracle.SetUnstablePeriod(id, 7)
	}
	s := fixtures(100)[0]
	if lb, c := IMILookback(14), mustLookback(t, "IMI", 14); lb != c || lb != 13 {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	got, _ := IMI(s.O, s.C, 14)
	r := mustCall(t, "IMI", s.ohlcv(), 14)
	assertReal(t, "IMI unstable", got, r.AlignedReal(0, 100), relTol)
}
