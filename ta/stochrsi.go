package ta

// STOCHRSI — Stochastic Relative Strength Index (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/stochrsi and src/ta_func/ta_STOCHRSI.c.

// stochrsiCheck validates the STOCHRSI optional inputs in the C prologue's
// order and applies its MATypeDefault substitution (TA_MAType_DEFAULT -> SMA).
func stochrsiCheck(optInTimePeriod, optInFastKPeriod, optInFastDPeriod int, optInFastDMAType MAType) (MAType, error) {
	if err := checkInt("STOCHRSI", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return optInFastDMAType, err
	}
	if err := checkInt("STOCHRSI", "optInFastK_Period", optInFastKPeriod, 1, 100000); err != nil {
		return optInFastDMAType, err
	}
	if err := checkInt("STOCHRSI", "optInFastD_Period", optInFastDPeriod, 1, 100000); err != nil {
		return optInFastDMAType, err
	}
	if err := checkMAType("STOCHRSI", "optInFastD_MAType", optInFastDMAType); err != nil {
		return optInFastDMAType, err
	}
	if optInFastDMAType == MATypeDefault {
		optInFastDMAType = MATypeSMA
	}
	return optInFastDMAType, nil
}

// STOCHRSILookback returns the number of leading bars STOCHRSI leaves empty
// (RSI lookback, including its unstable period, plus the STOCHF lookback),
// or -1 on an invalid parameter.
// optInTimePeriod: [2, 100000], default 14. optInFastK_Period: [1, 100000],
// default 5. optInFastD_Period: [1, 100000], default 3. optInFastD_MAType:
// default SMA.
func STOCHRSILookback(optInTimePeriod, optInFastK_Period, optInFastD_Period int, optInFastD_MAType MAType) int {
	mt, err := stochrsiCheck(optInTimePeriod, optInFastK_Period, optInFastD_Period, optInFastD_MAType)
	if err != nil {
		return -1
	}
	return stochrsiLookback(optInTimePeriod, optInFastK_Period, optInFastD_Period, mt)
}

// stochrsiLookback is the unvalidated tail of TA_STOCHRSI_Lookback; the
// MAType must already have MATypeDefault substituted.
func stochrsiLookback(optInTimePeriod, optInFastK_Period, optInFastD_Period int, optInFastD_MAType MAType) int {
	return rsiLookback(optInTimePeriod) + stochfLookback(optInFastK_Period, optInFastD_Period, optInFastD_MAType)
}

// STOCHRSI applies the fast stochastic to the RSI of inReal:
//
//	rsi   = RSI(inReal, optInTimePeriod)
//	FASTK = (rsi - min(rsi, K)) / (max(rsi, K) - min(rsi, K)) * 100
//	FASTD = MA(FASTK, optInFastD_Period, optInFastD_MAType)
//
// A flat RSI window yields %K = 0. Both outputs are index-aligned with
// inReal; the first STOCHRSILookback bars are NaN.
// optInTimePeriod: [2, 100000], default 14. optInFastK_Period: [1, 100000],
// default 5. optInFastD_Period: [1, 100000], default 3. optInFastD_MAType:
// default SMA.
func STOCHRSI(inReal []float64, optInTimePeriod, optInFastK_Period, optInFastD_Period int, optInFastD_MAType MAType) (outFastK, outFastD []float64, err error) {
	if _, err := stochrsiCheck(optInTimePeriod, optInFastK_Period, optInFastD_Period, optInFastD_MAType); err != nil {
		return nil, nil, err
	}
	n, err := checkInputs("STOCHRSI", []string{"inReal"}, inReal)
	if err != nil {
		return nil, nil, err
	}
	outFastK = make([]float64, n)
	outFastD = make([]float64, n)
	if n == 0 {
		return outFastK, outFastD, nil
	}
	beg, nb, err := stochrsiRange(0, n-1, inReal, optInTimePeriod, optInFastK_Period, optInFastD_Period, optInFastD_MAType, outFastK, outFastD)
	if err != nil {
		return nil, nil, err
	}
	alignReal(outFastK, beg, nb)
	alignReal(outFastD, beg, nb)
	return outFastK, outFastD, nil
}

