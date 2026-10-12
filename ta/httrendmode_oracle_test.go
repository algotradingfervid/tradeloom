//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture and
// every edge length. HT_TRENDMODE has no optional inputs.

func httrendmodeOracleCompare(t *testing.T, tag string) {
	t.Helper()
	lb := HTTRENDMODELookback()
	if c := mustLookback(t, "HT_TRENDMODE"); lb != c {
		t.Fatalf("%slookback: go %d c %d", tag, lb, c)
	}
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := HTTRENDMODE(s.Real())
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "HT_TRENDMODE", realIn(s.Real()))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl(tag, "HT_TRENDMODE ", s.Name, " n=", n), got, r.AlignedInt(0, n))
		}
	}
}

func TestHTTRENDMODEOracle(t *testing.T) {
	httrendmodeOracleCompare(t, "")
}

func TestHTTRENDMODEOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 10, 50} {
		if err := SetUnstablePeriod(UnstHTTrendMode, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstHTTrendMode), u)
		httrendmodeOracleCompare(t, lbl("unstable=", u, " "))
	}
}

// TestHTTRENDMODEOracleBadParams: there are no optional inputs, so the only
// rejections are the index checks; Go and C must agree on them.
func TestHTTRENDMODEOracleBadParams(t *testing.T) {
	in := fixtures(100)[0].Real()
	for _, rg := range [][2]int{{-1, 10}, {10, 5}, {0, -1}} {
		r := mustCallRange(t, "HT_TRENDMODE", realIn(in), rg[0], rg[1])
		_, _, err := httrendmodeRange(rg[0], rg[1], in, make([]int, len(in)))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestHTTRENDMODEOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(300)[0], fixtures(300)[1], fixtures(300)[11]} {
		in := s.Real()
		last := len(in) - 1
		for _, rg := range [][2]int{{0, 0}, {0, last}, {3, 3}, {0, 62}, {0, 63}, {63, 63}, {64, 64},
			{10, 80}, {62, 70}, {100, 101}, {120, 250}, {200, last}, {last, last}} {
			out := make([]int, len(in))
			beg, nb, err := httrendmodeRange(rg[0], rg[1], in, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "HT_TRENDMODE", realIn(in), rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%s range %v: go (%d,%d) c (%d,%d)", s.Name, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl(s.Name, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
}

// TestHTTRENDMODEOracleSwitches checks Go and C both emit 0 and 1 on a long
// fixture, so the comparison above is not vacuous.
func TestHTTRENDMODEOracleSwitches(t *testing.T) {
	in := fixtures(1000)[0].Real()
	r := mustCall(t, "HT_TRENDMODE", realIn(in))
	seen := [2]bool{}
	for _, v := range r.Outputs[0].Int[:r.NbElement] {
		seen[v] = true
	}
	if !seen[0] || !seen[1] {
		t.Errorf("C on walk n=1000: seen 0=%v 1=%v", seen[0], seen[1])
	}
}

// TestHTTRENDMODEOracleExtra adds flat series at other levels (where the DFT
// sums are pure rounding residue, so every trig ulp matters) and pure cycles.
func TestHTTRENDMODEOracleExtra(t *testing.T) {
	var cases []series
	for _, v := range []float64{1, 3.7, 1e6, -50, 0.001} {
		cases = append(cases, constant(lbl("flat ", v), 500, v))
	}
	for _, p := range []float64{8, 15, 20, 33, 45} {
		s := series{Name: lbl("cycle ", p)}
		for i := range 500 {
			s.C = append(s.C, 100+10*math.Sin(2*math.Pi*float64(i)/p))
		}
		cases = append(cases, s)
	}
	for _, s := range cases {
		got, err := HTTRENDMODE(s.Real())
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "HT_TRENDMODE", realIn(s.Real()))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		assertInt(t, "HT_TRENDMODE "+s.Name, got, r.AlignedInt(0, len(s.C)))
	}
}
