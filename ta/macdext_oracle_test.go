//go:build oracle

package ta

import (
	"math"
	"testing"

	"tradeloom/ta/internal/oracle"
)

func (p macdextParams) opt() []float64 {
	return []float64{float64(p.fast), float64(p.fastT), float64(p.slow), float64(p.slowT), float64(p.signal), float64(p.sigT)}
}

// macdextOracleSets is a spread of parameters: every MAType in every slot,
// swapped periods, minimum and default periods, signal period 1 and all-EMA.
func macdextOracleSets() []macdextParams {
	sets := []macdextParams{
		{12, MATypeSMA, 26, MATypeSMA, 9, MATypeSMA},
		{12, MATypeEMA, 26, MATypeEMA, 9, MATypeEMA},
		{26, MATypeEMA, 12, MATypeEMA, 9, MATypeEMA},
		{12, MATypeEMA, 26, MATypeEMA, 1, MATypeEMA},
		{2, MATypeEMA, 2, MATypeEMA, 2, MATypeEMA},
		{2, MATypeSMA, 2, MATypeSMA, 1, MATypeSMA},
		{2, MATypeWMA, 3, MATypeKAMA, 1, MATypeMAMA},
		{30, MATypeWMA, 5, MATypeTEMA, 7, MATypeKAMA},
		{12, MATypeDefault, 26, MATypeDefault, 9, MATypeDefault},
		{100, MATypeSMA, 3, MATypeHMA, 20, MATypeRMA},
	}
	for _, mt := range macdextAllTypes {
		sets = append(sets,
			macdextParams{12, mt, 26, mt, 9, mt},
			macdextParams{5, mt, 13, MATypeSMA, 4, MATypeEMA},
			macdextParams{13, MATypeEMA, 5, mt, 3, MATypeWMA},
			macdextParams{3, MATypeTRIMA, 8, MATypeDEMA, 5, mt},
			macdextParams{4, MATypeEMA, 9, MATypeEMA, 1, mt},
		)
	}
	return sets
}

// macdextScaleTol is the tolerance relative to the input magnitude used on
// top of relTol. MACD is fast MA minus slow MA: on the "huge" fixture
// (prices near 1e9) both MAs are ~1e9 while their difference is ~1e2, so the
// last-ulp differences the WMA/HMA dependencies already have against C (up to
// ~70 ulps of 1e9, i.e. ~1e-14 relative, within their own relTol) become
// ~1e-9 relative to the MACD value. Measured worst case: 3.2e-6 absolute at
// input scale 1e9 (3e-15 relative to the input). 1e-12 relative to the input
// still rejects any real porting error.
const macdextScaleTol = 1e-12

func macdextInScale(in []float64) float64 {
	m := 0.0
	for _, v := range in {
		m = math.Max(m, math.Abs(v))
	}
	return m
}

// macdextAssert is assertReal with relTol, additionally accepting an absolute
// difference up to macdextScaleTol*inScale (see macdextScaleTol).
func macdextAssert(t *testing.T, label string, got, want []float64, inScale float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len(got)=%d len(want)=%d", label, len(got), len(want))
	}
	bad := 0
	for i := range got {
		if closeEnough(got[i], want[i], relTol) {
			continue
		}
		if !math.IsNaN(got[i]) && !math.IsNaN(want[i]) && math.Abs(got[i]-want[i]) <= macdextScaleTol*inScale {
			continue
		}
		if bad < 5 {
			t.Errorf("%s: [%d] got %.17g want %.17g", label, i, got[i], want[i])
		}
		bad++
	}
	if bad > 5 {
		t.Errorf("%s: %d mismatches in total", label, bad)
	}
}

func macdextCompare(t *testing.T, label string, in []float64, p macdextParams) {
	t.Helper()
	n := len(in)
	m, sg, h, err := p.run(in)
	if err != nil {
		t.Fatalf("%s: %v", label, err)
	}
	r := mustCall(t, "MACDEXT", realIn(in), p.opt()...)
	if r.RetCode != oracle.Success {
		t.Fatalf("%s: C retcode %d", label, r.RetCode)
	}
	sc := macdextInScale(in)
	macdextAssert(t, label+" macd", m, r.AlignedReal(0, n), sc)
	macdextAssert(t, label+" signal", sg, r.AlignedReal(1, n), sc)
	macdextAssert(t, label+" hist", h, r.AlignedReal(2, n), sc)
}

func TestMACDEXTOracle(t *testing.T) {
	for _, p := range macdextOracleSets() {
		lb := p.lookback()
		if c := mustLookback(t, "MACDEXT", p.opt()...); lb != c {
			t.Fatalf("lookback %+v: go %d c %d", p, lb, c)
		}
		for _, n := range edgeLengths(lb) {
			if n == 0 {
				continue // C rejects endIdx -1; Go returns empty output
			}
			for _, s := range fixtures(n) {
				macdextCompare(t, lbl("MACDEXT ", s.Name, " n=", n, " ", p), s.Real(), p)
			}
		}
	}
}

func TestMACDEXTOracleMaxPeriods(t *testing.T) {
	// Periods at the upper bound with a short series: everything is inside
	// the lookback, in Go and in C.
	in := fixtures(300)[0].Real()
	for _, p := range []macdextParams{
		{100000, MATypeSMA, 100000, MATypeSMA, 100000, MATypeSMA},
		{2, MATypeSMA, 100000, MATypeWMA, 1, MATypeSMA},
		{100000, MATypeEMA, 2, MATypeEMA, 100000, MATypeEMA},
	} {
		if lb, c := p.lookback(), mustLookback(t, "MACDEXT", p.opt()...); lb != c {
			t.Fatalf("lookback %+v: go %d c %d", p, lb, c)
		}
		macdextCompare(t, lbl("MACDEXT max ", p), in, p)
	}
}

