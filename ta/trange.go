package ta

import "math"

// TRANGE — True Range (Volatility Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/trange and src/ta_func/ta_TRANGE.c.

// TRANGELookback returns the number of leading bars TRANGE leaves empty
// (always 1: the first bar has no previous close).
func TRANGELookback() int {
	return trangeLookback()
}

func trangeLookback() int {
	return 1
}

// TRANGE returns the true range of each bar: the greatest of high-low,
// |prevClose-high| and |prevClose-low|. The output is index-aligned with the
// inputs; the first bar is NaN.
func TRANGE(inHigh, inLow, inClose []float64) ([]float64, error) {
	n, err := checkInputs("TRANGE", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := trangeRange(0, n-1, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// trangeRange mirrors TA_TRANGE: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx.
func trangeRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TRANGE", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TRANGE", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// Always one price bar gets consumed.
	if startIdx < 1 {
		startIdx = 1
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	outIdx := 0
	today := startIdx
	for today <= endIdx {
		tempLT := inLow[today]
		tempHT := inHigh[today]
		tempCY := inClose[today-1]
		greatest := tempHT - tempLT
		val2 := math.Abs(tempCY - tempHT)
		if val2 > greatest {
			greatest = val2
		}
		val3 := math.Abs(tempCY - tempLT)
		if val3 > greatest {
			greatest = val3
		}
		outReal[outIdx] = greatest
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
