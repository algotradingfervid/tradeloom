package ta

// FRACTAL — Williams Fractal (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/fractal and src/ta_func/ta_FRACTAL.c.

// FRACTALLookback returns the number of leading bars FRACTAL leaves empty:
// optInLeftBars + optInRightBars.
// optInLeftBars: [1, 100000], default 2. optInRightBars: [1, 100000], default 2.
func FRACTALLookback(optInLeftBars, optInRightBars int) int {
	if checkInt("FRACTAL", "optInLeftBars", optInLeftBars, 1, 100000) != nil {
		return -1
	}
	if checkInt("FRACTAL", "optInRightBars", optInRightBars, 1, 100000) != nil {
		return -1
	}
	return fractalLookback(optInLeftBars, optInRightBars)
}

func fractalLookback(optInLeftBars, optInRightBars int) int {
	return optInLeftBars + optInRightBars
}

// FRACTAL returns the Williams Fractal swing-high and swing-low flags of
// (inHigh, inLow), in TA-Lib output order. A value at index i is 100 when the
// bar optInRightBars back (i - optInRightBars) strictly dominates the
// optInLeftBars bars before it and the optInRightBars bars after it, else 0.
// Both outputs are index-aligned with the inputs; the first FRACTALLookback
// bars are 0.
// optInLeftBars: [1, 100000], default 2. optInRightBars: [1, 100000], default 2.
func FRACTAL(inHigh, inLow []float64, optInLeftBars, optInRightBars int) (outSwingHigh, outSwingLow []int, err error) {
	if err := checkInt("FRACTAL", "optInLeftBars", optInLeftBars, 1, 100000); err != nil {
		return nil, nil, err
	}
	if err := checkInt("FRACTAL", "optInRightBars", optInRightBars, 1, 100000); err != nil {
		return nil, nil, err
	}
	n, err := checkInputs("FRACTAL", []string{"inHigh", "inLow"}, inHigh, inLow)
	if err != nil {
		return nil, nil, err
	}
	hi := make([]int, n)
	lo := make([]int, n)
	if n == 0 {
		return hi, lo, nil
	}
	beg, nb, err := fractalRange(0, n-1, inHigh, inLow, optInLeftBars, optInRightBars, hi, lo)
	if err != nil {
		return nil, nil, err
	}
	alignInt(hi, beg, nb)
	alignInt(lo, beg, nb)
	return hi, lo, nil
}

// fractalRange mirrors TA_FRACTAL: it computes [startIdx, endIdx] and writes
// outNBElement values to outSwingHigh[0:] and outSwingLow[0:], the first one
// belonging to bar outBegIdx. The two outputs must be distinct buffers (C
// rejects outSwingHigh == outSwingLow).
func fractalRange(startIdx, endIdx int, inHigh, inLow []float64, optInLeftBars, optInRightBars int, outSwingHigh, outSwingLow []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("FRACTAL", "optInLeftBars", optInLeftBars, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkInt("FRACTAL", "optInRightBars", optInRightBars, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("FRACTAL", endIdx, []string{"inHigh", "inLow"}, inHigh, inLow); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("FRACTAL", endIdx-startIdx, "outSwingHigh", outSwingHigh); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("FRACTAL", endIdx-startIdx, "outSwingLow", outSwingLow); err != nil {
		return 0, 0, err
	}
	if &outSwingHigh[0] == &outSwingLow[0] {
		return 0, 0, badParam("FRACTAL", "outSwingLow", "must not be the same buffer as outSwingHigh")
	}

	lookbackTotal := fractalLookback(optInLeftBars, optInRightBars)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	// Make sure there is still something to evaluate.
	if startIdx > endIdx {
		return 0, 0, nil
	}

	outIdx := 0
	today := startIdx
	for today <= endIdx {
		pivotHigh := 0.0
		pivotLow := 0.0
		otherHigh := 0.0
		otherLow := 0.0
		for i := optInLeftBars + optInRightBars; i >= 0; i-- {
			tempHigh := inHigh[today-i]
			tempLow := inLow[today-i]
			if i == optInRightBars {
				pivotHigh = tempHigh
				pivotLow = tempLow
			} else if i == optInLeftBars+optInRightBars {
				// optInLeftBars is at least 1, so the oldest bar of the window
				// is never the pivot and always seeds the rest-of-window extrema.
				otherHigh = tempHigh
				otherLow = tempLow
			} else {
				if tempHigh > otherHigh {
					otherHigh = tempHigh
				}
				if tempLow < otherLow {
					otherLow = tempLow
				}
			}
		}
		if pivotHigh > otherHigh {
			outSwingHigh[outIdx] = 100
		} else {
			outSwingHigh[outIdx] = 0
		}
		if pivotLow < otherLow {
			outSwingLow[outIdx] = 100
		} else {
			outSwingLow[outIdx] = 0
		}
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
