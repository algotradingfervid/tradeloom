//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func cmoOracleCompare(t *testing.T, prefix string, periods []int) {
	t.Helper()
	for _, p := range periods {
		lb := CMOLookback(p)
		if c := mustLookback(t, "CMO", float64(p)); lb != c {
			t.Fatalf("%slookback p=%d: go %d c %d", prefix, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := CMO(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "CMO", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl(prefix, "CMO ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestCMOOracle(t *testing.T) {
	cmoOracleCompare(t, "", []int{2, 3, 5, 9, 14, 21, 30, 200})
}

func TestCMOOracleMaxPeriod(t *testing.T) {
	p := 100000
	if lb, c := CMOLookback(p), mustLookback(t, "CMO", float64(p)); lb != c {
		t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
	}
	for _, n := range []int{1, 1000, p - 1, p, p + 1, p + 50} {
		for _, s := range fixtures(n)[:3] {
			got, err := CMO(s.Real(), p)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "CMO", realIn(s.Real()), float64(p))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("CMO max ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

func TestCMOOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{1, 0, -5, 100001, 2, 100000} {
		r := mustCall(t, "CMO", realIn(in), float64(p))
		_, err := CMO(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if lb, c := CMOLookback(p), mustLookback(t, "CMO", float64(p)); lb != c {
			t.Errorf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
}

func cmoOracleRanges(t *testing.T, prefix string) {
	t.Helper()
	for _, s := range []series{fixtures(120)[1], fixtures(120)[3], fixtures(120)[5], fixtures(120)[9]} {
		in := s.Real()
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 9}, {10, 10}, {11, 11}, {30, 119}, {2, 2}} {
			for _, p := range []int{2, 3, 10, 14} {
				out := make([]float64, len(in))
				beg, nb, err := cmoRange(rg[0], rg[1], in, p, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "CMO", realIn(in), rg[0], rg[1], float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%srange %s %v p=%d: go (%d,%d) c (%d,%d)", prefix, s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl(prefix, "CMO range ", s.Name, " ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

func TestCMOOracleRanges(t *testing.T) {
	cmoOracleRanges(t, "")
}

func TestCMOOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 37} {
		SetUnstablePeriod(UnstCMO, u)
		oracle.SetUnstablePeriod(int(UnstCMO), u)
		cmoOracleCompare(t, lbl("unst=", u, " "), []int{2, 3, 14, 30})
		cmoOracleRanges(t, lbl("unst=", u, " "))
	}
	// Another function's unstable period must not leak into CMO.
	SetAllUnstablePeriods(0)
	oracle.ResetUnstablePeriods()
	SetUnstablePeriod(UnstRSI, 9)
	oracle.SetUnstablePeriod(int(UnstRSI), 9)
	cmoOracleCompare(t, "rsi-unst ", []int{3, 14})
}
