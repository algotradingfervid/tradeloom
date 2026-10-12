package ta

// SINH — Vector Trigonometric Sinh (Math Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/sinh and src/ta_func/ta_SINH.c.

// SINHLookback returns the number of leading bars SINH leaves empty (always
// 0). SINH has no optional inputs, so it never returns -1.
func SINHLookback() int {
	return sinhLookback()
}

func sinhLookback() int {
	return 0
}

// SINH returns the hyperbolic sine of each element of inReal. Large
// magnitudes (|x| above about 710.48) overflow to +/-Inf and NaN stays NaN,
// as in C. The output is index-aligned with inReal; there is no lookback.
func SINH(inReal []float64) ([]float64, error) {
	n, err := checkInputs("SINH", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := sinhRange(0, n-1, inReal, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// sinhRange mirrors TA_SINH: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func sinhRange(startIdx, endIdx int, inReal []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SINH", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SINH", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = libmSinh(inReal[i])
		outIdx++
	}
	return startIdx, outIdx, nil
}
