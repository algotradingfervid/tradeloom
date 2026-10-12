package ta

// ADD — Vector Arithmetic Add (Math Operators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/add and src/ta_func/ta_ADD.c.

// ADDLookback returns the number of leading bars ADD leaves empty (always 0).
// ADD has no optional inputs, so it never returns -1.
func ADDLookback() int {
	return addLookback()
}

func addLookback() int {
	return 0
}

// ADD returns the element-wise sum inReal0[i] + inReal1[i].
// The output is index-aligned with the inputs; there is no lookback.
func ADD(inReal0, inReal1 []float64) ([]float64, error) {
	n, err := checkInputs("ADD", []string{"inReal0", "inReal1"}, inReal0, inReal1)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := addRange(0, n-1, inReal0, inReal1, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// addRange mirrors TA_ADD: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may alias either input.
func addRange(startIdx, endIdx int, inReal0, inReal1 []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ADD", endIdx, []string{"inReal0", "inReal1"}, inReal0, inReal1); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ADD", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = inReal0[i] + inReal1[i]
		outIdx++
	}
	return startIdx, outIdx, nil
}
