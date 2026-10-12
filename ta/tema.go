package ta

import "math"

// TEMA — Triple Exponential Moving Average (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/tema and src/ta_func/ta_TEMA.c.

// TEMALookback returns the number of leading bars TEMA leaves empty:
// three times the EMA lookback (which includes the UnstEMA unstable period).
// optInTimePeriod: [1, 100000], default 30.
func TEMALookback(optInTimePeriod int) int {
	if checkInt("TEMA", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return temaLookback(optInTimePeriod)
}

func temaLookback(optInTimePeriod int) int {
	return emaLookback(optInTimePeriod) * 3
}

// TEMA returns the triple exponential moving average of inReal:
// 3*EMA1 - 3*EMA2 + EMA3, where EMA2 = EMA(EMA1) and EMA3 = EMA(EMA2), all
// over optInTimePeriod bars. The output is index-aligned with inReal; the
// first TEMALookback bars are NaN. Period 1 returns a copy of the input.
// optInTimePeriod: [1, 100000], default 30.
func TEMA(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("TEMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("TEMA", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := temaRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// temaRange mirrors TA_TEMA: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
//
// TA_TEMA v0.8.1 does not call TA_EMA: it runs the three EMAs inline in one
// lockstep pass, so this port does the same (only TA_EMA_Lookback is used).
func temaRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("TEMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TEMA", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TEMA", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackEMA := emaLookback(optInTimePeriod)
	lookbackTotal := lookbackEMA * 3
	if startIdx < lookbackTotal {
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

	optInK1 := 2.0 / float64(optInTimePeriod+1)

	// Seed EMA1 with the simple average of the first period bars.
	today := startIdx - lookbackTotal
	tempReal := 0.0
	for i := optInTimePeriod; i > 0; i-- {
		tempReal += inReal[today]
		today++
	}
	prevEMA1 := tempReal / float64(optInTimePeriod)

	// C uses fma() explicitly, so math.FMA reproduces it bit for bit.
	// Advance EMA1 alone up to the bar where EMA2 seeding begins.
	for today <= startIdx-lookbackEMA*2 {
		prevEMA1 = math.FMA(inReal[today]-prevEMA1, optInK1, prevEMA1)
		today++
	}

	// Seed EMA2 with the average of the first period EMA1 values.
	tempReal = 0.0
	tempReal += prevEMA1
	for i := optInTimePeriod - 1; i > 0; i-- {
		prevEMA1 = math.FMA(inReal[today]-prevEMA1, optInK1, prevEMA1)
		today++
		tempReal += prevEMA1
	}
	prevEMA2 := tempReal / float64(optInTimePeriod)

	// Advance EMA1 and EMA2 up to the bar where EMA3 seeding begins.
	for today <= startIdx-lookbackEMA {
		prevEMA1 = math.FMA(inReal[today]-prevEMA1, optInK1, prevEMA1)
		today++
		prevEMA2 = math.FMA(prevEMA1-prevEMA2, optInK1, prevEMA2)
	}

	// Seed EMA3 with the average of the first period EMA2 values.
	tempReal = 0.0
	tempReal += prevEMA2
	for i := optInTimePeriod - 1; i > 0; i-- {
		prevEMA1 = math.FMA(inReal[today]-prevEMA1, optInK1, prevEMA1)
		today++
		prevEMA2 = math.FMA(prevEMA1-prevEMA2, optInK1, prevEMA2)
		tempReal += prevEMA2
	}
	prevEMA3 := tempReal / float64(optInTimePeriod)

	// Advance all three EMAs up to the first output bar.
	for today <= startIdx {
		prevEMA1 = math.FMA(inReal[today]-prevEMA1, optInK1, prevEMA1)
		today++
		prevEMA2 = math.FMA(prevEMA1-prevEMA2, optInK1, prevEMA2)
		prevEMA3 = math.FMA(prevEMA2-prevEMA3, optInK1, prevEMA3)
	}

	outReal[0] = temaCombine(prevEMA1, prevEMA2, prevEMA3)
	outIdx := 1
	for today <= endIdx {
		prevEMA1 = math.FMA(inReal[today]-prevEMA1, optInK1, prevEMA1)
		today++
		prevEMA2 = math.FMA(prevEMA1-prevEMA2, optInK1, prevEMA2)
		prevEMA3 = math.FMA(prevEMA2-prevEMA3, optInK1, prevEMA3)
		outReal[outIdx] = temaCombine(prevEMA1, prevEMA2, prevEMA3)
		outIdx++
	}
	return startIdx, outIdx, nil
}

// temaCombine is C's prevEMA3 + (3.0*prevEMA1 - 3.0*prevEMA2). The C library
// is built with -ffp-contract=off; the explicit float64 conversions stop the
// Go compiler from fusing the products into an FMA (Go spec, "Floating-point
// operators"), so the rounding matches C.
func temaCombine(prevEMA1, prevEMA2, prevEMA3 float64) float64 {
	return prevEMA3 + (float64(3.0*prevEMA1) - float64(3.0*prevEMA2))
}
