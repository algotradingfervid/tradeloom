package ta

import "math"

// STOCH — Stochastic (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/stoch and src/ta_func/ta_STOCH.c.
//
// FastK = 100*(Close-LowestLow)/(HighestHigh-LowestLow) over optInFastK_Period
// bars (0 when the window is flat), SlowK = MA(FastK), SlowD = MA(SlowK).

// stochParams validates the optional inputs in C's order and applies the C
// prologue's MATypeDefault substitution (TA_MAType_DEFAULT -> SMA).
func stochParams(optInFastK_Period, optInSlowK_Period int, optInSlowK_MAType MAType, optInSlowD_Period int, optInSlowD_MAType MAType) (MAType, MAType, error) {
	if err := checkInt("STOCH", "optInFastK_Period", optInFastK_Period, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkInt("STOCH", "optInSlowK_Period", optInSlowK_Period, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkMAType("STOCH", "optInSlowK_MAType", optInSlowK_MAType); err != nil {
		return 0, 0, err
	}
	if optInSlowK_MAType == MATypeDefault {
		optInSlowK_MAType = MATypeSMA
	}
	if err := checkInt("STOCH", "optInSlowD_Period", optInSlowD_Period, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkMAType("STOCH", "optInSlowD_MAType", optInSlowD_MAType); err != nil {
		return 0, 0, err
	}
	if optInSlowD_MAType == MATypeDefault {
		optInSlowD_MAType = MATypeSMA
	}
	return optInSlowK_MAType, optInSlowD_MAType, nil
}

// STOCHLookback returns the number of leading bars STOCH leaves empty:
// (FastK_Period-1) + MALookback(SlowK) + MALookback(SlowD), or -1 on an
// invalid parameter.
// optInFastK_Period: [1, 100000], default 5. optInSlowK_Period: [1, 100000],
// default 3. optInSlowD_Period: [1, 100000], default 3. MA types: default SMA.
func STOCHLookback(optInFastK_Period, optInSlowK_Period int, optInSlowK_MAType MAType, optInSlowD_Period int, optInSlowD_MAType MAType) int {
	kt, dt, err := stochParams(optInFastK_Period, optInSlowK_Period, optInSlowK_MAType, optInSlowD_Period, optInSlowD_MAType)
	if err != nil {
		return -1
	}
	return stochLookback(optInFastK_Period, optInSlowK_Period, kt, optInSlowD_Period, dt)
}

// stochLookback is the unvalidated tail of TA_STOCH_Lookback; the MA types
// must already have MATypeDefault substituted.
func stochLookback(optInFastK_Period, optInSlowK_Period int, optInSlowK_MAType MAType, optInSlowD_Period int, optInSlowD_MAType MAType) int {
	retValue := optInFastK_Period - 1
	retValue += maLookback(optInSlowK_Period, optInSlowK_MAType)
	retValue += maLookback(optInSlowD_Period, optInSlowD_MAType)
	return retValue
}

// STOCH returns the Slow-%K and Slow-%D lines of the stochastic oscillator
// over (inHigh, inLow, inClose). The outputs are index-aligned with the
// inputs; the first STOCHLookback bars are NaN.
// optInFastK_Period: [1, 100000], default 5. optInSlowK_Period: [1, 100000],
// default 3. optInSlowD_Period: [1, 100000], default 3. MA types: default SMA.
func STOCH(inHigh, inLow, inClose []float64, optInFastK_Period, optInSlowK_Period int, optInSlowK_MAType MAType, optInSlowD_Period int, optInSlowD_MAType MAType) (outSlowK, outSlowD []float64, err error) {
	if _, _, err := stochParams(optInFastK_Period, optInSlowK_Period, optInSlowK_MAType, optInSlowD_Period, optInSlowD_MAType); err != nil {
		return nil, nil, err
	}
	n, err := checkInputs("STOCH", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, nil, err
	}
	outSlowK = make([]float64, n)
	outSlowD = make([]float64, n)
	if n == 0 {
		return outSlowK, outSlowD, nil
	}
	beg, nb, err := stochRange(0, n-1, inHigh, inLow, inClose, optInFastK_Period, optInSlowK_Period, optInSlowK_MAType, optInSlowD_Period, optInSlowD_MAType, outSlowK, outSlowD)
	if err != nil {
		return nil, nil, err
	}
	alignReal(outSlowK, beg, nb)
	alignReal(outSlowD, beg, nb)
	return outSlowK, outSlowD, nil
}

// stochRange mirrors TA_STOCH: it computes [startIdx, endIdx] and writes
// outNBElement values to outSlowK[0:] and outSlowD[0:], the first one
// belonging to bar outBegIdx. An output may alias an input, but outSlowK and
// outSlowD must be distinct (C returns TA_BAD_PARAM otherwise).
func stochRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInFastK_Period, optInSlowK_Period int, optInSlowK_MAType MAType, optInSlowD_Period int, optInSlowD_MAType MAType, outSlowK, outSlowD []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	optInSlowK_MAType, optInSlowD_MAType, err = stochParams(optInFastK_Period, optInSlowK_Period, optInSlowK_MAType, optInSlowD_Period, optInSlowD_MAType)
	if err != nil {
		return 0, 0, err
	}
	if err := checkBuf("STOCH", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("STOCH", endIdx-startIdx, []string{"outSlowK", "outSlowD"}, outSlowK, outSlowD); err != nil {
		return 0, 0, err
	}
	if &outSlowK[0] == &outSlowD[0] {
		return 0, 0, badParam("STOCH", "outSlowD", "outSlowK and outSlowD must be distinct buffers")
	}

	// Identify the lookback needed.
	lookbackK := optInFastK_Period - 1
	lookbackKSlow := maLookback(optInSlowK_Period, optInSlowK_MAType)
	lookbackDSlow := maLookback(optInSlowD_Period, optInSlowD_MAType)
	lookbackTotal := lookbackK + lookbackDSlow + lookbackKSlow
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	outIdx := 0
	trailingIdx := startIdx - lookbackTotal
	today := trailingIdx + lookbackK
	highestIdx := 0 - 1
	lowestIdx := highestIdx
	lowest := 0.0
	highest := lowest

	// C reuses outSlowK as the K scratch when it aliases a price input (the
	// K writes trail the window reads, so the result is the same); Go always
	// allocates the scratch, which yields identical values and also covers a
	// caller buffer shorter than the scratch.
	tempBuffer := make([]float64, endIdx-today+1)

	// Do the K calculation.
	for today <= endIdx {
		// Set the lowest low.
		tmp := inLow[today]
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
		// Divide by the range, scale after; flat window (scaled guard) -> 0.
		if !isZeroScaled(highest-lowest, math.Abs(highest)+math.Abs(lowest)) {
			tempBuffer[outIdx] = (inClose[today] - lowest) / (highest - lowest) * 100.0
		} else {
			tempBuffer[outIdx] = 0.0
		}
		outIdx++
		trailingIdx++
		today++
	}

	// Smooth the raw K in place into Slow-K.
	_, nbK, err := maRange(0, outIdx-1, tempBuffer, optInSlowK_Period, optInSlowK_MAType, tempBuffer)
	if err != nil || nbK == 0 {
		return 0, 0, err
	}

	// %D is a moving average of the smoothed K. C writes it straight into
	// outSlowD; maRange's Go buffer check wants room for nbK elements while
	// the caller only guarantees nbK-lookbackDSlow, so use a scratch when the
	// caller's buffer is shorter.
	dst := outSlowD
	if len(dst) < nbK {
		dst = make([]float64, nbK)
	}
	_, nbD, err := maRange(0, nbK-1, tempBuffer, optInSlowD_Period, optInSlowD_MAType, dst)
	if err != nil {
		return 0, 0, err
	}
	if len(outSlowD) < nbK {
		copy(outSlowD[:nbD], dst[:nbD])
	}

	copy(outSlowK[:nbD], tempBuffer[lookbackDSlow:lookbackDSlow+nbD])
	return startIdx, nbD, nil
}
