package ta

// VAR — Variance (Statistic Functions).
// Ported from TA-Lib v0.8.1 ta_codegen/input/var and src/ta_func/ta_VAR.c
// (the shifted-sums / reseed variant of the compiled C).
//
// The C library is built with -ffp-contract=off. Go may fuse x*y+z into an
// FMA (also across statements), so every product that feeds an addition is
// wrapped in an explicit float64() conversion, which the spec guarantees
// rounds and therefore blocks fusion.

// VARLookback returns the number of leading bars VAR leaves empty.
// optInTimePeriod: [1, 100000], default 5. optInNbDev: [-3e37, 3e37], default 1 (unused).
func VARLookback(optInTimePeriod int, optInNbDev float64) int {
	if checkInt("VAR", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	if checkReal("VAR", "optInNbDev", optInNbDev, taRealMin, taRealMax) != nil {
		return -1
	}
	return varLookback(optInTimePeriod, optInNbDev)
}

func varLookback(optInTimePeriod int, optInNbDev float64) int {
	return optInTimePeriod - 1
}

// VAR returns the rolling population variance of inReal over optInTimePeriod
// bars. The output is index-aligned with inReal; the first VARLookback bars are
// NaN. optInNbDev is validated but, as in TA-Lib, does not affect the result.
// optInTimePeriod: [1, 100000], default 5. optInNbDev: [-3e37, 3e37], default 1.
func VAR(inReal []float64, optInTimePeriod int, optInNbDev float64) ([]float64, error) {
	if err := checkInt("VAR", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	if err := checkReal("VAR", "optInNbDev", optInNbDev, taRealMin, taRealMax); err != nil {
		return nil, err
	}
	n, err := checkInputs("VAR", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := varRange(0, n-1, inReal, optInTimePeriod, optInNbDev, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// varRange mirrors TA_VAR: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func varRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, optInNbDev float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("VAR", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkReal("VAR", "optInNbDev", optInNbDev, taRealMin, taRealMax); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("VAR", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("VAR", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}
	return varCore(startIdx, endIdx, inReal, optInTimePeriod, outReal)
}

// varCore is the TA_VAR body after validation.
func varCore(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (int, int, error) {
	nbInitialElementNeeded := optInTimePeriod - 1
	if startIdx < nbInitialElementNeeded {
		startIdx = nbInitialElementNeeded
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}
	invPeriod := 1.0 / float64(optInTimePeriod)

	trailingIdx := startIdx - nbInitialElementNeeded
	shift := inReal[trailingIdx]
	periodTotal1 := 0.0
	periodTotal2 := 0.0
	var tempReal float64
	for j := trailingIdx; j < startIdx; j++ {
		tempReal = inReal[j] - shift
		periodTotal1 += tempReal
		tempReal = float64(tempReal * tempReal)
		periodTotal2 += tempReal
	}

	i := startIdx
	outIdx := 0
	barsSinceReseed := 32 * optInTimePeriod
	for {
		// Add the incoming value, measured against the shift.
		tempReal = inReal[i] - shift
		periodTotal1 += tempReal
		tempReal = float64(tempReal * tempReal)
		periodTotal2 += tempReal
		meanValue1 := float64(periodTotal1 * invPeriod)
		variance := float64(periodTotal2*invPeriod) - float64(meanValue1*meanValue1)
		// Remove the trailing value (prepares the next window).
		tempReal = inReal[trailingIdx] - shift
		periodTotal1 -= tempReal
		tempReal = float64(tempReal * tempReal)
		periodTotal2 -= tempReal
		trailingIdx++

		barsSinceReseed--
		if variance < float64(0.000001*float64(periodTotal2*invPeriod)) ||
			tempReal > float64(1000000.0*periodTotal2) || barsSinceReseed <= 0 {
			barsSinceReseed = 32 * optInTimePeriod
			windowStart := i - nbInitialElementNeeded
			tempReal = 0.0
			for j := windowStart; j <= i; j++ {
				tempReal += inReal[j]
			}
			shift = float64(tempReal * invPeriod)
			periodTotal1 = 0.0
			periodTotal2 = 0.0
			for j := windowStart; j <= i; j++ {
				tempReal = inReal[j] - shift
				periodTotal1 += tempReal
				tempReal = float64(tempReal * tempReal)
				periodTotal2 += tempReal
			}
			meanValue1 = float64(periodTotal1 * invPeriod)
			variance = float64(periodTotal2*invPeriod) - float64(meanValue1*meanValue1)
			// Floor the fresh figure (scale-relative dead zone, see ta_VAR.c).
			if variance < float64(0.000000000001*float64(periodTotal2*invPeriod)) {
				variance = 0.0
			}
			// Re-remove the trailing value under the new shift.
			tempReal = inReal[windowStart] - shift
			periodTotal1 -= tempReal
			tempReal = float64(tempReal * tempReal)
			periodTotal2 -= tempReal
		}
		outReal[outIdx] = variance
		outIdx++
		i++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
