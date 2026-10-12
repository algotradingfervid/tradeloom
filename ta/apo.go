package ta

// APO — Absolute Price Oscillator (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/apo and src/ta_func/ta_APO.c.

// apoCheck validates the APO optional inputs and applies the C prologue's
// MATypeDefault substitution (TA_MAType_DEFAULT -> EMA).
func apoCheck(optInFastPeriod, optInSlowPeriod int, optInMAType MAType) (MAType, error) {
	if err := checkInt("APO", "optInFastPeriod", optInFastPeriod, 2, 100000); err != nil {
		return optInMAType, err
	}
	if err := checkInt("APO", "optInSlowPeriod", optInSlowPeriod, 2, 100000); err != nil {
		return optInMAType, err
	}
	if err := checkMAType("APO", "optInMAType", optInMAType); err != nil {
		return optInMAType, err
	}
	if optInMAType == MATypeDefault {
		optInMAType = MATypeEMA
	}
	return optInMAType, nil
}

// APOLookback returns the number of leading bars APO leaves empty: the MA
// lookback of the longer of the two periods. It returns -1 on an invalid
// parameter.
// optInFastPeriod: [2, 100000], default 12. optInSlowPeriod: [2, 100000],
// default 26. optInMAType: default EMA (MATypeDefault maps to EMA).
func APOLookback(optInFastPeriod, optInSlowPeriod int, optInMAType MAType) int {
	t, err := apoCheck(optInFastPeriod, optInSlowPeriod, optInMAType)
	if err != nil {
		return -1
	}
	return apoLookback(optInFastPeriod, optInSlowPeriod, t)
}

// apoLookback is the unvalidated tail of TA_APO_Lookback; optInMAType must
// already have MATypeDefault substituted.
func apoLookback(optInFastPeriod, optInSlowPeriod int, optInMAType MAType) int {
	// The slow MA is the key factor determining the lookback period.
	return maLookback(cMaxInt(optInSlowPeriod, optInFastPeriod), optInMAType)
}

// APO returns MA(fast) - MA(slow) of inReal, using the moving average selected
// by optInMAType. If the slow period is shorter than the fast one they are
// swapped. The output is index-aligned with inReal; the first APOLookback
// bars are NaN.
// optInFastPeriod: [2, 100000], default 12. optInSlowPeriod: [2, 100000],
// default 26. optInMAType: default EMA (MATypeDefault maps to EMA).
func APO(inReal []float64, optInFastPeriod, optInSlowPeriod int, optInMAType MAType) ([]float64, error) {
	if _, err := apoCheck(optInFastPeriod, optInSlowPeriod, optInMAType); err != nil {
		return nil, err
	}
	n, err := checkInputs("APO", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := apoRange(0, n-1, inReal, optInFastPeriod, optInSlowPeriod, optInMAType, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// apoRange mirrors TA_APO: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func apoRange(startIdx, endIdx int, inReal []float64, optInFastPeriod, optInSlowPeriod int, optInMAType MAType, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	optInMAType, err = apoCheck(optInFastPeriod, optInSlowPeriod, optInMAType)
	if err != nil {
		return 0, 0, err
	}
	if err := checkBuf("APO", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("APO", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// Nothing to produce: the range is shorter than the lookback.
	if maLookback(cMaxInt(optInSlowPeriod, optInFastPeriod), optInMAType) > endIdx {
		return 0, 0, nil
	}

	// Intermediate buffer for the fast MA.
	tempBuffer := make([]float64, endIdx-startIdx+1)

	// Make sure slow is really slower than the fast period; if not, swap.
	if optInSlowPeriod < optInFastPeriod {
		optInSlowPeriod, optInFastPeriod = optInFastPeriod, optInSlowPeriod
	}

	// Calculate the fast MA into the tempBuffer.
	_, fastNb, err := maRange(startIdx, endIdx, inReal, optInFastPeriod, optInMAType, tempBuffer)
	if err != nil {
		return 0, 0, err
	}
	// Calculate the slow MA into the output.
	outBegIdx, outNBElement, err = maRange(startIdx, endIdx, inReal, optInSlowPeriod, optInMAType, outReal)
	if err != nil {
		return 0, 0, err
	}

	// fastNb - outNBElement == slowBeg - fastBeg, so tempBuffer[i+offset] is
	// the fast MA at the same bar as outReal[i].
	offset := fastNb - outNBElement
	// Calculate (fast MA)-(slow MA) in the output.
	for i := 0; i < outNBElement; i++ {
		outReal[i] = tempBuffer[i+offset] - outReal[i]
	}
	return outBegIdx, outNBElement, nil
}
