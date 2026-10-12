package ta

// PLUS_DI — Plus Directional Indicator (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/plus_di and src/ta_func/ta_PLUS_DI.c.

// PLUSDILookback returns the number of leading bars PLUSDI leaves empty:
// 1 for period 1, otherwise period plus the PLUS_DI unstable period.
// optInTimePeriod: [1, 100000], default 14.
func PLUSDILookback(optInTimePeriod int) int {
	if checkInt("PLUS_DI", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return plusdiLookback(optInTimePeriod)
}

func plusdiLookback(optInTimePeriod int) int {
	if optInTimePeriod > 1 {
		return optInTimePeriod + UnstablePeriod(UnstPlusDI)
	}
	return 1
}

// PLUSDI returns Wilder's plus directional indicator, 100 * +DM / TR with
// both terms Wilder-smoothed over optInTimePeriod bars. Like TA-Lib, period 1
// returns the unscaled one-bar ratio +DM1 / TR1 (not multiplied by 100).
// The output is index-aligned with the inputs; the first PLUSDILookback bars
// are NaN. optInTimePeriod: [1, 100000], default 14.
func PLUSDI(inHigh, inLow, inClose []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("PLUS_DI", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("PLUS_DI", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := plusdiRange(0, n-1, inHigh, inLow, inClose, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// plusdiRange mirrors TA_PLUS_DI: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may be one of the input slices.
func plusdiRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("PLUS_DI", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("PLUS_DI", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("PLUS_DI", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	unstable := UnstablePeriod(UnstPlusDI)
	var lookbackTotal int
	if optInTimePeriod > 1 {
		lookbackTotal = optInTimePeriod + unstable
	} else {
		lookbackTotal = 1
	}
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	outIdx := 0
	var today int
	var prevHigh, prevLow, prevClose, tempReal, diffP, diffM float64

	// No smoothing: +DI1 = +DM1 / TR1 (unscaled, as in C).
	if optInTimePeriod <= 1 {
		today = startIdx - 1
		prevHigh = inHigh[today]
		prevLow = inLow[today]
		prevClose = inClose[today]
		for today < endIdx {
			today++
			tempReal = inHigh[today]
			diffP = tempReal - prevHigh
			prevHigh = tempReal
			tempReal = inLow[today]
			diffM = prevLow - tempReal
			prevLow = tempReal
			if diffP > 0 && diffP > diffM {
				tempReal = trueRange(prevHigh, prevLow, prevClose)
				if tempReal <= 0.0 {
					outReal[outIdx] = 0.0
				} else {
					outReal[outIdx] = diffP / tempReal
				}
			} else {
				outReal[outIdx] = 0.0
			}
			outIdx++
			prevClose = inClose[today]
		}
		return startIdx, outIdx, nil
	}

	period := float64(optInTimePeriod)
	prevPlusDM := 0.0
	prevTR := 0.0
	today = startIdx - lookbackTotal
	prevHigh = inHigh[today]
	prevLow = inLow[today]
	prevClose = inClose[today]

	// Initial sums of the first period-1 one-bar +DM and TR.
	for i := optInTimePeriod - 1; i > 0; i-- {
		today++
		tempReal = inHigh[today]
		diffP = tempReal - prevHigh
		prevHigh = tempReal
		tempReal = inLow[today]
		diffM = prevLow - tempReal
		prevLow = tempReal
		if diffP > 0 && diffP > diffM {
			prevPlusDM += diffP
		}
		tempReal = trueRange(prevHigh, prevLow, prevClose)
		prevTR += tempReal
		prevClose = inClose[today]
	}

	// Skip the unstable period; runs at least once to produce the first DI.
	for i := unstable + 1; i != 0; i-- {
		today++
		tempReal = inHigh[today]
		diffP = tempReal - prevHigh
		prevHigh = tempReal
		tempReal = inLow[today]
		diffM = prevLow - tempReal
		prevLow = tempReal
		if diffP > 0 && diffP > diffM {
			prevPlusDM = prevPlusDM - prevPlusDM/period + diffP
		} else {
			prevPlusDM = prevPlusDM - prevPlusDM/period
		}
		tempReal = trueRange(prevHigh, prevLow, prevClose)
		prevTR = prevTR - prevTR/period + tempReal
		prevClose = inClose[today]
	}

	if prevTR > 0.0 {
		outReal[0] = 100.0 * (prevPlusDM / prevTR)
	} else {
		outReal[0] = 0.0
	}
	outIdx = 1
	for today < endIdx {
		today++
		tempReal = inHigh[today]
		diffP = tempReal - prevHigh
		prevHigh = tempReal
		tempReal = inLow[today]
		diffM = prevLow - tempReal
		prevLow = tempReal
		if diffP > 0 && diffP > diffM {
			prevPlusDM = prevPlusDM - prevPlusDM/period + diffP
		} else {
			prevPlusDM = prevPlusDM - prevPlusDM/period
		}
		tempReal = trueRange(prevHigh, prevLow, prevClose)
		prevTR = prevTR - prevTR/period + tempReal
		prevClose = inClose[today]
		if prevTR > 0.0 {
			outReal[outIdx] = 100.0 * (prevPlusDM / prevTR)
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++
	}
	return startIdx, outIdx, nil
}
