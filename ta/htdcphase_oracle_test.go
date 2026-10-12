//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length, with and without an unstable period.

func htdcphaseOracleCompare(t *testing.T, tag string) {
	t.Helper()
	lb := HTDCPHASELookback()
	if c := mustLookback(t, "HT_DCPHASE"); lb != c {
		t.Fatalf("%slookback: go %d c %d", tag, lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := HTDCPHASE(s.Real())
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "HT_DCPHASE", realIn(s.Real()))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("HTDCPHASE ", tag, s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

func TestHTDCPHASEOracle(t *testing.T) {
	htdcphaseOracleCompare(t, "")
}

func TestHTDCPHASEOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 10, 50, 200} {
		if err := SetUnstablePeriod(UnstHTDCPhase, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstHTDCPhase), u)
		htdcphaseOracleCompare(t, lbl("unstable=", u, " "))
	}
}

// TestHTDCPHASEOracleSignals compares on series that exercise the period
// estimator across its whole [6, 50] band and the imagPart == 0 branch.
func TestHTDCPHASEOracleSignals(t *testing.T) {
	var cases []series
	for _, p := range []float64{4, 6, 10, 20, 35, 50, 80} {
		s := series{Name: lbl("sine", p)}
		for i := range 500 {
			s.C = append(s.C, 100+10*math.Sin(2*math.Pi*float64(i)/p))
		}
		cases = append(cases, s)
	}
	step := series{Name: "step"}
	for i := range 300 {
		v := 0.0
		if i >= 150 {
			v = 50
		}
		step.C = append(step.C, v)
	}
	cases = append(cases, step)
	for _, s := range cases {
		n := len(s.C)
		got, err := HTDCPHASE(s.C)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "HT_DCPHASE", realIn(s.C))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		assertReal(t, "HTDCPHASE "+s.Name, got, r.AlignedReal(0, n), relTol)
	}
}

func TestHTDCPHASEOracleBadParams(t *testing.T) {
	// No optional inputs: the only bad parameters are the indices, which
	// both sides reject.
	in := fixtures(100)[0].Real()
	for _, rg := range [][2]int{{-1, 10}, {10, 5}, {0, -1}} {
		r := mustCallRange(t, "HT_DCPHASE", realIn(in), rg[0], rg[1])
		_, _, err := htdcphaseRange(rg[0], rg[1], in, make([]float64, len(in)))
		if (r.RetCode == oracle.Success) == (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
	r := mustCall(t, "HT_DCPHASE", realIn(in))
	if _, err := HTDCPHASE(in); err != nil || r.RetCode != oracle.Success {
		t.Errorf("valid call: C retcode %d, Go err %v", r.RetCode, err)
	}
}

func TestHTDCPHASEOracleRanges(t *testing.T) {
	check := func(tag string) {
		for _, s := range []series{fixtures(300)[1], fixtures(300)[11], fixtures(300)[4]} {
			in := s.Real()
			last := len(in) - 1
			for _, rg := range [][2]int{{0, 0}, {0, last}, {3, 3}, {0, 62}, {0, 63}, {63, 63}, {64, 64},
				{10, 70}, {62, 64}, {70, 71}, {100, 299}, {150, 200}, {last, last}, {60, 80}} {
				out := make([]float64, len(in))
				beg, nb, err := htdcphaseRange(rg[0], rg[1], in, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "HT_DCPHASE", realIn(in), rg[0], rg[1])
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s%s range %v: go (%d,%d) c (%d,%d)", tag, s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("HTDCPHASE ", tag, s.Name, " range ", rg), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
	check("")
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	if err := SetUnstablePeriod(UnstHTDCPhase, 15); err != nil {
		t.Fatal(err)
	}
	oracle.SetUnstablePeriod(int(UnstHTDCPhase), 15)
	check("unstable=15 ")
}
