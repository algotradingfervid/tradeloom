package ta

// ROCP — Rate of change Percentage: (price-prevPrice)/prevPrice (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/rocp and src/ta_func/ta_ROCP.c.

// ROCPLookback returns the number of leading bars ROCP leaves empty.
// optInTimePeriod: [1, 100000], default 10.
func ROCPLookback(optInTimePeriod int) int {
	if checkInt("ROCP", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return rocpLookback(optInTimePeriod)
}

func rocpLookback(optInTimePeriod int) int {
	return optInTimePeriod
}

// ROCP returns (inReal[i]-inReal[i-optInTimePeriod])/inReal[i-optInTimePeriod].
// When the previous price is exactly 0 the output is 0 (as in TA-Lib).
// The output is index-aligned with inReal; the first ROCPLookback bars are NaN.
// optInTimePeriod: [1, 100000], default 10.
func ROCP(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("ROCP", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("ROCP", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := rocpRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// rocpRange mirrors TA_ROCP: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func rocpRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("ROCP", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ROCP", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ROCP", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
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
			outReal[outIdx] = (inReal[inIdx] - tempReal) / tempReal
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++
		inIdx++
	}
	return startIdx, outIdx, nil
}
