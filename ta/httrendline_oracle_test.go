//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length, with and without an unstable period.

func httrendlineOracleCompare(t *testing.T, tag string) {
	t.Helper()
	lb := HTTRENDLINELookback()
	if c := mustLookback(t, "HT_TRENDLINE"); lb != c {
		t.Fatalf("%slookback: go %d c %d", tag, lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := HTTRENDLINE(s.Real())
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "HT_TRENDLINE", realIn(s.Real()))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("HT_TRENDLINE ", tag, s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

func TestHTTRENDLINEOracle(t *testing.T) {
	httrendlineOracleCompare(t, "")
}

func TestHTTRENDLINEOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 10, 50} {
		if err := SetUnstablePeriod(UnstHTTrendline, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstHTTrendline), u)
		httrendlineOracleCompare(t, lbl("unstable=", u, " "))
	}
}

// HT_TRENDLINE has no optional inputs, so the only bad-parameter cases are
// the index checks; C and Go must reject the same ranges.
func TestHTTRENDLINEOracleBadParams(t *testing.T) {
	in := fixtures(100)[0].Real()
	for _, rg := range [][2]int{{5, 4}, {99, 0}} {
		r := mustCallRange(t, "HT_TRENDLINE", realIn(in), rg[0], rg[1])
		_, _, err := httrendlineRange(rg[0], rg[1], in, make([]float64, len(in)))
		if (r.RetCode == oracle.Success) == (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestHTTRENDLINEOracleRanges(t *testing.T) {
	check := func(tag string) {
		for _, s := range []series{fixtures(300)[1], fixtures(300)[9], fixtures(300)[4]} {
			in := s.Real()
			n := len(in)
			lb := HTTRENDLINELookback()
			for _, rg := range [][2]int{
				{0, 0}, {0, n - 1}, {3, 3}, {10, 50}, {n - 1, n - 1}, {0, lb - 1},
				{lb - 1, lb - 1}, {lb, lb}, {lb, lb + 1}, {lb + 1, n - 1}, {100, 250}, {101, 101},
			} {
				out := make([]float64, n)
				beg, nb, err := httrendlineRange(rg[0], rg[1], in, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "HT_TRENDLINE", realIn(in), rg[0], rg[1])
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s%s range %v: go (%d,%d) c (%d,%d)", tag, s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("HT_TRENDLINE range ", tag, s.Name, " ", rg), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
	check("")
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	if err := SetUnstablePeriod(UnstHTTrendline, 15); err != nil {
		t.Fatal(err)
	}
	oracle.SetUnstablePeriod(int(UnstHTTrendline), 15)
	check("unstable=15 ")
}
