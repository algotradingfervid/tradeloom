package ta

// MA — Moving average (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/ma and src/ta_func/ta_MA.c.

// maCheck validates the MA optional inputs and applies the C prologue's
// MATypeDefault substitution (TA_MAType_DEFAULT -> SMA).
func maCheck(optInTimePeriod int, optInMAType MAType) (MAType, error) {
	if err := checkInt("MA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return optInMAType, err
	}
	if err := checkMAType("MA", "optInMAType", optInMAType); err != nil {
		return optInMAType, err
	}
	if optInMAType == MATypeDefault {
		optInMAType = MATypeSMA
	}
	return optInMAType, nil
}

// MALookback returns the number of leading bars MA leaves empty: 0 for
// period 1 or MATypeDisabled, otherwise the lookback of the selected moving
// average (MAMA with limits 0.5/0.05, T3 with vfactor 0.7). It returns -1 on
// an invalid parameter.
// optInTimePeriod: [1, 100000], default 30. optInMAType: default SMA.
func MALookback(optInTimePeriod int, optInMAType MAType) int {
	t, err := maCheck(optInTimePeriod, optInMAType)
	if err != nil {
		return -1
	}
	return maLookback(optInTimePeriod, t)
}

// maLookback mirrors the unvalidated tail of TA_MA_Lookback; optInMAType must
// already have MATypeDefault substituted.
func maLookback(optInTimePeriod int, optInMAType MAType) int {
	if optInTimePeriod <= 1 || optInMAType == MATypeDisabled {
		return 0
	}
	switch optInMAType {
	case MATypeSMA:
		return smaLookback(optInTimePeriod)
	case MATypeEMA:
		return emaLookback(optInTimePeriod)
	case MATypeWMA:
		return wmaLookback(optInTimePeriod)
	case MATypeDEMA:
		return demaLookback(optInTimePeriod)
	case MATypeTEMA:
		return temaLookback(optInTimePeriod)
	case MATypeTRIMA:
		return trimaLookback(optInTimePeriod)
	case MATypeKAMA:
		return kamaLookback(optInTimePeriod)
	case MATypeMAMA:
		return mamaLookback(0.5, 0.05)
	case MATypeT3:
		return t3Lookback(optInTimePeriod, 0.7)
	case MATypeHMA:
		return hmaLookback(optInTimePeriod)
	case MATypeZLEMA:
		return zlemaLookback(optInTimePeriod)
	case MATypeRMA:
		return rmaLookback(optInTimePeriod)
	default:
		return 0
	}
}

// MA returns the moving average of inReal selected by optInMAType over
// optInTimePeriod bars. MAMA ignores the period (limits 0.5/0.05, MAMA line
// only); T3 uses vfactor 0.7. Period 1 or MATypeDisabled copies the input.
// The output is index-aligned with inReal; the first MALookback bars are NaN.
// optInTimePeriod: [1, 100000], default 30. optInMAType: default SMA.
func MA(inReal []float64, optInTimePeriod int, optInMAType MAType) ([]float64, error) {
	if _, err := maCheck(optInTimePeriod, optInMAType); err != nil {
		return nil, err
	}
	n, err := checkInputs("MA", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := maRange(0, n-1, inReal, optInTimePeriod, optInMAType, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// maRange mirrors TA_MA: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func maRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, optInMAType MAType, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	optInMAType, err = maCheck(optInTimePeriod, optInMAType)
	if err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MA", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MA", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// Nothing to produce: the range is shorter than the lookback.
	if maLookback(optInTimePeriod, optInMAType) > endIdx {
		return 0, 0, nil
	}

	// No-smoothing identity: period 1 (every MA type) or MATypeDisabled.
	if optInTimePeriod == 1 || optInMAType == MATypeDisabled {
		nbElement := endIdx - startIdx + 1
		for todayIdx, outIdx := startIdx, 0; outIdx < nbElement; outIdx, todayIdx = outIdx+1, todayIdx+1 {
			outReal[outIdx] = inReal[todayIdx]
		}
		return startIdx, nbElement, nil
	}

	// Simply forward the job to the corresponding TA function.
	switch optInMAType {
	case MATypeSMA:
		return smaRange(startIdx, endIdx, inReal, optInTimePeriod, outReal)
	case MATypeEMA:
		return emaRange(startIdx, endIdx, inReal, optInTimePeriod, outReal)
	case MATypeWMA:
		return wmaRange(startIdx, endIdx, inReal, optInTimePeriod, outReal)
	case MATypeDEMA:
		return demaRange(startIdx, endIdx, inReal, optInTimePeriod, outReal)
	case MATypeTEMA:
		return temaRange(startIdx, endIdx, inReal, optInTimePeriod, outReal)
	case MATypeTRIMA:
		return trimaRange(startIdx, endIdx, inReal, optInTimePeriod, outReal)
	case MATypeKAMA:
		return kamaRange(startIdx, endIdx, inReal, optInTimePeriod, outReal)
	case MATypeMAMA:
		// The period is ignored; FAMA is nullable, so only MAMA is computed.
		return mamaRange(startIdx, endIdx, inReal, 0.5, 0.05, outReal, nil)
	case MATypeT3:
		return t3Range(startIdx, endIdx, inReal, optInTimePeriod, 0.7, outReal)
	case MATypeHMA:
		return hmaRange(startIdx, endIdx, inReal, optInTimePeriod, outReal)
	case MATypeZLEMA:
		return zlemaRange(startIdx, endIdx, inReal, optInTimePeriod, outReal)
	case MATypeRMA:
		return rmaRange(startIdx, endIdx, inReal, optInTimePeriod, outReal)
	default:
		return 0, 0, badParam("MA", "optInMAType", "unsupported MAType %d", int(optInMAType))
	}
}
