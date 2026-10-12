package ta

// CUMSUM — Cumulative Sum (Math Operators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cumsum and src/ta_func/ta_CUMSUM.c.

// CUMSUMLookback returns the number of leading bars CUMSUM leaves empty
// (always 0). CUMSUM has no optional inputs, so it never returns -1.
func CUMSUMLookback() int {
	return cumsumLookback()
}

func cumsumLookback() int {
	return 0
}

// CUMSUM returns the running total of inReal from the first bar forward,
// accumulated left to right in one float64 with no compensation (as C).
// The output is index-aligned with inReal; there is no lookback.
func CUMSUM(inReal []float64) ([]float64, error) {
	n, err := checkInputs("CUMSUM", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cumsumRange(0, n-1, inReal, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// cumsumRange mirrors TA_CUMSUM: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. The accumulator re-seeds at startIdx: outReal[0] is
// inReal[startIdx], not the sum of inReal[0..startIdx]. inReal and outReal
// may be the same slice.
func cumsumRange(startIdx, endIdx int, inReal []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CUMSUM", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CUMSUM", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	total := 0.0
	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		total += inReal[i]
		outReal[outIdx] = total
		outIdx++
	}
	return startIdx, outIdx, nil
}
