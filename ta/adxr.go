package ta

// ADXR — Average Directional Movement Index Rating (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/adxr and src/ta_func/ta_ADXR.c.

// ADXRLookback returns the number of leading bars ADXR leaves empty:
// optInTimePeriod - 1 plus ADXLookback (which includes the ADX unstable period).
// optInTimePeriod: [2, 100000], default 14.
func ADXRLookback(optInTimePeriod int) int {
	if checkInt("ADXR", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return adxrLookback(optInTimePeriod)
}

func adxrLookback(optInTimePeriod int) int {
	if optInTimePeriod > 1 {
		return optInTimePeriod + adxLookback(optInTimePeriod) - 1
	}
	return 3
}

// ADXR returns the average of the current ADX and the ADX optInTimePeriod-1
// bars earlier, of (inHigh, inLow, inClose). The output is index-aligned with
// the inputs; the first ADXRLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 14.
func ADXR(inHigh, inLow, inClose []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("ADXR", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("ADXR", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := adxrRange(0, n-1, inHigh, inLow, inClose, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// adxrRange mirrors TA_ADXR: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may be one of the input slices (ADX goes to a
// temporary buffer first).
func adxrRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("ADXR", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ADXR", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ADXR", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookback := adxrLookback(optInTimePeriod)
	if startIdx < lookback {
		startIdx = lookback
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	adx := make([]float64, endIdx-startIdx+optInTimePeriod)
	// ADX over a range starting (period-1) bars earlier, so each ADXR bar can
	// pair the current ADX with the one (period-1) bars ago.
	_, adxNB, err := adxRange(startIdx-(optInTimePeriod-1), endIdx, inHigh, inLow, inClose, optInTimePeriod, adx)
	if err != nil {
		return 0, 0, err
	}

	nbElement := adxNB - (optInTimePeriod - 1)
	for outIdx := 0; outIdx < nbElement; outIdx++ {
		outReal[outIdx] = (adx[outIdx+(optInTimePeriod-1)] + adx[outIdx]) / 2.0
	}
	return startIdx, nbElement, nil
}
