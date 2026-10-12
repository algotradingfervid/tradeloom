package ta

// CDLUNIQUE3RIVER — Unique 3 River (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlunique3river and src/ta_func/ta_CDLUNIQUE3RIVER.c.

// CDLUNIQUE3RIVERLookback returns the number of leading bars CDLUNIQUE3RIVER
// leaves empty. It depends on the current BodyLong and BodyShort candle
// settings. It has no parameters, so it never returns -1.
func CDLUNIQUE3RIVERLookback() int {
	return cdlunique3riverLookback()
}

func cdlunique3riverLookback() int {
	cs := candleSettingsSnapshot()
	return cdlunique3riverLookbackFrom(&cs)
}

func cdlunique3riverLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[BodyShort].AvgPeriod, cs[BodyLong].AvgPeriod) + 2
}

// CDLUNIQUE3RIVER recognises the Unique 3 River: a long black candle, a black
// harami candle (open not above the first open, close above the first close)
// with a lower low, then a short white candle opening above the second low.
// The output is index-aligned with the inputs: 100 where the pattern is found
// (it is always bullish; the trend is not checked), 0 otherwise and inside the
// lookback.
func CDLUNIQUE3RIVER(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLUNIQUE3RIVER", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlunique3riverRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlunique3riverRange mirrors TA_CDLUNIQUE3RIVER: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlunique3riverRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLUNIQUE3RIVER", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLUNIQUE3RIVER", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	bodyShort := cs[BodyShort]

	lookbackTotal := cdlunique3riverLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rngLong := func(i int) float64 {
		return candleRange(bodyLong.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}
	rngShort := func(i int) float64 {
		return candleRange(bodyShort.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	// Add up the initial period, except for the last value.
	bodyLongPeriodTotal := 0.0
	bodyShortPeriodTotal := 0.0
	bodyLongTrailingIdx := startIdx - 2 - bodyLong.AvgPeriod
	bodyShortTrailingIdx := startIdx - bodyShort.AvgPeriod
	for i := bodyLongTrailingIdx; i < startIdx-2; i++ {
		bodyLongPeriodTotal += rngLong(i)
	}
	for i := bodyShortTrailingIdx; i < startIdx; i++ {
		bodyShortPeriodTotal += rngShort(i)
	}

	i := startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-2], inOpen[i-2]) == -1 && // 1st black
			candleColor(inClose[i-1], inOpen[i-1]) == -1 && // 2nd black
			candleColor(inClose[i], inOpen[i]) == 1 && // 3rd white
			inClose[i-1] > inClose[i-2] &&
			inOpen[i-1] <= inOpen[i-2] && // harami
			inLow[i-1] < inLow[i-2] && // lower low
			inOpen[i] > inLow[i-1] && // open not lower
			realBody(inClose[i-2], inOpen[i-2]) > candleAverage(bodyLong, bodyLongPeriodTotal, inOpen[i-2], inHigh[i-2], inLow[i-2], inClose[i-2]) && // 1st long
			realBody(inClose[i], inOpen[i]) < candleAverage(bodyShort, bodyShortPeriodTotal, inOpen[i], inHigh[i], inLow[i], inClose[i]) { // 3rd short
			outInteger[outIdx] = 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyLongPeriodTotal += rngLong(i-2) - rngLong(bodyLongTrailingIdx)
		bodyShortPeriodTotal += rngShort(i) - rngShort(bodyShortTrailingIdx)
		i++
		bodyLongTrailingIdx++
		bodyShortTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
