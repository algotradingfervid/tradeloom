package ta

// DONCHIAN — Donchian Channels (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/donchian and src/ta_func/ta_DONCHIAN.c.

// DONCHIANLookback returns the number of leading bars DONCHIAN leaves empty.
// optInTimePeriod: [2, 100000], default 20.
func DONCHIANLookback(optInTimePeriod int) int {
	if checkInt("DONCHIAN", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return donchianLookback(optInTimePeriod)
}

func donchianLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// DONCHIAN returns the upper (highest inHigh), middle (midpoint) and lower
// (lowest inLow) Donchian bands over the optInTimePeriod bars ending at each
// bar, in TA-Lib output order. Outputs are index-aligned with the inputs; the
// first DONCHIANLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 20.
func DONCHIAN(inHigh, inLow []float64, optInTimePeriod int) (outRealUpperBand, outRealMiddleBand, outRealLowerBand []float64, err error) {
	if err := checkInt("DONCHIAN", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, nil, nil, err
	}
	n, err := checkInputs("DONCHIAN", []string{"inHigh", "inLow"}, inHigh, inLow)
	if err != nil {
		return nil, nil, nil, err
	}
	upper := make([]float64, n)
	middle := make([]float64, n)
	lower := make([]float64, n)
	if n == 0 {
		return upper, middle, lower, nil
	}
	beg, nb, err := donchianRange(0, n-1, inHigh, inLow, optInTimePeriod, upper, middle, lower)
	if err != nil {
		return nil, nil, nil, err
	}
	alignReal(upper, beg, nb)
	alignReal(middle, beg, nb)
	alignReal(lower, beg, nb)
	return upper, middle, lower, nil
}

// donchianRange mirrors TA_DONCHIAN: it computes [startIdx, endIdx] and writes
// outNBElement values to each output from index 0, the first one belonging to
// bar outBegIdx. Outputs may alias the inputs, but the three outputs must be
// distinct buffers (C rejects any two being the same pointer).
func donchianRange(startIdx, endIdx int, inHigh, inLow []float64, optInTimePeriod int, outRealUpperBand, outRealMiddleBand, outRealLowerBand []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("DONCHIAN", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("DONCHIAN", endIdx, []string{"inHigh", "inLow"}, inHigh, inLow); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("DONCHIAN", endIdx-startIdx, []string{"outRealUpperBand", "outRealMiddleBand", "outRealLowerBand"}, outRealUpperBand, outRealMiddleBand, outRealLowerBand); err != nil {
		return 0, 0, err
	}
	if &outRealUpperBand[0] == &outRealMiddleBand[0] || &outRealUpperBand[0] == &outRealLowerBand[0] || &outRealMiddleBand[0] == &outRealLowerBand[0] {
		return 0, 0, badParam("DONCHIAN", "outRealMiddleBand", "the three outputs must be distinct buffers")
	}

	nbInitialElementNeeded := optInTimePeriod - 1
	if startIdx < nbInitialElementNeeded {
		startIdx = nbInitialElementNeeded
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	outIdx := 0
	today := startIdx
	trailingIdx := startIdx - nbInitialElementNeeded
	highestIdx := -1
	highest := 0.0
	lowestIdx := -1
	lowest := 0.0
	for today <= endIdx {
		tmpHigh := inHigh[today]
		tmpLow := inLow[today]
		if highestIdx < trailingIdx {
			highestIdx = trailingIdx
			highest = inHigh[highestIdx]
			for i := highestIdx + 1; i <= today; i++ {
				tmpHigh = inHigh[i]
				if tmpHigh > highest {
					highestIdx = i
					highest = tmpHigh
				}
			}
		} else if tmpHigh >= highest {
			highestIdx = today
			highest = tmpHigh
		}
		if lowestIdx < trailingIdx {
			lowestIdx = trailingIdx
			lowest = inLow[lowestIdx]
			for i := lowestIdx + 1; i <= today; i++ {
				tmpLow = inLow[i]
				if tmpLow < lowest {
					lowestIdx = i
					lowest = tmpLow
				}
			}
		} else if tmpLow <= lowest {
			lowestIdx = today
			lowest = tmpLow
		}
		outRealUpperBand[outIdx] = highest
		outRealLowerBand[outIdx] = lowest
		outRealMiddleBand[outIdx] = (highest + lowest) / 2.0
		outIdx++
		trailingIdx++
		today++
	}
	return startIdx, outIdx, nil
}
