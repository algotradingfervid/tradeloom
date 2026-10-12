package ta

import "math"

// WILLR — Williams' %R (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/willr and src/ta_func/ta_WILLR.c.
//
// The compiled C uses a Van Herk / Gil-Werman block scan (suffix extrema of
// the older block combined with prefix extrema of the newer block); this port
// reproduces it statement by statement.

// WILLRLookback returns the number of leading bars WILLR leaves empty.
// optInTimePeriod: [2, 100000], default 14.
func WILLRLookback(optInTimePeriod int) int {
	if checkInt("WILLR", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return willrLookback(optInTimePeriod)
}

func willrLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// WILLR returns Williams' %R of (inHigh, inLow, inClose) over optInTimePeriod
// bars, in [-100, 0]. The output is index-aligned with the inputs; the first
// WILLRLookback bars are NaN. optInTimePeriod: [2, 100000], default 14.
func WILLR(inHigh, inLow, inClose []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("WILLR", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("WILLR", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := willrRange(0, n-1, inHigh, inLow, inClose, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// willrValue is the guarded %R expression shared by both emit sites of
// TA_WILLR.
func willrValue(highest, lowest, close float64) float64 {
	if !isZeroScaled(highest-lowest, math.Abs(highest)+math.Abs(lowest)) {
		tempReal := (highest - close) / (highest - lowest) * (0 - 100.0)
		if tempReal > 0.0 {
			tempReal = 0.0
		} else if tempReal < 0-100.0 {
			tempReal = 0 - 100.0
		}
		return tempReal
	}
	return 0.0
}

// willrRange mirrors TA_WILLR: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may be one of the input slices.
func willrRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("WILLR", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("WILLR", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("WILLR", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
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
	// Scratch arrays hold copies, so inputs and output may alias.
	sufHighest := make([]float64, optInTimePeriod)
	preHighest := make([]float64, optInTimePeriod)
	sufLowest := make([]float64, optInTimePeriod)
	preLowest := make([]float64, optInTimePeriod)

	blockStart := trailingIdx
	for today <= endIdx {
		// Suffix extrema of the block [blockStart, blockStart+p-1].
		i := blockStart + optInTimePeriod - 1
		highest := inHigh[i]
		lowest := inLow[i]
		sufHighest[optInTimePeriod-1] = highest
		sufLowest[optInTimePeriod-1] = lowest
		for i > blockStart {
			i--
			tmp := inHigh[i]
			if tmp > highest {
				highest = tmp
			}
			tmp = inLow[i]
			if tmp < lowest {
				lowest = tmp
			}
			sufHighest[i-blockStart] = highest
			sufLowest[i-blockStart] = lowest
		}
		highest = sufHighest[0]
		lowest = sufLowest[0]
		outReal[outIdx] = willrValue(highest, lowest, inClose[today])
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
			highest = inHigh[blockNext]
			lowest = inLow[blockNext]
			preHighest[0] = highest
			preLowest[0] = lowest
			i = 1
			for i < nAvail {
				tmp := inHigh[blockNext+i]
				if tmp > highest {
					highest = tmp
				}
				tmp = inLow[blockNext+i]
				if tmp < lowest {
					lowest = tmp
				}
				preHighest[i] = highest
				preLowest[i] = lowest
				i++
			}
			// Combine and emit; bar for offset m is today+m-1.
			for m := 1; m <= nAvail; m++ {
				highest = sufHighest[m]
				if preHighest[m-1] > highest {
					highest = preHighest[m-1]
				}
				lowest = sufLowest[m]
				if preLowest[m-1] < lowest {
					lowest = preLowest[m-1]
				}
				outReal[outIdx] = willrValue(highest, lowest, inClose[today+m-1])
				outIdx++
			}
			trailingIdx += nAvail
			today += nAvail
			blockStart += optInTimePeriod
		}
	}
	return startIdx, outIdx, nil
}
