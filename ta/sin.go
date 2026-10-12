package ta

// SIN — Vector Trigonometric Sin (Math Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/sin and src/ta_func/ta_SIN.c.

// SINLookback returns the number of leading bars SIN leaves empty (always 0).
// SIN has no optional inputs, so it never returns -1.
func SINLookback() int {
	return sinLookback()
}

func sinLookback() int {
	return 0
}

// SIN returns the sine of each element of inReal (angles in radians).
// NaN and ±Inf inputs produce NaN, as in C. The output is index-aligned with
// inReal; there is no lookback.
func SIN(inReal []float64) ([]float64, error) {
	n, err := checkInputs("SIN", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := sinRange(0, n-1, inReal, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// sinRange mirrors TA_SIN: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func sinRange(startIdx, endIdx int, inReal []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SIN", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SIN", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = libmSin(inReal[i])
		outIdx++
	}
	return startIdx, outIdx, nil
}
