package ta

// CDLHOMINGPIGEON — Homing Pigeon (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlhomingpigeon and src/ta_func/ta_CDLHOMINGPIGEON.c.

// CDLHOMINGPIGEONLookback returns the number of leading bars CDLHOMINGPIGEON
// leaves empty. It depends on the current BodyLong and BodyShort candle
// settings. It has no parameters, so it never returns -1.
func CDLHOMINGPIGEONLookback() int {
	return cdlhomingpigeonLookback()
}

func cdlhomingpigeonLookback() int {
	cs := candleSettingsSnapshot()
	return cdlhomingpigeonLookbackFrom(&cs)
}

func cdlhomingpigeonLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[BodyShort].AvgPeriod, cs[BodyLong].AvgPeriod) + 1
}

// CDLHOMINGPIGEON recognises the Homing Pigeon: a long black candle followed
// by a short black candle whose real body lies strictly inside the first
// body. The output is index-aligned with the inputs: 100 where the pattern is
// found (it is always bullish; the trend is not checked), 0 otherwise and
// inside the lookback.
func CDLHOMINGPIGEON(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLHOMINGPIGEON", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlhomingpigeonRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlhomingpigeonRange mirrors TA_CDLHOMINGPIGEON: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlhomingpigeonRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLHOMINGPIGEON", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLHOMINGPIGEON", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	bodyShort := cs[BodyShort]

	lookbackTotal := cdlhomingpigeonLookbackFrom(&cs)
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
	bodyLongTrailingIdx := startIdx - bodyLong.AvgPeriod
	bodyShortTrailingIdx := startIdx - bodyShort.AvgPeriod
	for i := bodyLongTrailingIdx; i < startIdx; i++ {
		bodyLongPeriodTotal += rngLong(i - 1)
	}
	for i := bodyShortTrailingIdx; i < startIdx; i++ {
		bodyShortPeriodTotal += rngShort(i)
	}

	i := startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-1], inOpen[i-1]) == -1 && // 1st black
			candleColor(inClose[i], inOpen[i]) == -1 && // 2nd black
			realBody(inClose[i-1], inOpen[i-1]) > candleAverage(bodyLong, bodyLongPeriodTotal, inOpen[i-1], inHigh[i-1], inLow[i-1], inClose[i-1]) && // 1st long
			realBody(inClose[i], inOpen[i]) <= candleAverage(bodyShort, bodyShortPeriodTotal, inOpen[i], inHigh[i], inLow[i], inClose[i]) && // 2nd short
			inOpen[i] < inOpen[i-1] && // 2nd engulfed by 1st
			inClose[i] > inClose[i-1] {
			outInteger[outIdx] = 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyLongPeriodTotal += rngLong(i-1) - rngLong(bodyLongTrailingIdx-1)
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
