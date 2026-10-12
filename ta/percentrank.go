package ta

// PERCENTRANK — Percent Rank (Statistic Functions).
// Ported from TA-Lib v0.8.1 ta_codegen/input/percentrank and
// src/ta_func/ta_PERCENTRANK.c.

// PERCENTRANKLookback returns the number of leading bars PERCENTRANK leaves
// empty, or -1 for an invalid parameter.
// optInTimePeriod: [2, 100000], default 100.
func PERCENTRANKLookback(optInTimePeriod int) int {
	if checkInt("PERCENTRANK", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return percentrankLookback(optInTimePeriod)
}

func percentrankLookback(optInTimePeriod int) int {
	return optInTimePeriod
}

// PERCENTRANK returns, for each bar, the percentage of the optInTimePeriod
// preceding values that are strictly below the current value (0 to 100).
// The output is index-aligned with inReal; the first PERCENTRANKLookback bars
// are NaN. optInTimePeriod: [2, 100000], default 100.
func PERCENTRANK(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("PERCENTRANK", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("PERCENTRANK", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := percentrankRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// percentrankRange mirrors TA_PERCENTRANK: it computes [startIdx, endIdx] and
// writes outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx.
func percentrankRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("PERCENTRANK", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("PERCENTRANK", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("PERCENTRANK", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := percentrankLookback(optInTimePeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	// Make sure there is still something to evaluate.
	if startIdx > endIdx {
		return 0, 0, nil
	}
	outIdx := 0
	today := startIdx
	for today <= endIdx {
		current := inReal[today]
		count := 0
		for i := optInTimePeriod; i >= 1; i-- {
			if inReal[today-i] < current {
				count++
			}
		}
		// Divide, then scale, as C does: (count/N)*100 and 100*count/N
		// round differently.
		outReal[outIdx] = float64(count) / float64(optInTimePeriod) * 100.0
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
