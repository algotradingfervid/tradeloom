package ta

// CDLDOJI — Doji (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdldoji and src/ta_func/ta_CDLDOJI.c.

// CDLDOJILookback returns the number of leading bars CDLDOJI leaves empty.
// It equals the BodyDoji candle setting's average period. It has no
// parameters, so it never returns -1.
func CDLDOJILookback() int {
	return cdldojiLookback()
}

func cdldojiLookback() int {
	cs := candleSettingsSnapshot()
	return cdldojiLookbackFrom(&cs)
}

func cdldojiLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cs[BodyDoji].AvgPeriod
}

// CDLDOJI recognises the Doji: a candle whose real body |close-open| is at
// or below the BodyDoji threshold (by default 10% of the average high-low
// range of the previous 10 candles). The output is index-aligned with the
// inputs: 100 where a doji is found, 0 otherwise and inside the lookback.
// A doji signals indecision; 100 does not mean bullish.
func CDLDOJI(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLDOJI", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdldojiRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdldojiRange mirrors TA_CDLDOJI: it evaluates [startIdx, endIdx] and
// writes outNBElement values to outInteger[0:], the first belonging to bar
// outBegIdx.
func cdldojiRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLDOJI", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLDOJI", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyDoji := cs[BodyDoji]

	lookbackTotal := cdldojiLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(i int) float64 {
		return candleRange(bodyDoji.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	// Add up the initial period, except for the last value.
	bodyDojiPeriodTotal := 0.0
	bodyDojiTrailingIdx := startIdx - bodyDoji.AvgPeriod
	i := bodyDojiTrailingIdx
	for i < startIdx {
		bodyDojiPeriodTotal += rng(i)
		i++
	}

	outIdx := 0
	for {
		if realBody(inClose[i], inOpen[i]) <= candleAverage(bodyDoji, bodyDojiPeriodTotal, inOpen[i], inHigh[i], inLow[i], inClose[i]) {
			outInteger[outIdx] = 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyDojiPeriodTotal += rng(i) - rng(bodyDojiTrailingIdx)
		i++
		bodyDojiTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
