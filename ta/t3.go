package ta

import "math"

// T3 — Triple Exponential Moving Average (T3) (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/t3 and src/ta_func/ta_T3.c.

// T3Lookback returns the number of leading bars T3 leaves empty:
// 6*(optInTimePeriod-1) plus the T3 unstable period. It returns -1 on an
// invalid parameter.
// optInTimePeriod: [1, 100000], default 5. optInVFactor: [0, 1], default 0.7.
func T3Lookback(optInTimePeriod int, optInVFactor float64) int {
	if t3Check(optInTimePeriod, optInVFactor) != nil {
		return -1
	}
	return t3Lookback(optInTimePeriod, optInVFactor)
}

func t3Lookback(optInTimePeriod int, optInVFactor float64) int {
	return 6*(optInTimePeriod-1) + UnstablePeriod(UnstT3)
}

func t3Check(optInTimePeriod int, optInVFactor float64) error {
	if err := checkInt("T3", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return err
	}
	return checkReal("T3", "optInVFactor", optInVFactor, 0, 1)
}

// T3 returns Tillson's T3 of inReal: six chained EMAs of optInTimePeriod
// combined with coefficients derived from the volume factor optInVFactor.
// The output is index-aligned with inReal; the first T3Lookback bars are NaN.
// optInTimePeriod: [1, 100000], default 5. optInVFactor: [0, 1], default 0.7.
func T3(inReal []float64, optInTimePeriod int, optInVFactor float64) ([]float64, error) {
	if err := t3Check(optInTimePeriod, optInVFactor); err != nil {
		return nil, err
	}
	n, err := checkInputs("T3", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := t3Range(0, n-1, inReal, optInTimePeriod, optInVFactor, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// t3Range mirrors TA_T3: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func t3Range(startIdx, endIdx int, inReal []float64, optInTimePeriod int, optInVFactor float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := t3Check(optInTimePeriod, optInVFactor); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("T3", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("T3", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := t3Lookback(optInTimePeriod, optInVFactor)
	if startIdx <= lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// No smoothing at period 1: the output is a copy of the input.
	if optInTimePeriod == 1 {
		outIdx := 0
		for today := startIdx; today <= endIdx; today++ {
			outReal[outIdx] = inReal[today]
			outIdx++
		}
		return startIdx, outIdx, nil
	}

	// The C source uses explicit fma() calls; math.FMA reproduces them
	// exactly. k*x is a separate rounded product, as in C.
	today := startIdx - lookbackTotal
	k := 2.0 / (float64(optInTimePeriod) + 1.0)
	oneMinusK := 1.0 - k
	period := float64(optInTimePeriod)

	// Initialize e1.
	tempReal := inReal[today]
	today++
	for i := optInTimePeriod - 1; i > 0; i-- {
		tempReal += inReal[today]
		today++
	}
	e1 := tempReal / period

	// Initialize e2.
	tempReal = e1
	for i := optInTimePeriod - 1; i > 0; i-- {
		e1 = math.FMA(oneMinusK, e1, k*inReal[today])
		today++
		tempReal += e1
	}
	e2 := tempReal / period

	// Initialize e3.
	tempReal = e2
	for i := optInTimePeriod - 1; i > 0; i-- {
		e1 = math.FMA(oneMinusK, e1, k*inReal[today])
		today++
		e2 = math.FMA(oneMinusK, e2, k*e1)
		tempReal += e2
	}
	e3 := tempReal / period

	// Initialize e4.
	tempReal = e3
	for i := optInTimePeriod - 1; i > 0; i-- {
		e1 = math.FMA(oneMinusK, e1, k*inReal[today])
		today++
		e2 = math.FMA(oneMinusK, e2, k*e1)
		e3 = math.FMA(oneMinusK, e3, k*e2)
		tempReal += e3
	}
	e4 := tempReal / period

	// Initialize e5.
	tempReal = e4
	for i := optInTimePeriod - 1; i > 0; i-- {
		e1 = math.FMA(oneMinusK, e1, k*inReal[today])
		today++
		e2 = math.FMA(oneMinusK, e2, k*e1)
		e3 = math.FMA(oneMinusK, e3, k*e2)
		e4 = math.FMA(oneMinusK, e4, k*e3)
		tempReal += e4
	}
	e5 := tempReal / period

	// Initialize e6.
	tempReal = e5
	for i := optInTimePeriod - 1; i > 0; i-- {
		e1 = math.FMA(oneMinusK, e1, k*inReal[today])
		today++
		e2 = math.FMA(oneMinusK, e2, k*e1)
		e3 = math.FMA(oneMinusK, e3, k*e2)
		e4 = math.FMA(oneMinusK, e4, k*e3)
		e5 = math.FMA(oneMinusK, e5, k*e4)
		tempReal += e5
	}
	e6 := tempReal / period

	// Skip the unstable period.
	for today <= startIdx {
		e1 = math.FMA(oneMinusK, e1, k*inReal[today])
		today++
		e2 = math.FMA(oneMinusK, e2, k*e1)
		e3 = math.FMA(oneMinusK, e3, k*e2)
		e4 = math.FMA(oneMinusK, e4, k*e3)
		e5 = math.FMA(oneMinusK, e5, k*e4)
		e6 = math.FMA(oneMinusK, e6, k*e5)
	}

	// Calculate the constants.
	tempReal = float64(optInVFactor * optInVFactor)
	c1 := 0 - float64(tempReal*optInVFactor)
	c2 := float64(3.0 * (tempReal - c1))
	c3 := float64((0-6.0)*tempReal) - float64(3.0*(optInVFactor-c1))
	c4 := math.FMA(3.0, tempReal, math.FMA(3.0, optInVFactor, 1.0)-c1)

	// Write the first output.
	outIdx := 0
	outReal[outIdx] = math.FMA(c4, e3, math.FMA(c3, e4, math.FMA(c1, e6, c2*e5)))
	outIdx++

	// Calculate and output the remaining of the range.
	for today <= endIdx {
		e1 = math.FMA(oneMinusK, e1, k*inReal[today])
		today++
		e2 = math.FMA(oneMinusK, e2, k*e1)
		e3 = math.FMA(oneMinusK, e3, k*e2)
		e4 = math.FMA(oneMinusK, e4, k*e3)
		e5 = math.FMA(oneMinusK, e5, k*e4)
		e6 = math.FMA(oneMinusK, e6, k*e5)
		outReal[outIdx] = math.FMA(c4, e3, math.FMA(c3, e4, math.FMA(c1, e6, c2*e5)))
		outIdx++
	}
	return startIdx, outIdx, nil
}
