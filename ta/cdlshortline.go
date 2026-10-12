package ta

// CDLSHORTLINE — Short Line Candle (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlshortline and src/ta_func/ta_CDLSHORTLINE.c.

// CDLSHORTLINELookback returns the number of leading bars CDLSHORTLINE leaves
// empty: the larger of the BodyShort and ShadowShort candle settings' average
// periods. It has no parameters, so it never returns -1.
func CDLSHORTLINELookback() int {
	return cdlshortlineLookback()
}

func cdlshortlineLookback() int {
	cs := candleSettingsSnapshot()
	return cdlshortlineLookbackFrom(&cs)
}

func cdlshortlineLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[BodyShort].AvgPeriod, cs[ShadowShort].AvgPeriod)
}

// CDLSHORTLINE recognises the Short Line candle: a short real body (below the
// BodyShort average) with both a short upper and a short lower shadow (each
// below the ShadowShort average). The output is index-aligned with the
// inputs: +100 for a white (close >= open) hit, -100 for a black hit, 0
// otherwise and inside the lookback.
func CDLSHORTLINE(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLSHORTLINE", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlshortlineRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlshortlineRange mirrors TA_CDLSHORTLINE: it evaluates [startIdx, endIdx]
// and writes outNBElement values to outInteger[0:], the first belonging to
// bar outBegIdx.
func cdlshortlineRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLSHORTLINE", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLSHORTLINE", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyShort := cs[BodyShort]
	shadowShort := cs[ShadowShort]

	lookbackTotal := cdlshortlineLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	bodyRange := func(i int) float64 {
		return candleRange(bodyShort.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}
	shadowRange := func(i int) float64 {
		return candleRange(shadowShort.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	// Add up the initial period, except for the last value.
	bodyPeriodTotal := 0.0
	bodyTrailingIdx := startIdx - bodyShort.AvgPeriod
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

	// Must have a short real body and short upper and lower shadows.
	outIdx := 0
	for {
		o, h, l, c := inOpen[i], inHigh[i], inLow[i], inClose[i]
		if realBody(c, o) < candleAverage(bodyShort, bodyPeriodTotal, o, h, l, c) &&
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
