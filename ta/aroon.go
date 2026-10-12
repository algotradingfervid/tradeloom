package ta

// AROON — Aroon (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/aroon and src/ta_func/ta_AROON.c.

// AROONLookback returns the number of leading bars AROON leaves empty: the
// window covers optInTimePeriod+1 bars, so the lookback is optInTimePeriod.
// optInTimePeriod: [2, 100000], default 14.
func AROONLookback(optInTimePeriod int) int {
	if checkInt("AROON", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return aroonLookback(optInTimePeriod)
}

func aroonLookback(optInTimePeriod int) int {
	return optInTimePeriod
}

// AROON returns Aroon Down and Aroon Up of (inHigh, inLow) over
// optInTimePeriod bars, in TA-Lib output order. Both outputs are
// index-aligned with the inputs; the first AROONLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 14.
func AROON(inHigh, inLow []float64, optInTimePeriod int) (outAroonDown, outAroonUp []float64, err error) {
	if err := checkInt("AROON", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, nil, err
	}
	n, err := checkInputs("AROON", []string{"inHigh", "inLow"}, inHigh, inLow)
	if err != nil {
		return nil, nil, err
	}
	down := make([]float64, n)
	up := make([]float64, n)
	if n == 0 {
		return down, up, nil
	}
	beg, nb, err := aroonRange(0, n-1, inHigh, inLow, optInTimePeriod, down, up)
	if err != nil {
		return nil, nil, err
	}
	alignReal(down, beg, nb)
	alignReal(up, beg, nb)
	return down, up, nil
}

// aroonRange mirrors TA_AROON: it computes [startIdx, endIdx] and writes
// outNBElement values to outAroonDown[0:] and outAroonUp[0:], the first one
// belonging to bar outBegIdx. Either output may alias an input, but the two
// outputs must be distinct buffers (C rejects outAroonDown == outAroonUp).
func aroonRange(startIdx, endIdx int, inHigh, inLow []float64, optInTimePeriod int, outAroonDown, outAroonUp []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("AROON", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("AROON", endIdx, []string{"inHigh", "inLow"}, inHigh, inLow); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("AROON", endIdx-startIdx, []string{"outAroonDown", "outAroonUp"}, outAroonDown, outAroonUp); err != nil {
		return 0, 0, err
	}
	if &outAroonDown[0] == &outAroonUp[0] {
		return 0, 0, badParam("AROON", "outAroonUp", "must not be the same buffer as outAroonDown")
	}

	if startIdx < optInTimePeriod {
		startIdx = optInTimePeriod
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	outIdx := 0
	today := startIdx
	trailingIdx := startIdx - optInTimePeriod
	lowestIdx := -1
	highestIdx := -1
	lowest := 0.0
	highest := 0.0
	factor := 100.0 / float64(optInTimePeriod)
	for today <= endIdx {
		// Keep track of the lowestIdx.
		tmp := inLow[today]
		if lowestIdx < trailingIdx {
			lowestIdx = trailingIdx
			lowest = inLow[lowestIdx]
			i := lowestIdx
			for i++; i <= today; i++ {
				tmp = inLow[i]
				if tmp <= lowest {
					lowestIdx = i
					lowest = tmp
				}
			}
		} else if tmp <= lowest {
			lowestIdx = today
			lowest = tmp
		}
		// Keep track of the highestIdx.
		tmp = inHigh[today]
		if highestIdx < trailingIdx {
			highestIdx = trailingIdx
			highest = inHigh[highestIdx]
			i := highestIdx
			for i++; i <= today; i++ {
				tmp = inHigh[i]
				if tmp >= highest {
					highestIdx = i
					highest = tmp
				}
			}
		} else if tmp >= highest {
			highestIdx = today
			highest = tmp
		}
		// Outputs may alias the inputs: write them last.
		outAroonUp[outIdx] = factor * float64(optInTimePeriod-(today-highestIdx))
		outAroonDown[outIdx] = factor * float64(optInTimePeriod-(today-lowestIdx))
		outIdx++
		trailingIdx++
		today++
	}
	return startIdx, outIdx, nil
}
