//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestDONCHIANOracle(t *testing.T) {
	periods := []int{2, 3, 5, 14, 20, 55, 200}
	for _, p := range periods {
		lb := DONCHIANLookback(p)
		if c := mustLookback(t, "DONCHIAN", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				u, m, l, err := DONCHIAN(s.H, s.L, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "DONCHIAN", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("DONCHIAN upper ", s.Name, " n=", n, " p=", p), u, r.AlignedReal(0, n), relTol)
				assertReal(t, lbl("DONCHIAN middle ", s.Name, " n=", n, " p=", p), m, r.AlignedReal(1, n), relTol)
				assertReal(t, lbl("DONCHIAN lower ", s.Name, " n=", n, " p=", p), l, r.AlignedReal(2, n), relTol)
			}
		}
	}
	// Maximum period: lookback check, and a short series entirely inside it.
	if lb, c := DONCHIANLookback(100000), mustLookback(t, "DONCHIAN", 100000); lb != c {
		t.Fatalf("lookback p=100000: go %d c %d", lb, c)
	}
	s := fixtures(64)[0]
	u, m, l, _ := DONCHIAN(s.H, s.L, 100000)
	r := mustCall(t, "DONCHIAN", s.ohlcv(), 100000)
	assertReal(t, "DONCHIAN upper p=100000", u, r.AlignedReal(0, 64), relTol)
	assertReal(t, "DONCHIAN middle p=100000", m, r.AlignedReal(1, 64), relTol)
	assertReal(t, "DONCHIAN lower p=100000", l, r.AlignedReal(2, 64), relTol)
}

func TestDONCHIANOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{1, 0, -5, 100001} {
		r := mustCall(t, "DONCHIAN", s.ohlcv(), float64(p))
		_, _, _, err := DONCHIAN(s.H, s.L, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if lb, c := DONCHIANLookback(p), mustLookback(t, "DONCHIAN", float64(p)); lb != c {
			t.Errorf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
}

func TestDONCHIANOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {5, 30}, {100, 119}} {
		for _, p := range []int{2, 10} {
			u := make([]float64, 120)
			m := make([]float64, 120)
			l := make([]float64, 120)
			beg, nb, err := donchianRange(rg[0], rg[1], s.H, s.L, p, u, m, l)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "DONCHIAN", s.ohlcv(), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("DONCHIAN upper range ", rg, " p=", p), u[:nb], r.Outputs[0].Real, relTol)
			assertReal(t, lbl("DONCHIAN middle range ", rg, " p=", p), m[:nb], r.Outputs[1].Real, relTol)
			assertReal(t, lbl("DONCHIAN lower range ", rg, " p=", p), l[:nb], r.Outputs[2].Real, relTol)
		}
	}
}

// The bands are documented as bit-identical to C's MAX(high), MIDPRICE and
// MIN(low); check that against the oracle too.
func TestDONCHIANOracleEquivalents(t *testing.T) {
	for _, s := range fixtures(257) {
		for _, p := range []int{2, 20} {
			u, m, l, _ := DONCHIAN(s.H, s.L, p)
			assertReal(t, lbl("DONCHIAN vs MAX ", s.Name, " p=", p), u, mustCall(t, "MAX", realIn(s.H), float64(p)).AlignedReal(0, 257), 0)
			assertReal(t, lbl("DONCHIAN vs MIN ", s.Name, " p=", p), l, mustCall(t, "MIN", realIn(s.L), float64(p)).AlignedReal(0, 257), 0)
			assertReal(t, lbl("DONCHIAN vs MIDPRICE ", s.Name, " p=", p), m, mustCall(t, "MIDPRICE", s.ohlcv(), float64(p)).AlignedReal(0, 257), 0)
		}
	}
}
