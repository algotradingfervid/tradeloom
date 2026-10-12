package ta

import "math"

// ULTOSC — Ultimate Oscillator (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/ultosc and src/ta_func/ta_ULTOSC.c.

// ULTOSCLookback returns the number of leading bars ULTOSC leaves empty.
// optInTimePeriod1, optInTimePeriod2, optInTimePeriod3: [1, 100000],
// defaults 7, 14, 28.
func ULTOSCLookback(optInTimePeriod1, optInTimePeriod2, optInTimePeriod3 int) int {
	if ultoscCheckParams(optInTimePeriod1, optInTimePeriod2, optInTimePeriod3) != nil {
		return -1
	}
	return ultoscLookback(optInTimePeriod1, optInTimePeriod2, optInTimePeriod3)
}

// ultoscLookback is the SMA lookback of the longest period plus 1 for the
// true range.
func ultoscLookback(optInTimePeriod1, optInTimePeriod2, optInTimePeriod3 int) int {
	maxPeriod := cMaxInt(cMaxInt(optInTimePeriod1, optInTimePeriod2), optInTimePeriod3)
	return smaLookback(maxPeriod) + 1
}

func ultoscCheckParams(optInTimePeriod1, optInTimePeriod2, optInTimePeriod3 int) error {
	if err := checkInt("ULTOSC", "optInTimePeriod1", optInTimePeriod1, 1, 100000); err != nil {
		return err
	}
	if err := checkInt("ULTOSC", "optInTimePeriod2", optInTimePeriod2, 1, 100000); err != nil {
		return err
	}
	return checkInt("ULTOSC", "optInTimePeriod3", optInTimePeriod3, 1, 100000)
}

