//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// cdl3starsinsouthOracleCompare compares CDL3STARSINSOUTH with C on every
// fixture and edge length under the current candle settings, and returns how
// many bars fired.
func cdl3starsinsouthOracleCompare(t *testing.T, tag string) int {
	t.Helper()
	lb := CDL3STARSINSOUTHLookback()
	if c := mustLookback(t, "CDL3STARSINSOUTH"); lb != c {
		t.Fatalf("%s lookback: go %d c %d", tag, lb, c)
	}
	fired := 0
	for _, n := range edgeLengths(lb) {
		for _, s := range fixtures(n) {
			got, err := CDL3STARSINSOUTH(s.O, s.H, s.L, s.C)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			r := mustCall(t, "CDL3STARSINSOUTH", s.ohlcv())
			if r.RetCode != oracle.Success {
				t.Fatalf("C retcode %d", r.RetCode)
			}
			assertInt(t, lbl("CDL3STARSINSOUTH ", tag, " ", s.Name, " n=", n), got, r.AlignedInt(0, n))
			for _, v := range got {
				if v != 0 {
					fired++
				}
			}
		}
	}
	// Series on which the pattern actually fires (fixtures almost never do).
	for k, in := range cdl3starsinsouthFiringSeries() {
		got, err := CDL3STARSINSOUTH(in.Open, in.High, in.Low, in.Close)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDL3STARSINSOUTH", in)
		if r.RetCode != oracle.Success {
			t.Fatalf("C retcode %d", r.RetCode)
		}
		assertInt(t, lbl("CDL3STARSINSOUTH ", tag, " firing#", k), got, r.AlignedInt(0, len(in.Close)))
		fired += cdl3starsinsouthCount(got)
	}
	return fired
}

// cdl3starsinsouthFiringSeries returns long series built to make the pattern
// fire: the hand-checked pattern embedded in a random walk, and a dense
// series of random black candles.
func cdl3starsinsouthFiringSeries() []oracle.Inputs {
	s := fixtures(400)[0]
	o := append([]float64(nil), s.O...)
	h := append([]float64(nil), s.H...)
	l := append([]float64(nil), s.L...)
	c := append([]float64(nil), s.C...)
	for at := 50; at+3 <= len(o); at += 60 {
		d := c[at-1] - 100 // shift the pattern to the local price level
		for j, b := range cdl3starsinsouthPattern {
			o[at+j], h[at+j], l[at+j], c[at+j] = b.o+d, b.h+d, b.l+d, b.c+d
		}
	}
	embedded := oracle.Inputs{Open: o, High: h, Low: l, Close: c}

	// Random black candles: body, shadows and gaps from a fixed LCG.
	var dense oracle.Inputs
	seed := uint64(12345)
	rnd := func() float64 {
		seed = seed*6364136223846793005 + 1442695040888963407
		return float64(seed>>11) / float64(1<<53)
	}
	price := 1000.0
	for range 3000 {
		op := price + (rnd()-0.4)*8
		cl := op - rnd()*6
		hi := op + rnd()*rnd()*3
		lo := cl - rnd()*6
		dense.Open = append(dense.Open, op)
		dense.High = append(dense.High, hi)
		dense.Low = append(dense.Low, lo)
		dense.Close = append(dense.Close, cl)
		price = cl + (rnd()-0.3)*6
		if price < 100 {
			price = 1000
		}
	}
	return []oracle.Inputs{embedded, dense}
}

func TestCDL3STARSINSOUTHOracle(t *testing.T) {
	// The pattern is rare (Bulkowski: 9 in 4.7 million bars) and never
	// fires on the plain fixtures; the firing series make it fire in both.
	if fired := cdl3starsinsouthOracleCompare(t, "default"); fired == 0 {
		t.Fatal("pattern never fired")
	}
}

func TestCDL3STARSINSOUTHOracleConstructed(t *testing.T) {
	// The hand-built patterns (and near misses) must agree with C, and the
	// base pattern must fire in both.
	p := cdl3starsinsouthPattern
	variants := [][3]cdl3starsinsouthBar{
		p,
		{p[0], {100.5, 100.5, 92, 98}, p[2]}, // 2nd opens at 1st high
		{p[0], {98, 98.5, 89, 95.5}, p[2]},   // 2nd low at 1st low
		{p[0], {98, 98.5, 95.4, 95.5}, p[2]}, // 2nd no lower shadow
		{p[0], p[1], {95, 95.6, 93.8, 94}},   // 3rd upper shadow long
		{p[0], p[1], {96, 96.1, 93.4, 93.5}}, // 3rd body too big
		{{100, 100.5, 90, 95}, p[1], p[2]},   // 1st lower shadow = body
	}
	for k, v := range variants {
		o, h, l, c := cdl3starsinsouthBuild(v)
		got, err := CDL3STARSINSOUTH(o, h, l, c)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCall(t, "CDL3STARSINSOUTH", oracle.Inputs{Open: o, High: h, Low: l, Close: c})
		want := r.AlignedInt(0, len(o))
		assertInt(t, lbl("constructed#", k), got, want)
		if k == 0 && want[14] != 100 {
			t.Fatalf("C did not fire on the constructed pattern: %v", want)
		}
	}

}

func cdl3starsinsouthCount(v []int) int {
	n := 0
	for _, x := range v {
		if x != 0 {
			n++
		}
	}
	return n
}

