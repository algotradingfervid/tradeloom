package ta

// MAX — Highest value over a specified period (Math Operators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/max and src/ta_func/ta_MAX.c.

// MAXLookback returns the number of leading bars MAX leaves empty.
// optInTimePeriod: [2, 100000], default 30.
func MAXLookback(optInTimePeriod int) int {
	if checkInt("MAX", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return maxLookback(optInTimePeriod)
}

func maxLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// MAX returns the highest value of inReal over the trailing optInTimePeriod
// bars. The output is index-aligned with inReal; the first MAXLookback bars
// are NaN. optInTimePeriod: [2, 100000], default 30.
func MAX(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("MAX", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("MAX", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := maxRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// maxRange mirrors TA_MAX: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func maxRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("MAX", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MAX", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MAX", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	nbInitialElementNeeded := optInTimePeriod - 1
	if startIdx < nbInitialElementNeeded {
		startIdx = nbInitialElementNeeded
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// Van Herk / Gil-Werman block scan, block-batched form, as in the C.
	// The scratch arrays hold copies, so inReal and outReal may alias.
	outIdx := 0
	today := startIdx
	trailingIdx := startIdx - nbInitialElementNeeded
	sufHighest := make([]float64, optInTimePeriod)
	preHighest := make([]float64, optInTimePeriod)
	blockStart := trailingIdx
	for today <= endIdx {
		// Suffix extrema of [blockStart, blockStart+p-1], backward,
		// keeping the incumbent on a tie.
		i := blockStart + optInTimePeriod - 1
		highest := inReal[i]
		sufHighest[optInTimePeriod-1] = highest
		for i > blockStart {
			i--
			tmp := inReal[i]
			if tmp > highest {
				highest = tmp
			}
			sufHighest[i-blockStart] = highest
		}
		highest = sufHighest[0]
		outReal[outIdx] = highest
		outIdx++
		trailingIdx++
		today++
		if today > endIdx {
			blockStart += optInTimePeriod
		} else {
			// Prefix extrema of the next block, clamped to what remains.
			nAvail := endIdx - (blockStart + optInTimePeriod) + 1
			if nAvail > optInTimePeriod-1 {
				nAvail = optInTimePeriod - 1
			}
			highest = inReal[blockStart+optInTimePeriod]
			preHighest[0] = highest
			i = 1
			for i < nAvail {
				tmp := inReal[blockStart+optInTimePeriod+i]
				if tmp > highest {
					highest = tmp
				}
				preHighest[i] = highest
				i++
			}
			// Combine; the older suffix half wins ties.
			for m := 1; m <= nAvail; m++ {
				highest = sufHighest[m]
				if preHighest[m-1] > highest {
					highest = preHighest[m-1]
				}
				outReal[outIdx] = highest
				outIdx++
			}
			trailingIdx += nAvail
			today += nAvail
			blockStart += optInTimePeriod
		}
	}
	return startIdx, outIdx, nil
}
