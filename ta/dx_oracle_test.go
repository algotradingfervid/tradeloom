//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func dxOracleFull(t *testing.T, periods []int, tag string) {
	t.Helper()
	for _, p := range periods {
		lb := DXLookback(p)
		if c := mustLookback(t, "DX", float64(p)); lb != c {
			t.Fatalf("%s lookback p=%d: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := DX(s.H, s.L, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "DX", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("DX ", tag, " ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestDXOracle(t *testing.T) {
	dxOracleFull(t, []int{2, 3, 5, 14, 30, 200}, "")
}

func TestDXOracleMaxPeriod(t *testing.T) {
	p := 100000
	if lb, c := DXLookback(p), mustLookback(t, "DX", float64(p)); lb != c {
		t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
	}
	for _, n := range []int{p - 1, p, p + 1, p + 50} {
		s := fixtures(n)[0]
		got, err := DX(s.H, s.L, s.C, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "DX", s.ohlcv(), float64(p))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		assertReal(t, lbl("DX max n=", n), got, r.AlignedReal(0, n), relTol)
	}
}

func TestDXOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{0, -5, 1, 2, 100000, 100001} {
		r := mustCall(t, "DX", s.ohlcv(), float64(p))
		_, err := DX(s.H, s.L, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := DXLookback(p); (err != nil) != (c == -1) {
			t.Errorf("p=%d: lookback %d, err %v", p, c, err)
		}
	}
}

func dxOracleRanges(t *testing.T, s series, ranges [][2]int, periods []int, tag string) {
	t.Helper()
	n := len(s.H)
	for _, rg := range ranges {
		for _, p := range periods {
			out := make([]float64, n)
			beg, nb, err := dxRange(rg[0], rg[1], s.H, s.L, s.C, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "DX", s.ohlcv(), rg[0], rg[1], float64(p))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v p=%d: go (%d,%d) c (%d,%d)", tag, rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("DX ", tag, " range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}

func TestDXOracleRanges(t *testing.T) {
	ranges := [][2]int{{0, 0}, {0, 119}, {1, 1}, {2, 2}, {3, 3}, {10, 50}, {119, 119}, {0, 9}, {10, 10}, {9, 9}, {0, 10}, {11, 11}, {50, 119}}
	for _, idx := range []int{0, 1, 5, 9, 11} {
		dxOracleRanges(t, fixtures(120)[idx], ranges, []int{2, 10, 14}, fixtures(0)[idx].Name)
	}
}

func TestDXOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 40} {
		if err := SetUnstablePeriod(UnstDX, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstDX), u)
		dxOracleFull(t, []int{2, 14}, lbl("unst=", u))

		ranges := [][2]int{{0, 149}, {20, 20}, {30, 100}, {149, 149}, {0, 14 + u - 1}, {14 + u, 14 + u}}
		dxOracleRanges(t, fixtures(150)[4], ranges, []int{2, 14}, lbl("unst=", u))
	}
}
