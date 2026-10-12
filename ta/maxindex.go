package ta

// MAXINDEX — Index of highest value over a specified period (Math Operators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/maxindex and src/ta_func/ta_MAXINDEX.c.

// MAXINDEXLookback returns the number of leading bars MAXINDEX leaves empty,
// or -1 for an invalid period.
// optInTimePeriod: [2, 100000], default 30.
func MAXINDEXLookback(optInTimePeriod int) int {
	if checkInt("MAXINDEX", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return maxindexLookback(optInTimePeriod)
}

func maxindexLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// MAXINDEX returns, for each bar, the absolute index into inReal of the
// highest value in the window of optInTimePeriod bars ending at that bar.
// The output is index-aligned with inReal; the first MAXINDEXLookback bars
// are 0. On ties the index returned follows TA-Lib (not necessarily the
// first or the last occurrence).
// optInTimePeriod: [2, 100000], default 30.
func MAXINDEX(inReal []float64, optInTimePeriod int) ([]int, error) {
	if err := checkInt("MAXINDEX", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("MAXINDEX", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := maxindexRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// maxindexRange mirrors TA_MAXINDEX: it computes [startIdx, endIdx] and
// writes outNBElement absolute indices to outInteger[0:], the first one
// belonging to bar outBegIdx.
func maxindexRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("MAXINDEX", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MAXINDEX", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("MAXINDEX", endIdx-startIdx, "outInteger", outInteger); err != nil {
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
	highestIdx := -1
	highest := 0.0
	for today <= endIdx {
		tmp := inReal[today]
		if highestIdx < trailingIdx {
			highestIdx = trailingIdx
			highest = inReal[highestIdx]
			i := highestIdx
			for {
				i++
				if i > today {
					break
				}
				tmp = inReal[i]
				if tmp > highest {
					highestIdx = i
					highest = tmp
				}
			}
		} else if tmp >= highest {
			highestIdx = today
			highest = tmp
		}
		outInteger[outIdx] = highestIdx
		outIdx++
		trailingIdx++
		today++
	}
	return startIdx, outIdx, nil
}