// ULTOSC returns the Ultimate Oscillator of the high/low/close series. The
// three periods are sorted internally, so the 4/2/1 weighting always applies
// to the shortest, middle and longest period. The output is index-aligned with
// the inputs; the first ULTOSCLookback bars are NaN.
func ULTOSC(inHigh, inLow, inClose []float64, optInTimePeriod1, optInTimePeriod2, optInTimePeriod3 int) ([]float64, error) {
	if err := ultoscCheckParams(optInTimePeriod1, optInTimePeriod2, optInTimePeriod3); err != nil {
		return nil, err
	}
	n, err := checkInputs("ULTOSC", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := ultoscRange(0, n-1, inHigh, inLow, inClose, optInTimePeriod1, optInTimePeriod2, optInTimePeriod3, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// ultoscRange mirrors TA_ULTOSC. outReal may alias an input slice.
func ultoscRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInTimePeriod1, optInTimePeriod2, optInTimePeriod3 int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := ultoscCheckParams(optInTimePeriod1, optInTimePeriod2, optInTimePeriod3); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ULTOSC", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ULTOSC", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// Sort the periods: optInTimePeriod1 becomes the shortest,
	// optInTimePeriod3 the longest.
	periods := [3]int{optInTimePeriod1, optInTimePeriod2, optInTimePeriod3}
	var usedFlag [3]int
	var sortedPeriods [3]int
	for i := 0; i < 3; i++ {
		longestPeriod := 0
		longestIndex := 0
		for j := 0; j < 3; j++ {
			if usedFlag[j] == 0 && periods[j] > longestPeriod {
				longestPeriod = periods[j]
				longestIndex = j
			}
		}
		usedFlag[longestIndex] = 1
		sortedPeriods[i] = longestPeriod
	}
	optInTimePeriod1 = sortedPeriods[2]
	optInTimePeriod2 = sortedPeriods[1]
	optInTimePeriod3 = sortedPeriods[0]

	lookbackTotal := ultoscLookback(optInTimePeriod1, optInTimePeriod2, optInTimePeriod3)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// Ring buffers of per-bar terms, one entry per bar of the longest window.
	termCloseMinusTrueLow := make([]float64, optInTimePeriod3)
	termTrueRange := make([]float64, optInTimePeriod3)
	maxIdxTerm := optInTimePeriod3 - 1
	termIdx := 0

	var a1Total, b1Total, a2Total, b2Total, a3Total, b3Total float64
	nullRun := 0
	for i := startIdx - optInTimePeriod3 + 1; i < startIdx; i++ {
		tempLT := inLow[i]
		tempHT := inHigh[i]
		tempCY := inClose[i-1]
		trueLow := cMin(tempLT, tempCY)
		closeMinusTrueLow := inClose[i] - trueLow
		trueRange := tempHT - tempLT
		tempDouble := math.Abs(tempCY - tempHT)
		if tempDouble > trueRange {
			trueRange = tempDouble
		}
		tempDouble = math.Abs(tempCY - tempLT)
		if tempDouble > trueRange {
			trueRange = tempDouble
		}
		termCloseMinusTrueLow[termIdx] = closeMinusTrueLow
		termTrueRange[termIdx] = trueRange
		termIdx++
		if termIdx > maxIdxTerm {
			termIdx = 0
		}
		if trueRange == 0.0 && closeMinusTrueLow == 0.0 {
			nullRun++
		} else {
			nullRun = 0
		}
		if i >= startIdx-optInTimePeriod1+1 {
			a1Total += closeMinusTrueLow
			b1Total += trueRange
		}
		if i >= startIdx-optInTimePeriod2+1 {
			a2Total += closeMinusTrueLow
			b2Total += trueRange
		}
		a3Total += closeMinusTrueLow
		b3Total += trueRange
	}

	today := startIdx
	outIdx := 0
	trailingPos1 := termIdx + optInTimePeriod3 - optInTimePeriod1 + 1
	if trailingPos1 >= optInTimePeriod3 {
		trailingPos1 -= optInTimePeriod3
	}
	trailingPos2 := termIdx + optInTimePeriod3 - optInTimePeriod2 + 1
	if trailingPos2 >= optInTimePeriod3 {
		trailingPos2 -= optInTimePeriod3
	}
	for today <= endIdx {
		tempLT := inLow[today]
		tempHT := inHigh[today]
		tempCY := inClose[today-1]
		trueLow := cMin(tempLT, tempCY)
		closeMinusTrueLow := inClose[today] - trueLow
		trueRange := tempHT - tempLT
		tempDouble := math.Abs(tempCY - tempHT)
		if tempDouble > trueRange {
			trueRange = tempDouble
		}
		tempDouble = math.Abs(tempCY - tempLT)
		if tempDouble > trueRange {
			trueRange = tempDouble
		}
		termCloseMinusTrueLow[termIdx] = closeMinusTrueLow
		termTrueRange[termIdx] = trueRange
		a1Total += closeMinusTrueLow
		a2Total += closeMinusTrueLow
		a3Total += closeMinusTrueLow
		b1Total += trueRange
		b2Total += trueRange
		b3Total += trueRange

		// Once a whole window of no-contribution bars has gone by, its
		// totals are exactly zero and the rounding residue is dropped.
		if trueRange == 0.0 && closeMinusTrueLow == 0.0 {
			nullRun++
		} else {
			nullRun = 0
		}
		if nullRun >= optInTimePeriod1 {
			a1Total = 0.0
			b1Total = 0.0
			if nullRun >= optInTimePeriod2 {
				a2Total = 0.0
				b2Total = 0.0
				if nullRun >= optInTimePeriod3 {
					nullRun = optInTimePeriod3
					a3Total = 0.0
					b3Total = 0.0
				}
			}
		}

		output := 0.0
		if b1Total > 0.0 {
			output += float64(4.0 * (a1Total / b1Total))
		}
		if b2Total > 0.0 {
			output += 2.0 * (a2Total / b2Total)
		}
		if b3Total > 0.0 {
			output += a3Total / b3Total
		}

		a1Total -= termCloseMinusTrueLow[trailingPos1]
		b1Total -= termTrueRange[trailingPos1]
		trailingPos1++
		if trailingPos1 >= optInTimePeriod3 {
			trailingPos1 = 0
		}
		a2Total -= termCloseMinusTrueLow[trailingPos2]
		b2Total -= termTrueRange[trailingPos2]
		trailingPos2++
		if trailingPos2 >= optInTimePeriod3 {
			trailingPos2 = 0
		}
		termIdx++
		if termIdx > maxIdxTerm {
			termIdx = 0
		}
		a3Total -= termCloseMinusTrueLow[termIdx]
		b3Total -= termTrueRange[termIdx]

		outReal[outIdx] = 100.0 * (output / 7.0)
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
