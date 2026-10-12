package ta

// OBV — On Balance Volume (Volume Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/obv and src/ta_func/ta_OBV.c.

// OBVLookback returns the number of leading bars OBV leaves empty (always 0).
// OBV has no optional inputs, so it never returns -1.
func OBVLookback() int {
	return obvLookback()
}

func obvLookback() int {
	return 0
}

// OBV returns the On Balance Volume of inReal and inVolume: a running total
// seeded with the first bar's volume, adding the volume on bars where inReal
// rises and subtracting it where inReal falls. The output is index-aligned
// with the inputs; there is no lookback.
func OBV(inReal, inVolume []float64) ([]float64, error) {
	n, err := checkInputs("OBV", []string{"inReal", "inVolume"}, inReal, inVolume)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := obvRange(0, n-1, inReal, inVolume, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// obvRange mirrors TA_OBV: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. The running total is seeded with inVolume[startIdx] (as in C), so
// a sub-range is not a slice of the full-series result. outReal may alias
// either input.
func obvRange(startIdx, endIdx int, inReal, inVolume []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("OBV", endIdx, []string{"inReal", "inVolume"}, inReal, inVolume); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("OBV", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	prevOBV := inVolume[startIdx]
	prevReal := inReal[startIdx]
	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		tempReal := inReal[i]
		if tempReal > prevReal {
			prevOBV += inVolume[i]
		} else if tempReal < prevReal {
			prevOBV -= inVolume[i]
		}
		outReal[outIdx] = prevOBV
		outIdx++
		prevReal = tempReal
	}
	return startIdx, outIdx, nil
}
