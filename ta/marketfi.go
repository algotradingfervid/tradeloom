package ta

// MARKETFI — Market Facilitation Index (Volume Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/marketfi and src/ta_func/ta_MARKETFI.c.

// MARKETFILookback returns the number of leading bars MARKETFI leaves empty
// (always 0). MARKETFI has no optional inputs, so it never returns -1.
func MARKETFILookback() int {
	return marketfiLookback()
}

func marketfiLookback() int {
	return 0
}

// MARKETFI returns Bill Williams' Market Facilitation Index,
// (high - low) / volume, for each bar. A bar whose volume is exactly 0
// reports 0. The output is index-aligned with the inputs; there is no lookback.
func MARKETFI(inHigh, inLow, inVolume []float64) ([]float64, error) {
	n, err := checkInputs("MARKETFI", []string{"inHigh", "inLow", "inVolume"}, inHigh, inLow, inVolume)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := marketfiRange(0, n-1, inHigh, inLow, inVolume, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// marketfiRange mirrors TA_MARKETFI: it computes [startIdx, endIdx] and
// writes outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may alias any of the inputs.
func marketfiRange(startIdx, endIdx int, inHigh, inLow, inVolume []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MARKETFI", endIdx, []string{"inHigh", "inLow", "inVolume"}, inHigh, inLow, inVolume); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MARKETFI", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// MARKETFI = (High - Low) / Volume. The zero-volume guard is an exact
	// != 0.0 comparison (not TA_IS_ZERO), as in C; -0 also counts as zero
	// and NaN volume falls through to the division.
	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		if inVolume[i] != 0.0 {
			outReal[outIdx] = (inHigh[i] - inLow[i]) / inVolume[i]
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++
	}
	return startIdx, outIdx, nil
}
