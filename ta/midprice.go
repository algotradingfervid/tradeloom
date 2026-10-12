package ta

// MIDPRICE — Midpoint Price over period (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/midprice and src/ta_func/ta_MIDPRICE.c.

// MIDPRICELookback returns the number of leading bars MIDPRICE leaves empty.
// optInTimePeriod: [2, 100000], default 14.
func MIDPRICELookback(optInTimePeriod int) int {
	if checkInt("MIDPRICE", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return midpriceLookback(optInTimePeriod)
}

func midpriceLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// MIDPRICE returns (highest inHigh + lowest inLow) / 2 over the
// optInTimePeriod bars ending at each bar. The output is index-aligned with
// the inputs; the first MIDPRICELookback bars are NaN.
// optInTimePeriod: [2, 100000], default 14.
func MIDPRICE(inHigh, inLow []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("MIDPRICE", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("MIDPRICE", []string{"inHigh", "inLow"}, inHigh, inLow)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := midpriceRange(0, n-1, inHigh, inLow, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// midpriceRange mirrors TA_MIDPRICE: it computes [startIdx, endIdx] and
// writes outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx.
//
// The C uses a Van Herk / Gil-Werman block scan: per block of
// optInTimePeriod bars, a backward pass builds suffix extrema of the older
// block, a forward pass builds prefix extrema of the newer one, and a third
// pass combines them. The scratch arrays hold copies, so outReal may alias
// an input when the whole series is computed (startIdx <= lookback).
func midpriceRange(startIdx, endIdx int, inHigh, inLow []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("MIDPRICE", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MIDPRICE", endIdx, []string{"inHigh", "inLow"}, inHigh, inLow); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MIDPRICE", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
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

	var highest, lowest, tmpHigh, tmpLow float64
	blockStart := trailingIdx
	for today <= endIdx {
		// Suffix extrema of the block [blockStart, blockStart+p-1], scanned
		// backward keeping the incumbent on a tie.
		i := blockStart + optInTimePeriod - 1
		highest = inHigh[i]
		lowest = inLow[i]
		sufHighest[optInTimePeriod-1] = highest
		sufLowest[optInTimePeriod-1] = lowest
		for i > blockStart {
			i--
			tmpHigh = inHigh[i]
			if tmpHigh > highest {
				highest = tmpHigh
			}
			tmpLow = inLow[i]
			if tmpLow < lowest {
				lowest = tmpLow
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
			highest = inHigh[blockNext]
			lowest = inLow[blockNext]
			preHighest[0] = highest
			preLowest[0] = lowest
			i = 1
			for i < nAvail {
				tmpHigh = inHigh[blockNext+i]
				if tmpHigh > highest {
					highest = tmpHigh
				}
				tmpLow = inLow[blockNext+i]
				if tmpLow < lowest {
					lowest = tmpLow
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
