package ta

import "math"

// FOSC — Forecast Oscillator (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/fosc and src/ta_func/ta_FOSC.c.
//
// FOSC[t] = 100 * (P[t] - TSF[t-1]) / P[t], where TSF[t-1] is the linear
// regression forecast fitted over the optInTimePeriod bars ending at t-1 and
// evaluated one step beyond that window. A zero close yields 0.
//
// TA-Lib is built with -ffp-contract=off, so the only fused operation is the
// explicit fma() in the forecast. Products below are wrapped in float64(...)
// so the Go compiler cannot fuse them either (an explicit conversion forbids
// fusion), keeping the results bit-identical to C.

// FOSCLookback returns the number of leading bars FOSC leaves empty.
// optInTimePeriod: [2, 100000], default 5.
func FOSCLookback(optInTimePeriod int) int {
	if checkInt("FOSC", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return foscLookback(optInTimePeriod)
}

func foscLookback(optInTimePeriod int) int {
	return optInTimePeriod
}

// FOSC returns the Forecast Oscillator of inReal over optInTimePeriod bars.
// The output is index-aligned with inReal; the first FOSCLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 5.
func FOSC(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("FOSC", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("FOSC", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := foscRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// foscRange mirrors TA_FOSC: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func foscRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("FOSC", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("FOSC", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("FOSC", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := foscLookback(optInTimePeriod)
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
	sumXY := 0.0
	sumY := 0.0
	sumAbs := 0.0
	for i := optInTimePeriod - 1; i >= 0; i-- {
		tempValue1 := inReal[today-1-i]
		sumY += tempValue1
		sumXY += float64(float64(i) * tempValue1)
		sumAbs += math.Abs(tempValue1)
	}
	m := (float64(period*sumXY) - float64(sumX*sumY)) / divisor
	b := (sumY - float64(m*sumX)) / period
	barsSinceReseed := 32 * optInTimePeriod
	trailingValue := inReal[trailingIdx]
	trailingIdx++
	closeValue := inReal[today]
	if closeValue != 0.0 {
		outReal[outIdx] = float64(100.0*(closeValue-math.FMA(m, period, b))) / closeValue
	} else {
		outReal[outIdx] = 0.0
	}
	outIdx++
	today++

	for today <= endIdx {
		weightedTrailing := float64(period * trailingValue)
		sumXY = sumXY + sumY - weightedTrailing
		sumY = sumY - trailingValue + inReal[today-1]
		sumAbs = sumAbs - math.Abs(trailingValue) + math.Abs(inReal[today-1])
		barsSinceReseed--
		if barsSinceReseed <= 0 || math.Abs(weightedTrailing) > float64(100.0*sumAbs) {
			barsSinceReseed = 32 * optInTimePeriod
			windowStart := today - lookbackTotal
			sumY = 0
			sumXY = 0
			sumAbs = 0
			tempValue2 := float64(optInTimePeriod - 1)
			for j := windowStart; j < today; j++ {
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
		closeValue = inReal[today]
		if closeValue != 0.0 {
			outReal[outIdx] = float64(100.0*(closeValue-math.FMA(m, period, b))) / closeValue
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
