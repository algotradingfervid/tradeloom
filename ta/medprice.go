package ta

// MEDPRICE — Median Price (Price Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/medprice and src/ta_func/ta_MEDPRICE.c.

// MEDPRICELookback returns the number of leading bars MEDPRICE leaves empty
// (always 0). MEDPRICE has no optional inputs, so it never returns -1.
func MEDPRICELookback() int {
	return medpriceLookback()
}

func medpriceLookback() int {
	return 0
}

// MEDPRICE returns the median price (high + low) / 2 of each bar.
// The output is index-aligned with the inputs; there is no lookback.
func MEDPRICE(inHigh, inLow []float64) ([]float64, error) {
	n, err := checkInputs("MEDPRICE", []string{"inHigh", "inLow"}, inHigh, inLow)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := medpriceRange(0, n-1, inHigh, inLow, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// medpriceRange mirrors TA_MEDPRICE: it computes [startIdx, endIdx] and
// writes outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may alias either input.
func medpriceRange(startIdx, endIdx int, inHigh, inLow []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MEDPRICE", endIdx, []string{"inHigh", "inLow"}, inHigh, inLow); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MEDPRICE", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// MEDPRICE = (High + Low) / 2 of the same price bar.
	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = (inHigh[i] + inLow[i]) / 2.0
		outIdx++
	}
	return startIdx, outIdx, nil
}
