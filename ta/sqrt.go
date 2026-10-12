package ta

import "math"

// SQRT — Vector Square Root (Math Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/sqrt and src/ta_func/ta_SQRT.c.

// SQRTLookback returns the number of leading bars SQRT leaves empty (always 0).
// SQRT has no optional inputs, so it never returns -1.
func SQRTLookback() int {
	return sqrtLookback()
}

func sqrtLookback() int {
	return 0
}

// SQRT returns the square root of each element of inReal. As in C, a
// negative input or NaN gives NaN, -0 gives -0 and +Inf gives +Inf. The
// output is index-aligned with inReal; there is no lookback.
func SQRT(inReal []float64) ([]float64, error) {
	n, err := checkInputs("SQRT", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := sqrtRange(0, n-1, inReal, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// sqrtRange mirrors TA_SQRT: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func sqrtRange(startIdx, endIdx int, inReal []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SQRT", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SQRT", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = math.Sqrt(inReal[i])
		outIdx++
	}
	return startIdx, outIdx, nil
}
