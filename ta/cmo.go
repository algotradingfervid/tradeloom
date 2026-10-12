package ta

// CMO — Chande Momentum Oscillator (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cmo and src/ta_func/ta_CMO.c.

// CMOLookback returns the number of leading bars CMO leaves empty:
// optInTimePeriod plus the CMO unstable period.
// optInTimePeriod: [2, 100000], default 14.
func CMOLookback(optInTimePeriod int) int {
	if checkInt("CMO", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return cmoLookback(optInTimePeriod)
}

func cmoLookback(optInTimePeriod int) int {
	return optInTimePeriod + UnstablePeriod(UnstCMO)
}

// CMO returns the Chande Momentum Oscillator of inReal over optInTimePeriod
// bars (Wilder-smoothed gains and losses), in [-100, 100]. The output is
// index-aligned with inReal; the first CMOLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 14.
func CMO(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("CMO", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("CMO", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cmoRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// cmoRange mirrors TA_CMO: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func cmoRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("CMO", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CMO", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CMO", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := cmoLookback(optInTimePeriod)
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

	period := float64(optInTimePeriod)
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
		if tempValue2 < 0 {
			prevLoss -= tempValue2
		} else {
			prevGain += tempValue2
		}
	}
	prevLoss /= period
	prevGain /= period

	if today > startIdx {
		tempValue1 := prevGain + prevLoss
		if tempValue1 > 0.0 {
			outReal[outIdx] = 100.0 * ((prevGain - prevLoss) / tempValue1)
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
			if tempValue2 < 0 {
				prevLoss -= tempValue2
			} else {
				prevGain += tempValue2
			}
			prevLoss /= period
			prevGain /= period
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
		if tempValue2 < 0 {
			prevLoss -= tempValue2
		} else {
			prevGain += tempValue2
		}
		prevLoss /= period
		prevGain /= period
		tempValue1 = prevGain + prevLoss
		if tempValue1 > 0.0 {
			outReal[outIdx] = 100.0 * ((prevGain - prevLoss) / tempValue1)
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++
	}
	return startIdx, outIdx, nil
}
