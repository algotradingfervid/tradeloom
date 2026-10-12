package ta

// ASIN — Vector Trigonometric ASin (Math Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/asin and src/ta_func/ta_ASIN.c.

// ASINLookback returns the number of leading bars ASIN leaves empty (always
// 0). ASIN has no optional inputs, so it never returns -1.
func ASINLookback() int {
	return asinLookback()
}

func asinLookback() int {
	return 0
}

// ASIN returns the arc sine, in radians, of each element of inReal.
// Elements outside [-1, 1] (and NaN) produce NaN, as in C. The output is
// index-aligned with inReal; there is no lookback.
func ASIN(inReal []float64) ([]float64, error) {
	n, err := checkInputs("ASIN", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := asinRange(0, n-1, inReal, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// asinRange mirrors TA_ASIN: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func asinRange(startIdx, endIdx int, inReal []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ASIN", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ASIN", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = libmAsin(inReal[i])
		outIdx++
	}
	return startIdx, outIdx, nil
}
