package ta

import "math"

// ATR — Average True Range (Volatility Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/atr and src/ta_func/ta_ATR.c.

// ATRLookback returns the number of leading bars ATR leaves empty: the
// period plus the ATR unstable period. optInTimePeriod: [1, 100000], default 14.
func ATRLookback(optInTimePeriod int) int {
	if checkInt("ATR", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return atrLookback(optInTimePeriod)
}

func atrLookback(optInTimePeriod int) int {
	return optInTimePeriod + UnstablePeriod(UnstATR)
}

// ATR returns Wilder's Average True Range of (inHigh, inLow, inClose) over
// optInTimePeriod bars. The output is index-aligned with the inputs; the
// first ATRLookback bars are NaN. optInTimePeriod: [1, 100000], default 14.
func ATR(inHigh, inLow, inClose []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("ATR", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("ATR", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := atrRange(0, n-1, inHigh, inLow, inClose, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// atrRange mirrors TA_ATR: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may be one of the input slices.
func atrRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("ATR", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ATR", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ATR", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	unstable := UnstablePeriod(UnstATR)
	lookbackTotal := optInTimePeriod + unstable
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// wAlpha is derived from wBeta (not 1/period) so wAlpha+wBeta == 1 exactly.
	wBeta := float64(optInTimePeriod-1) / float64(optInTimePeriod)
	wAlpha := 1.0 - wBeta

	today := startIdx - lookbackTotal + 1

	// Seed: simple average of the first optInTimePeriod true ranges.
	periodTotal := 0.0
	for i := optInTimePeriod; i > 0; i-- {
		periodTotal += trueRange(inHigh[today], inLow[today], inClose[today-1])
		today++
	}
	prevATR := periodTotal / float64(optInTimePeriod)

	// Skip the unstable period. C uses an explicit fma(); math.FMA matches it bit for bit.
	for i := unstable; i != 0; i-- {
		greatest := trueRange(inHigh[today], inLow[today], inClose[today-1])
		prevATR = math.FMA(wBeta, prevATR, wAlpha*greatest)
		today++
	}

	outIdx := 1
	outReal[0] = prevATR
	for nbATR := endIdx - startIdx + 1; nbATR > 1; nbATR-- {
		greatest := trueRange(inHigh[today], inLow[today], inClose[today-1])
		prevATR = math.FMA(wBeta, prevATR, wAlpha*greatest)
		outReal[outIdx] = prevATR
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
