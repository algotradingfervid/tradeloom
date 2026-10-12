//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

// betaOraclePairs returns (inReal0, inReal1) pairs built from the fixtures:
// each series against itself, against the next fixture, and the next against it.
func betaOraclePairs(n int) []struct {
	name string
	x, y []float64
} {
	fx := fixtures(n)
	var out []struct {
		name string
		x, y []float64
	}
	for k, s := range fx {
		o := fx[(k+1)%len(fx)]
		out = append(out,
			struct {
				name string
				x, y []float64
			}{s.Name + "/self", s.Real(), s.Real()},
			struct {
				name string
				x, y []float64
			}{s.Name + "/" + o.Name, s.Real(), o.Real()},
			struct {
				name string
				x, y []float64
			}{o.Name + "/" + s.Name, o.Real(), s.Real()},
			struct {
				name string
				x, y []float64
			}{s.Name + "/open-close", s.O, s.C},
		)
	}
	return out
}

func TestBETAOracle(t *testing.T) {
	periods := []int{1, 2, 3, 5, 14, 30, 200}
	for _, p := range periods {
		lb := BETALookback(p)
		if c := mustLookback(t, "BETA", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, pr := range betaOraclePairs(n) {
				got, err := BETA(pr.x, pr.y, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "BETA", realIn(pr.x, pr.y), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("BETA ", pr.name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
	for _, p := range []int{100000} {
		if lb, c := BETALookback(p), mustLookback(t, "BETA", float64(p)); lb != c {
			t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
}

// TestBETAOracleOutliers drives the reseed triggers (spikes on either axis,
// zero prices, drifting series) and long series that cross the periodic
// 32*period re-anchor several times.
func TestBETAOracleOutliers(t *testing.T) {
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
				got, err := BETA(x, y, p)
				if err != nil {
					t.Fatal(err)
				}
				r := mustCall(t, "BETA", realIn(x, y), float64(p))
				assertReal(t, lbl("BETA spike ", spike, " axis ", axis, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestBETAOracleBadParams(t *testing.T) {
	s := fixtures(20)
	in0, in1 := s[0].Real(), s[1].Real()
	for _, p := range []int{0, -5, 100001, 1, 100000} {
		r := mustCall(t, "BETA", realIn(in0, in1), float64(p))
		_, err := BETA(in0, in1, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if lb, c := BETALookback(p), mustLookback(t, "BETA", float64(p)); lb != c {
			t.Errorf("lookback p=%d: go %d c %d", p, lb, c)
		}
	}
}

func TestBETAOracleRanges(t *testing.T) {
	fx := fixtures(120)
	in0, in1 := fx[1].Real(), fx[4].Real()
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 9}, {10, 10}, {9, 12}, {100, 119}} {
		for _, p := range []int{1, 5, 10} {
			out := make([]float64, len(in0))
			beg, nb, err := betaRange(rg[0], rg[1], in0, in1, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "BETA", realIn(in0, in1), rg[0], rg[1], float64(p))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("BETA range ", rg, " p=", p), out[:nb], r.Outputs[0].Real[:nb], relTol)
		}
	}
}
