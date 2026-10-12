//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

type correlPair struct {
	name string
	x, y []float64
}

// correlOraclePairs returns (inReal0, inReal1) pairs built from the fixtures:
// each series against itself, against the next fixture, the next against it,
// and its open against its close.
func correlOraclePairs(n int) []correlPair {
	fx := fixtures(n)
	var out []correlPair
	for k, s := range fx {
		o := fx[(k+1)%len(fx)]
		out = append(out,
			correlPair{s.Name + "/self", s.Real(), s.Real()},
			correlPair{s.Name + "/" + o.Name, s.Real(), o.Real()},
			correlPair{o.Name + "/" + s.Name, o.Real(), s.Real()},
			correlPair{s.Name + "/open-close", s.O, s.C},
		)
	}
	return out
}

func TestCORRELOracle(t *testing.T) {
	periods := []int{1, 2, 3, 5, 14, 30, 200}
	for _, p := range periods {
		lb := CORRELLookback(p)
		if c := mustLookback(t, "CORREL", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, pr := range correlOraclePairs(n) {
				got, err := CORREL(pr.x, pr.y, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "CORREL", realIn(pr.x, pr.y), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("CORREL ", pr.name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
	for _, p := range []int{100000} {
		if lb, c := CORRELLookback(p), mustLookback(t, "CORREL", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
}

// TestCORRELOracleOutliers drives the reseed triggers (spikes on either axis,
// flat stretches, tiny spreads at a high level) and long series that cross
// the periodic 32*period re-anchor several times.
func TestCORRELOracleOutliers(t *testing.T) {
	n := 1000
	base := fixtures(n)
	for _, spike := range []float64{1e3, 1e5, 1e8, 1e-6, 0} {
		for _, axis := range []int{0, 1} {
			x := append([]float64(nil), base[0].Real()...)
			y := append([]float64(nil), base[1].Real()...)
			tgt := x
			if axis == 1 {
				tgt = y
			}
			for _, at := range []int{20, 21, 300, 301, 302, 777} {
				tgt[at] *= spike
			}
			for _, p := range []int{2, 5, 14, 30} {
				got, err := CORREL(x, y, p)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCall(t, "CORREL", realIn(x, y), float64(p))
				assertReal(t, lbl("CORREL spike ", spike, " axis ", axis, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
	// Flat stretches inside a walk, and a tiny spread at a $100 level.
	x := append([]float64(nil), base[0].Real()...)
	for i := 100; i < 200; i++ {
		x[i] = 1234.5
	}
	tiny := make([]float64, n)
	tiny2 := make([]float64, n)
	for i := range tiny {
		tiny[i] = 100 + 1e-5*base[3].Real()[i]*1e6
		tiny2[i] = 100 + 1e-5*base[2].Real()[i]
	}
	for _, pr := range []correlPair{{"flat/walk2", x, base[1].Real()}, {"walk2/flat", base[1].Real(), x}, {"tiny/tiny2", tiny, tiny2}, {"tiny/self", tiny, tiny}} {
		for _, p := range []int{2, 5, 14, 30, 200} {
			got, _ := CORREL(pr.x, pr.y, p)
			r := mustCall(t, "CORREL", realIn(pr.x, pr.y), float64(p))
			assertReal(t, lbl("CORREL ", pr.name, " p=", p), got, r.AlignedReal(0, n), relTol)
		}
	}
}

func TestCORRELOracleBadParams(t *testing.T) {
	s := fixtures(20)
	in0, in1 := s[0].Real(), s[1].Real()
	for _, p := range []int{0, -5, 100001, 1, 100000} {
		r := mustCall(t, "CORREL", realIn(in0, in1), float64(p))
		_, err := CORREL(in0, in1, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if lb, c := CORRELLookback(p), mustLookback(t, "CORREL", float64(p)); lb != c {
			t.Errorf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
}

func TestCORRELOracleRanges(t *testing.T) {
	fx := fixtures(120)
	in0, in1 := fx[1].Real(), fx[4].Real()
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 8}, {9, 9}, {8, 12}, {100, 119}} {
		for _, p := range []int{1, 5, 10} {
			out := make([]float64, len(in0))
			beg, nb, err := correlRange(rg[0], rg[1], in0, in1, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CORREL", realIn(in0, in1), rg[0], rg[1], float64(p))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("CORREL range ", rg, " p=", p), out[:nb], r.Outputs[0].Real[:nb], relTol)
		}
	}
}
