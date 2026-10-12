//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

var ultoscOracleParams = [][3]int{
	{1, 1, 1}, {1, 2, 3}, {3, 2, 1}, {7, 14, 28}, {28, 14, 7}, {14, 7, 28},
	{5, 5, 5}, {2, 2, 9}, {9, 2, 2}, {10, 33, 40}, {1, 200, 50},
}

func TestULTOSCOracle(t *testing.T) {
	for _, ps := range ultoscOracleParams {
		lb := ULTOSCLookback(ps[0], ps[1], ps[2])
		if c := mustLookback(t, "ULTOSC", float64(ps[0]), float64(ps[1]), float64(ps[2])); lb != c {
			t.Fatalf("lookback p=%v: go %d c %d", ps, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := ULTOSC(s.H, s.L, s.C, ps[0], ps[1], ps[2])
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "ULTOSC", s.ohlcv(), float64(ps[0]), float64(ps[1]), float64(ps[2]))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("ULTOSC ", s.Name, " n=", n, " p=", ps), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
	// Maximum period: lookback, plus one series just past it.
	for _, ps := range [][3]int{{100000, 7, 14}, {1, 100000, 100000}} {
		lb := ULTOSCLookback(ps[0], ps[1], ps[2])
		if c := mustLookback(t, "ULTOSC", float64(ps[0]), float64(ps[1]), float64(ps[2])); lb != c {
			t.Fatalf("lookback p=%v: go %d c %d", ps, lb, c)
		}
	}
	s := fixtures(100005)[0]
	got, err := ULTOSC(s.H, s.L, s.C, 100000, 7, 14)
	if err != nil {
		t.Fatal(err)
	}
	r := mustCall(t, "ULTOSC", s.ohlcv(), 100000, 7, 14)
	if r.RetCode != oracle.Success {
		t.Fatalf("C retcode %d", r.RetCode)
	}
	assertReal(t, "ULTOSC p=100000", got, r.AlignedReal(0, len(s.C)), relTol)
}

// A series that moves, then goes flat, then moves again exercises the
// null-run reseed.
func TestULTOSCOracleFlatRuns(t *testing.T) {
	s := fixtures(200)[2]
	h, l, c := append([]float64(nil), s.H...), append([]float64(nil), s.L...), append([]float64(nil), s.C...)
	for i := 60; i < 120; i++ {
		h[i], l[i], c[i] = c[59], c[59], c[59]
	}
	in := oracle.Inputs{Open: s.O, High: h, Low: l, Close: c, Volume: s.V}
	for _, ps := range [][3]int{{7, 14, 28}, {3, 5, 10}, {1, 1, 1}, {2, 30, 70}} {
		got, err := ULTOSC(h, l, c, ps[0], ps[1], ps[2])
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "ULTOSC", in, float64(ps[0]), float64(ps[1]), float64(ps[2]))
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		assertReal(t, lbl("ULTOSC flat p=", ps), got, r.AlignedReal(0, len(c)), relTol)
	}
}

func TestULTOSCOracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{0, -5, 100001, 1, 100000} {
		for k := 0; k < 3; k++ {
			ps := [3]int{7, 14, 28}
			ps[k] = p
			r := mustCall(t, "ULTOSC", s.ohlcv(), float64(ps[0]), float64(ps[1]), float64(ps[2]))
			_, err := ULTOSC(s.H, s.L, s.C, ps[0], ps[1], ps[2])
			if (r.RetCode == oracle.BadParam) != (err != nil) {
				t.Errorf("p=%v: C retcode %d, Go err %v", ps, r.RetCode, err)
			}
			lb, cerr := oracle.Lookback("ULTOSC", float64(ps[0]), float64(ps[1]), float64(ps[2]))
			if cerr != nil {
				t.Fatal(cerr)
			}
			if glb := ULTOSCLookback(ps[0], ps[1], ps[2]); glb != lb {
				t.Errorf("lookback p=%v: go %d c %d", ps, glb, lb)
			}
		}
	}
}

func TestULTOSCOracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	for _, rg := range [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 27}, {28, 28}, {27, 30}, {5, 119}} {
		for _, ps := range [][3]int{{1, 1, 1}, {7, 14, 28}, {28, 3, 10}} {
			out := make([]float64, len(s.C))
			beg, nb, err := ultoscRange(rg[0], rg[1], s.H, s.L, s.C, ps[0], ps[1], ps[2], out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "ULTOSC", s.ohlcv(), rg[0], rg[1], float64(ps[0]), float64(ps[1]), float64(ps[2]))
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%v: go (%d,%d) c (%d,%d)", rg, ps, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("ULTOSC range ", rg, " p=", ps), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}
