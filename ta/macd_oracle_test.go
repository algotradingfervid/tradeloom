//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func macdOracleCompare(t *testing.T, tag string, params [][3]int) {
	t.Helper()
	for _, p := range params {
		opt := []float64{float64(p[0]), float64(p[1]), float64(p[2])}
		lb := MACDLookback(p[0], p[1], p[2])
		if c := mustLookback(t, "MACD", opt...); lb != c {
			t.Fatalf("%s lookback p=%v: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				m, sg, h, err := MACD(s.Real(), p[0], p[1], p[2])
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "MACD", realIn(s.Real()), opt...)
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				l := lbl("MACD ", tag, s.Name, " n=", n, " p=", p)
				assertReal(t, l+" macd", m, r.AlignedReal(0, n), relTol)
				assertReal(t, l+" signal", sg, r.AlignedReal(1, n), relTol)
				assertReal(t, l+" hist", h, r.AlignedReal(2, n), relTol)
			}
		}
	}
}

func TestMACDOracle(t *testing.T) {
	macdOracleCompare(t, "", [][3]int{
		{12, 26, 9}, {26, 12, 9}, {2, 2, 1}, {2, 3, 1}, {3, 2, 2}, {2, 100, 1},
		{5, 35, 5}, {8, 17, 1}, {10, 10, 10}, {50, 200, 30}, {2, 2, 100000},
		{100000, 2, 1},
	})
}

func TestMACDOracleBadParams(t *testing.T) {
	in := fixtures(40)[0].Real()
	// math.MinInt32 is TA_INTEGER_DEFAULT, which C replaces with the default;
	// Go has no sentinel, so it is not included here.
	for _, p := range [][3]int{
		{1, 26, 9}, {0, 26, 9}, {-1, 26, 9}, {100001, 26, 9},
		{12, 1, 9}, {12, 0, 9}, {12, 100001, 9},
		{12, 26, 0}, {12, 26, -2}, {12, 26, 100001},
		{2, 2, 1}, {100000, 100000, 100000}, {12, 26, 9},
	} {
		opt := []float64{float64(p[0]), float64(p[1]), float64(p[2])}
		r := mustCall(t, "MACD", realIn(in), opt...)
		_, _, _, err := MACD(in, p[0], p[1], p[2])
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%v: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "MACD", opt...); c != MACDLookback(p[0], p[1], p[2]) {
			t.Errorf("lookback p=%v: go %d c %d", p, MACDLookback(p[0], p[1], p[2]), c)
		}
	}
}

func macdOracleRanges(t *testing.T, tag string, ss []series, ranges [][2]int, params [][3]int) {
	t.Helper()
	for _, s := range ss {
		in := s.Real()
		for _, rg := range ranges {
			for _, p := range params {
				o1, o2, o3 := make([]float64, len(in)), make([]float64, len(in)), make([]float64, len(in))
				beg, nb, err := macdRange(rg[0], rg[1], in, p[0], p[1], p[2], o1, o2, o3)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "MACD", realIn(in), rg[0], rg[1], float64(p[0]), float64(p[1]), float64(p[2]))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s %s range %v p=%v: go (%d,%d) c (%d,%d)", tag, s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				l := lbl("MACD range ", tag, s.Name, rg, " p=", p)
				assertReal(t, l+" macd", o1[:nb], r.Outputs[0].Real[:nb], relTol)
				assertReal(t, l+" signal", o2[:nb], r.Outputs[1].Real[:nb], relTol)
				assertReal(t, l+" hist", o3[:nb], r.Outputs[2].Real[:nb], relTol)
			}
		}
	}
}

func TestMACDOracleRanges(t *testing.T) {
	f := fixtures(120)
	macdOracleRanges(t, "", []series{f[1], f[4], f[9], f[13]},
		[][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 32}, {33, 33}, {32, 33}, {40, 119}, {100, 105}},
		[][3]int{{12, 26, 9}, {2, 2, 1}, {3, 5, 2}, {26, 12, 1}})
}

func TestMACDOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 37} {
		SetUnstablePeriod(UnstEMA, u)
		oracle.SetUnstablePeriod(int(UnstEMA), u)
		macdOracleCompare(t, lbl("unst=", u, " "), [][3]int{{12, 26, 9}, {2, 2, 1}, {3, 5, 2}, {26, 12, 1}})
		macdOracleRanges(t, lbl("unst=", u, " "), []series{fixtures(200)[0]},
			[][2]int{{0, 199}, {80, 80}, {0, 10}, {100, 199}},
			[][3]int{{12, 26, 9}, {2, 3, 1}})
	}
	// Another function's unstable period must not leak into MACD.
	SetAllUnstablePeriods(0)
	oracle.ResetUnstablePeriods()
	SetUnstablePeriod(UnstRSI, 9)
	oracle.SetUnstablePeriod(int(UnstRSI), 9)
	macdOracleCompare(t, "rsi-unst ", [][3]int{{12, 26, 9}})
}
