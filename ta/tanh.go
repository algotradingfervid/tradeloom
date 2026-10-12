package ta

// TANH — Vector Trigonometric Tanh (Math Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/tanh and src/ta_func/ta_TANH.c.

// TANHLookback returns the number of leading bars TANH leaves empty (always
// 0). TANH has no optional inputs, so it never returns -1.
func TANHLookback() int {
	return tanhLookback()
}

func tanhLookback() int {
	return 0
}

// TANH returns the hyperbolic tangent of each element of inReal. The result
// lies in [-1, 1]; ±Inf maps to ±1 and NaN stays NaN, as in C. The output is
// index-aligned with inReal; there is no lookback.
func TANH(inReal []float64) ([]float64, error) {
	n, err := checkInputs("TANH", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := tanhRange(0, n-1, inReal, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// tanhRange mirrors TA_TANH: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func tanhRange(startIdx, endIdx int, inReal []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TANH", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TANH", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = libmTanh(inReal[i])
		outIdx++
	}
	return startIdx, outIdx, nil
}
