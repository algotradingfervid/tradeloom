package ta

// CDLMARUBOZU — Marubozu (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlmarubozu and
// src/ta_func/ta_CDLMARUBOZU.c.

// CDLMARUBOZULookback returns the number of leading bars CDLMARUBOZU leaves
// empty. It depends on the current candle settings (BodyLong,
// ShadowVeryShort). It has no parameters, so it never returns -1.
func CDLMARUBOZULookback() int {
	return cdlmarubozuLookback()
}

func cdlmarubozuLookback() int {
	cs := candleSettingsSnapshot()
	return cdlmarubozuLookbackFrom(&cs)
}

func cdlmarubozuLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[BodyLong].AvgPeriod, cs[ShadowVeryShort].AvgPeriod)
}

// CDLMARUBOZU recognises the Marubozu: a long real body with no or very short
// upper and lower shadows. The output is index-aligned with the inputs: 100
// for a white one, -100 for a black one, 0 otherwise and inside the lookback.
func CDLMARUBOZU(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLMARUBOZU", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlmarubozuRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlmarubozuRange mirrors TA_CDLMARUBOZU: it evaluates [startIdx, endIdx]
// and writes outNBElement values to outInteger[0:], the first belonging to
// bar outBegIdx.
func cdlmarubozuRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLMARUBOZU", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLMARUBOZU", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	shadowVeryShort := cs[ShadowVeryShort]

	lookbackTotal := cdlmarubozuLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(s CandleSetting, i int) float64 {
		return candleRange(s.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	bodyLongPeriodTotal := 0.0
	bodyLongTrailingIdx := startIdx - bodyLong.AvgPeriod
	shadowVeryShortPeriodTotal := 0.0
	shadowVeryShortTrailingIdx := startIdx - shadowVeryShort.AvgPeriod

	for i := bodyLongTrailingIdx; i < startIdx; i++ {
		bodyLongPeriodTotal += rng(bodyLong, i)
	}
	for i := shadowVeryShortTrailingIdx; i < startIdx; i++ {
		shadowVeryShortPeriodTotal += rng(shadowVeryShort, i)
	}

	i := startIdx
	outIdx := 0
	for {
		o, h, l, c := inOpen[i], inHigh[i], inLow[i], inClose[i]
		if realBody(c, o) > candleAverage(bodyLong, bodyLongPeriodTotal, o, h, l, c) && // long body
			upperShadow(h, c, o) < candleAverage(shadowVeryShort, shadowVeryShortPeriodTotal, o, h, l, c) && // very short upper shadow
			lowerShadow(l, c, o) < candleAverage(shadowVeryShort, shadowVeryShortPeriodTotal, o, h, l, c) { // very short lower shadow
			outInteger[outIdx] = candleColor(c, o) * 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyLongPeriodTotal += rng(bodyLong, i) - rng(bodyLong, bodyLongTrailingIdx)
		shadowVeryShortPeriodTotal += rng(shadowVeryShort, i) - rng(shadowVeryShort, shadowVeryShortTrailingIdx)
		i++
		bodyLongTrailingIdx++
		shadowVeryShortTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
