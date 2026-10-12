package ta

// ATAN — Vector Trigonometric ATan (Math Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/atan and src/ta_func/ta_ATAN.c.

// ATANLookback returns the number of leading bars ATAN leaves empty (always
// 0). ATAN has no optional inputs, so it never returns -1.
func ATANLookback() int {
	return atanLookback()
}

func atanLookback() int {
	return 0
}

// ATAN returns the arc tangent, in radians, of each element of inReal. The
// result lies in [-pi/2, pi/2] (±Inf maps to ±pi/2); only a NaN input
// produces NaN. The output is index-aligned with inReal; there is no lookback.
func ATAN(inReal []float64) ([]float64, error) {
	n, err := checkInputs("ATAN", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := atanRange(0, n-1, inReal, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// atanRange mirrors TA_ATAN: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func atanRange(startIdx, endIdx int, inReal []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ATAN", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ATAN", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = libmAtan(inReal[i])
		outIdx++
	}
	return startIdx, outIdx, nil
}
