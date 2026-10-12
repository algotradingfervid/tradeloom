package ta

import "math"

// FLOOR — Vector Floor (Math Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/floor and src/ta_func/ta_FLOOR.c.

// FLOORLookback returns the number of leading bars FLOOR leaves empty (always
// 0). FLOOR has no optional inputs, so it never returns -1.
func FLOORLookback() int {
	return floorLookback()
}

func floorLookback() int {
	return 0
}

// FLOOR returns the floor (round down to the nearest integer) of each element
// of inReal, as C's floor: NaN and ±Inf pass through, -0 stays -0, and inputs
// in (0, 1) produce +0. The output is index-aligned with inReal; there is no
// lookback.
func FLOOR(inReal []float64) ([]float64, error) {
	n, err := checkInputs("FLOOR", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := floorRange(0, n-1, inReal, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// floorRange mirrors TA_FLOOR: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func floorRange(startIdx, endIdx int, inReal []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("FLOOR", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("FLOOR", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		outReal[outIdx] = math.Floor(inReal[i])
		outIdx++
	}
	return startIdx, outIdx, nil
}
