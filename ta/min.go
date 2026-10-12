package ta

// MIN — Lowest value over a specified period (Math Operators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/min and src/ta_func/ta_MIN.c.

// MINLookback returns the number of leading bars MIN leaves empty.
// optInTimePeriod: [2, 100000], default 30.
func MINLookback(optInTimePeriod int) int {
	if checkInt("MIN", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return minLookback(optInTimePeriod)
}

func minLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// MIN returns the lowest value of inReal over the trailing optInTimePeriod
// bars. The output is index-aligned with inReal; the first MINLookback bars
// are NaN. optInTimePeriod: [2, 100000], default 30.
func MIN(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("MIN", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("MIN", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := minRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// minRange mirrors TA_MIN: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func minRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("MIN", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MIN", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MIN", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
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
	sufLowest := make([]float64, optInTimePeriod)
	preLowest := make([]float64, optInTimePeriod)
	blockStart := trailingIdx
	for today <= endIdx {
		// Suffix extrema of [blockStart, blockStart+p-1], backward,
		// keeping the incumbent on a tie.
		i := blockStart + optInTimePeriod - 1
		lowest := inReal[i]
		sufLowest[optInTimePeriod-1] = lowest
		for i > blockStart {
			i--
			tmp := inReal[i]
			if tmp < lowest {
				lowest = tmp
			}
			sufLowest[i-blockStart] = lowest
		}
		lowest = sufLowest[0]
		outReal[outIdx] = lowest
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
			lowest = inReal[blockStart+optInTimePeriod]
			preLowest[0] = lowest
			i = 1
			for i < nAvail {
				tmp := inReal[blockStart+optInTimePeriod+i]
				if tmp < lowest {
					lowest = tmp
				}
				preLowest[i] = lowest
				i++
			}
			// Combine; the older suffix half wins ties.
			for m := 1; m <= nAvail; m++ {
				lowest = sufLowest[m]
				if preLowest[m-1] < lowest {
					lowest = preLowest[m-1]
				}
				outReal[outIdx] = lowest
				outIdx++
			}
			trailingIdx += nAvail
			today += nAvail
			blockStart += optInTimePeriod
		}
	}
	return startIdx, outIdx, nil
}
