//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func vwmaIn(real, volume []float64) oracle.Inputs {
	return oracle.Inputs{Real: [][]float64{real}, Volume: volume}
}

func TestVWMAOracle(t *testing.T) {
	periods := []int{1, 2, 3, 5, 14, 30, 200}
	for _, p := range periods {
		lb := VWMALookback(p)
		if c := mustLookback(t, "VWMA", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := VWMA(s.Real(), s.V, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "VWMA", vwmaIn(s.Real(), s.V), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("VWMA ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestVWMAOracleMaxPeriod(t *testing.T) {
	const p = 100000
	if c := mustLookback(t, "VWMA", float64(p)); VWMALookback(p) != c {
		t.Fatalf("lookback p=%d: go %d c %d", p, VWMALookback(p), c)
	}
	for _, n := range []int{p - 1, p, p + 3} {
		s := fixtures(n)[0]
		got, err := VWMA(s.Real(), s.V, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "VWMA", vwmaIn(s.Real(), s.V), float64(p))
		assertReal(t, lbl("VWMA max n=", n), got, r.AlignedReal(0, n), relTol)
	}
}

func TestVWMAOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{0, -5, 100001, 1, 100000} {
		r := mustCall(t, "VWMA", vwmaIn(s.Real(), s.V), float64(p))
		_, err := VWMA(s.Real(), s.V, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
	}
	for _, p := range []int{0, -5, 100001} {
		if c := mustLookback(t, "VWMA", float64(p)); c != -1 || VWMALookback(p) != -1 {
			t.Errorf("lookback p=%d: go %d c %d", p, VWMALookback(p), c)
		}
	}
}

func TestVWMAOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(120)[1], fixtures(120)[6], fixtures(120)[9]} {
		in, vol := s.Real(), s.V
		for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {5, 12}} {
			for _, p := range []int{1, 2, 10} {
				out := make([]float64, len(in))
				beg, nb, err := vwmaRange(rg[0], rg[1], in, vol, p, out)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCallRange(t, "VWMA", vwmaIn(in, vol), rg[0], rg[1], float64(p))
				if beg != r.BegIdx || nb != r.NbElement {
					t.Fatalf("%s range %v p=%d: go (%d,%d) c (%d,%d)", s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
				}
				assertReal(t, lbl("VWMA range ", s.Name, " ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
			}
		}
	}
}
