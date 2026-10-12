//go:build oracle

package ta

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Randomised differential test: every function, many random inputs
// (including NaN, ±Inf, ties, extreme magnitudes, inconsistent OHLC), random
// valid and invalid parameters, random sub-ranges, random unstable periods
// and candle settings, compared with the C library.
//
//	FUZZ_ITERS (default 40) iterations per function, FUZZ_SEED (default 1),
//	FUZZ_ONLY=NAME[,NAME] to restrict, FUZZ_NONAN=1 to keep inputs finite,
//	FUZZ_N=n to fix the series length, FUZZ_DUMP=dir to save failing cases,
//	FUZZ_SHOWINEXACT=1 to log cases that match only within tolerance,
//	FUZZ_EXACT=1 to fail them (use with -tags oracle,libmc).

func fuzzEnvInt(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return v
	}
	return def
}

type fuzzCase struct {
	n      int
	ohlcv  [5][]float64 // open, high, low, close, volume
	reals  [][]float64  // Real inputs in order
	opts   []float64
	unst   int
	candle bool
	desc   []string
}

func (c *fuzzCase) note(f string, a ...any) { c.desc = append(c.desc, fmt.Sprintf(f, a...)) }

// fuzzSeries returns n values of a randomly chosen shape.
func fuzzSeries(r *rand.Rand, n int, c *fuzzCase, label string, nan bool) []float64 {
	s := make([]float64, n)
	mode := r.IntN(9)
	scale := []float64{1, 100, 1e-6, 1e6, 0.01, 1e-300, 1e300, 1, 1}[mode]
	tick := 0.0
	if r.IntN(3) == 0 {
		tick = []float64{0.05, 1, 0.5}[r.IntN(3)]
	}
	x := scale * (1 + 9*r.Float64())
	for i := range s {
		switch mode {
		case 7: // constant with rare jumps
			if r.IntN(20) == 0 {
				x += scale * (r.Float64() - 0.5)
			}
		case 8: // signed walk crossing zero
			x += r.NormFloat64()
		default:
			x += x * 0.03 * r.NormFloat64()
		}
		v := x
		if tick > 0 && scale >= 1 && scale <= 100 {
			v = math.Round(v/tick) * tick
		}
		s[i] = v
	}
	if nan && r.IntN(4) == 0 {
		k := 1 + r.IntN(3)
		for j := 0; j < k && n > 0; j++ {
			i := r.IntN(n)
			s[i] = []float64{math.NaN(), math.Inf(1), math.Inf(-1), 0, -0.0}[r.IntN(5)]
		}
	}
	c.note("%s mode=%d tick=%g", label, mode, tick)
	return s
}

// fuzzGridOHLCV builds candles on an integer grid: bodies and shadows of a
// few ticks, often exactly zero, so equal-value ties (dojis, marubozus, equal
// highs/lows, touching gaps) are common and every <, <= branch is exercised.
func fuzzGridOHLCV(r *rand.Rand, n int, c *fuzzCase) {
	o, h, l, cl, v := make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
	ticks := []float64{0, 0, 0, 1, 1, 2, 3, 5, 8, 13}
	pick := func() float64 { return ticks[r.IntN(len(ticks))] }
	sign := func() float64 { return float64(1 - 2*r.IntN(2)) }
	x := 100.0
	for i := 0; i < n; i++ {
		switch r.IntN(4) {
		case 0:
			x += sign() * pick() // gap
		case 1:
			if i > 0 {
				x = cl[i-1]
			}
		}
		o[i] = x
		cl[i] = x + sign()*pick()
		h[i] = cMax(o[i], cl[i]) + pick()
		l[i] = cMin(o[i], cl[i]) - pick()
		v[i] = 100 * pick()
		x = cl[i]
	}
	c.note("grid candles")
	c.ohlcv = [5][]float64{o, h, l, cl, v}
}

