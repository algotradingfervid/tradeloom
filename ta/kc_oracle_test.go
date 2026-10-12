//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

type kcParams struct {
	p, a int
	d    float64
}

func kcOracleCompare(t *testing.T, s series, n int, ps kcParams) {
	t.Helper()
	u, m, l, err := KC(s.H, s.L, s.C, ps.p, ps.a, ps.d)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		return // C rejects endIdx -1; Go returns empty output
	}
	r := mustCall(t, "KC", s.ohlcv(), float64(ps.p), float64(ps.a), ps.d)
	if r.RetCode != oracle.Success {
		t.Fatalf("C retcode %d", r.RetCode)
	}
	tag := lbl(" ", s.Name, " n=", n, " ", ps)
	assertReal(t, "KC upper"+tag, u, r.AlignedReal(0, n), relTol)
	assertReal(t, "KC middle"+tag, m, r.AlignedReal(1, n), relTol)
	assertReal(t, "KC lower"+tag, l, r.AlignedReal(2, n), relTol)
}

func kcCheckLookback(t *testing.T, ps kcParams) int {
	t.Helper()
	lb := KCLookback(ps.p, ps.a, ps.d)
	if c := mustLookback(t, "KC", float64(ps.p), float64(ps.a), ps.d); lb != c {
		t.Fatalf("lookback %v: go %d c %d", ps, lb, c)
	}
	return lb
}

func TestKCOracle(t *testing.T) {
	sets := []kcParams{
		{2, 1, 2}, {2, 2, 1}, {3, 1, -1}, {20, 10, 2}, {10, 20, 2}, {14, 14, 1.5},
		{5, 30, 0}, {30, 5, 2.5}, {50, 3, -0.75}, {200, 14, 2}, {2, 200, 3}, {20, 10, 1e30},
	}
	for _, ps := range sets {
		lb := kcCheckLookback(t, ps)
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				kcOracleCompare(t, s, n, ps)
			}
		}
	}
}

func TestKCOracleMaxPeriod(t *testing.T) {
	for _, ps := range []kcParams{{100000, 1, 2}, {2, 100000, 2}, {100000, 100000, 2}} {
		lb := kcCheckLookback(t, ps)
		for _, n := range []int{lb, lb + 1, lb + 50} {
			kcOracleCompare(t, fixtures(n)[0], n, ps)
		}
	}
}

func TestKCOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []struct{ ema, atr int }{{5, 0}, {0, 7}, {3, 40}, {25, 2}} {
		SetUnstablePeriod(UnstEMA, u.ema)
		SetUnstablePeriod(UnstATR, u.atr)
		oracle.SetUnstablePeriod(int(UnstEMA), u.ema)
		oracle.SetUnstablePeriod(int(UnstATR), u.atr)
		for _, ps := range []kcParams{{2, 1, 2}, {20, 10, 2}, {5, 30, 1.5}} {
			lb := kcCheckLookback(t, ps)
			for _, n := range edgeLengths(lb) {
				for _, s := range fixtures(n) {
					kcOracleCompare(t, s, n, ps)
				}
			}
		}
	}
}

func TestKCOracleBadParams(t *testing.T) {
	s := fixtures(30)[0]
	cases := []kcParams{
		{0, 10, 2}, {1, 10, 2}, {-5, 10, 2}, {100001, 10, 2}, {2, 10, 2}, {100000, 10, 2},
		{20, 0, 2}, {20, -5, 2}, {20, 100001, 2}, {20, 1, 2}, {20, 100000, 2},
		{20, 10, 3.0000001e37}, {20, 10, -3.0000001e37}, {20, 10, 3e37}, {20, 10, -3e37}, {20, 10, 1e38},
	}
	for _, ps := range cases {
		r := mustCall(t, "KC", s.ohlcv(), float64(ps.p), float64(ps.a), ps.d)
		_, _, _, err := KC(s.H, s.L, s.C, ps.p, ps.a, ps.d)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("%v: C retcode %d, Go err %v", ps, r.RetCode, err)
		}
		if lb, c := KCLookback(ps.p, ps.a, ps.d), mustLookback(t, "KC", float64(ps.p), float64(ps.a), ps.d); lb != c {
			t.Errorf("lookback %v: go %d c %d", ps, lb, c)
		}
	}
}

func TestKCOracleRanges(t *testing.T) {
	ranges := [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {10, 10}, {0, 9},
		{50, 119}, {1, 1}, {2, 2}, {19, 19}, {20, 20}, {30, 119}, {0, 29}}
	for _, s := range []series{fixtures(120)[1], fixtures(120)[4], fixtures(120)[11]} {
		for _, rg := range ranges {
			for _, ps := range []kcParams{{2, 1, 2}, {3, 10, 1.5}, {10, 3, -2}, {20, 20, 2}} {
				u, m, l := make([]float64, 120), make([]float64, 120), make([]float64, 120)
				beg, nb, err := kcRange(rg[0], rg[1], s.H, s.L, s.C, ps.p, ps.a, ps.d, u, m, l)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "KC", s.ohlcv(), rg[0], rg[1], float64(ps.p), float64(ps.a), ps.d)
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v %v: go (%d,%d) c (%d,%d)", s.Name, rg, ps, beg, nb, r.BegIdx, r.NbElement)
				}
				tag := lbl(" ", s.Name, " ", rg, " ", ps)
				assertReal(t, "KC range upper"+tag, u[:nb], r.Outputs[0].Real, relTol)
				assertReal(t, "KC range middle"+tag, m[:nb], r.Outputs[1].Real, relTol)
				assertReal(t, "KC range lower"+tag, l[:nb], r.Outputs[2].Real, relTol)
			}
		}
	}
}
