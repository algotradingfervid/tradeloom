package ta

// CDLCLOSINGMARUBOZU — Closing Marubozu (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlclosingmarubozu and
// src/ta_func/ta_CDLCLOSINGMARUBOZU.c.

// CDLCLOSINGMARUBOZULookback returns the number of leading bars
// CDLCLOSINGMARUBOZU leaves empty. It depends on the current candle settings
// (BodyLong, ShadowVeryShort). It has no parameters, so it never returns -1.
func CDLCLOSINGMARUBOZULookback() int {
	return cdlclosingmarubozuLookback()
}

func cdlclosingmarubozuLookback() int {
	cs := candleSettingsSnapshot()
	return cdlclosingmarubozuLookbackFrom(&cs)
}

func cdlclosingmarubozuLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[BodyLong].AvgPeriod, cs[ShadowVeryShort].AvgPeriod)
}

// CDLCLOSINGMARUBOZU recognises the Closing Marubozu: a long real body with no
// or a very short shadow at the closing end. The output is index-aligned with
// the inputs: 100 for a white one, -100 for a black one, 0 otherwise and
// inside the lookback.
func CDLCLOSINGMARUBOZU(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLCLOSINGMARUBOZU", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlclosingmarubozuRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlclosingmarubozuRange mirrors TA_CDLCLOSINGMARUBOZU: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlclosingmarubozuRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLCLOSINGMARUBOZU", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLCLOSINGMARUBOZU", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	shadowVeryShort := cs[ShadowVeryShort]

	lookbackTotal := cdlclosingmarubozuLookbackFrom(&cs)
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
		color := candleColor(c, o)
		if realBody(c, o) > candleAverage(bodyLong, bodyLongPeriodTotal, o, h, l, c) && // long body
			((color == 1 && // white body and very short upper shadow
				upperShadow(h, c, o) < candleAverage(shadowVeryShort, shadowVeryShortPeriodTotal, o, h, l, c)) ||
				(color == -1 && // black body and very short lower shadow
					lowerShadow(l, c, o) < candleAverage(shadowVeryShort, shadowVeryShortPeriodTotal, o, h, l, c))) {
			outInteger[outIdx] = color * 100
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
