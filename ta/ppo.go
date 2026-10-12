package ta

// PPO — Percentage Price Oscillator (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/ppo and src/ta_func/ta_PPO.c.

// ppoCheck validates the PPO optional inputs and applies the C prologue's
// MATypeDefault substitution (TA_MAType_DEFAULT -> EMA).
func ppoCheck(optInFastPeriod, optInSlowPeriod int, optInMAType MAType) (MAType, error) {
	if err := checkInt("PPO", "optInFastPeriod", optInFastPeriod, 2, 100000); err != nil {
		return optInMAType, err
	}
	if err := checkInt("PPO", "optInSlowPeriod", optInSlowPeriod, 2, 100000); err != nil {
		return optInMAType, err
	}
	if err := checkMAType("PPO", "optInMAType", optInMAType); err != nil {
		return optInMAType, err
	}
	if optInMAType == MATypeDefault {
		optInMAType = MATypeEMA
	}
	return optInMAType, nil
}

// PPOLookback returns the number of leading bars PPO leaves empty: the MA
// lookback of the slower (longer) period. It returns -1 on an invalid
// parameter.
// optInFastPeriod: [2, 100000], default 12. optInSlowPeriod: [2, 100000],
// default 26. optInMAType: default EMA.
func PPOLookback(optInFastPeriod, optInSlowPeriod int, optInMAType MAType) int {
	t, err := ppoCheck(optInFastPeriod, optInSlowPeriod, optInMAType)
	if err != nil {
		return -1
	}
	return ppoLookback(optInFastPeriod, optInSlowPeriod, t)
}

// ppoLookback is the unvalidated tail of TA_PPO_Lookback; optInMAType must
// already have MATypeDefault substituted.
func ppoLookback(optInFastPeriod, optInSlowPeriod int, optInMAType MAType) int {
	return maLookback(cMaxInt(optInSlowPeriod, optInFastPeriod), optInMAType)
}

// PPO returns ((fast MA - slow MA) / slow MA) * 100 of inReal, 0 where the
// slow MA is zero. If optInSlowPeriod < optInFastPeriod the periods are
// swapped. The output is index-aligned with inReal; the first PPOLookback
// bars are NaN.
// optInFastPeriod: [2, 100000], default 12. optInSlowPeriod: [2, 100000],
// default 26. optInMAType: default EMA.
func PPO(inReal []float64, optInFastPeriod, optInSlowPeriod int, optInMAType MAType) ([]float64, error) {
	if _, err := ppoCheck(optInFastPeriod, optInSlowPeriod, optInMAType); err != nil {
		return nil, err
	}
	n, err := checkInputs("PPO", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := ppoRange(0, n-1, inReal, optInFastPeriod, optInSlowPeriod, optInMAType, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// ppoRange mirrors TA_PPO: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx.
func ppoRange(startIdx, endIdx int, inReal []float64, optInFastPeriod, optInSlowPeriod int, optInMAType MAType, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	optInMAType, err = ppoCheck(optInFastPeriod, optInSlowPeriod, optInMAType)
	if err != nil {
		return 0, 0, err
	}
	if err := checkBuf("PPO", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("PPO", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// Nothing to produce: the range is shorter than the lookback.
	if maLookback(cMaxInt(optInSlowPeriod, optInFastPeriod), optInMAType) > endIdx {
		return 0, 0, nil
	}

	tempBuffer := make([]float64, endIdx-startIdx+1)

	// Make sure slow is really slower than the fast period; if not, swap.
	if optInSlowPeriod < optInFastPeriod {
		optInSlowPeriod, optInFastPeriod = optInFastPeriod, optInSlowPeriod
	}

	// Fast MA into tempBuffer.
	_, fastNb, err := maRange(startIdx, endIdx, inReal, optInFastPeriod, optInMAType, tempBuffer)
	if err != nil {
		return 0, 0, err
	}
	// Slow MA into the output.
	outBegIdx, outNBElement, err = maRange(startIdx, endIdx, inReal, optInSlowPeriod, optInMAType, outReal)
	if err != nil {
		return 0, 0, err
	}

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