func fuzzOHLCV(r *rand.Rand, n int, c *fuzzCase, nan bool) {
	if r.IntN(2) == 0 {
		fuzzGridOHLCV(r, n, c)
		return
	}
	cl := fuzzSeries(r, n, c, "close", nan)
	o, h, l, v := make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
	consistent := r.IntN(5) != 0
	doji := r.IntN(4) == 0
	for i := range cl {
		prev := cl[i]
		if i > 0 {
			prev = cl[i-1]
		}
		o[i] = prev + (cl[i]-prev)*r.Float64()*0.5
		if doji && r.IntN(3) == 0 {
			o[i] = cl[i]
		}
		if r.IntN(10) == 0 {
			o[i] = cl[i] * (1 + 0.05*r.NormFloat64()) // gap
		}
		span := math.Abs(cl[i]) * 0.02 * r.Float64()
		if r.IntN(6) == 0 {
			span = 0
		}
		h[i] = cMax(o[i], cl[i]) + span*r.Float64()
		l[i] = cMin(o[i], cl[i]) - span*r.Float64()
		if !consistent {
			h[i], l[i] = l[i], h[i]
		}
		v[i] = math.Round(1000 * r.Float64() * 1e3)
		if r.IntN(8) == 0 {
			v[i] = 0
		}
	}
	if !consistent {
		c.note("inconsistent OHLC")
	}
	if doji {
		c.note("dojis")
	}
	c.ohlcv = [5][]float64{o, h, l, cl, v}
}

func fuzzOpt(r *rand.Rand, oi oracle.OptInfo, n int) float64 {
	if oi.List != nil {
		if r.IntN(12) == 0 {
			return []float64{oi.Min - 1, oi.Max + 1, -100}[r.IntN(3)]
		}
		return float64(oi.List[r.IntN(len(oi.List))])
	}
	if oi.IsReal {
		lo, hi := math.Max(oi.Min, -1e3), math.Min(oi.Max, 1e3)
		switch k := r.IntN(14); {
		case k == 0:
			return oi.Min
		case k == 1:
			return oi.Max
		case k == 2 && oi.Min > -1e30:
			return math.Nextafter(oi.Min, math.Inf(-1))
		case k == 3 && oi.Max < 1e30:
			return math.Nextafter(oi.Max, math.Inf(1))
		case k == 4:
			return math.NaN()
		case k <= 7:
			return oi.Default
		default:
			return lo + (hi-lo)*r.Float64()
		}
	}
	lo, hi := int(oi.Min), int(oi.Max)
	cap := cMinInt(hi, cMaxInt(lo, n+5))
	switch k := r.IntN(16); {
	case k == 0:
		return float64(lo - 1)
	case k == 1:
		return float64(hi + 1)
	case k == 2:
		return float64(hi)
	case k == 3:
		return float64(lo)
	case k <= 6:
		return oi.Default
	case k <= 9:
		return float64(lo + r.IntN(cMinInt(cap, lo+10)-lo+1))
	default:
		return float64(lo + r.IntN(cap-lo+1))
	}
}

// fuzzArgs builds the Go argument list (inputs then opts) for fn's type.
func fuzzArgs(ft reflect.Type, fi oracle.FuncInfo, c *fuzzCase, lead []reflect.Value) []reflect.Value {
	args := append([]reflect.Value(nil), lead...)
	ri := 0
	for _, in := range fi.Inputs {
		switch in.Kind {
		case oracle.InputPrice:
			for k, flag := range []int{oracle.PriceOpen, oracle.PriceHigh, oracle.PriceLow, oracle.PriceClose, oracle.PriceVolume} {
				if in.Flags&flag != 0 {
					args = append(args, reflect.ValueOf(c.ohlcv[k]))
				}
			}
		case oracle.InputReal:
			args = append(args, reflect.ValueOf(c.reals[ri]))
			ri++
		}
	}
	for _, v := range c.opts {
		pt := ft.In(len(args))
		switch pt.Kind() {
		case reflect.Float64:
			args = append(args, reflect.ValueOf(v).Convert(pt))
		default:
			args = append(args, reflect.ValueOf(int(v)).Convert(pt))
		}
	}
	return args
}

