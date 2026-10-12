package ta

// LN — Vector Log Natural (Math Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/ln and src/ta_func/ta_LN.c.

// LNLookback returns the number of leading bars LN leaves empty (always 0).
// LN has no optional inputs, so it never returns -1.
func LNLookback() int {
	return lnLookback()
}

func lnLookback() int {
	return 0
}

// LN returns the natural logarithm of each element of inReal. As in C, a
// negative input or NaN gives NaN, zero gives -Inf and +Inf gives +Inf. The
// output is index-aligned with inReal; there is no lookback.
func LN(inReal []float64) ([]float64, error) {
	n, err := checkInputs("LN", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := lnRange(0, n-1, inReal, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// lnRange mirrors TA_LN: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func lnRange(startIdx, endIdx int, inReal []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("LN", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("LN", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = libmLog(inReal[i])
		outIdx++
	}
	return startIdx, outIdx, nil
}
