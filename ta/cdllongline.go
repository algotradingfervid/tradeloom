package ta

// CDLLONGLINE — Long Line Candle (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdllongline and src/ta_func/ta_CDLLONGLINE.c.

// CDLLONGLINELookback returns the number of leading bars CDLLONGLINE leaves
// empty: the larger of the BodyLong and ShadowShort candle settings' average
// periods. It has no parameters, so it never returns -1.
func CDLLONGLINELookback() int {
	return cdllonglineLookback()
}

func cdllonglineLookback() int {
	cs := candleSettingsSnapshot()
	return cdllonglineLookbackFrom(&cs)
}

func cdllonglineLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[BodyLong].AvgPeriod, cs[ShadowShort].AvgPeriod)
}

// CDLLONGLINE recognises the Long Line candle: a long real body (above the
// BodyLong average) with both a short upper and a short lower shadow (each
// below the ShadowShort average). The output is index-aligned with the
// inputs: +100 for a white (close >= open) hit, -100 for a black hit, 0
// otherwise and inside the lookback.
func CDLLONGLINE(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLLONGLINE", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdllonglineRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdllonglineRange mirrors TA_CDLLONGLINE: it evaluates [startIdx, endIdx]
// and writes outNBElement values to outInteger[0:], the first belonging to
// bar outBegIdx.
func cdllonglineRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLLONGLINE", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLLONGLINE", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	shadowShort := cs[ShadowShort]

	lookbackTotal := cdllonglineLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	bodyRange := func(i int) float64 {
		return candleRange(bodyLong.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}
	shadowRange := func(i int) float64 {
		return candleRange(shadowShort.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	// Add up the initial period, except for the last value.
	bodyPeriodTotal := 0.0
	bodyTrailingIdx := startIdx - bodyLong.AvgPeriod
	shadowPeriodTotal := 0.0
	shadowTrailingIdx := startIdx - shadowShort.AvgPeriod
	i := bodyTrailingIdx
	for i < startIdx {
		bodyPeriodTotal += bodyRange(i)
		i++
	}
	i = shadowTrailingIdx
	for i < startIdx {
		shadowPeriodTotal += shadowRange(i)
		i++
	}

	// Must have a long real body and short upper and lower shadows.
	outIdx := 0
	for {
		o, h, l, c := inOpen[i], inHigh[i], inLow[i], inClose[i]
		if realBody(c, o) > candleAverage(bodyLong, bodyPeriodTotal, o, h, l, c) &&
			upperShadow(h, c, o) < candleAverage(shadowShort, shadowPeriodTotal, o, h, l, c) &&
			lowerShadow(l, c, o) < candleAverage(shadowShort, shadowPeriodTotal, o, h, l, c) {
			outInteger[outIdx] = candleColor(c, o) * 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyPeriodTotal += bodyRange(i) - bodyRange(bodyTrailingIdx)
		shadowPeriodTotal += shadowRange(i) - shadowRange(shadowTrailingIdx)
		i++
		bodyTrailingIdx++
		shadowTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
