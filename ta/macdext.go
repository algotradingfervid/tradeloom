package ta

// MACDEXT — MACD with controllable MA type (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/macdext and src/ta_func/ta_MACDEXT.c.

// macdextCheck validates the MACDEXT optional inputs and applies the C
// prologue's MATypeDefault substitution (TA_MAType_DEFAULT -> SMA).
func macdextCheck(optInFastPeriod int, optInFastMAType MAType, optInSlowPeriod int, optInSlowMAType MAType, optInSignalPeriod int, optInSignalMAType MAType) (MAType, MAType, MAType, error) {
	if err := checkInt("MACDEXT", "optInFastPeriod", optInFastPeriod, 2, 100000); err != nil {
		return 0, 0, 0, err
	}
	if err := checkMAType("MACDEXT", "optInFastMAType", optInFastMAType); err != nil {
		return 0, 0, 0, err
	}
	if optInFastMAType == MATypeDefault {
		optInFastMAType = MATypeSMA
	}
	if err := checkInt("MACDEXT", "optInSlowPeriod", optInSlowPeriod, 2, 100000); err != nil {
		return 0, 0, 0, err
	}
	if err := checkMAType("MACDEXT", "optInSlowMAType", optInSlowMAType); err != nil {
		return 0, 0, 0, err
	}
	if optInSlowMAType == MATypeDefault {
		optInSlowMAType = MATypeSMA
	}
	if err := checkInt("MACDEXT", "optInSignalPeriod", optInSignalPeriod, 1, 100000); err != nil {
		return 0, 0, 0, err
	}
	if err := checkMAType("MACDEXT", "optInSignalMAType", optInSignalMAType); err != nil {
		return 0, 0, 0, err
	}
	if optInSignalMAType == MATypeDefault {
		optInSignalMAType = MATypeSMA
	}
	return optInFastMAType, optInSlowMAType, optInSignalMAType, nil
}

// MACDEXTLookback returns the number of leading bars MACDEXT leaves empty:
// the larger of the fast and slow MA lookbacks plus the signal MA lookback.
// It returns -1 on an invalid parameter.
// optInFastPeriod: [2, 100000], default 12. optInSlowPeriod: [2, 100000],
// default 26. optInSignalPeriod: [1, 100000], default 9. MA types default SMA.
func MACDEXTLookback(optInFastPeriod int, optInFastMAType MAType, optInSlowPeriod int, optInSlowMAType MAType, optInSignalPeriod int, optInSignalMAType MAType) int {
	f, s, g, err := macdextCheck(optInFastPeriod, optInFastMAType, optInSlowPeriod, optInSlowMAType, optInSignalPeriod, optInSignalMAType)
	if err != nil {
		return -1
	}
	return macdextLookback(optInFastPeriod, f, optInSlowPeriod, s, optInSignalPeriod, g)
}

// macdextLookback is the unvalidated tail of TA_MACDEXT_Lookback; the MA
// types must already have MATypeDefault substituted.
func macdextLookback(optInFastPeriod int, optInFastMAType MAType, optInSlowPeriod int, optInSlowMAType MAType, optInSignalPeriod int, optInSignalMAType MAType) int {
	lookbackLargest := maLookback(optInFastPeriod, optInFastMAType)
	tempInteger := maLookback(optInSlowPeriod, optInSlowMAType)
	if tempInteger > lookbackLargest {
		lookbackLargest = tempInteger
	}
	return lookbackLargest + maLookback(optInSignalPeriod, optInSignalMAType)
}

// MACDEXT returns the MACD line (fast MA minus slow MA), its signal line (an
// MA of the MACD line) and the histogram (MACD minus signal), each MA using
// its own period and MAType. If the slow period is smaller than the fast one,
// the periods and their MA types are swapped. An all-EMA configuration
// (signal period >= 2) is computed exactly as MACD. The outputs are
// index-aligned with inReal; the first MACDEXTLookback bars are NaN.
func MACDEXT(inReal []float64, optInFastPeriod int, optInFastMAType MAType, optInSlowPeriod int, optInSlowMAType MAType, optInSignalPeriod int, optInSignalMAType MAType) (outMACD, outMACDSignal, outMACDHist []float64, err error) {
	if _, _, _, err := macdextCheck(optInFastPeriod, optInFastMAType, optInSlowPeriod, optInSlowMAType, optInSignalPeriod, optInSignalMAType); err != nil {
		return nil, nil, nil, err
	}
	n, err := checkInputs("MACDEXT", []string{"inReal"}, inReal)
	if err != nil {
		return nil, nil, nil, err
	}
	outMACD = make([]float64, n)
	outMACDSignal = make([]float64, n)
	outMACDHist = make([]float64, n)
	if n == 0 {
		return outMACD, outMACDSignal, outMACDHist, nil
	}
	beg, nb, err := macdextRange(0, n-1, inReal, optInFastPeriod, optInFastMAType, optInSlowPeriod, optInSlowMAType, optInSignalPeriod, optInSignalMAType, outMACD, outMACDSignal, outMACDHist)
	if err != nil {
		return nil, nil, nil, err
	}
	alignReal(outMACD, beg, nb)
	alignReal(outMACDSignal, beg, nb)
	alignReal(outMACDHist, beg, nb)
	return outMACD, outMACDSignal, outMACDHist, nil
}

