package ta

// AVGPRICE — Average Price (Price Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/avgprice and src/ta_func/ta_AVGPRICE.c.

// AVGPRICELookback returns the number of leading bars AVGPRICE leaves empty
// (always 0). AVGPRICE has no optional inputs, so it never returns -1.
func AVGPRICELookback() int {
	return avgpriceLookback()
}

func avgpriceLookback() int {
	return 0
}

// AVGPRICE returns the average price (open + high + low + close) / 4 of each
// bar. The output is index-aligned with the inputs; there is no lookback.
func AVGPRICE(inOpen, inHigh, inLow, inClose []float64) ([]float64, error) {
	n, err := checkInputs("AVGPRICE", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := avgpriceRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// avgpriceRange mirrors TA_AVGPRICE: it computes [startIdx, endIdx] and
// writes outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may alias any of the inputs.
func avgpriceRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("AVGPRICE", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("AVGPRICE", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// Average price = (High + Low + Open + Close) / 4
	// C sums in the order High, Low, Close, Open.
	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = (inHigh[i] + inLow[i] + inClose[i] + inOpen[i]) / 4
		outIdx++
	}
	return startIdx, outIdx, nil
}
