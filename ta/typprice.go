package ta

// TYPPRICE — Typical Price (Price Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/typprice and src/ta_func/ta_TYPPRICE.c.

// TYPPRICELookback returns the number of leading bars TYPPRICE leaves empty
// (always 0). TYPPRICE has no optional inputs, so it never returns -1.
func TYPPRICELookback() int {
	return typpriceLookback()
}

func typpriceLookback() int {
	return 0
}

// TYPPRICE returns the typical price (high + low + close) / 3 of each bar.
// The output is index-aligned with the inputs; there is no lookback.
func TYPPRICE(inHigh, inLow, inClose []float64) ([]float64, error) {
	n, err := checkInputs("TYPPRICE", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := typpriceRange(0, n-1, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// typpriceRange mirrors TA_TYPPRICE: it computes [startIdx, endIdx] and
// writes outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may alias any of the inputs.
func typpriceRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TYPPRICE", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TYPPRICE", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// Typical price = (High + Low + Close) / 3
	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = (inHigh[i] + inLow[i] + inClose[i]) / 3.0
		outIdx++
	}
	return startIdx, outIdx, nil
}
