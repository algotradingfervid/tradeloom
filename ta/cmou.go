package ta

// CMOU — Chande Momentum Oscillator, unsmoothed (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cmou and src/ta_func/ta_CMOU.c.

// CMOULookback returns the number of leading bars CMOU leaves empty.
// optInTimePeriod: [2, 100000], default 14.
func CMOULookback(optInTimePeriod int) int {
	if checkInt("CMOU", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return cmouLookback(optInTimePeriod)
}

// cmouLookback: optInTimePeriod changes need optInTimePeriod+1 prices; there
// is no unstable period.
func cmouLookback(optInTimePeriod int) int {
	return optInTimePeriod
}

// CMOU returns the unsmoothed Chande Momentum Oscillator of inReal:
// 100*(Su-Sd)/(Su+Sd) over the trailing optInTimePeriod price changes, where
// Su and Sd are the plain sums of the up-moves and |down-moves|; 0 for an
// exactly flat window. The output is index-aligned with inReal; the first
// CMOULookback bars are NaN.
// optInTimePeriod: [2, 100000], default 14.
func CMOU(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("CMOU", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("CMOU", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cmouRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// cmouRange mirrors TA_CMOU: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func cmouRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("CMOU", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CMOU", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CMOU", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := cmouLookback(optInTimePeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// First window: the optInTimePeriod changes ending at startIdx.
	// trailingValue caches the oldest price; trailingIdx points one past it.
	today := startIdx - lookbackTotal
	trailingIdx := today + 1
	prevValue := inReal[today]
	trailingValue := prevValue
	upSum := 0.0
	downSum := 0.0
	// Consecutive exactly-zero changes, so an emptied window is recognised
	// exactly (the sums keep add-then-subtract rounding residue).
	nullRun := 0
	for i := 0; i < optInTimePeriod; i++ {
		today++
		tempReal := inReal[today]
		diff := tempReal - prevValue
		prevValue = tempReal
		if diff > 0.0 {
			upSum += diff
		} else if diff < 0.0 {
			downSum -= diff
		}
		if diff == 0.0 {
			nullRun++
		} else {
			nullRun = 0
		}
	}

	// Scale-then-divide, as in C: (100*(Su-Sd))/(Su+Sd).
	outIdx := 0
	sum := upSum + downSum
	if sum > 0.0 {
		outReal[outIdx] = 100.0 * (upSum - downSum) / sum
	} else {
		outReal[outIdx] = 0.0
	}
	outIdx++

	today++
	for today <= endIdx {
		// Drop the oldest change; its older price comes from the cache, so
		// this is in-place safe.
		tempReal := inReal[trailingIdx]
		diff := tempReal - trailingValue
		trailingValue = tempReal
		trailingIdx++
		if diff > 0.0 {
			upSum -= diff
		} else if diff < 0.0 {
			downSum += diff
		}

		// Add the newest change.
		tempReal = inReal[today]
		diff = tempReal - prevValue
		prevValue = tempReal
		if diff > 0.0 {
			upSum += diff
		} else if diff < 0.0 {
			downSum -= diff
		}

		if diff == 0.0 {
			nullRun++
		} else {
			nullRun = 0
		}
		if nullRun >= optInTimePeriod {
			nullRun = optInTimePeriod
			upSum = 0.0
			downSum = 0.0
		}

		sum = upSum + downSum
		if sum > 0.0 {
			outReal[outIdx] = 100.0 * (upSum - downSum) / sum
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
