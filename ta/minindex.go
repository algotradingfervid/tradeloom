package ta

// MININDEX — Index of lowest value over a specified period (Math Operators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/minindex and src/ta_func/ta_MININDEX.c.

// MININDEXLookback returns the number of leading bars MININDEX leaves empty,
// or -1 for an invalid period.
// optInTimePeriod: [2, 100000], default 30.
func MININDEXLookback(optInTimePeriod int) int {
	if checkInt("MININDEX", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return minindexLookback(optInTimePeriod)
}

func minindexLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// MININDEX returns, for each bar, the absolute index into inReal of the
// lowest value in the window of optInTimePeriod bars ending at that bar.
// The output is index-aligned with inReal; the first MININDEXLookback bars
// are 0. On ties the index returned follows TA-Lib (not necessarily the
// first or the last occurrence).
// optInTimePeriod: [2, 100000], default 30.
func MININDEX(inReal []float64, optInTimePeriod int) ([]int, error) {
	if err := checkInt("MININDEX", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("MININDEX", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := minindexRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// minindexRange mirrors TA_MININDEX: it computes [startIdx, endIdx] and
// writes outNBElement absolute indices to outInteger[0:], the first one
// belonging to bar outBegIdx.
func minindexRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("MININDEX", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MININDEX", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("MININDEX", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
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
	lowestIdx := -1
	lowest := 0.0
	for today <= endIdx {
		tmp := inReal[today]
		if lowestIdx < trailingIdx {
			lowestIdx = trailingIdx
			lowest = inReal[lowestIdx]
			i := lowestIdx
			for {
				i++
				if i > today {
					break
				}
				tmp = inReal[i]
				if tmp < lowest {
					lowestIdx = i
					lowest = tmp
				}
			}
		} else if tmp <= lowest {
			lowestIdx = today
			lowest = tmp
		}
		outInteger[outIdx] = lowestIdx
		outIdx++
		trailingIdx++
		today++
	}
	return startIdx, outIdx, nil
}
