package ta

// PLUS_DM — Plus Directional Movement (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/plus_dm and src/ta_func/ta_PLUS_DM.c.

// PLUSDMLookback returns the number of leading bars PLUSDM leaves empty:
// 1 for period 1, otherwise period-1 plus the PLUS_DM unstable period.
// optInTimePeriod: [1, 100000], default 14.
func PLUSDMLookback(optInTimePeriod int) int {
	if checkInt("PLUS_DM", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return plusdmLookback(optInTimePeriod)
}

func plusdmLookback(optInTimePeriod int) int {
	if optInTimePeriod > 1 {
		return optInTimePeriod + UnstablePeriod(UnstPlusDM) - 1
	}
	return 1
}

// PLUSDM returns Wilder's smoothed plus directional movement of
// (inHigh, inLow) over optInTimePeriod bars (the raw one-bar +DM for period
// 1). The output is index-aligned with the inputs; the first
// PLUSDMLookback bars are NaN. optInTimePeriod: [1, 100000], default 14.
func PLUSDM(inHigh, inLow []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("PLUS_DM", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("PLUS_DM", []string{"inHigh", "inLow"}, inHigh, inLow)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := plusdmRange(0, n-1, inHigh, inLow, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// plusdmRange mirrors TA_PLUS_DM: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may be one of the input slices.
func plusdmRange(startIdx, endIdx int, inHigh, inLow []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("PLUS_DM", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("PLUS_DM", endIdx, []string{"inHigh", "inLow"}, inHigh, inLow); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("PLUS_DM", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	unstable := UnstablePeriod(UnstPlusDM)
	var lookbackTotal int
	if optInTimePeriod > 1 {
		lookbackTotal = optInTimePeriod + unstable - 1
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
	var prevHigh, prevLow, tempReal, diffP, diffM float64

	// No smoothing: the raw one-bar +DM.
	if optInTimePeriod <= 1 {
		today = startIdx - 1
		prevHigh = inHigh[today]
		prevLow = inLow[today]
		for today < endIdx {
			today++
			tempReal = inHigh[today]
			diffP = tempReal - prevHigh // Plus Delta
			prevHigh = tempReal
			tempReal = inLow[today]
			diffM = prevLow - tempReal // Minus Delta
			prevLow = tempReal
			if diffP > 0 && diffP > diffM {
				outReal[outIdx] = diffP
			} else {
				outReal[outIdx] = 0
			}
			outIdx++
		}
		return startIdx, outIdx, nil
	}

	period := float64(optInTimePeriod)

	// Initial sum of the first period-1 one-bar +DM.
	prevPlusDM := 0.0
	today = startIdx - lookbackTotal
	prevHigh = inHigh[today]
	prevLow = inLow[today]
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
	}

	// Skip the unstable period.
	for i := unstable; i != 0; i-- {
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
	}

	outReal[0] = prevPlusDM
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
		outReal[outIdx] = prevPlusDM
		outIdx++
	}
	return startIdx, outIdx, nil
}
