package ta

// COS — Vector Trigonometric Cos (Math Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cos and src/ta_func/ta_COS.c.

// COSLookback returns the number of leading bars COS leaves empty (always 0).
// COS has no optional inputs, so it never returns -1.
func COSLookback() int {
	return cosLookback()
}

func cosLookback() int {
	return 0
}

// COS returns the cosine of each element of inReal (angles in radians).
// NaN and ±Inf inputs produce NaN, as in C. The output is index-aligned with
// inReal; there is no lookback.
func COS(inReal []float64) ([]float64, error) {
	n, err := checkInputs("COS", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cosRange(0, n-1, inReal, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// cosRange mirrors TA_COS: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func cosRange(startIdx, endIdx int, inReal []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("COS", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("COS", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = libmCos(inReal[i])
		outIdx++
	}
	return startIdx, outIdx, nil
}
