package ta

// MINMAXINDEX — Indexes of lowest and highest values over a specified period
// (Math Operators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/minmaxindex and
// src/ta_func/ta_MINMAXINDEX.c.

// MINMAXINDEXLookback returns the number of leading bars MINMAXINDEX leaves
// empty. optInTimePeriod: [2, 100000], default 30.
func MINMAXINDEXLookback(optInTimePeriod int) int {
	if checkInt("MINMAXINDEX", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return minmaxindexLookback(optInTimePeriod)
}

func minmaxindexLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// MINMAXINDEX returns, for each bar, the absolute indexes into inReal of the
// lowest and highest values over the last optInTimePeriod bars, in TA-Lib
// output order (outMinIdx, outMaxIdx). Outputs are index-aligned with inReal;
// the first MINMAXINDEXLookback bars are 0. On ties the C algorithm's choice
// is kept (see minmaxindexRange).
// optInTimePeriod: [2, 100000], default 30.
func MINMAXINDEX(inReal []float64, optInTimePeriod int) ([]int, []int, error) {
	if err := checkInt("MINMAXINDEX", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, nil, err
	}
	n, err := checkInputs("MINMAXINDEX", []string{"inReal"}, inReal)
	if err != nil {
		return nil, nil, err
	}
	outMinIdx := make([]int, n)
	outMaxIdx := make([]int, n)
	if n == 0 {
		return outMinIdx, outMaxIdx, nil
	}
	beg, nb, err := minmaxindexRange(0, n-1, inReal, optInTimePeriod, outMinIdx, outMaxIdx)
	if err != nil {
		return nil, nil, err
	}
	alignInt(outMinIdx, beg, nb)
	alignInt(outMaxIdx, beg, nb)
	return outMinIdx, outMaxIdx, nil
}

// minmaxindexRange mirrors TA_MINMAXINDEX: it computes [startIdx, endIdx] and
// writes outNBElement values to outMinIdx[0:] and outMaxIdx[0:], the first
// belonging to bar outBegIdx. The values are absolute indexes into inReal.
// outMinIdx and outMaxIdx must not share storage (C returns TA_BAD_PARAM).
func minmaxindexRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outMinIdx, outMaxIdx []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("MINMAXINDEX", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MINMAXINDEX", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("MINMAXINDEX", endIdx-startIdx, "outMinIdx", outMinIdx); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("MINMAXINDEX", endIdx-startIdx, "outMaxIdx", outMaxIdx); err != nil {
		return 0, 0, err
	}
	// C: if( outMinIdx == outMaxIdx ) return TA_BAD_PARAM;
	if &outMinIdx[0] == &outMaxIdx[0] {
		return 0, 0, badParam("MINMAXINDEX", "outMaxIdx", "same buffer as outMinIdx")
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
		tmpHigh := inReal[today]
		tmpLow := tmpHigh
		if highestIdx < trailingIdx {
			highestIdx = trailingIdx
			highest = inReal[highestIdx]
			i := highestIdx
			for i++; i <= today; i++ {
				tmpHigh = inReal[i]
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
			lowest = inReal[lowestIdx]
			i := lowestIdx
			for i++; i <= today; i++ {
				tmpLow = inReal[i]
				if tmpLow < lowest {
					lowestIdx = i
					lowest = tmpLow
				}
			}
		} else if tmpLow <= lowest {
			lowestIdx = today
			lowest = tmpLow
		}
		outMaxIdx[outIdx] = highestIdx
		outMinIdx[outIdx] = lowestIdx
		outIdx++
		trailingIdx++
		today++
	}
	return startIdx, outIdx, nil
}
