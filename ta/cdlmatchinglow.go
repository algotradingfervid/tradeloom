package ta

// CDLMATCHINGLOW — Matching Low (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlmatchinglow and src/ta_func/ta_CDLMATCHINGLOW.c.

// CDLMATCHINGLOWLookback returns the number of leading bars CDLMATCHINGLOW
// leaves empty. It depends on the current Equal candle setting. It has no
// parameters, so it never returns -1.
func CDLMATCHINGLOWLookback() int {
	return cdlmatchinglowLookback()
}

func cdlmatchinglowLookback() int {
	cs := candleSettingsSnapshot()
	return cdlmatchinglowLookbackFrom(&cs)
}

func cdlmatchinglowLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cs[Equal].AvgPeriod + 1
}

// CDLMATCHINGLOW recognises the Matching Low: two consecutive black candles
// whose closes are equal within the Equal candle-setting tolerance. The
// output is index-aligned with the inputs: 100 where the pattern is found (it
// is always bullish; the trend is not checked), 0 otherwise and inside the
// lookback.
func CDLMATCHINGLOW(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLMATCHINGLOW", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlmatchinglowRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlmatchinglowRange mirrors TA_CDLMATCHINGLOW: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlmatchinglowRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLMATCHINGLOW", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLMATCHINGLOW", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	equal := cs[Equal]

	lookbackTotal := cdlmatchinglowLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(i int) float64 {
		return candleRange(equal.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	// Add up the initial period, except for the last value.
	equalPeriodTotal := 0.0
	equalTrailingIdx := startIdx - equal.AvgPeriod
	for i := equalTrailingIdx; i < startIdx; i++ {
		equalPeriodTotal += rng(i - 1)
	}

	i := startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-1], inOpen[i-1]) == -1 && // 1st black
			candleColor(inClose[i], inOpen[i]) == -1 && // 2nd black
			inClose[i] <= inClose[i-1]+candleAverage(equal, equalPeriodTotal, inOpen[i-1], inHigh[i-1], inLow[i-1], inClose[i-1]) && // same close
			inClose[i] >= inClose[i-1]-candleAverage(equal, equalPeriodTotal, inOpen[i-1], inHigh[i-1], inLow[i-1], inClose[i-1]) {
			outInteger[outIdx] = 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		equalPeriodTotal += rng(i-1) - rng(equalTrailingIdx-1)
		i++
		equalTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
