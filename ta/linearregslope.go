package ta

import "math"

// LINEARREG_SLOPE — Linear Regression Slope (Statistic Functions).
// Ported from TA-Lib v0.8.1 ta_codegen/input/linearreg_slope and
// src/ta_func/ta_LINEARREG_SLOPE.c.
//
// m = (n*SumXY - SumX*SumY) / Divisor over the last optInTimePeriod bars,
// where x counts bars back from the current one (0 = today).
//
// TA-Lib is built with -ffp-contract=off, so no operation here is fused.
// Products are wrapped in float64(...) so the Go compiler cannot fuse them
// either (an explicit conversion forbids fusion), keeping results bit-identical
// to C.

// LINEARREGSLOPELookback returns the number of leading bars LINEARREG_SLOPE
// leaves empty. optInTimePeriod: [2, 100000], default 14.
func LINEARREGSLOPELookback(optInTimePeriod int) int {
	if checkInt("LINEARREG_SLOPE", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return linearregslopeLookback(optInTimePeriod)
}

func linearregslopeLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// LINEARREGSLOPE returns the slope of the least-squares line fitted over the
// last optInTimePeriod bars of inReal. The output is index-aligned with
// inReal; the first LINEARREGSLOPELookback bars are NaN.
// optInTimePeriod: [2, 100000], default 14.
func LINEARREGSLOPE(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("LINEARREG_SLOPE", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("LINEARREG_SLOPE", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := linearregslopeRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// linearregslopeRange mirrors TA_LINEARREG_SLOPE: it computes
// [startIdx, endIdx] and writes outNBElement values to outReal[0:], the first
// one belonging to bar outBegIdx. inReal and outReal may be the same slice.
func linearregslopeRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("LINEARREG_SLOPE", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("LINEARREG_SLOPE", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("LINEARREG_SLOPE", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := linearregslopeLookback(optInTimePeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	period := float64(optInTimePeriod)
	outIdx := 0
	today := startIdx
	trailingIdx := startIdx - lookbackTotal
	sumX := float64(float64(period*float64(optInTimePeriod-1)) * 0.5)
	sumXSqr := float64(float64(period*float64(optInTimePeriod-1))*float64(2*optInTimePeriod-1)) / 6.0
	divisor := float64(sumX*sumX) - float64(period*sumXSqr)

	// Priming scan: i runs period-1 .. 0 (C: for(i=period; i-- != 0;)).
	sumXY := 0.0
	sumY := 0.0
	sumAbs := 0.0
	for i := optInTimePeriod - 1; i >= 0; i-- {
		tempValue1 := inReal[today-i]
		sumY += tempValue1
		sumXY += float64(float64(i) * tempValue1)
		sumAbs += math.Abs(tempValue1)
	}
	barsSinceReseed := 32 * optInTimePeriod
	trailingValue := inReal[trailingIdx]
	trailingIdx++
	outReal[outIdx] = (float64(period*sumXY) - float64(sumX*sumY)) / divisor
	outIdx++
	today++

	for today <= endIdx {
		weightedTrailing := float64(period * trailingValue)
		sumXY = sumXY + sumY - weightedTrailing
		sumY = sumY - trailingValue + inReal[today]
		sumAbs = sumAbs - math.Abs(trailingValue) + math.Abs(inReal[today])
		// Re-anchor (C issue #254): every 32*period bars, or when the departing
		// value outweighs everything left in the window.
		barsSinceReseed--
		if barsSinceReseed <= 0 || math.Abs(weightedTrailing) > float64(100.0*sumAbs) {
			barsSinceReseed = 32 * optInTimePeriod
			windowStart := today - lookbackTotal
			sumY = 0
			sumXY = 0
			sumAbs = 0
			tempValue2 := float64(lookbackTotal)
			for j := windowStart; j <= today; j++ {
				tempValue1 := inReal[j]
				sumY += tempValue1
				sumXY += float64(tempValue2 * tempValue1)
				sumAbs += math.Abs(tempValue1)
				tempValue2 -= 1.0
			}
		}
		// Read the departing value before writing this bar's output: with
		// outReal aliasing inReal the write may land on that cell (C #130).
		trailingValue = inReal[trailingIdx]
		trailingIdx++
		outReal[outIdx] = (float64(period*sumXY) - float64(sumX*sumY)) / divisor
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
