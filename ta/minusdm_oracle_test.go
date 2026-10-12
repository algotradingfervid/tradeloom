//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func minusdmOracleFull(t *testing.T, periods []int, tag string) {
	t.Helper()
	for _, p := range periods {
		lb := MINUSDMLookback(p)
		if c := mustLookback(t, "MINUS_DM", float64(p)); lb != c {
			t.Fatalf("%s lookback p=%d: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := MINUSDM(s.H, s.L, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "MINUS_DM", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("MINUS_DM ", tag, " ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestMINUSDMOracle(t *testing.T) {
	minusdmOracleFull(t, []int{1, 2, 3, 5, 14, 30, 200}, "")
}

func TestMINUSDMOracleMaxPeriod(t *testing.T) {
	p := 100000
	if lb, c := MINUSDMLookback(p), mustLookback(t, "MINUS_DM", float64(p)); lb != c {
		t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
	}
	for _, n := range []int{p - 1, p, p + 1, p + 50} {
		s := fixtures(n)[0]
		got, err := MINUSDM(s.H, s.L, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "MINUS_DM", s.ohlcv(), float64(p))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		assertReal(t, lbl("MINUS_DM max n=", n), got, r.AlignedReal(0, n), relTol)
	}
}

func TestMINUSDMOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{0, -5, 1, 2, 100000, 100001} {
		r := mustCall(t, "MINUS_DM", s.ohlcv(), float64(p))
		_, err := MINUSDM(s.H, s.L, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := MINUSDMLookback(p); err != nil && c != -1 {
			t.Errorf("p=%d: lookback %d want -1", p, c)
		}
	}
}

func minusdmOracleRanges(t *testing.T, s series, ranges [][2]int, periods []int, tag string) {
	t.Helper()
	n := len(s.H)
	for _, rg := range ranges {
		for _, p := range periods {
			out := make([]float64, n)
			beg, nb, err := minusdmRange(rg[0], rg[1], s.H, s.L, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "MINUS_DM", s.ohlcv(), rg[0], rg[1], float64(p))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v p=%d: go (%d,%d) c (%d,%d)", tag, rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("MINUS_DM ", tag, " range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}

func TestMINUSDMOracleRanges(t *testing.T) {
	ranges := [][2]int{{0, 0}, {0, 119}, {1, 1}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {8, 8}, {0, 9}, {10, 10}, {50, 119}}
	for _, idx := range []int{1, 9, 11} {
		minusdmOracleRanges(t, fixtures(120)[idx], ranges, []int{1, 2, 10}, fixtures(0)[idx].Name)
	}
}

func TestMINUSDMOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 40} {
		if err := SetUnstablePeriod(UnstMinusDM, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstMinusDM), u)
		minusdmOracleFull(t, []int{1, 2, 14}, lbl("unst=", u))

		ranges := [][2]int{{0, 149}, {20, 20}, {30, 100}, {149, 149}, {0, 14 + u - 2}, {14 + u - 1, 14 + u - 1}}
		minusdmOracleRanges(t, fixtures(150)[4], ranges, []int{1, 14}, lbl("unst=", u))
	}
}
