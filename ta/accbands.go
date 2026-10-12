package ta

import "math"

// ACCBANDS — Acceleration Bands (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/accbands and src/ta_func/ta_ACCBANDS.c.
//
// The compiled C does not call TA_SMA: it fuses the three simple moving
// averages into one loop over a shared trailing window, so this port does the
// same (only TA_SMA_Lookback is reused, as smaLookback).

// ACCBANDSLookback returns the number of leading bars ACCBANDS leaves empty.
// optInTimePeriod: [2, 100000], default 20.
func ACCBANDSLookback(optInTimePeriod int) int {
	if checkInt("ACCBANDS", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return accbandsLookback(optInTimePeriod)
}

func accbandsLookback(optInTimePeriod int) int {
	return smaLookback(optInTimePeriod)
}

// ACCBANDS returns the upper, middle and lower acceleration bands:
//
//	factor = 4*(H-L)/(H+L)
//	upper  = SMA(H*(1+factor), N), middle = SMA(C, N), lower = SMA(L*(1-factor), N)
//
// When H+L cancels to zero relative to |H|+|L|, the bar contributes H and L
// unscaled. Outputs are index-aligned with the inputs; the first
// ACCBANDSLookback bars are NaN. optInTimePeriod: [2, 100000], default 20.
func ACCBANDS(inHigh, inLow, inClose []float64, optInTimePeriod int) (outRealUpperBand, outRealMiddleBand, outRealLowerBand []float64, err error) {
	if err := checkInt("ACCBANDS", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, nil, nil, err
	}
	n, err := checkInputs("ACCBANDS", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, nil, nil, err
	}
	upper := make([]float64, n)
	middle := make([]float64, n)
	lower := make([]float64, n)
	if n == 0 {
		return upper, middle, lower, nil
	}
	beg, nb, err := accbandsRange(0, n-1, inHigh, inLow, inClose, optInTimePeriod, upper, middle, lower)
	if err != nil {
		return nil, nil, nil, err
	}
	alignReal(upper, beg, nb)
	alignReal(middle, beg, nb)
	alignReal(lower, beg, nb)
	return upper, middle, lower, nil
}

// accbandsSameBuf reports whether two buffers start at the same address, the
// Go analogue of C's output pointer equality test.
func accbandsSameBuf(a, b []float64) bool {
	return len(a) > 0 && len(b) > 0 && &a[0] == &b[0]
}

// accbandsRange mirrors TA_ACCBANDS: it computes [startIdx, endIdx] and
// writes outNBElement values to each output from index 0, the first one
// belonging to bar outBegIdx. The three outputs must be distinct buffers (as
// in C); an output may be the same slice as an input.
func accbandsRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInTimePeriod int,
	outRealUpperBand, outRealMiddleBand, outRealLowerBand []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("ACCBANDS", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ACCBANDS", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ACCBANDS", endIdx-startIdx, []string{"outRealUpperBand", "outRealMiddleBand", "outRealLowerBand"},
		outRealUpperBand, outRealMiddleBand, outRealLowerBand); err != nil {
		return 0, 0, err
	}
	if accbandsSameBuf(outRealUpperBand, outRealMiddleBand) || accbandsSameBuf(outRealUpperBand, outRealLowerBand) ||
		accbandsSameBuf(outRealMiddleBand, outRealLowerBand) {
		return 0, 0, badParam("ACCBANDS", "outRealUpperBand", "output buffers must be distinct")
	}

	lookbackTotal := smaLookback(optInTimePeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	periodTotalUpper := 0.0
	periodTotalMiddle := 0.0
	periodTotalLower := 0.0
	trailingIdx := startIdx - lookbackTotal
	i := trailingIdx
	for i < startIdx {
		tempReal := inHigh[i] + inLow[i]
		if !isZeroScaled(tempReal, math.Abs(inHigh[i])+math.Abs(inLow[i])) {
			tempReal = 4 * (inHigh[i] - inLow[i]) / tempReal
			periodTotalUpper += float64(inHigh[i] * (1 + tempReal))
			periodTotalLower += float64(inLow[i] * (1 - tempReal))
		} else {
			periodTotalUpper += inHigh[i]
			periodTotalLower += inLow[i]
		}
		periodTotalMiddle += inClose[i]
		i++
	}

	outIdx := 0
	for i <= endIdx {
		tempReal := inHigh[i] + inLow[i]
		if !isZeroScaled(tempReal, math.Abs(inHigh[i])+math.Abs(inLow[i])) {
			tempReal = 4 * (inHigh[i] - inLow[i]) / tempReal
			periodTotalUpper += float64(inHigh[i] * (1 + tempReal))
			periodTotalLower += float64(inLow[i] * (1 - tempReal))
		} else {
			periodTotalUpper += inHigh[i]
			periodTotalLower += inLow[i]
		}
		periodTotalMiddle += inClose[i]
		i++
		tempUpper := periodTotalUpper
		tempMiddle := periodTotalMiddle
		tempLower := periodTotalLower
		tempReal = inHigh[trailingIdx] + inLow[trailingIdx]
		if !isZeroScaled(tempReal, math.Abs(inHigh[trailingIdx])+math.Abs(inLow[trailingIdx])) {
			tempReal = 4 * (inHigh[trailingIdx] - inLow[trailingIdx]) / tempReal
			periodTotalUpper -= float64(inHigh[trailingIdx] * (1 + tempReal))
			periodTotalLower -= float64(inLow[trailingIdx] * (1 - tempReal))
		} else {
			periodTotalUpper -= inHigh[trailingIdx]
			periodTotalLower -= inLow[trailingIdx]
		}
		periodTotalMiddle -= inClose[trailingIdx]
		trailingIdx++
		outRealUpperBand[outIdx] = tempUpper / float64(optInTimePeriod)
		outRealMiddleBand[outIdx] = tempMiddle / float64(optInTimePeriod)
		outRealLowerBand[outIdx] = tempLower / float64(optInTimePeriod)
		outIdx++
	}
	return startIdx, outIdx, nil
}
