package ta

import "math"

// LINEARREGINTERCEPT — Linear Regression Intercept (Statistic Functions).
// Ported from TA-Lib v0.8.1 ta_codegen/input/linearreg_intercept and
// src/ta_func/ta_LINEARREG_INTERCEPT.c.
//
// For each bar a least-squares line is fitted over the last optInTimePeriod
// values and its intercept b (the fitted value at the oldest bar of the
// window) is returned.
//
// TA-Lib is built with -ffp-contract=off, so C performs no fused
// multiply-adds here. Products below are wrapped in float64(...) so the Go
// compiler cannot fuse them either (an explicit conversion forbids fusion),
// keeping the results bit-identical to C.

// LINEARREGINTERCEPTLookback returns the number of leading bars
// LINEARREGINTERCEPT leaves empty. optInTimePeriod: [2, 100000], default 14.
func LINEARREGINTERCEPTLookback(optInTimePeriod int) int {
	if checkInt("LINEARREG_INTERCEPT", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return linearreginterceptLookback(optInTimePeriod)
}

func linearreginterceptLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// LINEARREGINTERCEPT returns the linear regression intercept of inReal over
// optInTimePeriod bars. The output is index-aligned with inReal; the first
// LINEARREGINTERCEPTLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 14.
func LINEARREGINTERCEPT(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("LINEARREG_INTERCEPT", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("LINEARREG_INTERCEPT", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := linearreginterceptRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// linearreginterceptRange mirrors TA_LINEARREG_INTERCEPT: it computes
// [startIdx, endIdx] and writes outNBElement values to outReal[0:], the first
// one belonging to bar outBegIdx. inReal and outReal may be the same slice.
func linearreginterceptRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("LINEARREG_INTERCEPT", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("LINEARREG_INTERCEPT", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("LINEARREG_INTERCEPT", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := linearreginterceptLookback(optInTimePeriod)
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

	// Prime the window sums with a full scan, oldest first, weight = bars ago.
	sumXY := 0.0
	sumY := 0.0
	sumAbs := 0.0
	for i := optInTimePeriod - 1; i >= 0; i-- {
		tempValue1 := inReal[today-i]
		sumY += tempValue1
		sumXY += float64(float64(i) * tempValue1)
		sumAbs += math.Abs(tempValue1)
	}
	m := (float64(period*sumXY) - float64(sumX*sumY)) / divisor
	barsSinceReseed := 32 * optInTimePeriod
	trailingValue := inReal[trailingIdx]
	trailingIdx++
	outReal[outIdx] = (sumY - float64(m*sumX)) / period
	outIdx++
	today++

	for today <= endIdx {
		weightedTrailing := float64(period * trailingValue)
		sumXY = sumXY + sumY - weightedTrailing
		sumY = sumY - trailingValue + inReal[today]
		sumAbs = sumAbs - math.Abs(trailingValue) + math.Abs(inReal[today])
		// Re-anchor (C issue #254): rebuild both sums from the window every
		// 32*period bars, or when the departing value outweighs the window.
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
		m = (float64(period*sumXY) - float64(sumX*sumY)) / divisor
		// Read the departing value before writing this bar's output: with
		// outReal aliasing inReal the write lands on that cell (C issue #130).
		trailingValue = inReal[trailingIdx]
		trailingIdx++
		outReal[outIdx] = (sumY - float64(m*sumX)) / period
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
