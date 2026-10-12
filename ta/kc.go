package ta

// KC — Keltner Channels (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/kc and src/ta_func/ta_KC.c.
//
// Middle = EMA(TYPPRICE, optInTimePeriod); band = ATR(optInATRPeriod) *
// optInNbDev; upper/lower = middle ± band. Each leg is entered at its own
// lookback, so KC inherits the EMA and ATR unstable periods.

// KCLookback returns the number of leading bars KC leaves empty: the larger
// of the EMA and ATR lookbacks. Returns -1 on an invalid parameter.
// optInTimePeriod: [2, 100000], default 20; optInATRPeriod: [1, 100000],
// default 10; optInNbDev: [TA_REAL_MIN, TA_REAL_MAX], default 2.
func KCLookback(optInTimePeriod, optInATRPeriod int, optInNbDev float64) int {
	if kcCheckParams(optInTimePeriod, optInATRPeriod, optInNbDev) != nil {
		return -1
	}
	return kcLookback(optInTimePeriod, optInATRPeriod, optInNbDev)
}

func kcLookback(optInTimePeriod, optInATRPeriod int, optInNbDev float64) int {
	emaLb := emaLookback(optInTimePeriod)
	atrLb := atrLookback(optInATRPeriod)
	if emaLb > atrLb {
		return emaLb
	}
	return atrLb
}

func kcCheckParams(optInTimePeriod, optInATRPeriod int, optInNbDev float64) error {
	if err := checkInt("KC", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return err
	}
	if err := checkInt("KC", "optInATRPeriod", optInATRPeriod, 1, 100000); err != nil {
		return err
	}
	return checkReal("KC", "optInNbDev", optInNbDev, taRealMin, taRealMax)
}

// KC returns the upper, middle and lower Keltner Channel bands. Outputs are
// index-aligned with the inputs; the first KCLookback bars are NaN.
func KC(inHigh, inLow, inClose []float64, optInTimePeriod, optInATRPeriod int, optInNbDev float64) (outRealUpperBand, outRealMiddleBand, outRealLowerBand []float64, err error) {
	if err := kcCheckParams(optInTimePeriod, optInATRPeriod, optInNbDev); err != nil {
		return nil, nil, nil, err
	}
	n, err := checkInputs("KC", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, nil, nil, err
	}
	upper := make([]float64, n)
	middle := make([]float64, n)
	lower := make([]float64, n)
	if n == 0 {
		return upper, middle, lower, nil
	}
	beg, nb, err := kcRange(0, n-1, inHigh, inLow, inClose, optInTimePeriod, optInATRPeriod, optInNbDev, upper, middle, lower)
	if err != nil {
		return nil, nil, nil, err
	}
	alignReal(upper, beg, nb)
	alignReal(middle, beg, nb)
	alignReal(lower, beg, nb)
	return upper, middle, lower, nil
}

// kcSameBuf reports whether two buffers start at the same address, the Go
// analogue of C's output pointer equality test.
func kcSameBuf(a, b []float64) bool {
	return len(a) > 0 && len(b) > 0 && &a[0] == &b[0]
}

// kcRange mirrors TA_KC: it computes [startIdx, endIdx] and writes
// outNBElement values to each output from index 0, the first one belonging
// to bar outBegIdx. The three outputs must be distinct buffers (as in C); an
// output may be the same slice as an input.
func kcRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInTimePeriod, optInATRPeriod int, optInNbDev float64,
	outRealUpperBand, outRealMiddleBand, outRealLowerBand []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := kcCheckParams(optInTimePeriod, optInATRPeriod, optInNbDev); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("KC", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("KC", endIdx-startIdx, []string{"outRealUpperBand", "outRealMiddleBand", "outRealLowerBand"},
		outRealUpperBand, outRealMiddleBand, outRealLowerBand); err != nil {
		return 0, 0, err
	}
	if kcSameBuf(outRealUpperBand, outRealMiddleBand) || kcSameBuf(outRealUpperBand, outRealLowerBand) ||
		kcSameBuf(outRealMiddleBand, outRealLowerBand) {
		return 0, 0, badParam("KC", "outRealUpperBand", "output buffers must be distinct")
	}

	emaLb := emaLookback(optInTimePeriod)
	lookbackTotal := kcLookback(optInTimePeriod, optInATRPeriod, optInNbDev)
	if lookbackTotal > endIdx {
		return 0, 0, nil
	}
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	tpStartIdx := startIdx - emaLb
	tempTP := make([]float64, endIdx-tpStartIdx+1)
	tempATR := make([]float64, endIdx-startIdx+1)
	if _, _, err := typpriceRange(tpStartIdx, endIdx, inHigh, inLow, inClose, tempTP); err != nil {
		return 0, 0, err
	}
	// The ATR consumes the price inputs before the EMA below writes the
	// middle band, which may alias one of them.
	if _, _, err := atrRange(startIdx, endIdx, inHigh, inLow, inClose, optInATRPeriod, tempATR); err != nil {
		return 0, 0, err
	}
	// tempTP is bar-tpStartIdx relative: entering the EMA at its own lookback
	// puts its first output on startIdx, where the ATR's already is.
	_, nb, err := emaRange(emaLb, endIdx-tpStartIdx, tempTP, optInTimePeriod, outRealMiddleBand)
	if err != nil || nb == 0 {
		return 0, 0, err
	}
	for i := 0; i < nb; i++ {
		middle := outRealMiddleBand[i]
		tempReal := float64(tempATR[i] * optInNbDev)
		outRealUpperBand[i] = middle + tempReal
		outRealLowerBand[i] = middle - tempReal
	}
	return startIdx, nb, nil
}
