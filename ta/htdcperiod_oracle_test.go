//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length, with and without an unstable period.

func htdcperiodOracleCompare(t *testing.T, tag string) {
	t.Helper()
	lb := HTDCPERIODLookback()
	if c := mustLookback(t, "HT_DCPERIOD"); lb != c {
		t.Fatalf("%slookback: go %d c %d", tag, lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := HTDCPERIOD(s.Real())
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "HT_DCPERIOD", realIn(s.Real()))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("HT_DCPERIOD ", tag, s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

func TestHTDCPERIODOracle(t *testing.T) {
	htdcperiodOracleCompare(t, "")
}

func TestHTDCPERIODOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 10, 50} {
		if err := SetUnstablePeriod(UnstHTDCPeriod, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstHTDCPeriod), u)
		htdcperiodOracleCompare(t, lbl("unstable=", u, " "))
	}
}

func TestHTDCPERIODOracleBadParams(t *testing.T) {
	// No optional inputs: the only parameter-free call must succeed in both.
	in := fixtures(60)[0].Real()
	r := mustCall(t, "HT_DCPERIOD", realIn(in))
	_, err := HTDCPERIOD(in)
	if (r.RetCode == oracle.BadParam) != (err != nil) || r.RetCode != oracle.Success {
		t.Errorf("C retcode %d, Go err %v", r.RetCode, err)
	}
}

func TestHTDCPERIODOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(300)[1], fixtures(300)[11]} {
		in := s.Real()
		last := len(in) - 1
		for _, rg := range [][2]int{{0, 0}, {0, last}, {3, 3}, {0, 31}, {0, 32}, {32, 32}, {33, 33},
			{10, 50}, {40, 41}, {41, 120}, {100, 299}, {last, last}, {31, 40}} {
			out := make([]float64, len(in))
			beg, nb, err := htdcperiodRange(rg[0], rg[1], in, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "HT_DCPERIOD", realIn(in), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("HT_DCPERIOD range ", s.Name, " ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
