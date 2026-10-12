//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Differential tests: the Go port must match TA-Lib C on every fixture, every
// edge length and a spread of parameters.

func natrOracleFull(t *testing.T, periods []int, tag string) {
	t.Helper()
	for _, p := range periods {
		lb := NATRLookback(p)
		if c := mustLookback(t, "NATR", float64(p)); lb != c {
			t.Fatalf("%s lookback p=%d: go %d c %d", tag, p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			for _, s := range fixtures(n) {
				got, err := NATR(s.H, s.L, s.C, p)
				if err != nil {
					t.Fatal(err)
				}
				if n == 0 {
					continue // C rejects endIdx -1; Go returns empty output
				}
				r := mustCall(t, "NATR", s.ohlcv(), float64(p))
				if r.RetCode != oracle.Success {
					t.Fatalf("C retcode %d", r.RetCode)
				}
				assertReal(t, lbl("NATR ", tag, " ", s.Name, " n=", n, " p=", p), got, r.AlignedReal(0, n), relTol)
			}
		}
	}
}

func TestNATROracle(t *testing.T) {
	natrOracleFull(t, []int{1, 2, 3, 5, 14, 30, 200}, "")
}

func TestNATROracleMaxPeriod(t *testing.T) {
	p := 100000
	if lb, c := NATRLookback(p), mustLookback(t, "NATR", float64(p)); lb != c {
		t.Fatalf("lookback p=%d: go %d c %d", p, lb, c)
	}
	for _, n := range []int{p, p + 1, p + 50} {
		s := fixtures(n)[0]
		got, err := NATR(s.H, s.L, s.C, p)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "NATR", s.ohlcv(), float64(p))
		assertReal(t, lbl("NATR max n=", n), got, r.AlignedReal(0, n), relTol)
	}
}

func TestNATROracleBadParams(t *testing.T) {
	s := fixtures(20)[0]
	for _, p := range []int{0, -5, 1, 100000, 100001} {
		r := mustCall(t, "NATR", s.ohlcv(), float64(p))
		_, err := NATR(s.H, s.L, s.C, p)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("p=%d: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		if c := NATRLookback(p); err != nil && c != -1 {
			t.Errorf("p=%d: lookback %d want -1", p, c)
		}
	}
}

func TestNATROracleRanges(t *testing.T) {
	s := fixtures(120)[1]
	ranges := [][2]int{{0, 0}, {0, 119}, {3, 3}, {10, 50}, {119, 119}, {0, 9}, {10, 10}, {9, 9}, {11, 11}, {0, 10}, {50, 119}}
	for _, rg := range ranges {
		for _, p := range []int{1, 2, 10} {
			out := make([]float64, 120)
			beg, nb, err := natrRange(rg[0], rg[1], s.H, s.L, s.C, p, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "NATR", s.ohlcv(), rg[0], rg[1], float64(p))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("NATR range ", rg, " p=", p), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}

func TestNATROracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	for _, u := range []int{1, 5, 40} {
		if err := SetUnstablePeriod(UnstNATR, u); err != nil {
			t.Fatal(err)
		}
		oracle.SetUnstablePeriod(int(UnstNATR), u)
		natrOracleFull(t, []int{1, 2, 14}, lbl("unst=", u))

		s := fixtures(150)[4]
		for _, rg := range [][2]int{{0, 149}, {20, 20}, {30, 100}, {149, 149}, {0, 14 + u - 1}} {
			out := make([]float64, 150)
			beg, nb, err := natrRange(rg[0], rg[1], s.H, s.L, s.C, 14, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "NATR", s.ohlcv(), rg[0], rg[1], 14)
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("unst=%d range %v: go (%d,%d) c (%d,%d)", u, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertReal(t, lbl("NATR unst=", u, " range ", rg), out[:nb], r.Outputs[0].Real, relTol)
		}
	}
}

// Closes that are exactly zero (output 0), tiny, or negative, at arbitrary bars,
// including the first output bar.
func TestNATROracleZeroClose(t *testing.T) {
	for _, base := range []series{fixtures(200)[0], fixtures(200)[3], fixtures(200)[13]} {
		s := base
		s.C = append([]float64(nil), base.C...)
		for _, i := range []int{0, 2, 3, 14, 15, 50, 51, 120, 199} {
			s.C[i] = 0
		}
		s.C[60] = 1e-300
		s.C[61] = -1e-300
		for _, p := range []int{1, 2, 3, 14} {
			got, err := NATR(s.H, s.L, s.C, p)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCall(t, "NATR", s.ohlcv(), float64(p))
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertReal(t, lbl("NATR zero-close ", s.Name, " p=", p), got, r.AlignedReal(0, len(s.C)), relTol)
			for _, rg := range [][2]int{{p, p}, {15, 15}, {14, 60}, {50, 199}} {
				out := make([]float64, len(s.C))
				beg, nb, err := natrRange(rg[0], rg[1], s.H, s.L, s.C, p, out)
				if err != nil {
					t.Fatal(err)
				}
				rr := mustCallRange(t, "NATR", s.ohlcv(), rg[0], rg[1], float64(p))
				if beg != rr.BegIdx || nb != rr.NbElement {
					t.Fatalf("range %v p=%d: go (%d,%d) c (%d,%d)", rg, p, beg, nb, rr.BegIdx, rr.NbElement)
				}
				assertReal(t, lbl("NATR zero-close range ", rg, " p=", p), out[:nb], rr.Outputs[0].Real, relTol)
			}
		}
	}
}