func (c *fuzzCase) oracleInputs(fi oracle.FuncInfo) oracle.Inputs {
	var in oracle.Inputs
	for _, ii := range fi.Inputs {
		if ii.Kind == oracle.InputPrice {
			f := ii.Flags
			pick := func(flag, k int) []float64 {
				if f&flag != 0 {
					return c.ohlcv[k]
				}
				return nil
			}
			in.Open, in.High, in.Low = pick(oracle.PriceOpen, 0), pick(oracle.PriceHigh, 1), pick(oracle.PriceLow, 2)
			in.Close, in.Volume = pick(oracle.PriceClose, 3), pick(oracle.PriceVolume, 4)
		}
	}
	in.Real = c.reals
	return in
}

// lastFuzzCase is the case fuzzOne ran most recently, for FUZZ_DUMP.
var lastFuzzCase *fuzzCase

// fuzzDump writes the failing case as JSON to $FUZZ_DUMP/<name>-<it>.json.
func fuzzDump(t *testing.T, name string, it int) {
	dir := os.Getenv("FUZZ_DUMP")
	if dir == "" || lastFuzzCase == nil {
		return
	}
	c := lastFuzzCase
	b, err := json.Marshal(map[string]any{"ohlcv": c.ohlcv, "reals": c.reals, "opts": c.opts, "unstable": c.unst, "desc": c.desc})
	if err != nil {
		t.Log(err) // NaN/Inf inputs are not representable in JSON
		return
	}
	os.WriteFile(fmt.Sprintf("%s/%s-%d.json", dir, name, it), b, 0o644)
}

func fuzzCall(fn reflect.Value, args []reflect.Value) (out []reflect.Value, panicked any) {
	defer func() { panicked = recover() }()
	return fn.Call(args), nil
}

func cloneAll(ss [][]float64) [][]float64 {
	out := make([][]float64, len(ss))
	for i, s := range ss {
		out[i] = append([]float64(nil), s...)
	}
	return out
}

func sameBits(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Float64bits(a[i]) != math.Float64bits(b[i]) && !(math.IsNaN(a[i]) && math.IsNaN(b[i])) {
			return false
		}
	}
	return true
}

func TestFuzzOracle(t *testing.T) {
	iters, seed := fuzzEnvInt("FUZZ_ITERS", 40), uint64(fuzzEnvInt("FUZZ_SEED", 1))
	only := os.Getenv("FUZZ_ONLY")
	nan := os.Getenv("FUZZ_NONAN") == ""
	defer SetAllUnstablePeriods(0)
	defer oracle.SetAllUnstablePeriods(0)
	defer RestoreAllCandleDefaultSettings()
	defer oracle.RestoreCandleDefaults()
	inexact := 0
	var inexactFns []string
	for _, e := range fuzzRegistry {
		if only != "" && !strings.Contains(","+only+",", ","+e.name+",") {
			continue
		}
		t.Run(e.name, func(t *testing.T) {
			fi, err := oracle.Info(e.name)
			if err != nil {
				t.Fatal(err)
			}
			h := fnv.New64a()
			h.Write([]byte(e.name))
			pub, rng, lb := reflect.ValueOf(e.pub), reflect.ValueOf(e.rng), reflect.ValueOf(e.lb)
			fails, fnInexact := 0, 0
			// Patterns are cheap and fire rarely; give them more cases so tie
			// comparisons inside multi-candle patterns are reached.
			n := iters
			if strings.HasPrefix(e.name, "CDL") {
				n *= 5
			}
			for it := 0; it < n && fails < 3; it++ {
				r := rand.New(rand.NewPCG(seed, h.Sum64()+uint64(it)))
				ok, ex := fuzzOne(t, e.name, fi, pub, rng, lb, r, nan)
				if !ok {
					fails++
					fuzzDump(t, e.name, it)
					t.Logf("reproduce: FUZZ_SEED=%d FUZZ_ONLY=%s, iteration %d", seed, e.name, it)
				}
				if !ex && os.Getenv("FUZZ_EXACT") != "" && ok {
					fails++
					t.Errorf("not bit-exact: %s", strings.Join(lastFuzzCase.desc, "; "))
					t.Logf("reproduce: FUZZ_SEED=%d FUZZ_ONLY=%s, iteration %d", seed, e.name, it)
				}
				if !ex {
					if os.Getenv("FUZZ_SHOWINEXACT") != "" {
						t.Logf("inexact: %s", strings.Join(lastFuzzCase.desc, "; "))
					}
					inexact++
					fnInexact++
				}
			}
			if fnInexact > 0 {
				inexactFns = append(inexactFns, fmt.Sprintf("%s:%d", e.name, fnInexact))
			}
		})
	}
	t.Logf("%d iterations matched within tolerance but not bit for bit", inexact)
	t.Logf("not bit-exact (function:iterations): %s", strings.Join(inexactFns, " "))
}

