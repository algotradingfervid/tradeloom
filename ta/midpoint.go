package ta

// MIDPOINT — MidPoint over period (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/midpoint and src/ta_func/ta_MIDPOINT.c.

// MIDPOINTLookback returns the number of leading bars MIDPOINT leaves empty.
// optInTimePeriod: [2, 100000], default 14.
func MIDPOINTLookback(optInTimePeriod int) int {
	if checkInt("MIDPOINT", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return midpointLookback(optInTimePeriod)
}

func midpointLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// MIDPOINT returns (highest + lowest) / 2 of inReal over optInTimePeriod bars.
// The output is index-aligned with inReal; the first MIDPOINTLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 14.
func MIDPOINT(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("MIDPOINT", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("MIDPOINT", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := midpointRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// midpointRange mirrors TA_MIDPOINT: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
//
// The C uses a Van Herk / Gil-Werman block scan (suffix extrema of the older
// block, prefix extrema of the newer block, combined); it is ported as is.
func midpointRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("MIDPOINT", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MIDPOINT", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MIDPOINT", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
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

	sufHighest := make([]float64, optInTimePeriod)
	preHighest := make([]float64, optInTimePeriod)
	sufLowest := make([]float64, optInTimePeriod)
	preLowest := make([]float64, optInTimePeriod)

	blockStart := trailingIdx
	for today <= endIdx {
		// Suffix extrema of the block [blockStart, blockStart+p-1].
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
		outReal[outIdx] = (sufHighest[0] + sufLowest[0]) / 2.0
		outIdx++
		trailingIdx++
		today++
		if today > endIdx {
			blockStart = blockStart + optInTimePeriod
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
			// Combine; the suffix half is older and wins ties.
			for m := 1; m <= nAvail; m++ {
				highest = sufHighest[m]
				if preHighest[m-1] > highest {
					highest = preHighest[m-1]
				}
				lowest = sufLowest[m]
				if preLowest[m-1] < lowest {
					lowest = preLowest[m-1]
				}
				outReal[outIdx] = (highest + lowest) / 2.0
				outIdx++
			}
			trailingIdx = trailingIdx + nAvail
			today = today + nAvail
			blockStart = blockStart + optInTimePeriod
		}
	}
	return startIdx, outIdx, nil
}
