package ta

// DIV — Vector Arithmetic Div (Math Operators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/div and src/ta_func/ta_DIV.c.

// DIVLookback returns the number of leading bars DIV leaves empty (always 0).
// DIV has no optional inputs, so it never returns -1.
func DIVLookback() int {
	return divLookback()
}

func divLookback() int {
	return 0
}

// DIV returns the element-wise quotient inReal0[i] / inReal1[i].
// The output is index-aligned with the inputs; there is no lookback.
// Division by zero is not an error: x/0 is ±Inf and 0/0 is NaN, as in C.
func DIV(inReal0, inReal1 []float64) ([]float64, error) {
	n, err := checkInputs("DIV", []string{"inReal0", "inReal1"}, inReal0, inReal1)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := divRange(0, n-1, inReal0, inReal1, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// divRange mirrors TA_DIV: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may alias either input.
func divRange(startIdx, endIdx int, inReal0, inReal1 []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("DIV", endIdx, []string{"inReal0", "inReal1"}, inReal0, inReal1); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("DIV", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = inReal0[i] / inReal1[i]
		outIdx++
	}
	return startIdx, outIdx, nil
}
