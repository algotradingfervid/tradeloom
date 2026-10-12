package ta

import "math"

// LINEARREG_ANGLE — Linear Regression Angle (Statistic Functions).
// Ported from TA-Lib v0.8.1 ta_codegen/input/linearreg_angle and
// src/ta_func/ta_LINEARREG_ANGLE.c (the running-sums / reseed variant of the
// compiled C).
//
// The C library is built with -ffp-contract=off. Go may fuse x*y+z into an
// FMA (also across statements), so every product that feeds an addition or
// subtraction is wrapped in an explicit float64() conversion, which the spec
// guarantees rounds and therefore blocks fusion.

// LINEARREGANGLELookback returns the number of leading bars LINEARREG_ANGLE
// leaves empty. optInTimePeriod: [2, 100000], default 14.
func LINEARREGANGLELookback(optInTimePeriod int) int {
	if checkInt("LINEARREG_ANGLE", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return linearregangleLookback(optInTimePeriod)
}

func linearregangleLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// LINEARREGANGLE returns the angle, in degrees, of the least-squares line
// fitted to the last optInTimePeriod values of inReal. The output is
// index-aligned with inReal; the first LINEARREGANGLELookback bars are NaN.
// optInTimePeriod: [2, 100000], default 14.
func LINEARREGANGLE(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("LINEARREG_ANGLE", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("LINEARREG_ANGLE", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := linearregangleRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// linearregangleRange mirrors TA_LINEARREG_ANGLE: it computes
// [startIdx, endIdx] and writes outNBElement values to outReal[0:], the first
// one belonging to bar outBegIdx. inReal and outReal may be the same slice.
func linearregangleRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("LINEARREG_ANGLE", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("LINEARREG_ANGLE", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("LINEARREG_ANGLE", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := linearregangleLookback(optInTimePeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	const radToDeg = 180.0 / 3.141592653589793
	p := float64(optInTimePeriod)

	outIdx := 0
	today := startIdx
	trailingIdx := startIdx - lookbackTotal
	sumX := p * float64(optInTimePeriod-1) * 0.5
	sumXSqr := p * float64(optInTimePeriod-1) * float64(2*optInTimePeriod-1) / 6.0
	divisor := float64(sumX*sumX) - float64(p*sumXSqr)

	// Prime the window sums with a full scan, oldest first, weight counting down.
	sumXY := 0.0
	sumY := 0.0
	sumAbs := 0.0
	for i := optInTimePeriod - 1; i >= 0; i-- {
		tempValue1 := inReal[today-i]
		sumY += tempValue1
		sumXY += float64(float64(i) * tempValue1)
		sumAbs += math.Abs(tempValue1)
	}
	m := (float64(p*sumXY) - float64(sumX*sumY)) / divisor
	barsSinceReseed := 32 * optInTimePeriod
	trailingValue := inReal[trailingIdx]
	trailingIdx++
	outReal[outIdx] = float64(libmAtan(m) * radToDeg)
	outIdx++
	today++

	for today <= endIdx {
		weightedTrailing := float64(p * trailingValue)
		sumXY = sumXY + sumY - weightedTrailing
		sumY = sumY - trailingValue + inReal[today]
		sumAbs = sumAbs - math.Abs(trailingValue) + math.Abs(inReal[today])

		// Re-anchor every 32*period bars, or when the departing value
		// outweighs the whole remaining window (see the C source, #254).
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
		m = (float64(p*sumXY) - float64(sumX*sumY)) / divisor
		trailingValue = inReal[trailingIdx]
		trailingIdx++
		outReal[outIdx] = float64(libmAtan(m) * radToDeg)
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
