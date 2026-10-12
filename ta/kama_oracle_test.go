//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func kamaOracleCompare(t *testing.T, prefix string, periods []int) {
	t.Helper()
	for _, p := range periods {
		lb := KAMALookback(p)
		if c := mustLookback(t, "KAMA", float64(p)); lb != c {
			t.Fatalf("%slookback p=%d: go %d c %d", prefix, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := KAMA(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "KAMA", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl(prefix, "KAMA ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestKAMAOracle(t *testing.T) {
	kamaOracleCompare(t, "", []int{1, 2, 3, 5, 10, 14, 30, 200})
}

func TestKAMAOracleMaxPeriod(t *testing.T) {
	p := 100000
	if lb, c := KAMALookback(p), mustLookback(t, "KAMA", float64(p)); lb != c {
		t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
	}
	for _, n := range []int{1, 1000, p, p + 1, p + 50} {
		for _, s := range fixtures(n)[:3] {
			got, err := KAMA(s.Real(), p)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "KAMA", realIn(s.Real()), float64(p))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("KAMA max ", s.Name, " n=", n), got, r.AlignedReal(0, n), relTol)
		}
	}
}

func TestKAMAOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{0, -5, 100001, 1, 100000} {
		r := mustCall(t, "KAMA", realIn(in), float64(p))
		_, err := KAMA(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if lb, c := KAMALookback(p), mustLookback(t, "KAMA", float64(p)); lb != c {
			t.Errorf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
}

func kamaOracleRanges(t *testing.T, prefix string) {
	t.Helper()
	s := fixtures(120)[1]
	in := s.Real()
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 9}, {10, 10}, {11, 11}, {30, 119}} {
		for _, p := range []int{1, 2, 10} {
			out := make([]float64, len(in))
			beg, nb, err := kamaRange(rg[0], rg[1], in, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "KAMA", realIn(in), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("%srange %v p=%d: go (%d,%d) c (%d,%d)", prefix, rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl(prefix, "KAMA range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}

func TestKAMAOracleRanges(t *testing.T) {
	kamaOracleRanges(t, "")
}

func TestKAMAOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 37} {
		SetUnstablePeriod(UnstKAMA, u)
		oracle.SetUnstablePeriod(int(UnstKAMA), u)
		kamaOracleCompare(t, lbl("unst=", u, " "), []int{1, 2, 5, 14, 30})
		kamaOracleRanges(t, lbl("unst=", u, " "))
	}
	// Another function's unstable period must not leak into KAMA.
	SetAllUnstablePeriods(0)
	oracle.ResetUnstablePeriods()
	SetUnstablePeriod(UnstRSI, 9)
	oracle.SetUnstablePeriod(int(UnstRSI), 9)
	kamaOracleCompare(t, "rsi-unst ", []int{3, 14})
}
