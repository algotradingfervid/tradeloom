package ta

// MINMAX — Lowest and highest values over a specified period (Math Operators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/minmax and src/ta_func/ta_MINMAX.c.

// MINMAXLookback returns the number of leading bars MINMAX leaves empty.
// optInTimePeriod: [2, 100000], default 30.
func MINMAXLookback(optInTimePeriod int) int {
	if checkInt("MINMAX", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return minmaxLookback(optInTimePeriod)
}

func minmaxLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// MINMAX returns the lowest (outMin) and highest (outMax) values of inReal
// over the trailing optInTimePeriod bars. Both outputs are index-aligned with
// inReal; the first MINMAXLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 30.
func MINMAX(inReal []float64, optInTimePeriod int) (outMin, outMax []float64, err error) {
	if err := checkInt("MINMAX", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, nil, err
	}
	n, err := checkInputs("MINMAX", []string{"inReal"}, inReal)
	if err != nil {
		return nil, nil, err
	}
	outMin = make([]float64, n)
	outMax = make([]float64, n)
	if n == 0 {
		return outMin, outMax, nil
	}
	beg, nb, err := minmaxRange(0, n-1, inReal, optInTimePeriod, outMin, outMax)
	if err != nil {
		return nil, nil, err
	}
	alignReal(outMin, beg, nb)
	alignReal(outMax, beg, nb)
	return outMin, outMax, nil
}

// minmaxRange mirrors TA_MINMAX: it computes [startIdx, endIdx] and writes
// outNBElement values to outMin[0:] and outMax[0:], the first one belonging
// to bar outBegIdx. Either output may alias inReal, but the two outputs must
// be distinct buffers (C rejects outMin == outMax).
func minmaxRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outMin, outMax []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("MINMAX", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MINMAX", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MINMAX", endIdx-startIdx, []string{"outMin", "outMax"}, outMin, outMax); err != nil {
		return 0, 0, err
	}
	if &outMin[0] == &outMax[0] {
		return 0, 0, badParam("MINMAX", "outMax", "must not be the same buffer as outMin")
	}

	nbInitialElementNeeded := optInTimePeriod - 1
	if startIdx < nbInitialElementNeeded {
		startIdx = nbInitialElementNeeded
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// Van Herk / Gil-Werman block scan, block-batched form, as in the C.
	// The scratch arrays hold copies, so inReal and the outputs may alias.
	outIdx := 0
	today := startIdx
	trailingIdx := startIdx - nbInitialElementNeeded
	sufHighest := make([]float64, optInTimePeriod)
	preHighest := make([]float64, optInTimePeriod)
	sufLowest := make([]float64, optInTimePeriod)
	preLowest := make([]float64, optInTimePeriod)
	blockStart := trailingIdx
	for today <= endIdx {
		// Suffix extrema of [blockStart, blockStart+p-1], backward,
		// keeping the incumbent on a tie.
		i := blockStart + optInTimePeriod - 1
		highest := inReal[i]
		lowest := highest
		sufHighest[optInTimePeriod-1] = highest
		sufLowest[optInTimePeriod-1] = lowest
		for i > blockStart {
			i--
			tmpHigh := inReal[i]
			if tmpHigh > highest {
				highest = tmpHigh
			}
			if tmpHigh < lowest {
				lowest = tmpHigh
			}
			sufHighest[i-blockStart] = highest
			sufLowest[i-blockStart] = lowest
		}
		outMax[outIdx] = sufHighest[0]
		outMin[outIdx] = sufLowest[0]
		outIdx++
		trailingIdx++
		today++
		if today > endIdx {
			blockStart += optInTimePeriod
		} else {
			// Prefix extrema of the next block, clamped to what remains.
			blockNext := blockStart + optInTimePeriod
			nAvail := endIdx - blockNext + 1
			if nAvail > optInTimePeriod-1 {
				nAvail = optInTimePeriod - 1
			}
			highest = inReal[blockNext]
			lowest = highest
			preHighest[0] = highest
			preLowest[0] = lowest
			i = 1
			for i < nAvail {
				tmpHigh := inReal[blockNext+i]
				if tmpHigh > highest {
					highest = tmpHigh
				}
				if tmpHigh < lowest {
					lowest = tmpHigh
				}
				preHighest[i] = highest
				preLowest[i] = lowest
				i++
			}
			// Combine; the older suffix half wins ties.
			for m := 1; m <= nAvail; m++ {
				highest = sufHighest[m]
				if preHighest[m-1] > highest {
					highest = preHighest[m-1]
				}
				lowest = sufLowest[m]
				if preLowest[m-1] < lowest {
					lowest = preLowest[m-1]
				}
				outMax[outIdx] = highest
				outMin[outIdx] = lowest
				outIdx++
			}
			trailingIdx += nAvail
			today += nAvail
			blockStart += optInTimePeriod
		}
	}
	return startIdx, outIdx, nil
}
