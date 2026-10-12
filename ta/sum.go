package ta

// SUM — Summation (Math Operators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/sum and src/ta_func/ta_SUM.c.

// SUMLookback returns the number of leading bars SUM leaves empty.
// optInTimePeriod: [2, 100000], default 30.
func SUMLookback(optInTimePeriod int) int {
	if checkInt("SUM", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return sumLookback(optInTimePeriod)
}

func sumLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// SUM returns the rolling sum of inReal over optInTimePeriod bars.
// The output is index-aligned with inReal; the first SUMLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 30.
func SUM(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("SUM", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("SUM", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := sumRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// sumRange mirrors TA_SUM: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func sumRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("SUM", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SUM", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SUM", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := optInTimePeriod - 1
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	periodTotal := 0.0
	trailingIdx := startIdx - lookbackTotal
	i := trailingIdx
	if optInTimePeriod > 1 {
		for i < startIdx {
			periodTotal += inReal[i]
			i++
		}
	}

	// C uses do { ... } while (i <= endIdx); here i == startIdx <= endIdx on
	// entry, so the first iteration always runs either way.
	outIdx := 0
	for {
		periodTotal += inReal[i]
		i++
		tempReal := periodTotal
		periodTotal -= inReal[trailingIdx]
		trailingIdx++
		outReal[outIdx] = tempReal
		outIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
