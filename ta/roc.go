package ta

// ROC — Rate of change: ((price/prevPrice)-1)*100 (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/roc and src/ta_func/ta_ROC.c.

// ROCLookback returns the number of leading bars ROC leaves empty.
// optInTimePeriod: [1, 100000], default 10.
func ROCLookback(optInTimePeriod int) int {
	if checkInt("ROC", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return rocLookback(optInTimePeriod)
}

func rocLookback(optInTimePeriod int) int {
	return optInTimePeriod
}

// ROC returns ((inReal[i]/inReal[i-optInTimePeriod])-1)*100.
// When the previous price is exactly 0 the output is 0 (as in TA-Lib).
// The output is index-aligned with inReal; the first ROCLookback bars are NaN.
// optInTimePeriod: [1, 100000], default 10.
func ROC(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("ROC", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("ROC", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := rocRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// rocRange mirrors TA_ROC: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func rocRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("ROC", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ROC", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ROC", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	if startIdx < optInTimePeriod {
		startIdx = optInTimePeriod
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	outIdx := 0
	inIdx := startIdx
	trailingIdx := startIdx - optInTimePeriod
	for inIdx <= endIdx {
		tempReal := inReal[trailingIdx]
		trailingIdx++
		if tempReal != 0.0 {
			outReal[outIdx] = (inReal[inIdx]/tempReal - 1.0) * 100.0
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++
		inIdx++
	}
	return startIdx, outIdx, nil
}
