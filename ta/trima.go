package ta

// TRIMA — Triangular Moving Average (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/trima and src/ta_func/ta_TRIMA.c.

// TRIMALookback returns the number of leading bars TRIMA leaves empty.
// optInTimePeriod: [1, 100000], default 30.
func TRIMALookback(optInTimePeriod int) int {
	if checkInt("TRIMA", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return trimaLookback(optInTimePeriod)
}

func trimaLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// TRIMA returns the triangular moving average of inReal over optInTimePeriod
// bars (an SMA of an SMA, weighting the middle of the window most heavily).
// The output is index-aligned with inReal; the first TRIMALookback bars are NaN.
// optInTimePeriod: [1, 100000], default 30.
func TRIMA(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("TRIMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("TRIMA", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := trimaRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// trimaRange mirrors TA_TRIMA: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func trimaRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("TRIMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TRIMA", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TRIMA", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := optInTimePeriod - 1
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	var numerator, numeratorSub, numeratorAdd, factor, tempReal float64
	var i, outIdx, todayIdx, trailingIdx, middleIdx int

	if optInTimePeriod%2 == 1 {
		// Odd period: weights sum to (n+1)^2 with n = period>>1.
		i = optInTimePeriod >> 1
		factor = float64(i+1) * float64(i+1)
		factor = 1.0 / factor

		trailingIdx = startIdx - lookbackTotal
		middleIdx = trailingIdx + i
		todayIdx = middleIdx + i
		numerator = 0.0
		numeratorSub = 0.0
		for i = middleIdx; i >= trailingIdx; i-- {
			tempReal = inReal[i]
			numeratorSub += tempReal
			numerator += numeratorSub
		}
		numeratorAdd = 0.0
		middleIdx++
		for i = middleIdx; i <= todayIdx; i++ {
			tempReal = inReal[i]
			numeratorAdd += tempReal
			numerator += numeratorAdd
		}

		// First output; the trailing value is saved before writing in case
		// inReal and outReal share a buffer.
		outIdx = 0
		tempReal = inReal[trailingIdx]
		trailingIdx++
		outReal[outIdx] = numerator * factor
		outIdx++
		todayIdx++

		for todayIdx <= endIdx {
			// Step (1)
			numerator -= numeratorSub
			numeratorSub -= tempReal
			tempReal = inReal[middleIdx]
			middleIdx++
			numeratorSub += tempReal
			// Step (2)
			numerator += numeratorAdd
			numeratorAdd -= tempReal
			tempReal = inReal[todayIdx]
			todayIdx++
			numeratorAdd += tempReal
			// Step (3)
			numerator += tempReal
			// Step (4)
			tempReal = inReal[trailingIdx]
			trailingIdx++
			outReal[outIdx] = numerator * factor
			outIdx++
		}
	} else {
		// Even period: weights sum to n*(n+1) with n = period>>1.
		i = optInTimePeriod >> 1
		factor = float64(i) * float64(i+1)
		factor = 1.0 / factor

		trailingIdx = startIdx - lookbackTotal
		middleIdx = trailingIdx + i - 1
		todayIdx = middleIdx + i
		numerator = 0.0
		numeratorSub = 0.0
		for i = middleIdx; i >= trailingIdx; i-- {
			tempReal = inReal[i]
			numeratorSub += tempReal
			numerator += numeratorSub
		}
		numeratorAdd = 0.0
		middleIdx++
		for i = middleIdx; i <= todayIdx; i++ {
			tempReal = inReal[i]
			numeratorAdd += tempReal
			numerator += numeratorAdd
		}

		outIdx = 0
		tempReal = inReal[trailingIdx]
		trailingIdx++
		outReal[outIdx] = numerator * factor
		outIdx++
		todayIdx++

		for todayIdx <= endIdx {
			// Step (1)
			numerator -= numeratorSub
			numeratorSub -= tempReal
			tempReal = inReal[middleIdx]
			middleIdx++
			numeratorSub += tempReal
			// Step (2): note the order differs from the odd case.
			numeratorAdd -= tempReal
			numerator += numeratorAdd
			tempReal = inReal[todayIdx]
			todayIdx++
			numeratorAdd += tempReal
			// Step (3)
			numerator += tempReal
			// Step (4)
			tempReal = inReal[trailingIdx]
			trailingIdx++
			outReal[outIdx] = numerator * factor
			outIdx++
		}
	}
	return startIdx, outIdx, nil
}
