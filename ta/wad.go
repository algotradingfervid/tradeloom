package ta

// WAD — Williams' Accumulation/Distribution (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/wad and src/ta_func/ta_WAD.c.

// WADLookback returns the number of leading bars WAD leaves empty (always 0).
// WAD has no optional inputs, so it never returns -1.
func WADLookback() int {
	return wadLookback()
}

func wadLookback() int {
	return 0
}

// WAD returns Williams' Accumulation/Distribution line (the no-volume form
// published by Achelis): the running sum, seeded at 0, of
// close - min(prevClose, low) on up closes and close - max(prevClose, high) on
// down closes; equal closes add nothing. The first bar has no previous close
// and contributes 0. The output is index-aligned with the inputs; there is no
// lookback.
func WAD(inHigh, inLow, inClose []float64) ([]float64, error) {
	n, err := checkInputs("WAD", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := wadRange(0, n-1, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// wadRange mirrors TA_WAD: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. The bar at startIdx is measured against its own close, so the
// sum restarts at 0 there (as in C) and a sub-range is not a slice of the
// full-series result. outReal may alias any of the inputs.
func wadRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("WAD", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("WAD", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	sum := 0.0
	outIdx := 0
	prevClose := inClose[startIdx]
	for i := startIdx; i <= endIdx; i++ {
		close := inClose[i]
		if close > prevClose {
			trueExtreme := inLow[i]
			if prevClose < trueExtreme {
				trueExtreme = prevClose
			}
			sum += close - trueExtreme
		} else if close < prevClose {
			trueExtreme := inHigh[i]
			if prevClose > trueExtreme {
				trueExtreme = prevClose
			}
			sum += close - trueExtreme
		}
		outReal[outIdx] = sum
		outIdx++
		prevClose = close
	}
	return startIdx, outIdx, nil
}
