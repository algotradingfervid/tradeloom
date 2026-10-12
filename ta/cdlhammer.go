package ta

// CDLHAMMER — Hammer (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlhammer and src/ta_func/ta_CDLHAMMER.c.

// CDLHAMMERLookback returns the number of leading bars CDLHAMMER leaves empty.
// It depends on the current candle settings (BodyShort, ShadowLong,
// ShadowVeryShort, Near). It has no parameters, so it never returns -1.
func CDLHAMMERLookback() int {
	return cdlhammerLookback()
}

func cdlhammerLookback() int {
	cs := candleSettingsSnapshot()
	return cdlhammerLookbackFrom(&cs)
}

func cdlhammerLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cMaxInt(cMaxInt(cs[BodyShort].AvgPeriod, cs[ShadowLong].AvgPeriod), cs[ShadowVeryShort].AvgPeriod), cs[Near].AvgPeriod) + 1
}

// CDLHAMMER recognises the Hammer: a small real body with a long lower shadow,
// no or a very short upper shadow, and the body near the prior candle's low.
// The output is index-aligned with the inputs: 100 where the pattern is found,
// 0 otherwise and inside the lookback.
func CDLHAMMER(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLHAMMER", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlhammerRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlhammerCheckIntBuf is checkBuf for an integer output buffer.
func cdlhammerCheckIntBuf(fn string, endIdx int, name string, b []int) error {
	if len(b) <= endIdx {
		return badParam(fn, name, "length %d too short for endIdx %d", len(b), endIdx)
	}
	return nil
}

// cdlhammerRange mirrors TA_CDLHAMMER: it evaluates [startIdx, endIdx] and
// writes outNBElement values to outInteger[0:], the first belonging to bar
// outBegIdx.
func cdlhammerRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLHAMMER", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := cdlhammerCheckIntBuf("CDLHAMMER", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyShort := cs[BodyShort]
	shadowLong := cs[ShadowLong]
	shadowVeryShort := cs[ShadowVeryShort]
	near := cs[Near]

	lookbackTotal := cdlhammerLookbackFrom(&cs)
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
			cMin(c, o) <= inLow[i-1]+candleAverage(near, nearPeriodTotal, inOpen[i-1], inHigh[i-1], inLow[i-1], inClose[i-1]) { // rb near the prior candle's lows
			outInteger[outIdx] = 100
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