func TestCDL3STARSINSOUTHOracleRanges(t *testing.T) {
	s := fixtures(300)[0]
	n := len(s.C)
	for _, rg := range [][2]int{{0, 0}, {0, n - 1}, {3, 3}, {0, 11}, {0, 12}, {12, 12}, {11, 50}, {13, 13}, {150, 299}, {n - 1, n - 1}} {
		out := make([]int, n)
		beg, nb, err := cdl3starsinsouthRange(rg[0], rg[1], s.O, s.H, s.L, s.C, out)
		if err != nil {
			t.Fatal(err)
		}
		r := mustCallRange(t, "CDL3STARSINSOUTH", s.ohlcv(), rg[0], rg[1])
		if r.RetCode != oracle.Success {
			t.Fatalf("range %v: C retcode %d", rg, r.RetCode)
		}
		if beg != r.BegIdx || nb != r.NbElement {
			t.Fatalf("range %v: go (%d,%d) c (%d,%d)", rg, beg, nb, r.BegIdx, r.NbElement)
		}
		assertInt(t, lbl("CDL3STARSINSOUTH range ", rg), out[:nb], r.Outputs[0].Int[:nb])
	}
}

func TestCDL3STARSINSOUTHOracleRangesFiring(t *testing.T) {
	for k, in := range cdl3starsinsouthFiringSeries() {
		n := len(in.Close)
		for _, rg := range [][2]int{{0, n - 1}, {12, 12}, {40, 120}, {52, 52}, {100, n - 1}, {n - 1, n - 1}} {
			out := make([]int, n)
			beg, nb, err := cdl3starsinsouthRange(rg[0], rg[1], in.Open, in.High, in.Low, in.Close, out)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "CDL3STARSINSOUTH", in, rg[0], rg[1])
			if r.RetCode != oracle.Success {
				t.Fatalf("firing#%d range %v: C retcode %d", k, rg, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("firing#%d range %v: go (%d,%d) c (%d,%d)", k, rg, beg, nb, r.BegIdx, r.NbElement)
			}
			assertInt(t, lbl("firing#", k, " range ", rg), out[:nb], r.Outputs[0].Int[:nb])
		}
	}
	// The embedded pattern completes at bar 52: a single-bar range fires.
	in := cdl3starsinsouthFiringSeries()[0]
	out := make([]int, 1)
	if _, nb, err := cdl3starsinsouthRange(52, 52, in.Open, in.High, in.Low, in.Close, out); err != nil || nb != 1 || out[0] != 100 {
		t.Errorf("bar 52: nb=%d out=%v err=%v", nb, out, err)
	}
}

func TestCDL3STARSINSOUTHOracleBadRanges(t *testing.T) {
	s := fixtures(30)[0]
	for _, rg := range [][2]int{{-1, 5}, {5, 4}, {0, -1}} {
		r := mustCallRange(t, "CDL3STARSINSOUTH", s.ohlcv(), rg[0], rg[1])
		_, _, err := cdl3starsinsouthRange(rg[0], rg[1], s.O, s.H, s.L, s.C, make([]int, 30))
		if (r.RetCode != oracle.Success) != (err != nil) {
			t.Errorf("range %v: C retcode %d, Go err %v", rg, r.RetCode, err)
		}
	}
}

func TestCDL3STARSINSOUTHOracleSettings(t *testing.T) {
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	type change struct {
		st     CandleSettingType
		rt     RangeType
		period int
		factor float64
	}
	sets := [][]change{
		{{BodyLong, RangeHighLow, 5, 0.5}},
		{{BodyShort, RangeShadows, 7, 1.5}},
		{{ShadowLong, RangeHighLow, 7, 0.4}},
		{{ShadowVeryShort, RangeShadows, 3, 0.3}},
		{{ShadowVeryShort, RangeRealBody, 0, 1.0}},
		{{BodyLong, RangeRealBody, 0, 0.3}, {BodyShort, RangeHighLow, 0, 2.0}},
		// All periods 0: lookback 2. Never fires: BodyShort period 0 makes the
		// 3rd candle compare its body with itself (body < body).
		{{BodyLong, RangeRealBody, 0, 1.0}, {BodyShort, RangeRealBody, 0, 1.0},
			{ShadowLong, RangeRealBody, 0, 1.0}, {ShadowVeryShort, RangeHighLow, 0, 0.1}},
		// Loose thresholds so the pattern fires more often.
		{{BodyLong, RangeRealBody, 10, 0.1}, {ShadowLong, RangeHighLow, 5, 0.01},
			{BodyShort, RangeHighLow, 10, 3.0}, {ShadowVeryShort, RangeHighLow, 10, 0.3}},
		// Long periods.
		{{BodyLong, RangeRealBody, 50, 1.5}, {ShadowVeryShort, RangeShadows, 30, 1.0}},
	}
	for k, set := range sets {
		RestoreAllCandleDefaultSettings()
		oracle.RestoreCandleDefaults()
		for _, ch := range set {
			if err := SetCandleSettings(ch.st, ch.rt, ch.period, ch.factor); err != nil {
				t.Fatal(err)
			}
			oracle.SetCandleSettings(int(ch.st), int(ch.rt), ch.period, ch.factor)
		}
		fired := cdl3starsinsouthOracleCompare(t, lbl("settings#", k))
		t.Logf("settings#%d fired %d", k, fired)
	}
}