func TestMACDEXTOracleBadParams(t *testing.T) {
	in := fixtures(60)[0].Real()
	def := macdextParams{12, MATypeSMA, 26, MATypeSMA, 9, MATypeSMA}
	var sets []macdextParams
	for _, v := range []int{1, 0, -5, 2, 100000, 100001} {
		p := def
		p.fast = v
		sets = append(sets, p)
		p = def
		p.slow = v
		sets = append(sets, p)
	}
	for _, v := range []int{0, -5, 1, 100000, 100001} {
		p := def
		p.signal = v
		sets = append(sets, p)
	}
	for _, mt := range []MAType{-1, 0, 13, 14, 20} {
		p := def
		p.fastT = mt
		sets = append(sets, p)
		p = def
		p.slowT = mt
		sets = append(sets, p)
		p = def
		p.sigT = mt
		sets = append(sets, p)
	}
	for _, p := range sets {
		r := mustCall(t, "MACDEXT", realIn(in), p.opt()...)
		_, _, _, err := p.run(in)
		if (r.RetCode == oracle.BadParam) != (err != nil) {
			t.Errorf("%+v: C retcode %d, Go err %v", p, r.RetCode, err)
		}
		lb := p.lookback()
		c, cerr := oracle.Lookback("MACDEXT", p.opt()...)
		if (cerr != nil || c == -1) != (lb == -1) {
			t.Errorf("lookback %+v: go %d, c %d (%v)", p, lb, c, cerr)
		}
	}
}

func TestMACDEXTOracleRanges(t *testing.T) {
	s := fixtures(150)[1]
	in := s.Real()
	sets := []macdextParams{
		{12, MATypeSMA, 26, MATypeSMA, 9, MATypeSMA},
		{12, MATypeEMA, 26, MATypeEMA, 9, MATypeEMA},
		{26, MATypeWMA, 5, MATypeKAMA, 4, MATypeT3},
		{3, MATypeMAMA, 8, MATypeHMA, 1, MATypeZLEMA},
		{2, MATypeSMA, 3, MATypeSMA, 2, MATypeRMA},
	}
	for _, p := range sets {
		lb := p.lookback()
		ranges := [][2]int{{0, 0}, {0, 149}, {3, 3}, {10, 60}, {149, 149}, {0, lb - 1}, {lb, lb}, {lb - 1, lb + 5}, {lb + 7, 149}}
		for _, rg := range ranges {
			if rg[0] < 0 || rg[1] < rg[0] {
				continue
			}
			m, sg, h := make([]float64, len(in)), make([]float64, len(in)), make([]float64, len(in))
			beg, nb, err := macdextRange(rg[0], rg[1], in, p.fast, p.fastT, p.slow, p.slowT, p.signal, p.sigT, m, sg, h)
			if err != nil {
				t.Fatal(err)
			}
			r := mustCallRange(t, "MACDEXT", realIn(in), rg[0], rg[1], p.opt()...)
			if r.RetCode != oracle.Success {
				t.Fatalf("range %v %+v: C retcode %d", rg, p, r.RetCode)
			}
			if beg != r.BegIdx || nb != r.NbElement {
				t.Fatalf("range %v %+v: go (%d,%d) c (%d,%d)", rg, p, beg, nb, r.BegIdx, r.NbElement)
			}
			l := lbl("MACDEXT range ", rg, " ", p)
			sc := macdextInScale(in)
			macdextAssert(t, l+" macd", m[:nb], r.Outputs[0].Real, sc)
			macdextAssert(t, l+" signal", sg[:nb], r.Outputs[1].Real, sc)
			macdextAssert(t, l+" hist", h[:nb], r.Outputs[2].Real, sc)
		}
	}
}

func TestMACDEXTOracleUnstable(t *testing.T) {
	defer SetAllUnstablePeriods(0)
	defer oracle.ResetUnstablePeriods()
	sets := []macdextParams{
		{12, MATypeEMA, 26, MATypeEMA, 9, MATypeEMA},
		{5, MATypeKAMA, 13, MATypeT3, 4, MATypeRMA},
		{4, MATypeMAMA, 9, MATypeEMA, 3, MATypeKAMA},
		{6, MATypeDEMA, 10, MATypeZLEMA, 5, MATypeTEMA},
	}
	for _, u := range []int{1, 7, 25} {
		for _, id := range []FuncUnstID{UnstEMA, UnstKAMA, UnstMAMA, UnstT3, UnstRMA} {
			if err := SetUnstablePeriod(id, u); err != nil {
				t.Fatal(err)
			}
			oracle.SetUnstablePeriod(int(id), u)
		}
		for _, p := range sets {
			lb := p.lookback()
			if c := mustLookback(t, "MACDEXT", p.opt()...); lb != c {
				t.Fatalf("unstable %d lookback %+v: go %d c %d", u, p, lb, c)
			}
			for _, n := range []int{lb, lb + 1, lb + 10, 300} {
				if n <= 0 {
					continue
				}
				for _, s := range fixtures(n) {
					macdextCompare(t, lbl("MACDEXT unstable ", u, " ", s.Name, " n=", n, " ", p), s.Real(), p)
				}
			}
		}
	}
}
