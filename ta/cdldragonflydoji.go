package ta

// CDLDRAGONFLYDOJI — Dragonfly Doji (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdldragonflydoji and
// src/ta_func/ta_CDLDRAGONFLYDOJI.c.

// CDLDRAGONFLYDOJILookback returns the number of leading bars
// CDLDRAGONFLYDOJI leaves empty. It depends on the current candle settings
// (BodyDoji, ShadowVeryShort). It has no parameters, so it never returns -1.
func CDLDRAGONFLYDOJILookback() int {
	return cdldragonflydojiLookback()
}

func cdldragonflydojiLookback() int {
	cs := candleSettingsSnapshot()
	return cdldragonflydojiLookbackFrom(&cs)
}

func cdldragonflydojiLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[BodyDoji].AvgPeriod, cs[ShadowVeryShort].AvgPeriod)
}

// CDLDRAGONFLYDOJI recognises the Dragonfly Doji: a doji body with no or a
// very short upper shadow and a lower shadow that is not very short. The
// output is index-aligned with the inputs: 100 where the pattern is present
// (this does not imply bullishness; read it against the trend), 0 otherwise
// and inside the lookback.
func CDLDRAGONFLYDOJI(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLDRAGONFLYDOJI", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdldragonflydojiRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdldragonflydojiRange mirrors TA_CDLDRAGONFLYDOJI: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdldragonflydojiRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLDRAGONFLYDOJI", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLDRAGONFLYDOJI", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyDoji := cs[BodyDoji]
	shadowVeryShort := cs[ShadowVeryShort]

	lookbackTotal := cdldragonflydojiLookbackFrom(&cs)
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

	// Must have: a doji body; open and close at the high of the day (no or a
	// very short upper shadow); a lower shadow that is not very short.
	i := startIdx
	outIdx := 0
	for {
		o, h, l, c := inOpen[i], inHigh[i], inLow[i], inClose[i]
		if realBody(c, o) <= candleAverage(bodyDoji, bodyDojiPeriodTotal, o, h, l, c) &&
			upperShadow(h, c, o) < candleAverage(shadowVeryShort, shadowVeryShortPeriodTotal, o, h, l, c) &&
			lowerShadow(l, c, o) > candleAverage(shadowVeryShort, shadowVeryShortPeriodTotal, o, h, l, c) {
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
