//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

func trixOracleCompare(t *testing.T, prefix string, periods []int) {
	t.Helper()
	for _, p := range periods {
		lb := TRIXLookback(p)
		if c := mustLookback(t, "TRIX", float64(p)); lb != c {
			t.Fatalf("%slookback p=%d: go %d c %d", prefix, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := TRIX(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "TRIX", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl(prefix, "TRIX ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestTRIXOracle(t *testing.T) {
	trixOracleCompare(t, "", []int{1, 2, 3, 5, 9, 14, 30, 100})
}

func TestTRIXOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{0, -5, 100001, 1, 100000} {
		r := mustCall(t, "TRIX", realIn(in), float64(p))
		_, err := TRIX(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "TRIX", float64(p)); c != TRIXLookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, TRIXLookback(p), c)
		}
	}
}

func trixOracleRanges(t *testing.T, prefix string) {
	t.Helper()
	for _, s := range fixtures(150) {
		in := s.Real()
		for _, rg := range [][2]int{{0, 0}, {0, 149}, {3, 3}, {10, 50}, {149, 149}, {0, 12}, {13, 13}, {39, 39}, {40, 149}, {100, 120}} {
			for _, p := range []int{1, 2, 5, 14} {
				out := make([]float64, len(in))
				beg, nb, err := trixRange(rg[0], rg[1], in, p, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "TRIX", realIn(in), rg[0], rg[1], float64(p))
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s%s range %v p=%d: go (%d,%d) c (%d,%d)", prefix, s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl(prefix, "TRIX range ", s.Name, rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}

func TestTRIXOracleRanges(t *testing.T) {
	trixOracleRanges(t, "")
}

func TestTRIXOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 37} {
		SetUnstablePeriod(UnstEMA, u)
		oracle.SetUnstablePeriod(int(UnstEMA), u)
		trixOracleCompare(t, lbl("unst=", u, " "), []int{1, 2, 5, 14})
		trixOracleRanges(t, lbl("unst=", u, " "))
	}
	// Another function's unstable period must not leak into TRIX.
	SetAllUnstablePeriods(0)
	oracle.ResetUnstablePeriods()
	SetUnstablePeriod(UnstRSI, 9)
	oracle.SetUnstablePeriod(int(UnstRSI), 9)
	trixOracleCompare(t, "rsi-unst ", []int{3, 14})
}
