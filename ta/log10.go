package ta

// LOG10 — Vector Log10 (Math Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/log10 and src/ta_func/ta_LOG10.c.

// LOG10Lookback returns the number of leading bars LOG10 leaves empty (always
// 0). LOG10 has no optional inputs, so it never returns -1.
func LOG10Lookback() int {
	return log10Lookback()
}

func log10Lookback() int {
	return 0
}

// LOG10 returns the base-10 logarithm of each element of inReal, as C's
// log10: 0 (either sign) gives -Inf, negative inputs and NaN give NaN, +Inf
// gives +Inf. The output is index-aligned with inReal; there is no lookback.
func LOG10(inReal []float64) ([]float64, error) {
	n, err := checkInputs("LOG10", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := log10Range(0, n-1, inReal, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// log10Range mirrors TA_LOG10: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func log10Range(startIdx, endIdx int, inReal []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("LOG10", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("LOG10", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = libmLog10(inReal[i])
		outIdx++
	}
	return startIdx, outIdx, nil
}
