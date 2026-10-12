package ta

// CDL3STARSINSOUTH — Three Stars In The South (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdl3starsinsouth and
// src/ta_func/ta_CDL3STARSINSOUTH.c.

// CDL3STARSINSOUTHLookback returns the number of leading bars CDL3STARSINSOUTH
// leaves empty. It depends on the current candle settings (BodyLong,
// BodyShort, ShadowLong, ShadowVeryShort). It has no parameters, so it never
// returns -1.
func CDL3STARSINSOUTHLookback() int {
	return cdl3starsinsouthLookback()
}

func cdl3starsinsouthLookback() int {
	cs := candleSettingsSnapshot()
	return cdl3starsinsouthLookbackFrom(&cs)
}

func cdl3starsinsouthLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cMaxInt(cs[ShadowVeryShort].AvgPeriod, cs[ShadowLong].AvgPeriod), cMaxInt(cs[BodyLong].AvgPeriod, cs[BodyShort].AvgPeriod)) + 2
}

// CDL3STARSINSOUTH recognises Three Stars In The South: three black candles,
// a long one with a long lower shadow, a smaller one opening higher within
// its range, trading lower but not below its low and closing off its low,
// and a small black marubozu engulfed by the second candle's range. The
// output is index-aligned with the inputs: 100 where the pattern completes,
// 0 otherwise and inside the lookback.
func CDL3STARSINSOUTH(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDL3STARSINSOUTH", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdl3starsinsouthRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdl3starsinsouthRange mirrors TA_CDL3STARSINSOUTH: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdl3starsinsouthRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDL3STARSINSOUTH", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDL3STARSINSOUTH", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	bodyShort := cs[BodyShort]
	shadowLong := cs[ShadowLong]
	shadowVeryShort := cs[ShadowVeryShort]

	lookbackTotal := cdl3starsinsouthLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(s CandleSetting, i int) float64 {
		return candleRange(s.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}
	avg := func(s CandleSetting, sum float64, i int) float64 {
		return candleAverage(s, sum, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	bodyLongPeriodTotal := 0.0
	bodyLongTrailingIdx := startIdx - bodyLong.AvgPeriod
	shadowLongPeriodTotal := 0.0
	shadowLongTrailingIdx := startIdx - shadowLong.AvgPeriod
	var shadowVeryShortPeriodTotal [2]float64
	shadowVeryShortTrailingIdx := startIdx - shadowVeryShort.AvgPeriod
	bodyShortPeriodTotal := 0.0
	bodyShortTrailingIdx := startIdx - bodyShort.AvgPeriod

	for i := bodyLongTrailingIdx; i < startIdx; i++ {
		bodyLongPeriodTotal += rng(bodyLong, i-2)
	}
	for i := shadowLongTrailingIdx; i < startIdx; i++ {
		shadowLongPeriodTotal += rng(shadowLong, i-2)
	}
	for i := shadowVeryShortTrailingIdx; i < startIdx; i++ {
		shadowVeryShortPeriodTotal[1] = shadowVeryShortPeriodTotal[1] + rng(shadowVeryShort, i-1)
		shadowVeryShortPeriodTotal[0] = shadowVeryShortPeriodTotal[0] + rng(shadowVeryShort, i)
	}
	for i := bodyShortTrailingIdx; i < startIdx; i++ {
		bodyShortPeriodTotal += rng(bodyShort, i)
	}

	i := startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-2], inOpen[i-2]) == -1 && // 1st black
			candleColor(inClose[i-1], inOpen[i-1]) == -1 && // 2nd black
			candleColor(inClose[i], inOpen[i]) == -1 && // 3rd black
			realBody(inClose[i-2], inOpen[i-2]) > avg(bodyLong, bodyLongPeriodTotal, i-2) && // 1st: long
			lowerShadow(inLow[i-2], inClose[i-2], inOpen[i-2]) > avg(shadowLong, shadowLongPeriodTotal, i-2) && // with long lower shadow
			realBody(inClose[i-1], inOpen[i-1]) < realBody(inClose[i-2], inOpen[i-2]) && // 2nd: smaller candle
			inOpen[i-1] > inClose[i-2] &&
			inOpen[i-1] <= inHigh[i-2] && // that opens higher but within 1st range
			inLow[i-1] < inClose[i-2] && // and trades lower than 1st close
			inLow[i-1] >= inLow[i-2] && // but not lower than 1st low
			lowerShadow(inLow[i-1], inClose[i-1], inOpen[i-1]) > avg(shadowVeryShort, shadowVeryShortPeriodTotal[1], i-1) && // and has a lower shadow
			realBody(inClose[i], inOpen[i]) < avg(bodyShort, bodyShortPeriodTotal, i) && // 3rd: small marubozu
			lowerShadow(inLow[i], inClose[i], inOpen[i]) < avg(shadowVeryShort, shadowVeryShortPeriodTotal[0], i) &&
			upperShadow(inHigh[i], inClose[i], inOpen[i]) < avg(shadowVeryShort, shadowVeryShortPeriodTotal[0], i) &&
			inLow[i] > inLow[i-1] &&
			inHigh[i] < inHigh[i-1] { // engulfed by prior candle's range
			outInteger[outIdx] = 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyLongPeriodTotal += rng(bodyLong, i-2) - rng(bodyLong, bodyLongTrailingIdx-2)
		shadowLongPeriodTotal += rng(shadowLong, i-2) - rng(shadowLong, shadowLongTrailingIdx-2)
		for totIdx := 1; totIdx >= 0; totIdx-- {
			shadowVeryShortPeriodTotal[totIdx] = shadowVeryShortPeriodTotal[totIdx] + (rng(shadowVeryShort, i-totIdx) - rng(shadowVeryShort, shadowVeryShortTrailingIdx-totIdx))
		}
		bodyShortPeriodTotal += rng(bodyShort, i) - rng(bodyShort, bodyShortTrailingIdx)
		i++
		bodyLongTrailingIdx++
		shadowLongTrailingIdx++
		shadowVeryShortTrailingIdx++
		bodyShortTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
