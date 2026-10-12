//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func atrOracleFull(t *testing.T, periods []int, tag string) {
	t.Helper()
	for _, p := range periods {
		lb := ATRLookback(p)
		if c := mustLookback(t, "ATR", float64(p)); lb != c {
			t.Fatalf("%s lookback p=%d: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := ATR(s.H, s.L, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "ATR", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("ATR ", tag, " ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestATROracle(t *testing.T) {
	atrOracleFull(t, []int{1, 2, 3, 5, 14, 30, 200}, "")
}

func TestATROracleMaxPeriod(t *testing.T) {
	p := 100000
	if lb, c := ATRLookback(p), mustLookback(t, "ATR", float64(p)); lb != c {
		t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
	}
	for _, n := range []int{p, p + 1, p + 50} {
		s := fixtures(n)[0]
		got, err := ATR(s.H, s.L, s.C, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "ATR", s.ohlcv(), float64(p))
		assertReal(t, lbl("ATR max n=", n), got, r.AlignedReal(0, n), relTol)
	}
}

func TestATROracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{0, -5, 1, 100000, 100001} {
		r := mustCall(t, "ATR", s.ohlcv(), float64(p))
		_, err := ATR(s.H, s.L, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := ATRLookback(p); err != nil && c != -1 {
			t.Errorf("p=%d: lookback %d want -1", p, c)
		}
	}
}

func TestATROracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	ranges := [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 9}, {10, 10}, {9, 9}, {11, 11}, {0, 10}, {50, 119}}
	for _, rg := range ranges {
		for _, p := range []int{1, 2, 10} {
			out := make([]float64, 120)
			beg, nb, err := atrRange(rg[0], rg[1], s.H, s.L, s.C, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "ATR", s.ohlcv(), rg[0], rg[1], float64(p))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("ATR range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}

func TestATROracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 40} {
		if err := SetUnstablePeriod(UnstATR, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstATR), u)
		atrOracleFull(t, []int{1, 2, 14}, lbl("unst=", u))

		s := fixtures(150)[4]
		for _, rg := range [][2]int{{0, 149}, {20, 20}, {30, 100}, {149, 149}, {0, 14 + u - 1}} {
			out := make([]float64, 150)
			beg, nb, err := atrRange(rg[0], rg[1], s.H, s.L, s.C, 14, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "ATR", s.ohlcv(), rg[0], rg[1], 14)
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("unst=%d range %v: go (%d,%d) c (%d,%d)", u, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("ATR unst=", u, " range ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