// fuzzOne runs one random case. ok is false on a mismatch; exact reports
// whether every compared value was bit-identical.
func fuzzOne(t *testing.T, name string, fi oracle.FuncInfo, pub, rng, lb reflect.Value, r *rand.Rand, nan bool) (ok, exact bool) {
	t.Helper()
	c := &fuzzCase{}
	lastFuzzCase = c
	switch k := r.IntN(10); {
	case k == 0:
		c.n = 1 + r.IntN(3)
	case k <= 6:
		c.n = 1 + r.IntN(120)
	case k <= 8:
		c.n = 100 + r.IntN(400)
	default:
		c.n = 600 + r.IntN(1500)
	}
	if n := fuzzEnvInt("FUZZ_N", 0); n > 0 {
		c.n = n
	}
	fuzzOHLCV(r, c.n, c, nan)
	nReal := 0
	for _, ii := range fi.Inputs {
		if ii.Kind == oracle.InputReal {
			nReal++
		}
	}
	for i := 0; i < nReal; i++ {
		var s []float64
		switch {
		case name == "MAVP" && i == 1:
			s = make([]float64, c.n)
			for j := range s {
				s[j] = math.Round(40 * r.Float64())
				if nan && r.IntN(50) == 0 {
					s[j] = math.NaN()
				}
			}
		case r.IntN(3) == 0:
			s = fuzzSeries(r, c.n, c, fmt.Sprint("real", i), nan)
		default:
			s = append([]float64(nil), c.ohlcv[r.IntN(4)]...)
		}
		// Bounded inputs for the math transforms keep both sides in-domain often.
		if strings.HasPrefix(name, "A") && (name == "ACOS" || name == "ASIN") && r.IntN(2) == 0 {
			for j := range s {
				s[j] = math.Sin(s[j])
			}
		}
		c.reals = append(c.reals, s)
	}
	for _, oi := range fi.Opts {
		c.opts = append(c.opts, fuzzOpt(r, oi, c.n))
	}
	c.note("n=%d opts=%v", c.n, c.opts)
	if r.IntN(4) == 0 {
		c.unst = []int{1, 3, 10, 40}[r.IntN(4)]
		SetAllUnstablePeriods(c.unst)
		oracle.SetAllUnstablePeriods(c.unst)
		c.note("unstable=%d", c.unst)
	}
	defer func() {
		if c.unst != 0 {
			SetAllUnstablePeriods(0)
			oracle.SetAllUnstablePeriods(0)
		}
		if c.candle {
			RestoreAllCandleDefaultSettings()
			oracle.RestoreCandleDefaults()
		}
	}()
	if strings.HasPrefix(name, "CDL") && r.IntN(3) == 0 {
		c.candle = true
		for st := 0; st < int(candleSettingCount); st++ {
			if r.IntN(2) == 0 {
				continue
			}
			rt, ap, f := r.IntN(3), r.IntN(15), []float64{0, 0.1, 0.5, 1, 2, 3}[r.IntN(6)]*(0.5+r.Float64())
			if err := SetCandleSettings(CandleSettingType(st), RangeType(rt), ap, f); err != nil {
				t.Fatal(err)
			}
			oracle.SetCandleSettings(st, rt, ap, f)
			c.note("candle[%d]={%d,%d,%g}", st, rt, ap, f)
		}
	}
	ok, exact = true, true
	fail := func(f string, a ...any) {
		if ok {
			t.Errorf("%s case: %s", name, strings.Join(c.desc, "; "))
		}
		ok = false
		t.Errorf("  "+f, a...)
	}
	cmpReal := func(label string, got, want []float64) {
		if len(got) != len(want) {
			fail("%s: len go %d c %d", label, len(got), len(want))
			return
		}
		bad := 0
		for i := range got {
			if !closeEnough(got[i], want[i], relTol) {
				if bad < 3 {
					fail("%s[%d]: go %.17g c %.17g", label, i, got[i], want[i])
				}
				bad++
			}
		}
		if bad > 3 {
			fail("%s: %d mismatches", label, bad)
		}
		if !sameBits(got, want) {
			exact = false
		}
	}
	cmpInt := func(label string, got, want []int) {
		if len(got) != len(want) {
			fail("%s: len go %d c %d", label, len(got), len(want))
			return
		}
		bad := 0
		for i := range got {
			if got[i] != want[i] {
				if bad < 3 {
					fail("%s[%d]: go %d c %d", label, i, got[i], want[i])
				}
				bad++
			}
		}
		if bad > 3 {
			fail("%s: %d mismatches", label, bad)
		}
	}

	// 1. Lookback.
	lbArgs := make([]reflect.Value, len(c.opts))
	for i, v := range c.opts {
		pt := lb.Type().In(i)
		if pt.Kind() == reflect.Float64 {
			lbArgs[i] = reflect.ValueOf(v).Convert(pt)
		} else {
			lbArgs[i] = reflect.ValueOf(int(v)).Convert(pt)
		}
	}
	lbOut, p := fuzzCall(lb, lbArgs)
	if p != nil {
		fail("Lookback panicked: %v", p)
		return
	}
	goLB := int(lbOut[0].Int())
	cLB, cErr := oracle.Lookback(name, c.opts...)
	switch {
	case cErr != nil && goLB != -1:
		fail("lookback: C error %v, Go %d", cErr, goLB)
	case cErr == nil && goLB != cLB:
		fail("lookback: go %d c %d", goLB, cLB)
	}

	// 2. Public form vs a full C call; inputs must not be modified.
	inBefore := cloneAll(append(c.ohlcv[:], c.reals...))
	args := fuzzArgs(pub.Type(), fi, c, nil)
	outs, p := fuzzCall(pub, args)
	if p != nil {
		fail("public form panicked: %v", p)
		return
	}
	for i, s := range append(c.ohlcv[:], c.reals...) {
		if !sameBits(s, inBefore[i]) {
			fail("public form modified input #%d", i)
		}
	}
	goErr, _ := outs[len(outs)-1].Interface().(error)
	res, err := oracle.Call(name, c.oracleInputs(fi), c.opts...)
	if err != nil {
		t.Fatal(err)
	}
	if (res.RetCode == oracle.BadParam) != (goErr != nil) {
		fail("public: C retcode %d, Go err %v", res.RetCode, goErr)
		return
	}
	if goErr != nil {
		if !errors.Is(goErr, ErrBadParam) {
			fail("public: error %v is not ErrBadParam", goErr)
		}
	} else {
		if res.RetCode != oracle.Success {
			fail("public: C retcode %d, Go ok", res.RetCode)
			return
		}
		for i := 0; i < fi.NbOutputs; i++ {
			l := fmt.Sprint("public out", i)
			if fi.IntOutputs[i] {
				cmpInt(l, outs[i].Interface().([]int), res.AlignedInt(i, c.n))
			} else {
				cmpReal(l, outs[i].Interface().([]float64), res.AlignedReal(i, c.n))
			}
		}
	}

	// 3. Range form on random sub-ranges, with exactly sized output buffers.
	for k := 0; k < 3; k++ {
		s, e := r.IntN(c.n), r.IntN(c.n)
		switch r.IntN(6) {
		case 0:
			s, e = 0, c.n-1
		case 1:
			s = e
		case 2:
			s, e = e, s // possibly reversed
		}
		size := cMaxInt(1, e-s+1)
		var bufs []reflect.Value
		var realBufs [][]float64
		var intBufs [][]int
		for i := 0; i < fi.NbOutputs; i++ {
			if fi.IntOutputs[i] {
				b := make([]int, size)
				intBufs = append(intBufs, b)
				bufs = append(bufs, reflect.ValueOf(b))
			} else {
				b := make([]float64, size)
				realBufs = append(realBufs, b)
				bufs = append(bufs, reflect.ValueOf(b))
			}
		}
		ra := fuzzArgs(rng.Type(), fi, c, []reflect.Value{reflect.ValueOf(s), reflect.ValueOf(e)})
		ro, p := fuzzCall(rng, append(ra, bufs...))
		if p != nil {
			fail("range [%d,%d] panicked: %v", s, e, p)
			return
		}
		beg, nb := int(ro[0].Int()), int(ro[1].Int())
		rerr, _ := ro[2].Interface().(error)
		cr, err := oracle.CallRange(name, c.oracleInputs(fi), s, e, c.opts...)
		if err != nil {
			t.Fatal(err)
		}
		want := map[int]error{oracle.BadParam: ErrBadParam, oracle.OutOfRangeStartIndex: ErrOutOfRangeStartIndex, oracle.OutOfRangeEndIndex: ErrOutOfRangeEndIndex}
		if cr.RetCode != oracle.Success {
			if we, ok2 := want[cr.RetCode]; !ok2 || !errors.Is(rerr, we) {
				fail("range [%d,%d]: C retcode %d, Go err %v", s, e, cr.RetCode, rerr)
			}
			continue
		}
		if rerr != nil {
			fail("range [%d,%d]: C ok, Go err %v", s, e, rerr)
			continue
		}
		if beg != cr.BegIdx || nb != cr.NbElement {
			fail("range [%d,%d]: go (%d,%d) c (%d,%d)", s, e, beg, nb, cr.BegIdx, cr.NbElement)
			continue
		}
		ri, ii := 0, 0
		for i := 0; i < fi.NbOutputs; i++ {
			l := fmt.Sprintf("range [%d,%d] out%d", s, e, i)
			if fi.IntOutputs[i] {
				cmpInt(l, intBufs[ii][:nb], cr.Outputs[i].Int)
				ii++
			} else {
				cmpReal(l, realBufs[ri][:nb], cr.Outputs[i].Real)
				ri++
			}
		}
	}

	// 4. In-place: output 0 aliasing the only real input (TA-Lib allows it).
	if goErr == nil && nReal == 1 && len(fi.Inputs) == 1 && !fi.IntOutputs[0] && c.n > 0 {
		ref := outs[0].Interface().([]float64)
		buf := append([]float64(nil), c.reals[0]...)
		saved := c.reals[0]
		c.reals[0] = buf
		var bufs []reflect.Value
		for i := 0; i < fi.NbOutputs; i++ {
			if i == 0 {
				bufs = append(bufs, reflect.ValueOf(buf))
			} else if fi.IntOutputs[i] {
				bufs = append(bufs, reflect.ValueOf(make([]int, c.n)))
			} else {
				bufs = append(bufs, reflect.ValueOf(make([]float64, c.n)))
			}
		}
		ra := fuzzArgs(rng.Type(), fi, c, []reflect.Value{reflect.ValueOf(0), reflect.ValueOf(c.n - 1)})
		ro, p := fuzzCall(rng, append(ra, bufs...))
		c.reals[0] = saved
		if p != nil {
			fail("in-place panicked: %v", p)
			return
		}
		beg, nb := int(ro[0].Int()), int(ro[1].Int())
		if e, _ := ro[2].Interface().(error); e != nil {
			fail("in-place: %v", e)
		} else if nb > 0 {
			cmpReal("in-place out0", buf[:nb], ref[beg:beg+nb])
		}
	}
	return ok, exact
}
