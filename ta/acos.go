package ta

// ACOS — Vector Trigonometric ACos (Math Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/acos and src/ta_func/ta_ACOS.c.

// ACOSLookback returns the number of leading bars ACOS leaves empty (always
// 0). ACOS has no optional inputs, so it never returns -1.
func ACOSLookback() int {
	return acosLookback()
}

func acosLookback() int {
	return 0
}

// ACOS returns the arc cosine, in radians, of each element of inReal.
// Elements outside [-1, 1] (and NaN) produce NaN, as in C. The output is
// index-aligned with inReal; there is no lookback.
func ACOS(inReal []float64) ([]float64, error) {
	n, err := checkInputs("ACOS", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := acosRange(0, n-1, inReal, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// acosRange mirrors TA_ACOS: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func acosRange(startIdx, endIdx int, inReal []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ACOS", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ACOS", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = libmAcos(inReal[i])
		outIdx++
	}
	return startIdx, outIdx, nil
}
