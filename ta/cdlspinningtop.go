package ta

// CDLSPINNINGTOP — Spinning Top (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlspinningtop and src/ta_func/ta_CDLSPINNINGTOP.c.

// CDLSPINNINGTOPLookback returns the number of leading bars CDLSPINNINGTOP
// leaves empty: the BodyShort candle setting's average period. It has no
// parameters, so it never returns -1.
func CDLSPINNINGTOPLookback() int {
	return cdlspinningtopLookback()
}

func cdlspinningtopLookback() int {
	cs := candleSettingsSnapshot()
	return cdlspinningtopLookbackFrom(&cs)
}

func cdlspinningtopLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cs[BodyShort].AvgPeriod
}

// CDLSPINNINGTOP recognises the Spinning Top: a short real body (below the
// BodyShort average) with an upper and a lower shadow each longer than the
// body. The output is index-aligned with the inputs: +100 for a white
// (close >= open) hit, -100 for a black hit, 0 otherwise and inside the
// lookback. The sign reports only the candle's colour, not a bullish or
// bearish signal.
func CDLSPINNINGTOP(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLSPINNINGTOP", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlspinningtopRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlspinningtopRange mirrors TA_CDLSPINNINGTOP: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlspinningtopRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLSPINNINGTOP", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLSPINNINGTOP", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyShort := cs[BodyShort]

	lookbackTotal := cdlspinningtopLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	bodyRange := func(i int) float64 {
		return candleRange(bodyShort.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	// Add up the initial period, except for the last value.
	bodyPeriodTotal := 0.0
	bodyTrailingIdx := startIdx - bodyShort.AvgPeriod
	i := bodyTrailingIdx
	for i < startIdx {
		bodyPeriodTotal += bodyRange(i)
		i++
	}

	// Must have a small real body and both shadows longer than the body.
	outIdx := 0
	for {
		o, h, l, c := inOpen[i], inHigh[i], inLow[i], inClose[i]
		body := realBody(c, o)
		if upperShadow(h, c, o) > body &&
			lowerShadow(l, c, o) > body &&
			body < candleAverage(bodyShort, bodyPeriodTotal, o, h, l, c) {
			outInteger[outIdx] = candleColor(c, o) * 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyPeriodTotal += bodyRange(i) - bodyRange(bodyTrailingIdx)
		i++
		bodyTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
