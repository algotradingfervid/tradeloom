package ta

import "math"

// SUPERTREND — SuperTrend (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/supertrend and src/ta_func/ta_SUPERTREND.c.

// SUPERTRENDLookback returns the number of leading bars SUPERTREND leaves
// empty: the ATR lookback (period plus the ATR unstable period), or -1 on an
// invalid parameter. optInTimePeriod: [2, 100000], default 10.
// optInMultiplier: [0, TA_REAL_MAX], default 3.
func SUPERTRENDLookback(optInTimePeriod int, optInMultiplier float64) int {
	if checkInt("SUPERTREND", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	if checkReal("SUPERTREND", "optInMultiplier", optInMultiplier, 0, taRealMax) != nil {
		return -1
	}
	return supertrendLookback(optInTimePeriod, optInMultiplier)
}

// supertrendLookback is exactly the ATR lookback, so SUPERTREND inherits the
// ATR unstable period (TA_FUNC_UNST_ATR), as in C.
func supertrendLookback(optInTimePeriod int, optInMultiplier float64) int {
	_ = optInMultiplier
	return atrLookback(optInTimePeriod)
}

// SUPERTREND returns the SuperTrend line and the trend direction (+1 up,
// riding the lower band; -1 down, riding the upper band) of
// (inHigh, inLow, inClose). Outputs are index-aligned with the inputs; the
// first SUPERTRENDLookback bars are NaN (line) and 0 (trend).
// optInTimePeriod: [2, 100000], default 10. optInMultiplier: [0, TA_REAL_MAX],
// default 3.
func SUPERTREND(inHigh, inLow, inClose []float64, optInTimePeriod int, optInMultiplier float64) ([]float64, []int, error) {
	if err := checkInt("SUPERTREND", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, nil, err
	}
	if err := checkReal("SUPERTREND", "optInMultiplier", optInMultiplier, 0, taRealMax); err != nil {
		return nil, nil, err
	}
	n, err := checkInputs("SUPERTREND", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, nil, err
	}
	outSupertrend := make([]float64, n)
	outTrend := make([]int, n)
	if n == 0 {
		return outSupertrend, outTrend, nil
	}
	beg, nb, err := supertrendRange(0, n-1, inHigh, inLow, inClose, optInTimePeriod, optInMultiplier, outSupertrend, outTrend)
	if err != nil {
		return nil, nil, err
	}
	alignReal(outSupertrend, beg, nb)
	alignInt(outTrend, beg, nb)
	return outSupertrend, outTrend, nil
}

// supertrendRange mirrors TA_SUPERTREND: it computes [startIdx, endIdx] and
// writes outNBElement values to outSupertrend[0:] and outTrend[0:], the first
// one belonging to bar outBegIdx.
//
// The Average True Range is carried inline, as in C (which does not call
// TA_ATR here); its arithmetic order is the same as atrRange's, including the
// explicit fma in the Wilder step.
func supertrendRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInTimePeriod int, optInMultiplier float64, outSupertrend []float64, outTrend []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("SUPERTREND", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkReal("SUPERTREND", "optInMultiplier", optInMultiplier, 0, taRealMax); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SUPERTREND", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SUPERTREND", endIdx-startIdx, []string{"outSupertrend"}, outSupertrend); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("SUPERTREND", endIdx-startIdx, "outTrend", outTrend); err != nil {
		return 0, 0, err
	}

	lookbackTotal := supertrendLookback(optInTimePeriod, optInMultiplier)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

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

	// Skip the ATR unstable period. C uses an explicit fma(); math.FMA matches it.
	for i := lookbackTotal - optInTimePeriod; i != 0; i-- {
		greatest := trueRange(inHigh[today], inLow[today], inClose[today-1])
		prevATR = math.FMA(wBeta, prevATR, wAlpha*greatest)
		today++
	}

	// First bar: unclamped bands, trend seeded up. The float64() conversions
	// force rounding of each product, so Go cannot fuse a multiply-add that
	// the C build (-ffp-contract=off) does not.
	medianPrice := float64((inHigh[startIdx] + inLow[startIdx]) / 2.0)
	band := float64(optInMultiplier * prevATR)
	finalUpper := medianPrice + band
	finalLower := medianPrice - band
	isUptrend := true
	prevClose := inClose[startIdx]
	outSupertrend[0] = finalLower
	outTrend[0] = 1

	outIdx := 1
	for today = startIdx + 1; today <= endIdx; today++ {
		tempLT := inLow[today]
		tempHT := inHigh[today]
		greatest := trueRange(tempHT, tempLT, inClose[today-1])
		prevATR = math.FMA(wBeta, prevATR, wAlpha*greatest)

		medianPrice = float64((tempHT + tempLT) / 2.0)
		band = float64(optInMultiplier * prevATR)
		basicUpper := medianPrice + band
		basicLower := medianPrice - band

		if basicUpper < finalUpper || prevClose > finalUpper {
			finalUpper = basicUpper
		}
		if basicLower > finalLower || prevClose < finalLower {
			finalLower = basicLower
		}

		closeToday := inClose[today]
		if isUptrend {
			if closeToday < finalLower {
				isUptrend = false
			}
		} else if closeToday > finalUpper {
			isUptrend = true
		}

		if isUptrend {
			outSupertrend[outIdx] = finalLower
			outTrend[outIdx] = 1
		} else {
			outSupertrend[outIdx] = finalUpper
			outTrend[outIdx] = -1
		}
		prevClose = closeToday
		outIdx++
	}
	return startIdx, outIdx, nil
}
