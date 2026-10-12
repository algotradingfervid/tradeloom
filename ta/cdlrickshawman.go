package ta

// CDLRICKSHAWMAN — Rickshaw Man (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlrickshawman and
// src/ta_func/ta_CDLRICKSHAWMAN.c.

// CDLRICKSHAWMANLookback returns the number of leading bars CDLRICKSHAWMAN
// leaves empty. It depends on the current candle settings (BodyDoji,
// ShadowLong, Near). It has no parameters, so it never returns -1.
func CDLRICKSHAWMANLookback() int {
	return cdlrickshawmanLookback()
}

func cdlrickshawmanLookback() int {
	cs := candleSettingsSnapshot()
	return cdlrickshawmanLookbackFrom(&cs)
}

func cdlrickshawmanLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cMaxInt(cs[BodyDoji].AvgPeriod, cs[ShadowLong].AvgPeriod), cs[Near].AvgPeriod)
}

// CDLRICKSHAWMAN recognises the Rickshaw Man: a doji body with two long
// shadows, the body near the midpoint of the high-low range. The output is
// index-aligned with the inputs: 100 where the pattern is present (this
// signals indecision, not bullishness), 0 otherwise and inside the lookback.
func CDLRICKSHAWMAN(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLRICKSHAWMAN", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlrickshawmanRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlrickshawmanRange mirrors TA_CDLRICKSHAWMAN: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlrickshawmanRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLRICKSHAWMAN", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLRICKSHAWMAN", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyDoji := cs[BodyDoji]
	shadowLong := cs[ShadowLong]
	near := cs[Near]

	lookbackTotal := cdlrickshawmanLookbackFrom(&cs)
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
	shadowLongPeriodTotal := 0.0
	shadowLongTrailingIdx := startIdx - shadowLong.AvgPeriod
	nearPeriodTotal := 0.0
	nearTrailingIdx := startIdx - near.AvgPeriod

	for i := bodyDojiTrailingIdx; i < startIdx; i++ {
		bodyDojiPeriodTotal += rng(bodyDoji, i)
	}
	for i := shadowLongTrailingIdx; i < startIdx; i++ {
		shadowLongPeriodTotal += rng(shadowLong, i)
	}
	for i := nearTrailingIdx; i < startIdx; i++ {
		nearPeriodTotal += rng(near, i)
	}

	// Must have: a doji body; two long shadows; the body near the midpoint
	// of the high-low range.
	i := startIdx
	outIdx := 0
	for {
		o, h, l, c := inOpen[i], inHigh[i], inLow[i], inClose[i]
		if realBody(c, o) <= candleAverage(bodyDoji, bodyDojiPeriodTotal, o, h, l, c) &&
			lowerShadow(l, c, o) > candleAverage(shadowLong, shadowLongPeriodTotal, o, h, l, c) &&
			upperShadow(h, c, o) > candleAverage(shadowLong, shadowLongPeriodTotal, o, h, l, c) &&
			(cMin(o, c) <= l+float64((h-l)/2)+candleAverage(near, nearPeriodTotal, o, h, l, c) &&
				cMax(o, c) >= l+float64((h-l)/2)-candleAverage(near, nearPeriodTotal, o, h, l, c)) {
			outInteger[outIdx] = 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyDojiPeriodTotal += rng(bodyDoji, i) - rng(bodyDoji, bodyDojiTrailingIdx)
		shadowLongPeriodTotal += rng(shadowLong, i) - rng(shadowLong, shadowLongTrailingIdx)
		nearPeriodTotal += rng(near, i) - rng(near, nearTrailingIdx)
		i++
		bodyDojiTrailingIdx++
		shadowLongTrailingIdx++
		nearTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
