package ta

// MOM — Momentum: price - prevPrice (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/mom and src/ta_func/ta_MOM.c.

// MOMLookback returns the number of leading bars MOM leaves empty.
// optInTimePeriod: [1, 100000], default 10.
func MOMLookback(optInTimePeriod int) int {
	if checkInt("MOM", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return momLookback(optInTimePeriod)
}

func momLookback(optInTimePeriod int) int {
	return optInTimePeriod
}

// MOM returns inReal[i] - inReal[i-optInTimePeriod].
// The output is index-aligned with inReal; the first MOMLookback bars are NaN.
// optInTimePeriod: [1, 100000], default 10.
func MOM(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("MOM", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("MOM", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := momRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// momRange mirrors TA_MOM: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func momRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("MOM", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MOM", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MOM", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
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
		outReal[outIdx] = inReal[inIdx] - inReal[trailingIdx]
		outIdx++
		inIdx++
		trailingIdx++
	}
	return startIdx, outIdx, nil
}