// stochrsiRange mirrors TA_STOCHRSI: it computes [startIdx, endIdx] and
// writes outNBElement values to outFastK[0:] and outFastD[0:], the first ones
// belonging to bar outBegIdx. outFastK and outFastD must be distinct buffers;
// either may alias inReal (the RSI is taken into a scratch buffer first).
func stochrsiRange(startIdx, endIdx int, inReal []float64, optInTimePeriod, optInFastK_Period, optInFastD_Period int, optInFastD_MAType MAType, outFastK, outFastD []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	optInFastD_MAType, err = stochrsiCheck(optInTimePeriod, optInFastK_Period, optInFastD_Period, optInFastD_MAType)
	if err != nil {
		return 0, 0, err
	}
	if err := checkBuf("STOCHRSI", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("STOCHRSI", endIdx-startIdx, []string{"outFastK", "outFastD"}, outFastK, outFastD); err != nil {
		return 0, 0, err
	}
	if stochfSame(outFastK, outFastD) {
		return 0, 0, badParam("STOCHRSI", "outFastD", "must not be the same buffer as outFastK")
	}

	// Adjust startIdx to account for the lookback period.
	lookbackSTOCHF := stochfLookback(optInFastK_Period, optInFastD_Period, optInFastD_MAType)
	lookbackTotal := rsiLookback(optInTimePeriod) + lookbackSTOCHF
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	// Make sure there is still something to evaluate.
	if startIdx > endIdx {
		return 0, 0, nil
	}

	outBegIdx = startIdx
	tempArraySize := endIdx - startIdx + 1 + lookbackSTOCHF
	tempRSIBuffer := make([]float64, tempArraySize)
	// C calls TA_RSI here. stochrsiRSI is TA_RSI without the (already
	// satisfied) validation and with Go's multiply-add fusion blocked; see
	// stochrsiRSI for why rsiRange is not used.
	_, outNbElement1 := stochrsiRSI(startIdx-lookbackSTOCHF, endIdx, inReal, optInTimePeriod, tempRSIBuffer)
	if outNbElement1 == 0 {
		return 0, 0, nil
	}

	// C hands its caller buffers straight to TA_STOCHF(0, tempArraySize-1,
	// ...), which writes only endIdx-startIdx+1 values; stochfRange's buffer
	// check wants tempArraySize slots, so use scratch outputs when the
	// caller's buffers are shorter.
	kBuf, dBuf := outFastK, outFastD
	if len(kBuf) < tempArraySize {
		kBuf = make([]float64, tempArraySize)
	}
	if len(dBuf) < tempArraySize {
		dBuf = make([]float64, tempArraySize)
	}
	_, nb, err := stochfRange(0, tempArraySize-1, tempRSIBuffer, tempRSIBuffer, tempRSIBuffer, optInFastK_Period, optInFastD_Period, optInFastD_MAType, kBuf, dBuf)
	if err != nil || nb == 0 {
		return 0, 0, err
	}
	if &kBuf[0] != &outFastK[0] {
		copy(outFastK, kBuf[:nb])
	}
	if &dBuf[0] != &outFastD[0] {
		copy(outFastD, dBuf[:nb])
	}
	return outBegIdx, nb, nil
}

// stochrsiRSI is a private copy of the TA_RSI body (as in rsiRange, minus the
// validation STOCHRSI has already done) that keeps every multiply and add a
// separately rounded operation, as the C library is built
// (-ffp-contract=off). The Go compiler may fuse "x *= a; x += b" into one
// FMA across statements; rsiRange gets fused on arm64, and the resulting
// last-ulp RSI differences, after the min/max normalisation of %K, are
// amplified by MAMA's phase logic into %D differences of several percent.
// The explicit float64() conversions forbid that fusion.
func stochrsiRSI(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int) {
	lookbackTotal := rsiLookback(optInTimePeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0
	}
	outIdx := 0

	// Period 1 is rejected by validation; the C keeps this branch.
	if optInTimePeriod == 1 {
		i := endIdx - startIdx + 1
		today := startIdx
		for outIdx = 0; outIdx < i; outIdx++ {
			outReal[outIdx] = inReal[today]
			today++
		}
		return startIdx, i
	}

	invPeriod := 1.0 / float64(optInTimePeriod)
	periodM1 := float64(optInTimePeriod - 1)

	// Accumulate Wilder's average gain and loss over the initial period.
	today := startIdx - lookbackTotal
	prevValue := inReal[today]
	prevGain := 0.0
	prevLoss := 0.0
	today++
	for i := optInTimePeriod; i > 0; i-- {
		tempValue1 := inReal[today]
		today++
		tempValue2 := tempValue1 - prevValue
		prevValue = tempValue1
		gainDelta := 0.0
		if tempValue2 > 0.0 {
			gainDelta = tempValue2
		}
		prevGain += gainDelta
		prevLoss += gainDelta - tempValue2
	}
	prevLoss = float64(prevLoss * invPeriod)
	prevGain = float64(prevGain * invPeriod)

	if today > startIdx {
		tempValue1 := prevGain + prevLoss
		if tempValue1 > 0.0 {
			outReal[outIdx] = 100.0 * (prevGain / tempValue1)
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++
	} else {
		// Skip the unstable period: process but do not write.
		for today < startIdx {
			tempValue1 := inReal[today]
			tempValue2 := tempValue1 - prevValue
			prevValue = tempValue1
			prevLoss = float64(prevLoss * periodM1)
			prevGain = float64(prevGain * periodM1)
			gainDelta := 0.0
			if tempValue2 > 0.0 {
				gainDelta = tempValue2
			}
			prevGain += gainDelta
			prevLoss += gainDelta - tempValue2
			prevLoss = float64(prevLoss * invPeriod)
			prevGain = float64(prevGain * invPeriod)
			today++
		}
	}

	for today <= endIdx {
		tempValue1 := inReal[today]
		today++
		tempValue2 := tempValue1 - prevValue
		prevValue = tempValue1
		prevLoss = float64(prevLoss * periodM1)
		prevGain = float64(prevGain * periodM1)
		gainDelta := 0.0
		if tempValue2 > 0.0 {
			gainDelta = tempValue2
		}
		prevGain += gainDelta
		prevLoss += gainDelta - tempValue2
		prevLoss = float64(prevLoss * invPeriod)
		prevGain = float64(prevGain * invPeriod)
		tempValue1 = prevGain + prevLoss
		if tempValue1 > 0.0 {
			outReal[outIdx] = 100.0 * (prevGain / tempValue1)
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++
	}
	return startIdx, outIdx
}
