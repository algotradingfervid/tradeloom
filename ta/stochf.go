package ta

import "math"

// STOCHF — Stochastic Fast (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/stochf and src/ta_func/ta_STOCHF.c.

// stochfCheck validates the STOCHF optional inputs and applies the C
// prologue's MATypeDefault substitution (TA_MAType_DEFAULT -> SMA).
func stochfCheck(optInFastKPeriod, optInFastDPeriod int, optInFastDMAType MAType) (MAType, error) {
	if err := checkInt("STOCHF", "optInFastK_Period", optInFastKPeriod, 1, 100000); err != nil {
		return optInFastDMAType, err
	}
	if err := checkInt("STOCHF", "optInFastD_Period", optInFastDPeriod, 1, 100000); err != nil {
		return optInFastDMAType, err
	}
	if err := checkMAType("STOCHF", "optInFastD_MAType", optInFastDMAType); err != nil {
		return optInFastDMAType, err
	}
	if optInFastDMAType == MATypeDefault {
		optInFastDMAType = MATypeSMA
	}
	return optInFastDMAType, nil
}

// STOCHFLookback returns the number of leading bars STOCHF leaves empty, or
// -1 on an invalid parameter.
// optInFastK_Period: [1, 100000], default 5. optInFastD_Period: [1, 100000],
// default 3. optInFastD_MAType: default SMA.
func STOCHFLookback(optInFastK_Period, optInFastD_Period int, optInFastD_MAType MAType) int {
	t, err := stochfCheck(optInFastK_Period, optInFastD_Period, optInFastD_MAType)
	if err != nil {
		return -1
	}
	return stochfLookback(optInFastK_Period, optInFastD_Period, t)
}

// stochfLookback is the unvalidated tail of TA_STOCHF_Lookback; the MAType
// must already have MATypeDefault substituted.
func stochfLookback(optInFastK_Period, optInFastD_Period int, optInFastD_MAType MAType) int {
	// Account for the initial data needed for Fast-K.
	retValue := optInFastK_Period - 1
	// Add the smoothing being done for Fast-D.
	retValue += maLookback(optInFastD_Period, optInFastD_MAType)
	return retValue
}

// STOCHF returns the fast stochastic lines %K and %D:
//
//	FASTK = (Close - LowestLow(K)) / (HighestHigh(K) - LowestLow(K)) * 100
//	FASTD = MA(FASTK, optInFastD_Period, optInFastD_MAType)
//
// A window whose range is zero relative to its extremes yields %K = 0. Both
// outputs are index-aligned with the inputs; the first STOCHFLookback bars
// are NaN.
// optInFastK_Period: [1, 100000], default 5. optInFastD_Period: [1, 100000],
// default 3. optInFastD_MAType: default SMA.
func STOCHF(inHigh, inLow, inClose []float64, optInFastK_Period, optInFastD_Period int, optInFastD_MAType MAType) (outFastK, outFastD []float64, err error) {
	if _, err := stochfCheck(optInFastK_Period, optInFastD_Period, optInFastD_MAType); err != nil {
		return nil, nil, err
	}
	n, err := checkInputs("STOCHF", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, nil, err
	}
	outFastK = make([]float64, n)
	outFastD = make([]float64, n)
	if n == 0 {
		return outFastK, outFastD, nil
	}
	beg, nb, err := stochfRange(0, n-1, inHigh, inLow, inClose, optInFastK_Period, optInFastD_Period, optInFastD_MAType, outFastK, outFastD)
	if err != nil {
		return nil, nil, err
	}
	alignReal(outFastK, beg, nb)
	alignReal(outFastD, beg, nb)
	return outFastK, outFastD, nil
}

// stochfSame reports whether a and b start at the same element (C pointer
// equality on the buffers).
func stochfSame(a, b []float64) bool {
	return len(a) > 0 && len(b) > 0 && &a[0] == &b[0]
}

