//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestEROracle(t *testing.T) {
	periods := []int{2, 3, 5, 10, 14, 30, 200, 100000}
	for _, p := range periods {
		lb := ERLookback(p)
		if c := mustLookback(t, "ER", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := ER(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "ER", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("ER ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestEROracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{1, 0, -5, 100001, 2, 100000} {
		r := mustCall(t, "ER", realIn(in), float64(p))
		_, err := ER(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "ER", float64(p)); c != ERLookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, ERLookback(p), c)
		}
	}
}

func TestEROracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[10], fixtures(120)[11]} {
		in := s.Real()
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 9}, {10, 10}, {5, 15}} {
			for _, p := range []int{2, 10} {
				out := make([]float64, len(in))
				beg, nb, err := erRange(rg[0], rg[1], in, p, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "ER", realIn(in), rg[0], rg[1], float64(p))
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%d: go (%d,%d) c (%d,%d)", s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("ER range ", s.Name, rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}
