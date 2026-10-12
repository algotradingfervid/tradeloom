//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func macdfixOracleCompare(t *testing.T, tag string, params []int) {
	t.Helper()
	for _, p := range params {
		lb := MACDFIXLookback(p)
		if c := mustLookback(t, "MACDFIX", float64(p)); lb != c {
			t.Fatalf("%s lookback p=%d: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				m, sg, h, err := MACDFIX(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "MACDFIX", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				l := lbl("MACDFIX ", tag, s.Name, " n=", n, " p=", p)
				assertReal(t, l+" macd", m, r.AlignedReal(0, n), relTol)
				assertReal(t, l+" signal", sg, r.AlignedReal(1, n), relTol)
				assertReal(t, l+" hist", h, r.AlignedReal(2, n), relTol)
			}
		}
	}
}

func TestMACDFIXOracle(t *testing.T) {
	macdfixOracleCompare(t, "", []int{1, 2, 3, 5, 9, 14, 30, 200, 1000})
}

func TestMACDFIXOracleMaxPeriod(t *testing.T) {
	// The maximum signal period on a series just around its lookback.
	p := 100000
	lb := MACDFIXLookback(p)
	if c := mustLookback(t, "MACDFIX", float64(p)); lb != c {
		t.Fatalf("lookback: go %d c %d", lb, c)
	}
	for _, n := range []int{1, lb, lb + 1, lb + 5} {
		s := fixtures(n)[0]
		m, sg, h, err := MACDFIX(s.Real(), p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "MACDFIX", realIn(s.Real()), float64(p))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		l := lbl("MACDFIX max n=", n)
		assertReal(t, l+" macd", m, r.AlignedReal(0, n), relTol)
		assertReal(t, l+" signal", sg, r.AlignedReal(1, n), relTol)
		assertReal(t, l+" hist", h, r.AlignedReal(2, n), relTol)
	}
}

func TestMACDFIXOracleBadParams(t *testing.T) {
	in := fixtures(40)[0].Real()
	// math.MinInt32 is TA_INTEGER_DEFAULT, which C replaces with the default;
	// Go has no sentinel, so it is not included here.
	for _, p := range []int{0, -1, -2, 100001, 1, 2, 9, 100000} {
		r := mustCall(t, "MACDFIX", realIn(in), float64(p))
		_, _, _, err := MACDFIX(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "MACDFIX", float64(p)); c != MACDFIXLookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, MACDFIXLookback(p), c)
		}
	}
}

func macdfixOracleRanges(t *testing.T, tag string, ss []series, ranges [][2]int, params []int) {
	t.Helper()
	for _, s := range ss {
		in := s.Real()
		for _, rg := range ranges {
			for _, p := range params {
				o1, o2, o3 := make([]float64, len(in)), make([]float64, len(in)), make([]float64, len(in))
				beg, nb, err := macdfixRange(rg[0], rg[1], in, p, o1, o2, o3)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "MACDFIX", realIn(in), rg[0], rg[1], float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s %s range %v p=%d: go (%d,%d) c (%d,%d)", tag, s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				l := lbl("MACDFIX range ", tag, s.Name, rg, " p=", p)
				assertReal(t, l+" macd", o1[:nb], r.Outputs[0].Real[:nb], relTol)
				assertReal(t, l+" signal", o2[:nb], r.Outputs[1].Real[:nb], relTol)
				assertReal(t, l+" hist", o3[:nb], r.Outputs[2].Real[:nb], relTol)
			}
		}
	}
}

func TestMACDFIXOracleRanges(t *testing.T) {
	f := fixtures(120)
	macdfixOracleRanges(t, "", []series{f[1], f[4], f[9], f[13]},
		[][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 32}, {33, 33}, {32, 33}, {25, 25}, {40, 119}, {100, 105}},
		[]int{1, 2, 9, 30})
}

func TestMACDFIXOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 37} {
		SetUnstablePeriod(UnstEMA, u)
		oracle.SetUnstablePeriod(int(UnstEMA), u)
		macdfixOracleCompare(t, lbl("unst=", u, " "), []int{1, 2, 9})
		macdfixOracleRanges(t, lbl("unst=", u, " "), []series{fixtures(200)[0]},
			[][2]int{{0, 199}, {80, 80}, {0, 10}, {100, 199}},
			[]int{1, 9})
	}
	// Another function's unstable period must not leak into MACDFIX.
	SetAllUnstablePeriods(0)
	oracle.ResetUnstablePeriods()
	SetUnstablePeriod(UnstRSI, 9)
	oracle.SetUnstablePeriod(int(UnstRSI), 9)
	macdfixOracleCompare(t, "rsi-unst ", []int{9})
}