// stochfRange mirrors TA_STOCHF: it computes [startIdx, endIdx] and writes
// outNBElement values to outFastK[0:] and outFastD[0:], the first ones
// belonging to bar outBegIdx. outFastK may alias a price input; outFastK and
// outFastD must be distinct buffers.
func stochfRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInFastK_Period, optInFastD_Period int, optInFastD_MAType MAType, outFastK, outFastD []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	optInFastD_MAType, err = stochfCheck(optInFastK_Period, optInFastD_Period, optInFastD_MAType)
	if err != nil {
		return 0, 0, err
	}
	if err := checkBuf("STOCHF", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("STOCHF", endIdx-startIdx, []string{"outFastK", "outFastD"}, outFastK, outFastD); err != nil {
		return 0, 0, err
	}
	if stochfSame(outFastK, outFastD) {
		return 0, 0, badParam("STOCHF", "outFastD", "must not be the same buffer as outFastK")
	}

	// Identify the lookback needed.
	lookbackK := optInFastK_Period - 1
	lookbackFastD := maLookback(optInFastD_Period, optInFastD_MAType)
	lookbackTotal := lookbackK + lookbackFastD

	// Move up the start index if there is not enough initial data.
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	// Make sure there is still something to evaluate.
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// Calculate just enough K for ending up with the caller requested range
	// (the range of K must consider all the lookback of the smoothing).
	outIdx := 0
	trailingIdx := startIdx - lookbackTotal
	today := trailingIdx + lookbackK
	highestIdx := -1
	lowestIdx := highestIdx
	lowest := 0.0
	highest := lowest

	// When outFastK aliases a price input the caller buffer doubles as the
	// scratch: the K writes trail the min/max window reads, and the final
	// copy is overlap-safe.
	var tempBuffer []float64
	switch {
	case stochfSame(outFastK, inHigh):
		tempBuffer = inHigh
	case stochfSame(outFastK, inLow):
		tempBuffer = inLow
	case stochfSame(outFastK, inClose):
		tempBuffer = inClose
	default:
		tempBuffer = make([]float64, endIdx-today+1)
	}

	// Do the K calculation.
	var tmp float64
	for today <= endIdx {
		// Set the lowest low.
		tmp = inLow[today]
		if lowestIdx < trailingIdx {
			lowestIdx = trailingIdx
			lowest = inLow[lowestIdx]
			i := lowestIdx
			for i++; i <= today; i++ {
				tmp = inLow[i]
				if tmp < lowest {
					lowestIdx = i
					lowest = tmp
				}
			}
		} else if tmp <= lowest {
			lowestIdx = today
			lowest = tmp
		}

		// Set the highest high.
		tmp = inHigh[today]
		if highestIdx < trailingIdx {
			highestIdx = trailingIdx
			highest = inHigh[highestIdx]
			i := highestIdx
			for i++; i <= today; i++ {
				tmp = inHigh[i]
				if tmp > highest {
					highestIdx = i
					highest = tmp
				}
			}
		} else if tmp >= highest {
			highestIdx = today
			highest = tmp
		}

		// The flat-window guard tests the very divisor, scaled by its extremes.
		if !isZeroScaled(highest-lowest, math.Abs(highest)+math.Abs(lowest)) {
			tempBuffer[outIdx] = (inClose[today] - lowest) / (highest - lowest) * 100.0
		} else {
			tempBuffer[outIdx] = 0.0
		}
		outIdx++
		trailingIdx++
		today++
	}

	// Fast-K is smoothed to become Fast-D. C passes outFastD straight to
	// TA_MA(0, outIdx-1, ...); maRange's buffer check needs outIdx slots, so
	// use a scratch output when the caller's buffer is shorter.
	dBuf := outFastD
	if len(dBuf) < outIdx {
		dBuf = make([]float64, outIdx)
	}
	_, nb, err := maRange(0, outIdx-1, tempBuffer, optInFastD_Period, optInFastD_MAType, dBuf)
	if err != nil || nb == 0 {
		return 0, 0, err
	}
	if &dBuf[0] != &outFastD[0] {
		copy(outFastD, dBuf[:nb])
	}

	// Copy the requested part of K into the caller buffer (overlap-safe).
	copy(outFastK, tempBuffer[lookbackFastD:lookbackFastD+nb])
	return startIdx, nb, nil
}
