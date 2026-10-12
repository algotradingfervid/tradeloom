package ta

// KDJ — KDJ Stochastic (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/kdj and src/ta_func/ta_KDJ.c.
//
// K = Slow-%K and D = Slow-%D of STOCH (both smoothed with Wilder's moving
// average, MATypeRMA, by default), J = 3*K - 2*D (unbounded).
//
// The C library is built with -ffp-contract=off. Go may fuse x*y-z into an
// FMA, so each product in J is wrapped in an explicit float64() conversion,
// which the spec guarantees rounds and therefore blocks fusion.

// kdjParams validates the optional inputs in C's order and applies the C
// prologue's MATypeDefault substitution (TA_MAType_DEFAULT -> 13, RMA).
func kdjParams(optInFastK_Period, optInSlowK_Period int, optInSlowK_MAType MAType, optInSlowD_Period int, optInSlowD_MAType MAType) (MAType, MAType, error) {
	if err := checkInt("KDJ", "optInFastK_Period", optInFastK_Period, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkInt("KDJ", "optInSlowK_Period", optInSlowK_Period, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkMAType("KDJ", "optInSlowK_MAType", optInSlowK_MAType); err != nil {
		return 0, 0, err
	}
	if optInSlowK_MAType == MATypeDefault {
		optInSlowK_MAType = MATypeRMA
	}
	if err := checkInt("KDJ", "optInSlowD_Period", optInSlowD_Period, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkMAType("KDJ", "optInSlowD_MAType", optInSlowD_MAType); err != nil {
		return 0, 0, err
	}
	if optInSlowD_MAType == MATypeDefault {
		optInSlowD_MAType = MATypeRMA
	}
	return optInSlowK_MAType, optInSlowD_MAType, nil
}

// KDJLookback returns the number of leading bars KDJ leaves empty (the STOCH
// lookback), or -1 on an invalid parameter.
// optInFastK_Period: [1, 100000], default 9. optInSlowK_Period: [1, 100000],
// default 3. optInSlowD_Period: [1, 100000], default 3. MA types: default RMA
// (MATypeDefault is substituted with MATypeRMA).
func KDJLookback(optInFastK_Period, optInSlowK_Period int, optInSlowK_MAType MAType, optInSlowD_Period int, optInSlowD_MAType MAType) int {
	kt, dt, err := kdjParams(optInFastK_Period, optInSlowK_Period, optInSlowK_MAType, optInSlowD_Period, optInSlowD_MAType)
	if err != nil {
		return -1
	}
	return kdjLookback(optInFastK_Period, optInSlowK_Period, kt, optInSlowD_Period, dt)
}

// kdjLookback is the unvalidated tail of TA_KDJ_Lookback; the MA types must
// already have MATypeDefault substituted.
func kdjLookback(optInFastK_Period, optInSlowK_Period int, optInSlowK_MAType MAType, optInSlowD_Period int, optInSlowD_MAType MAType) int {
	return stochLookback(optInFastK_Period, optInSlowK_Period, optInSlowK_MAType, optInSlowD_Period, optInSlowD_MAType)
}

// KDJ returns the K, D and J lines over (inHigh, inLow, inClose). The outputs
// are index-aligned with the inputs; the first KDJLookback bars are NaN.
// optInFastK_Period: [1, 100000], default 9. optInSlowK_Period: [1, 100000],
// default 3. optInSlowD_Period: [1, 100000], default 3. MA types: default RMA
// (MATypeDefault is substituted with MATypeRMA).
func KDJ(inHigh, inLow, inClose []float64, optInFastK_Period, optInSlowK_Period int, optInSlowK_MAType MAType, optInSlowD_Period int, optInSlowD_MAType MAType) (outK, outD, outJ []float64, err error) {
	if _, _, err := kdjParams(optInFastK_Period, optInSlowK_Period, optInSlowK_MAType, optInSlowD_Period, optInSlowD_MAType); err != nil {
		return nil, nil, nil, err
	}
	n, err := checkInputs("KDJ", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, nil, nil, err
	}
	outK = make([]float64, n)
	outD = make([]float64, n)
	outJ = make([]float64, n)
	if n == 0 {
		return outK, outD, outJ, nil
	}
	beg, nb, err := kdjRange(0, n-1, inHigh, inLow, inClose, optInFastK_Period, optInSlowK_Period, optInSlowK_MAType, optInSlowD_Period, optInSlowD_MAType, outK, outD, outJ)
	if err != nil {
		return nil, nil, nil, err
	}
	alignReal(outK, beg, nb)
	alignReal(outD, beg, nb)
	alignReal(outJ, beg, nb)
	return outK, outD, outJ, nil
}

// kdjRange mirrors TA_KDJ: it computes [startIdx, endIdx] and writes
// outNBElement values to outK[0:], outD[0:] and outJ[0:], the first one
// belonging to bar outBegIdx. The three outputs must be distinct buffers
// (C returns TA_BAD_PARAM otherwise).
func kdjRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInFastK_Period, optInSlowK_Period int, optInSlowK_MAType MAType, optInSlowD_Period int, optInSlowD_MAType MAType, outK, outD, outJ []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	optInSlowK_MAType, optInSlowD_MAType, err = kdjParams(optInFastK_Period, optInSlowK_Period, optInSlowK_MAType, optInSlowD_Period, optInSlowD_MAType)
	if err != nil {
		return 0, 0, err
	}
	if err := checkBuf("KDJ", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("KDJ", endIdx-startIdx, []string{"outK", "outD", "outJ"}, outK, outD, outJ); err != nil {
		return 0, 0, err
	}
	if &outK[0] == &outD[0] || &outK[0] == &outJ[0] || &outD[0] == &outJ[0] {
		return 0, 0, badParam("KDJ", "outJ", "outK, outD and outJ must be distinct buffers")
	}

	lookbackTotal := kdjLookback(optInFastK_Period, optInSlowK_Period, optInSlowK_MAType, optInSlowD_Period, optInSlowD_MAType)
	// Nothing to produce: the range is shorter than the lookback.
	if lookbackTotal > endIdx {
		return 0, 0, nil
	}
	outBegIdx, outNBElement, err = stochRange(startIdx, endIdx, inHigh, inLow, inClose, optInFastK_Period, optInSlowK_Period, optInSlowK_MAType, optInSlowD_Period, optInSlowD_MAType, outK, outD)
	if err != nil || outNBElement == 0 {
		return 0, 0, err
	}
	for i := 0; i < outNBElement; i++ {
		outJ[i] = float64(3.0*outK[i]) - float64(2.0*outD[i])
	}
	return outBegIdx, outNBElement, nil
}
