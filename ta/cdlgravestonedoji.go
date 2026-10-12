package ta

// CDLGRAVESTONEDOJI — Gravestone Doji (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlgravestonedoji and
// src/ta_func/ta_CDLGRAVESTONEDOJI.c.

// CDLGRAVESTONEDOJILookback returns the number of leading bars
// CDLGRAVESTONEDOJI leaves empty. It depends on the current candle settings
// (BodyDoji, ShadowVeryShort). It has no parameters, so it never returns -1.
func CDLGRAVESTONEDOJILookback() int {
	return cdlgravestonedojiLookback()
}

func cdlgravestonedojiLookback() int {
	cs := candleSettingsSnapshot()
	return cdlgravestonedojiLookbackFrom(&cs)
}

func cdlgravestonedojiLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[BodyDoji].AvgPeriod, cs[ShadowVeryShort].AvgPeriod)
}

// CDLGRAVESTONEDOJI recognises the Gravestone Doji: a doji body with open and
// close at the low of the day (no or a very short lower shadow) and an upper
// shadow that is not very short. The output is index-aligned with the inputs:
// 100 on a detected pattern (not a directional signal; read it against the
// trend), 0 otherwise and inside the lookback.
func CDLGRAVESTONEDOJI(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLGRAVESTONEDOJI", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlgravestonedojiRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlgravestonedojiRange mirrors TA_CDLGRAVESTONEDOJI: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlgravestonedojiRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLGRAVESTONEDOJI", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLGRAVESTONEDOJI", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyDoji := cs[BodyDoji]
	shadowVeryShort := cs[ShadowVeryShort]

	lookbackTotal := cdlgravestonedojiLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(s CandleSetting, i int) float64 {
		return candleRange(s.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	bodyDojiPeriodTotal := 0.0
	bodyDojiTrailingIdx := startIdx - bodyDoji.AvgPeriod
	shadowVeryShortPeriodTotal := 0.0
	shadowVeryShortTrailingIdx := startIdx - shadowVeryShort.AvgPeriod

	for i := bodyDojiTrailingIdx; i < startIdx; i++ {
		bodyDojiPeriodTotal += rng(bodyDoji, i)
	}
	for i := shadowVeryShortTrailingIdx; i < startIdx; i++ {
		shadowVeryShortPeriodTotal += rng(shadowVeryShort, i)
	}

	// Must have: a doji body; open and close at the low of the day (no or a
	// very short lower shadow); an upper shadow that is not very short.
	i := startIdx
	outIdx := 0
	for {
		o, h, l, c := inOpen[i], inHigh[i], inLow[i], inClose[i]
		if realBody(c, o) <= candleAverage(bodyDoji, bodyDojiPeriodTotal, o, h, l, c) &&
			lowerShadow(l, c, o) < candleAverage(shadowVeryShort, shadowVeryShortPeriodTotal, o, h, l, c) &&
			upperShadow(h, c, o) > candleAverage(shadowVeryShort, shadowVeryShortPeriodTotal, o, h, l, c) {
			outInteger[outIdx] = 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyDojiPeriodTotal += rng(bodyDoji, i) - rng(bodyDoji, bodyDojiTrailingIdx)
		shadowVeryShortPeriodTotal += rng(shadowVeryShort, i) - rng(shadowVeryShort, shadowVeryShortTrailingIdx)
		i++
		bodyDojiTrailingIdx++
		shadowVeryShortTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
