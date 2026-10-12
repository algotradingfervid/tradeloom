//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

var percentileOraclePcts = []float64{0, 1, 10, 20, 33.33, 50, 66.67, 90, 99.99, 100}

func TestPERCENTILEOracle(t *testing.T) {
	periods := []int{2, 3, 4, 5, 14, 30, 31, 200}
	for _, p := range periods {
		for _, q := range percentileOraclePcts {
			lb := PERCENTILELookback(p, q)
			if c := mustLookback(t, "PERCENTILE", float64(p), q); lb != c {
				t.Fatalf("lookback p=%d q=%g: go %d c %d", p, q, lb, c)
			}
			for _, n := range edgeLengths(lb) {
				for _, s := range fixtures(n) {
					got, err := PERCENTILE(s.Real(), p, q)
					if err != nil {
						t.Fatal(err)
					}
					if n == 0 {
						continue // C rejects endIdx -1; Go returns empty output
					}
					r := mustCall(t, "PERCENTILE", realIn(s.Real()), float64(p), q)
					if r.RetCode != oracle.Success {
						t.Fatalf("C retcode %d", r.RetCode)
					}
					assertReal(t, lbl("PERCENTILE ", s.Name, " n=", n, " p=", p, " q=", q), got, r.AlignedReal(0, n), relTol)
				}
			}
		}
	}
}

func TestPERCENTILEOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	cases := []struct {
		p int
		q float64
	}{
		{1, 50}, {0, 50}, {-5, 50}, {100001, 50},
		{5, -1}, {5, -0.001}, {5, 100.001}, {5, 1000},
		{2, 0}, {2, 100}, {5, 50},
	}
	for _, c := range cases {
		r := mustCall(t, "PERCENTILE", realIn(in), float64(c.p), c.q)
		_, err := PERCENTILE(in, c.p, c.q)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d q=%g: C retcode %d, Go err %v", c.p, c.q, r.RetCode, err)
		}
		lb := mustLookback(t, "PERCENTILE", float64(c.p), c.q)
		if glb := PERCENTILELookback(c.p, c.q); glb != lb {
			t.Errorf("lookback p=%d q=%g: go %d c %d", c.p, c.q, glb, lb)
		}
	}
}

func TestPERCENTILEOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.Real()
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {8, 9}, {100, 119}} {
		for _, p := range []int{2, 10, 40} {
			for _, q := range []float64{0, 50, 75, 100} {
				out := make([]float64, len(in))
				beg, nb, err := percentileRange(rg[0], rg[1], in, p, q, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "PERCENTILE", realIn(in), rg[0], rg[1], float64(p), q)
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("range %v p=%d q=%g: go (%d,%d) c (%d,%d)", rg, p, q, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("PERCENTILE range ", rg, " p=", p, " q=", q), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}
