package ta

// AROONOSC — Aroon Oscillator (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/aroonosc and src/ta_func/ta_AROONOSC.c.

// AROONOSCLookback returns the number of leading bars AROONOSC leaves empty.
// optInTimePeriod: [2, 100000], default 14.
func AROONOSCLookback(optInTimePeriod int) int {
	if checkInt("AROONOSC", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return aroonoscLookback(optInTimePeriod)
}

func aroonoscLookback(optInTimePeriod int) int {
	return optInTimePeriod
}

// AROONOSC returns AroonUp - AroonDown over optInTimePeriod bars, which is
// (100/optInTimePeriod) * (highestIdx - lowestIdx) where the indices locate
// the highest high and lowest low in the last optInTimePeriod+1 bars (ties go
// to the most recent bar). The output is index-aligned with the inputs; the
// first AROONOSCLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 14.
func AROONOSC(inHigh, inLow []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("AROONOSC", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("AROONOSC", []string{"inHigh", "inLow"}, inHigh, inLow)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := aroonoscRange(0, n-1, inHigh, inLow, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// aroonoscRange mirrors TA_AROONOSC: it computes [startIdx, endIdx] and
// writes outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may be the same slice as inHigh or inLow.
func aroonoscRange(startIdx, endIdx int, inHigh, inLow []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("AROONOSC", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("AROONOSC", endIdx, []string{"inHigh", "inLow"}, inHigh, inLow); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("AROONOSC", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	if startIdx < optInTimePeriod {
		startIdx = optInTimePeriod
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	outIdx := 0
	today := startIdx
	trailingIdx := startIdx - optInTimePeriod
	lowestIdx := -1
	highestIdx := -1
	lowest := 0.0
	highest := 0.0
	factor := 100.0 / float64(optInTimePeriod)
	for today <= endIdx {
		// Keep track of the lowestIdx.
		tmp := inLow[today]
		if lowestIdx < trailingIdx {
			lowestIdx = trailingIdx
			lowest = inLow[lowestIdx]
			for i := lowestIdx + 1; i <= today; i++ {
				tmp = inLow[i]
				if tmp <= lowest {
					lowestIdx = i
					lowest = tmp
				}
			}
		} else if tmp <= lowest {
			lowestIdx = today
			lowest = tmp
		}
		// Keep track of the highestIdx.
		tmp = inHigh[today]
		if highestIdx < trailingIdx {
			highestIdx = trailingIdx
			highest = inHigh[highestIdx]
			for i := highestIdx + 1; i <= today; i++ {
				tmp = inHigh[i]
				if tmp >= highest {
					highestIdx = i
					highest = tmp
				}
			}
		} else if tmp >= highest {
			highestIdx = today
			highest = tmp
		}
		// AroonOsc = factor*(highestIdx-lowestIdx). Written last because the
		// output may alias an input.
		outReal[outIdx] = factor * float64(highestIdx-lowestIdx)
		outIdx++
		trailingIdx++
		today++
	}
	return startIdx, outIdx, nil
}
