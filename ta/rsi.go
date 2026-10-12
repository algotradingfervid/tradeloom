package ta

// RSI — Relative Strength Index (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/rsi and src/ta_func/ta_RSI.c.

// RSILookback returns the number of leading bars RSI leaves empty:
// optInTimePeriod plus the RSI unstable period.
// optInTimePeriod: [2, 100000], default 14.
func RSILookback(optInTimePeriod int) int {
	if checkInt("RSI", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return rsiLookback(optInTimePeriod)
}

func rsiLookback(optInTimePeriod int) int {
	return optInTimePeriod + UnstablePeriod(UnstRSI)
}

// RSI returns Wilder's Relative Strength Index of inReal over
// optInTimePeriod bars, in [0, 100]. The output is index-aligned with inReal;
// the first RSILookback bars are NaN.
// optInTimePeriod: [2, 100000], default 14.
func RSI(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("RSI", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("RSI", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := rsiRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// rsiRange mirrors TA_RSI: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func rsiRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("RSI", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("RSI", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("RSI", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := rsiLookback(optInTimePeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}
	outIdx := 0

	// Period 1 is rejected by validation; the C keeps this branch, so does
	// the port.
	if optInTimePeriod == 1 {
		i := endIdx - startIdx + 1
		today := startIdx
		for outIdx = 0; outIdx < i; outIdx++ {
			outReal[outIdx] = inReal[today]
			today++
		}
		return startIdx, i, nil
	}

	invPeriod := 1.0 / float64(optInTimePeriod)
	periodM1 := float64(optInTimePeriod - 1)

	// Accumulate Wilder's average gain and loss over the initial period.
	today := startIdx - lookbackTotal
	prevValue := inReal[today]
	prevGain := 0.0
	prevLoss := 0.0
	today++
	for i := optInTimePeriod; i > 0; i-- {
		tempValue1 := inReal[today]
		today++
		tempValue2 := tempValue1 - prevValue
		prevValue = tempValue1
		gainDelta := 0.0
		if tempValue2 > 0.0 {
			gainDelta = tempValue2
		}
		prevGain += gainDelta
		prevLoss += gainDelta - tempValue2
	}
	prevLoss = float64(prevLoss * invPeriod)
	prevGain = float64(prevGain * invPeriod)

	if today > startIdx {
		tempValue1 := prevGain + prevLoss
		if tempValue1 > 0.0 {
			outReal[outIdx] = 100.0 * (prevGain / tempValue1)
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++
	} else {
		// Skip the unstable period: process but do not write.
		for today < startIdx {
			tempValue1 := inReal[today]
			tempValue2 := tempValue1 - prevValue
			prevValue = tempValue1
			prevLoss = float64(prevLoss * periodM1)
			prevGain = float64(prevGain * periodM1)
			gainDelta := 0.0
			if tempValue2 > 0.0 {
				gainDelta = tempValue2
			}
			prevGain += gainDelta
			prevLoss += gainDelta - tempValue2
			prevLoss = float64(prevLoss * invPeriod)
			prevGain = float64(prevGain * invPeriod)
			today++
		}
	}

	for today <= endIdx {
		tempValue1 := inReal[today]
		today++
		tempValue2 := tempValue1 - prevValue
		prevValue = tempValue1
		prevLoss = float64(prevLoss * periodM1)
		prevGain = float64(prevGain * periodM1)
		gainDelta := 0.0
		if tempValue2 > 0.0 {
			gainDelta = tempValue2
		}
		prevGain += gainDelta
		prevLoss += gainDelta - tempValue2
		prevLoss = float64(prevLoss * invPeriod)
		prevGain = float64(prevGain * invPeriod)
		tempValue1 = prevGain + prevLoss
		if tempValue1 > 0.0 {
			outReal[outIdx] = 100.0 * (prevGain / tempValue1)
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++
	}
	return startIdx, outIdx, nil
}
