//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func TestCMOUOracle(t *testing.T) {
	periods := []int{2, 3, 5, 9, 14, 30, 200}
	for _, p := range periods {
		lb := CMOULookback(p)
		if c := mustLookback(t, "CMOU", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := CMOU(s.Real(), p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "CMOU", realIn(s.Real()), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("CMOU ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
	// Maximum period on a series just long enough for a few outputs.
	p := 100000
	if c := mustLookback(t, "CMOU", float64(p)); c != CMOULookback(p) {
		t.Fatalf("lookback p=%d: go %d c %d", p, CMOULookback(p), c)
	}
	for _, s := range fixtures(p + 3)[:3] {
		got, err := CMOU(s.Real(), p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CMOU", realIn(s.Real()), float64(p))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		assertReal(t, lbl("CMOU max ", s.Name), got, r.AlignedReal(0, len(got)), relTol)
	}
}

func TestCMOUOracleFlatRuns(t *testing.T) {
	// Movement followed by flat runs exercises the nullRun residue reset.
	var in []float64
	for i := 0; i < 300; i++ {
		switch {
		case i%60 < 25:
			in = append(in, 100+0.1*float64(i%7)-0.37*float64(i%3))
		default:
			in = append(in, 101.3)
		}
	}
	for _, p := range []int{2, 5, 14, 30} {
		got, _ := CMOU(in, p)
		r := mustCall(t, "CMOU", realIn(in), float64(p))
		assertReal(t, lbl("CMOU flat p=", p), got, r.AlignedReal(0, len(in)), relTol)
	}
}

func TestCMOUOracleBadParams(t *testing.T) {
	in := fixtures(20)[0].Real()
	for _, p := range []int{1, 0, -5, 100001, 2, 14} {
		r := mustCall(t, "CMOU", realIn(in), float64(p))
		_, err := CMOU(in, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := mustLookback(t, "CMOU", float64(p)); c != CMOULookback(p) {
			t.Errorf("lookback p=%d: go %d c %d", p, CMOULookback(p), c)
		}
	}
}

func TestCMOUOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	in := s.Real()
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 9}, {10, 10}, {5, 12}} {
		for _, p := range []int{2, 10} {
			out := make([]float64, len(in))
			beg, nb, err := cmouRange(rg[0], rg[1], in, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CMOU", realIn(in), rg[0], rg[1], float64(p))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("CMOU range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