// macdextSameBuf reports whether two non-empty slices start at the same
// element (C's outMACD == outMACDSignal pointer comparison).
func macdextSameBuf(a, b []float64) bool {
	return len(a) > 0 && len(b) > 0 && &a[0] == &b[0]
}

// macdextRange mirrors TA_MACDEXT: it computes [startIdx, endIdx] and writes
// outNBElement values to each output from index 0, the first one belonging
// to bar outBegIdx. An output may be inReal, but the three outputs must be
// distinct buffers (C returns TA_BAD_PARAM otherwise).
func macdextRange(startIdx, endIdx int, inReal []float64, optInFastPeriod int, optInFastMAType MAType, optInSlowPeriod int, optInSlowMAType MAType, optInSignalPeriod int, optInSignalMAType MAType, outMACD, outMACDSignal, outMACDHist []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	optInFastMAType, optInSlowMAType, optInSignalMAType, err = macdextCheck(optInFastPeriod, optInFastMAType, optInSlowPeriod, optInSlowMAType, optInSignalPeriod, optInSignalMAType)
	if err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MACDEXT", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MACDEXT", endIdx-startIdx, []string{"outMACD", "outMACDSignal", "outMACDHist"}, outMACD, outMACDSignal, outMACDHist); err != nil {
		return 0, 0, err
	}
	if macdextSameBuf(outMACD, outMACDSignal) || macdextSameBuf(outMACD, outMACDHist) || macdextSameBuf(outMACDSignal, outMACDHist) {
		return 0, 0, badParam("MACDEXT", "outMACD", "output buffers must be distinct")
	}

	// An all-EMA MACDEXT computes exactly what MACD computes; C delegates to
	// its single-pass implementation. Period 1 stays on the generic path.
	if optInFastMAType == MATypeEMA && optInSlowMAType == MATypeEMA && optInSignalMAType == MATypeEMA && optInFastPeriod >= 2 && optInSlowPeriod >= 2 && optInSignalPeriod >= 2 {
		return macdRange(startIdx, endIdx, inReal, optInFastPeriod, optInSlowPeriod, optInSignalPeriod, outMACD, outMACDSignal, outMACDHist)
	}

	// Make sure slow is really slower than the fast period; if not, swap.
	if optInSlowPeriod < optInFastPeriod {
		optInSlowPeriod, optInFastPeriod = optInFastPeriod, optInSlowPeriod
		optInSlowMAType, optInFastMAType = optInFastMAType, optInSlowMAType
	}

	// Find the MA with the largest lookback.
	lookbackLargest := maLookback(optInFastPeriod, optInFastMAType)
	tempInteger := maLookback(optInSlowPeriod, optInSlowMAType)
	if tempInteger > lookbackLargest {
		lookbackLargest = tempInteger
	}
	// Add the lookback needed for the signal line.
	lookbackSignal := maLookback(optInSignalPeriod, optInSignalMAType)
	lookbackTotal := lookbackSignal + lookbackLargest
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// Intermediate buffers for the fast/slow MA.
	tempInteger = endIdx - startIdx + 1 + lookbackSignal
	fastMABuffer := make([]float64, tempInteger)
	slowMABuffer := make([]float64, tempInteger)

	// Move back startIdx by the signal lookback so that, once the signal is
	// computed, all the output starts at the requested startIdx.
	tempInteger = startIdx - lookbackSignal
	outBegIdx1, outNbElement1, err := maRange(tempInteger, endIdx, inReal, optInSlowPeriod, optInSlowMAType, slowMABuffer)
	if err != nil {
		return 0, 0, err
	}
	outBegIdx2, outNbElement2, err := maRange(tempInteger, endIdx, inReal, optInFastPeriod, optInFastMAType, fastMABuffer)
	if err != nil {
		return 0, 0, err
	}
	// C's "parano tests".
	if outBegIdx1 != tempInteger || outBegIdx2 != tempInteger || outNbElement1 != outNbElement2 || outNbElement1 != endIdx-startIdx+1+lookbackSignal {
		return 0, 0, badParam("MACDEXT", "inReal", "inconsistent fast/slow MA output range")
	}

	// (fast MA) - (slow MA).
	for i := 0; i < outNbElement1; i++ {
		fastMABuffer[i] = fastMABuffer[i] - slowMABuffer[i]
	}
	copy(outMACD[:endIdx-startIdx+1], fastMABuffer[lookbackSignal:])

	// Signal/trigger line.
	// C's TA_MA writes only the outNbElement2 (= endIdx-startIdx+1) values it
	// produces, but maRange's buffer check wants room up to its endIdx; give it
	// a scratch buffer when the caller's is sized exactly.
	sigBuf := outMACDSignal
	if len(sigBuf) < outNbElement1 {
		sigBuf = make([]float64, outNbElement1)
	}
	_, outNbElement2, err = maRange(0, outNbElement1-1, fastMABuffer, optInSignalPeriod, optInSignalMAType, sigBuf)
	if err != nil {
		return 0, 0, err
	}
	if &sigBuf[0] != &outMACDSignal[0] {
		copy(outMACDSignal[:outNbElement2], sigBuf[:outNbElement2])
	}

	// Histogram.
	for i := 0; i < outNbElement2; i++ {
		outMACDHist[i] = outMACD[i] - outMACDSignal[i]
	}
	return startIdx, outNbElement2, nil
}
