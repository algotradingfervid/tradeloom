package ta

// ROCR100 — Rate of change ratio 100 scale: (price/prevPrice)*100 (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/rocr100 and src/ta_func/ta_ROCR100.c.

// ROCR100Lookback returns the number of leading bars ROCR100 leaves empty.
// optInTimePeriod: [1, 100000], default 10.
func ROCR100Lookback(optInTimePeriod int) int {
	if checkInt("ROCR100", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return rocr100Lookback(optInTimePeriod)
}

func rocr100Lookback(optInTimePeriod int) int {
	return optInTimePeriod
}

// ROCR100 returns inReal[i] / inReal[i-optInTimePeriod] * 100. As in TA-Lib, a zero
// previous price yields 0 rather than an infinity or NaN.
// The output is index-aligned with inReal; the first ROCR100Lookback bars are NaN.
// optInTimePeriod: [1, 100000], default 10.
func ROCR100(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("ROCR100", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("ROCR100", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := rocr100Range(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// rocr100Range mirrors TA_ROCR100: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func rocr100Range(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("ROCR100", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ROCR100", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ROCR100", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// Move up the start index if there is not enough initial data.
	if startIdx < optInTimePeriod {
		startIdx = optInTimePeriod
	}
	// Make sure there is still something to evaluate.
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// Calculate Rate of change Ratio 100 scale: (price / prevPrice) * 100.
	outIdx := 0
	inIdx := startIdx
	trailingIdx := startIdx - optInTimePeriod
	for inIdx <= endIdx {
		tempReal := inReal[trailingIdx]
		trailingIdx++
		if tempReal != 0.0 {
			outReal[outIdx] = inReal[inIdx] / tempReal * 100.0
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++
		inIdx++
	}
	return startIdx, outIdx, nil
}
