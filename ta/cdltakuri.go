package ta

// CDLTAKURI — Takuri (Dragonfly Doji with very long lower shadow) (Pattern
// Recognition). Ported from TA-Lib v0.8.1 ta_codegen/input/cdltakuri and
// src/ta_func/ta_CDLTAKURI.c.

// CDLTAKURILookback returns the number of leading bars CDLTAKURI leaves
// empty. It depends on the current candle settings (BodyDoji,
// ShadowVeryShort, ShadowVeryLong). It has no parameters, so it never
// returns -1.
func CDLTAKURILookback() int {
	return cdltakuriLookback()
}

func cdltakuriLookback() int {
	cs := candleSettingsSnapshot()
	return cdltakuriLookbackFrom(&cs)
}

func cdltakuriLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cMaxInt(cs[BodyDoji].AvgPeriod, cs[ShadowVeryShort].AvgPeriod), cs[ShadowVeryLong].AvgPeriod)
}

// CDLTAKURI recognises the Takuri line: a doji body, no or a very short
// upper shadow and a very long lower shadow. The output is index-aligned
// with the inputs: 100 where the pattern is present (this does not imply
// bullishness; read it against the trend), 0 otherwise and inside the
// lookback.
func CDLTAKURI(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLTAKURI", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdltakuriRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdltakuriRange mirrors TA_CDLTAKURI: it evaluates [startIdx, endIdx] and
// writes outNBElement values to outInteger[0:], the first belonging to bar
// outBegIdx.
func cdltakuriRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLTAKURI", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLTAKURI", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyDoji := cs[BodyDoji]
	shadowVeryShort := cs[ShadowVeryShort]
	shadowVeryLong := cs[ShadowVeryLong]

	lookbackTotal := cdltakuriLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(s CandleSetting, i int) float64 {
		return candleRange(s.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	// Add up the initial period, except for the last value.
	bodyDojiPeriodTotal := 0.0
	bodyDojiTrailingIdx := startIdx - bodyDoji.AvgPeriod
	shadowVeryShortPeriodTotal := 0.0
	shadowVeryShortTrailingIdx := startIdx - shadowVeryShort.AvgPeriod
	shadowVeryLongPeriodTotal := 0.0
	shadowVeryLongTrailingIdx := startIdx - shadowVeryLong.AvgPeriod

	for i := bodyDojiTrailingIdx; i < startIdx; i++ {
		bodyDojiPeriodTotal += rng(bodyDoji, i)
	}
	for i := shadowVeryShortTrailingIdx; i < startIdx; i++ {
		shadowVeryShortPeriodTotal += rng(shadowVeryShort, i)
	}
	for i := shadowVeryLongTrailingIdx; i < startIdx; i++ {
		shadowVeryLongPeriodTotal += rng(shadowVeryLong, i)
	}

	// Must have: a doji body; open and close at the high of the day (no or a
	// very short upper shadow); a very long lower shadow.
	i := startIdx
	outIdx := 0
	for {
		o, h, l, c := inOpen[i], inHigh[i], inLow[i], inClose[i]
		if realBody(c, o) <= candleAverage(bodyDoji, bodyDojiPeriodTotal, o, h, l, c) &&
			upperShadow(h, c, o) < candleAverage(shadowVeryShort, shadowVeryShortPeriodTotal, o, h, l, c) &&
			lowerShadow(l, c, o) > candleAverage(shadowVeryLong, shadowVeryLongPeriodTotal, o, h, l, c) {
			outInteger[outIdx] = 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyDojiPeriodTotal += rng(bodyDoji, i) - rng(bodyDoji, bodyDojiTrailingIdx)
		shadowVeryShortPeriodTotal += rng(shadowVeryShort, i) - rng(shadowVeryShort, shadowVeryShortTrailingIdx)
		shadowVeryLongPeriodTotal += rng(shadowVeryLong, i) - rng(shadowVeryLong, shadowVeryLongTrailingIdx)
		i++
		bodyDojiTrailingIdx++
		shadowVeryShortTrailingIdx++
		shadowVeryLongTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
