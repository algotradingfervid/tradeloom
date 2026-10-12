package ta

import "math"

// TSF — Time Series Forecast (Statistic Functions).
// Ported from TA-Lib v0.8.1 ta_codegen/input/tsf and src/ta_func/ta_TSF.c.
//
// For each bar, a least-squares line y = b + m*x is fitted over the last
// optInTimePeriod bars and extrapolated one bar past the window,
// b + m*period, which is returned.
//
// TA-Lib is built with -ffp-contract=off, so the only fused operation is the
// explicit fma() in the output. Products below are wrapped in float64(...)
// so the Go compiler cannot fuse them either (an explicit conversion forbids
// fusion), keeping the results bit-identical to C.

// TSFLookback returns the number of leading bars TSF leaves empty.
// optInTimePeriod: [2, 100000], default 14.
func TSFLookback(optInTimePeriod int) int {
	if checkInt("TSF", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return tsfLookback(optInTimePeriod)
}

func tsfLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// TSF returns the time series forecast of inReal over
// optInTimePeriod bars. The output is index-aligned with inReal; the first
// TSFLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 14.
func TSF(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("TSF", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("TSF", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := tsfRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// tsfRange mirrors TA_TSF: it computes [startIdx, endIdx] and
// writes outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func tsfRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("TSF", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TSF", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TSF", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := tsfLookback(optInTimePeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	period := float64(optInTimePeriod)
	periodM1 := float64(optInTimePeriod - 1)
	outIdx := 0
	today := startIdx
	trailingIdx := startIdx - lookbackTotal
	sumX := float64(float64(period*periodM1) * 0.5)
	sumXSqr := float64(float64(period*periodM1)*float64(2*optInTimePeriod-1)) / 6.0
	divisor := float64(sumX*sumX) - float64(period*sumXSqr)

	// Prime the window sums with a full scan, weight counting down.
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
	b := (sumY - float64(m*sumX)) / period
	barsSinceReseed := 32 * optInTimePeriod
	trailingValue := inReal[trailingIdx]
	trailingIdx++
	outReal[outIdx] = math.FMA(m, period, b)
	outIdx++
	today++

	for today <= endIdx {
		weightedTrailing := float64(period * trailingValue)
		sumXY = sumXY + sumY - weightedTrailing
		sumY = sumY - trailingValue + inReal[today]
		sumAbs = sumAbs - math.Abs(trailingValue) + math.Abs(inReal[today])
		// Re-anchor every 32*period bars, or when the departing value
		// outweighs the whole window (TA-Lib issue #254).
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
		b = (sumY - float64(m*sumX)) / period
		trailingValue = inReal[trailingIdx]
		trailingIdx++
		outReal[outIdx] = math.FMA(m, period, b)
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
