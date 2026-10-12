package ta

import "math"

// WCLPRICE — Weighted Close Price (Price Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/wclprice and src/ta_func/ta_WCLPRICE.c.

// WCLPRICELookback returns the number of leading bars WCLPRICE leaves empty
// (always 0). WCLPRICE has no optional inputs, so it never returns -1.
func WCLPRICELookback() int {
	return wclpriceLookback()
}

func wclpriceLookback() int {
	return 0
}

// WCLPRICE returns the weighted close price (high + low + 2*close) / 4 of
// each bar. The output is index-aligned with the inputs; there is no lookback.
func WCLPRICE(inHigh, inLow, inClose []float64) ([]float64, error) {
	n, err := checkInputs("WCLPRICE", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := wclpriceRange(0, n-1, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// wclpriceRange mirrors TA_WCLPRICE: it computes [startIdx, endIdx] and
// writes outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may alias any of the inputs.
func wclpriceRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("WCLPRICE", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("WCLPRICE", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// Weighted Close Price = (High + Low + (Close*2) ) / 4
	// C uses fma(close, 2.0, high + low); math.FMA matches it bit for bit.
	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = math.FMA(inClose[i], 2.0, inHigh[i]+inLow[i]) / 4.0
		outIdx++
	}
	return startIdx, outIdx, nil
}
