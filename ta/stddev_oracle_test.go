//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

// stddevOracleOutlier is a large-offset series with sparse big outliers, built
// to exercise both VAR reseed triggers (private copy of the VAR test fixture).
func stddevOracleOutlier(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = 31498938283.624615
		if i%97 == 13 {
			out[i] += 1e6
		}
		if i%11 == 3 {
			out[i] += 1e-3
		}
	}
	return out
}

func TestSTDDEVOracle(t *testing.T) {
	periods := []int{2, 3, 5, 14, 30, 200}
	for _, p := range periods {
		for _, d := range []float64{1, -2, 0, 2.5} {
			lb := STDDEVLookback(p, d)
			if c := mustLookback(t, "STDDEV", float64(p), d); lb != c {
				t.Fatalf("lookback p=%d d=%g: go %d c %d", p, d, lb, c)
			}
			for _, n := range edgeLengths(lb) {
				for _, s := range fixtures(n) {
					got, err := STDDEV(s.Real(), p, d)
					if err != nil {
						t.Fatal(err)
					}
					if n == 0 {
						continue // C rejects endIdx -1; Go returns empty output
					}
					r := mustCall(t, "STDDEV", realIn(s.Real()), float64(p), d)
					if r.RetCode != oracle.Success {
						t.Fatalf("C retcode %d", r.RetCode)
					}
					assertReal(t, lbl("STDDEV ", s.Name, " n=", n, " p=", p, " d=", d), got, r.AlignedReal(0, n), relTol)
				}
				if n == 0 {
					continue
				}
				in := stddevOracleOutlier(n)
				got, _ := STDDEV(in, p, d)
				r := mustCall(t, "STDDEV", realIn(in), float64(p), d)
				assertReal(t, lbl("STDDEV outlier n=", n, " p=", p, " d=", d), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestSTDDEVOracleLargePeriod(t *testing.T) {
	in := fixtures(3000)[0].Real()
	for _, p := range []int{1000, 2999, 3000, 3001} {
		if lb, c := STDDEVLookback(p, 1), mustLookback(t, "STDDEV", float64(p), 1); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		got, _ := STDDEV(in, p, 1)
		r := mustCall(t, "STDDEV", realIn(in), float64(p), 1)
		assertReal(t, lbl("STDDEV large p=", p), got, r.AlignedReal(0, len(in)), relTol)
	}
	if lb, c := STDDEVLookback(100000, 1), mustLookback(t, "STDDEV", 100000, 1); lb != c {
		t.Fatalf("lookback max: go %d c %d", lb, c)
	}
}

func TestSTDDEVOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{0, 1, -5, 100001, 2, 100000} {
		r := mustCall(t, "STDDEV", realIn(in), float64(p), 1)
		_, err := STDDEV(in, p, 1)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if lb, c := STDDEVLookback(p, 1), mustLookback(t, "STDDEV", float64(p), 1); lb != c {
			t.Errorf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
	for _, d := range []float64{3.0000001e37, -3.0000001e37, 3e37, -3e37, 1e38} {
		r := mustCall(t, "STDDEV", realIn(in), 5, d)
		_, err := STDDEV(in, 5, d)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("d=%g: C retcode %d, Go err %v", d, r.RetCode, err)
		}
		if lb, c := STDDEVLookback(5, d), mustLookback(t, "STDDEV", 5, d); lb != c {
			t.Errorf("lookback d=%g: go %d c %d", d, lb, c)
		}
	}
}

func TestSTDDEVOracleRanges(t *testing.T) {
	for _, s := range []series{fixtures(400)[1], fixtures(400)[4]} {
		in := s.Real()
		for _, rg := range [][2]int{{0, 0}, {0, 399}, {3, 3}, {10, 50}, {399, 399}, {0, 8}, {9, 9}, {100, 399}} {
			for _, p := range []int{2, 10} {
				for _, d := range []float64{1, -1.5} {
					out := make([]float64, len(in))
					beg, nb, err := stddevRange(rg[0], rg[1], in, p, d, out)
					if err != nil {
						t.Fatal(err)
					}
					r := mustCallRange(t, "STDDEV", realIn(in), rg[0], rg[1], float64(p), d)
					if beg != r.BegIdx || nb != r.NbElement {
						t.Fatalf("%s range %v p=%d: go (%d,%d) c (%d,%d)", s.Name, rg, p, beg, nb, r.BegIdx, r.NbElement)
					}
					assertReal(t, lbl("STDDEV range ", s.Name, rg, " p=", p, " d=", d), out[:nb], r.Outputs[0].Real, relTol)
				}
			}
		}
	}
}
