package ta

// EXP — Vector Arithmetic Exp (Math Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/exp and src/ta_func/ta_EXP.c.

// EXPLookback returns the number of leading bars EXP leaves empty (always 0).
// EXP has no optional inputs, so it never returns -1.
func EXPLookback() int {
	return expLookback()
}

func expLookback() int {
	return 0
}

// EXP returns e raised to each element of inReal. Inputs above about 709.78
// overflow to +Inf, very negative inputs underflow to 0, and NaN stays NaN,
// as in C. The output is index-aligned with inReal; there is no lookback.
func EXP(inReal []float64) ([]float64, error) {
	n, err := checkInputs("EXP", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := expRange(0, n-1, inReal, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// expRange mirrors TA_EXP: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func expRange(startIdx, endIdx int, inReal []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("EXP", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("EXP", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = libmExp(inReal[i])
		outIdx++
	}
	return startIdx, outIdx, nil
}
