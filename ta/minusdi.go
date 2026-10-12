package ta

// MINUS_DI — Minus Directional Indicator (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/minus_di and src/ta_func/ta_MINUS_DI.c.

// MINUSDILookback returns the number of leading bars MINUSDI leaves empty:
// the period plus the MINUS_DI unstable period, or 1 when the period is 1.
// optInTimePeriod: [1, 100000], default 14.
func MINUSDILookback(optInTimePeriod int) int {
	if checkInt("MINUS_DI", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return minusdiLookback(optInTimePeriod)
}

func minusdiLookback(optInTimePeriod int) int {
	if optInTimePeriod > 1 {
		return optInTimePeriod + UnstablePeriod(UnstMinusDI)
	}
	return 1
}

// MINUSDI returns Wilder's minus directional indicator (-DI) of
// (inHigh, inLow, inClose) over optInTimePeriod bars, in [0, 100]. With
// period 1 it is the unsmoothed ratio -DM1/TR1, in [0, 1] (no ×100, like C).
// The output is index-aligned with the inputs; the first MINUSDILookback bars
// are NaN. optInTimePeriod: [1, 100000], default 14.
func MINUSDI(inHigh, inLow, inClose []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("MINUS_DI", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("MINUS_DI", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := minusdiRange(0, n-1, inHigh, inLow, inClose, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// minusdiRange mirrors TA_MINUS_DI: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may be one of the input slices.
func minusdiRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("MINUS_DI", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MINUS_DI", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MINUS_DI", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	unstable := UnstablePeriod(UnstMinusDI)
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

	var prevHigh, prevLow, prevClose, tempReal, diffP, diffM float64
	outIdx := 0

	// Trap the case where no smoothing is needed: -DI1 = -DM1 / TR1.
	if optInTimePeriod <= 1 {
		outBegIdx = startIdx
		today := startIdx - 1
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
			if diffM > 0 && diffP < diffM {
				// Case 2 and 4: +DM=0, -DM=diffM.
				tempReal = trueRange(prevHigh, prevLow, prevClose)
				if tempReal <= 0.0 {
					outReal[outIdx] = 0.0
				} else {
					outReal[outIdx] = diffM / tempReal
				}
				outIdx++
			} else {
				outReal[outIdx] = 0.0
				outIdx++
			}
			prevClose = inClose[today]
		}
		return outBegIdx, outIdx, nil
	}

	// Process the initial DM and TR.
	outBegIdx = startIdx
	period := float64(optInTimePeriod)
	prevMinusDM := 0.0
	prevTR := 0.0
	today := startIdx - lookbackTotal
	prevHigh = inHigh[today]
	prevLow = inLow[today]
	prevClose = inClose[today]
	for i := optInTimePeriod - 1; i > 0; i-- {
		today++
		tempReal = inHigh[today]
		diffP = tempReal - prevHigh
		prevHigh = tempReal
		tempReal = inLow[today]
		diffM = prevLow - tempReal
		prevLow = tempReal
		if diffM > 0 && diffP < diffM {
			prevMinusDM += diffM
		}
		tempReal = trueRange(prevHigh, prevLow, prevClose)
		prevTR += tempReal
		prevClose = inClose[today]
	}

	// Skip the unstable period; this loop runs at least once to compute the
	// first DI.
	for i := unstable + 1; i != 0; i-- {
		today++
		tempReal = inHigh[today]
		diffP = tempReal - prevHigh
		prevHigh = tempReal
		tempReal = inLow[today]
		diffM = prevLow - tempReal
		prevLow = tempReal
		if diffM > 0 && diffP < diffM {
			prevMinusDM = prevMinusDM - prevMinusDM/period + diffM
		} else {
			prevMinusDM = prevMinusDM - prevMinusDM/period
		}
		tempReal = trueRange(prevHigh, prevLow, prevClose)
		prevTR = prevTR - prevTR/period + tempReal
		prevClose = inClose[today]
	}

	if prevTR > 0.0 {
		outReal[0] = 100.0 * (prevMinusDM / prevTR)
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
		if diffM > 0 && diffP < diffM {
			prevMinusDM = prevMinusDM - prevMinusDM/period + diffM
		} else {
			prevMinusDM = prevMinusDM - prevMinusDM/period
		}
		tempReal = trueRange(prevHigh, prevLow, prevClose)
		prevTR = prevTR - prevTR/period + tempReal
		prevClose = inClose[today]
		if prevTR > 0.0 {
			outReal[outIdx] = 100.0 * (prevMinusDM / prevTR)
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++
	}
	return outBegIdx, outIdx, nil
}
