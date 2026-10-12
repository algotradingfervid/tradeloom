package ta

import "math"

// DEMA — Double Exponential Moving Average (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/dema and src/ta_func/ta_DEMA.c.

// DEMALookback returns the number of leading bars DEMA leaves empty:
// twice the EMA lookback (optInTimePeriod-1 plus the EMA unstable period).
// optInTimePeriod: [1, 100000], default 30.
func DEMALookback(optInTimePeriod int) int {
	if checkInt("DEMA", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return demaLookback(optInTimePeriod)
}

func demaLookback(optInTimePeriod int) int {
	return emaLookback(optInTimePeriod) * 2
}

// DEMA returns the double exponential moving average of inReal:
// 2*EMA1 - EMA2 where EMA1 = EMA(inReal) and EMA2 = EMA(EMA1).
// The output is index-aligned with inReal; the first DEMALookback bars are
// NaN. Period 1 returns a copy of the input.
// optInTimePeriod: [1, 100000], default 30.
func DEMA(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("DEMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("DEMA", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := demaRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// demaRange mirrors TA_DEMA: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
//
// Like the compiled C, both EMAs are computed inline in a single lockstep
// pass (TA_DEMA does not call TA_EMA), so emaRange is not used here.
func demaRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("DEMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("DEMA", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("DEMA", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}
	return demaCore(startIdx, endIdx, inReal, optInTimePeriod, outReal)
}

// demaCore is the TA_DEMA body after the validation prologue.
func demaCore(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (int, int, error) {
	lookbackEMA := emaLookback(optInTimePeriod)
	lookbackTotal := lookbackEMA * 2
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// No smoothing at period 1: the output is a copy of the input. The
	// unstable period still delays the first output (at twice EMA's rate).
	if optInTimePeriod == 1 {
		outIdx := 0
		for today := startIdx; today <= endIdx; today++ {
			outReal[outIdx] = inReal[today]
			outIdx++
		}
		return startIdx, outIdx, nil
	}

	optInK1 := 2.0 / float64(optInTimePeriod+1)

	// Seed EMA1 with the simple average of the first period bars.
	today := startIdx - lookbackTotal
	tempReal := 0.0
	for i := optInTimePeriod; i > 0; i-- {
		tempReal += inReal[today]
		today++
	}
	prevEMA1 := tempReal / float64(optInTimePeriod)

	// Advance EMA1 alone up to the bar where EMA2 seeding begins.
	// C uses fma() explicitly, so math.FMA reproduces it bit for bit.
	for today <= startIdx-lookbackEMA {
		prevEMA1 = math.FMA(inReal[today]-prevEMA1, optInK1, prevEMA1)
		today++
	}

	// Seed EMA2 with the simple average of the first period EMA1 values.
	tempReal = 0.0
	tempReal += prevEMA1
	for i := optInTimePeriod - 1; i > 0; i-- {
		prevEMA1 = math.FMA(inReal[today]-prevEMA1, optInK1, prevEMA1)
		today++
		tempReal += prevEMA1
	}
	prevEMA2 := tempReal / float64(optInTimePeriod)

	// Advance both EMAs in lockstep up to the first output bar.
	for today <= startIdx {
		prevEMA1 = math.FMA(inReal[today]-prevEMA1, optInK1, prevEMA1)
		today++
		prevEMA2 = math.FMA(prevEMA1-prevEMA2, optInK1, prevEMA2)
	}

	// 2.0*prevEMA1 is exact, so a fused or unfused subtraction gives the
	// same result.
	outReal[0] = 2.0*prevEMA1 - prevEMA2
	outIdx := 1
	for today <= endIdx {
		prevEMA1 = math.FMA(inReal[today]-prevEMA1, optInK1, prevEMA1)
		today++
		prevEMA2 = math.FMA(prevEMA1-prevEMA2, optInK1, prevEMA2)
		outReal[outIdx] = 2.0*prevEMA1 - prevEMA2
		outIdx++
	}
	return startIdx, outIdx, nil
}
