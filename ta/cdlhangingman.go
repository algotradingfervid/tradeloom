package ta

// CDLHANGINGMAN — Hanging Man (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlhangingman and src/ta_func/ta_CDLHANGINGMAN.c.

// CDLHANGINGMANLookback returns the number of leading bars CDLHANGINGMAN leaves
// empty. It depends on the current candle settings (BodyShort, ShadowLong,
// ShadowVeryShort, Near). It has no parameters, so it never returns -1.
func CDLHANGINGMANLookback() int {
	return cdlhangingmanLookback()
}

func cdlhangingmanLookback() int {
	cs := candleSettingsSnapshot()
	return cdlhangingmanLookbackFrom(&cs)
}

func cdlhangingmanLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cMaxInt(cMaxInt(cs[BodyShort].AvgPeriod, cs[ShadowLong].AvgPeriod), cs[ShadowVeryShort].AvgPeriod), cs[Near].AvgPeriod) + 1
}

// CDLHANGINGMAN recognises the Hanging Man: a small real body with a long
// lower shadow, no or a very short upper shadow, and the body near the prior
// candle's high. It does not check for the preceding uptrend.
// The output is index-aligned with the inputs: -100 where the pattern is
// found (always bearish), 0 otherwise and inside the lookback.
func CDLHANGINGMAN(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLHANGINGMAN", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlhangingmanRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlhangingmanRange mirrors TA_CDLHANGINGMAN: it evaluates [startIdx, endIdx]
// and writes outNBElement values to outInteger[0:], the first belonging to bar
// outBegIdx.
func cdlhangingmanRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLHANGINGMAN", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLHANGINGMAN", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyShort := cs[BodyShort]
	shadowLong := cs[ShadowLong]
	shadowVeryShort := cs[ShadowVeryShort]
	near := cs[Near]

	lookbackTotal := cdlhangingmanLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(s CandleSetting, i int) float64 {
		return candleRange(s.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	bodyPeriodTotal := 0.0
	bodyTrailingIdx := startIdx - bodyShort.AvgPeriod
	shadowLongPeriodTotal := 0.0
	shadowLongTrailingIdx := startIdx - shadowLong.AvgPeriod
	shadowVeryShortPeriodTotal := 0.0
	shadowVeryShortTrailingIdx := startIdx - shadowVeryShort.AvgPeriod
	nearPeriodTotal := 0.0
	nearTrailingIdx := startIdx - 1 - near.AvgPeriod

	for i := bodyTrailingIdx; i < startIdx; i++ {
		bodyPeriodTotal += rng(bodyShort, i)
	}
	for i := shadowLongTrailingIdx; i < startIdx; i++ {
		shadowLongPeriodTotal += rng(shadowLong, i)
	}
	for i := shadowVeryShortTrailingIdx; i < startIdx; i++ {
		shadowVeryShortPeriodTotal += rng(shadowVeryShort, i)
	}
	for i := nearTrailingIdx; i < startIdx-1; i++ {
		nearPeriodTotal += rng(near, i)
	}

	i := startIdx
	outIdx := 0
	for {
		o, h, l, c := inOpen[i], inHigh[i], inLow[i], inClose[i]
		if realBody(c, o) < candleAverage(bodyShort, bodyPeriodTotal, o, h, l, c) && // small rb
			lowerShadow(l, c, o) > candleAverage(shadowLong, shadowLongPeriodTotal, o, h, l, c) && // long lower shadow
			upperShadow(h, c, o) < candleAverage(shadowVeryShort, shadowVeryShortPeriodTotal, o, h, l, c) && // very short upper shadow
			cMin(c, o) >= inHigh[i-1]-candleAverage(near, nearPeriodTotal, inOpen[i-1], inHigh[i-1], inLow[i-1], inClose[i-1]) { // rb near the prior candle's highs
			outInteger[outIdx] = -100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyPeriodTotal += rng(bodyShort, i) - rng(bodyShort, bodyTrailingIdx)
		shadowLongPeriodTotal += rng(shadowLong, i) - rng(shadowLong, shadowLongTrailingIdx)
		shadowVeryShortPeriodTotal += rng(shadowVeryShort, i) - rng(shadowVeryShort, shadowVeryShortTrailingIdx)
		nearPeriodTotal += rng(near, i-1) - rng(near, nearTrailingIdx)
		i++
		bodyTrailingIdx++
		shadowLongTrailingIdx++
		shadowVeryShortTrailingIdx++
		nearTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
