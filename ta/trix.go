package ta

import "math"

// TRIX — 1-day Rate-Of-Change (ROC) of a Triple Smooth EMA (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/trix and src/ta_func/ta_TRIX.c.

// TRIXLookback returns the number of leading bars TRIX leaves empty:
// 3*EMALookback(optInTimePeriod) + 1 (the EMA unstable period counts three
// times). optInTimePeriod: [1, 100000], default 30.
func TRIXLookback(optInTimePeriod int) int {
	if checkInt("TRIX", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return trixLookback(optInTimePeriod)
}

func trixLookback(optInTimePeriod int) int {
	return emaLookback(optInTimePeriod)*3 + rocrLookback(1)
}

// TRIX returns the 1-day percent rate of change of a triple-smoothed EMA of
// inReal: 100*(E3[t]/E3[t-1]-1), where E3 = EMA(EMA(EMA(inReal))). As in
// TA-Lib, a zero previous E3 yields 0. The output is index-aligned with
// inReal; the first TRIXLookback bars are NaN.
// optInTimePeriod: [1, 100000], default 30.
func TRIX(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("TRIX", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("TRIX", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := trixRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// trixRange mirrors TA_TRIX: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
//
// TA-Lib v0.8.1 does not call TA_EMA/TA_ROCR here: it runs the three EMAs
// and the ROC in one lockstep pass (fix #98), so this port does the same
// rather than chaining emaRange/rocrRange; the arithmetic order is kept
// exactly (C uses fma(), reproduced bit for bit by math.FMA).
func trixRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("TRIX", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TRIX", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TRIX", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackEMA := emaLookback(optInTimePeriod)
	lookbackTotal := lookbackEMA*3 + rocrLookback(1)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	optInK1 := 2.0 / float64(optInTimePeriod+1)
	period := float64(optInTimePeriod)

	// Seed EMA1 with a simple average of the first 'period' bars.
	today := startIdx - lookbackTotal
	tempReal := 0.0
	for i := optInTimePeriod; i > 0; i-- {
		tempReal += inReal[today]
		today++
	}
	prevEMA1 := tempReal / period
	// Advance EMA1 alone up to the bar where EMA2 seeding begins.
	for today <= startIdx-(lookbackEMA*2+1) {
		prevEMA1 = math.FMA(inReal[today]-prevEMA1, optInK1, prevEMA1)
		today++
	}
	// Seed EMA2 with the average of the first 'period' EMA1 values.
	tempReal = 0.0
	tempReal += prevEMA1
	for i := optInTimePeriod - 1; i > 0; i-- {
		prevEMA1 = math.FMA(inReal[today]-prevEMA1, optInK1, prevEMA1)
		today++
		tempReal += prevEMA1
	}
	prevEMA2 := tempReal / period
	// Advance EMA1 and EMA2 up to the bar where EMA3 seeding begins.
	for today <= startIdx-(lookbackEMA+1) {
		prevEMA1 = math.FMA(inReal[today]-prevEMA1, optInK1, prevEMA1)
		today++
		prevEMA2 = math.FMA(prevEMA1-prevEMA2, optInK1, prevEMA2)
	}
	// Seed EMA3 with the average of the first 'period' EMA2 values.
	tempReal = 0.0
	tempReal += prevEMA2
	for i := optInTimePeriod - 1; i > 0; i-- {
		prevEMA1 = math.FMA(inReal[today]-prevEMA1, optInK1, prevEMA1)
		today++
		prevEMA2 = math.FMA(prevEMA1-prevEMA2, optInK1, prevEMA2)
		tempReal += prevEMA2
	}
	prevEMA3 := tempReal / period
	// Advance all three up to the bar before the first output.
	for today <= startIdx-1 {
		prevEMA1 = math.FMA(inReal[today]-prevEMA1, optInK1, prevEMA1)
		today++
		prevEMA2 = math.FMA(prevEMA1-prevEMA2, optInK1, prevEMA2)
		prevEMA3 = math.FMA(prevEMA2-prevEMA3, optInK1, prevEMA3)
	}
	// Stable zone: 1-day rate of change of EMA3.
	outIdx := 0
	for today <= endIdx {
		tempReal = prevEMA3
		prevEMA1 = math.FMA(inReal[today]-prevEMA1, optInK1, prevEMA1)
		today++
		prevEMA2 = math.FMA(prevEMA1-prevEMA2, optInK1, prevEMA2)
		prevEMA3 = math.FMA(prevEMA2-prevEMA3, optInK1, prevEMA3)
		if tempReal != 0.0 {
			outReal[outIdx] = (prevEMA3/tempReal - 1.0) * 100.0
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++
	}
	return startIdx, outIdx, nil
}
