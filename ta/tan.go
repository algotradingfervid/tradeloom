package ta

// TAN — Vector Trigonometric Tan (Math Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/tan and src/ta_func/ta_TAN.c.

// TANLookback returns the number of leading bars TAN leaves empty (always 0).
// TAN has no optional inputs, so it never returns -1.
func TANLookback() int {
	return tanLookback()
}

func tanLookback() int {
	return 0
}

// TAN returns the tangent of each element of inReal (in radians). NaN and
// ±Inf inputs produce NaN. The output is index-aligned with inReal; there is
// no lookback.
func TAN(inReal []float64) ([]float64, error) {
	n, err := checkInputs("TAN", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := tanRange(0, n-1, inReal, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// tanRange mirrors TA_TAN: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func tanRange(startIdx, endIdx int, inReal []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TAN", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TAN", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = libmTan(inReal[i])
		outIdx++
	}
	return startIdx, outIdx, nil
}
