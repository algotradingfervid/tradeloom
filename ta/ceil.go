package ta

import "math"

// CEIL — Vector Ceil (Math Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/ceil and src/ta_func/ta_CEIL.c.

// CEILLookback returns the number of leading bars CEIL leaves empty (always
// 0). CEIL has no optional inputs, so it never returns -1.
func CEILLookback() int {
	return ceilLookback()
}

func ceilLookback() int {
	return 0
}

// CEIL returns the ceiling (round up to the nearest integer) of each element
// of inReal, as C's ceil: NaN and ±Inf pass through, and inputs in (-1, 0)
// (and -0) produce -0. The output is index-aligned with inReal; there is no
// lookback.
func CEIL(inReal []float64) ([]float64, error) {
	n, err := checkInputs("CEIL", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := ceilRange(0, n-1, inReal, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// ceilRange mirrors TA_CEIL: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func ceilRange(startIdx, endIdx int, inReal []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CEIL", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CEIL", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = math.Ceil(inReal[i])
		outIdx++
	}
	return startIdx, outIdx, nil
}
