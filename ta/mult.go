package ta

// MULT — Vector Arithmetic Mult (Math Operators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/mult and src/ta_func/ta_MULT.c.

// MULTLookback returns the number of leading bars MULT leaves empty (always 0).
// MULT has no optional inputs, so it never returns -1.
func MULTLookback() int {
	return multLookback()
}

func multLookback() int {
	return 0
}

// MULT returns the element-wise product inReal0[i] * inReal1[i].
// The output is index-aligned with the inputs; there is no lookback.
func MULT(inReal0, inReal1 []float64) ([]float64, error) {
	n, err := checkInputs("MULT", []string{"inReal0", "inReal1"}, inReal0, inReal1)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := multRange(0, n-1, inReal0, inReal1, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// multRange mirrors TA_MULT: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may alias either input.
func multRange(startIdx, endIdx int, inReal0, inReal1 []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MULT", endIdx, []string{"inReal0", "inReal1"}, inReal0, inReal1); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MULT", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = inReal0[i] * inReal1[i]
		outIdx++
	}
	return startIdx, outIdx, nil
}
