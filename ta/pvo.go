package ta

// PVO — Percentage Volume Oscillator (Volume Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/pvo and src/ta_func/ta_PVO.c.

// pvoCheck validates the PVO optional inputs and applies the C prologue's
// MATypeDefault substitution (TA_MAType_DEFAULT -> EMA).
func pvoCheck(optInFastPeriod, optInSlowPeriod int, optInMAType MAType) (MAType, error) {
	if err := checkInt("PVO", "optInFastPeriod", optInFastPeriod, 2, 100000); err != nil {
		return optInMAType, err
	}
	if err := checkInt("PVO", "optInSlowPeriod", optInSlowPeriod, 2, 100000); err != nil {
		return optInMAType, err
	}
	if err := checkMAType("PVO", "optInMAType", optInMAType); err != nil {
		return optInMAType, err
	}
	if optInMAType == MATypeDefault {
		optInMAType = MATypeEMA
	}
	return optInMAType, nil
}

// PVOLookback returns the number of leading bars PVO leaves empty: the MA
// lookback of the slower of the two periods. It returns -1 on an invalid
// parameter.
// optInFastPeriod: [2, 100000], default 12. optInSlowPeriod: [2, 100000],
// default 26. optInMAType: default EMA.
func PVOLookback(optInFastPeriod, optInSlowPeriod int, optInMAType MAType) int {
	t, err := pvoCheck(optInFastPeriod, optInSlowPeriod, optInMAType)
	if err != nil {
		return -1
	}
	return pvoLookback(optInFastPeriod, optInSlowPeriod, t)
}

// pvoLookback is the unvalidated TA_PVO_Lookback tail; optInMAType must
// already have MATypeDefault substituted.
func pvoLookback(optInFastPeriod, optInSlowPeriod int, optInMAType MAType) int {
	return maLookback(cMaxInt(optInSlowPeriod, optInFastPeriod), optInMAType)
}

// PVO returns ((fastMA - slowMA) / slowMA) * 100 of inVolume, both MAs of type
// optInMAType (0 where slowMA is zero). If optInSlowPeriod < optInFastPeriod
// the two are swapped. The output is index-aligned with inVolume; the first
// PVOLookback bars are NaN.
// optInFastPeriod: [2, 100000], default 12. optInSlowPeriod: [2, 100000],
// default 26. optInMAType: default EMA.
func PVO(inVolume []float64, optInFastPeriod, optInSlowPeriod int, optInMAType MAType) ([]float64, error) {
	if _, err := pvoCheck(optInFastPeriod, optInSlowPeriod, optInMAType); err != nil {
		return nil, err
	}
	n, err := checkInputs("PVO", []string{"inVolume"}, inVolume)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := pvoRange(0, n-1, inVolume, optInFastPeriod, optInSlowPeriod, optInMAType, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// pvoRange mirrors TA_PVO: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx.
func pvoRange(startIdx, endIdx int, inVolume []float64, optInFastPeriod, optInSlowPeriod int, optInMAType MAType, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	optInMAType, err = pvoCheck(optInFastPeriod, optInSlowPeriod, optInMAType)
	if err != nil {
		return 0, 0, err
	}
	if err := checkBuf("PVO", endIdx, []string{"inVolume"}, inVolume); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("PVO", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// Nothing to produce: the range is shorter than the lookback.
	if pvoLookback(optInFastPeriod, optInSlowPeriod, optInMAType) > endIdx {
		return 0, 0, nil
	}

	tempBuffer := make([]float64, endIdx-startIdx+1)

	// Make sure slow is really slower than the fast period; if not, swap.
	if optInSlowPeriod < optInFastPeriod {
		optInSlowPeriod, optInFastPeriod = optInFastPeriod, optInSlowPeriod
	}

	_, fastNb, err := maRange(startIdx, endIdx, inVolume, optInFastPeriod, optInMAType, tempBuffer)
	if err != nil {
		return 0, 0, err
	}
	outBegIdx, outNBElement, err = maRange(startIdx, endIdx, inVolume, optInSlowPeriod, optInMAType, outReal)
	if err != nil {
		return 0, 0, err
	}

	// fastNb - outNBElement == slowBeg - fastBeg, so tempBuffer[i+offset] is
	// the fast MA at the same bar as outReal[i].
	offset := fastNb - outNBElement
	for i := 0; i < outNBElement; i++ {
		tempReal := outReal[i]
		if !isZero(tempReal) {
			outReal[i] = (tempBuffer[i+offset] - tempReal) / tempReal * 100.0
		} else {
			outReal[i] = 0.0
		}
	}
	return outBegIdx, outNBElement, nil
}
