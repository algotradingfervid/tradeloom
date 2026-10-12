package ta

import "math"

// CDLKICKINGBYLENGTH — Kicking - bull/bear determined by the longer marubozu (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlkickingbylength and src/ta_func/ta_CDLKICKINGBYLENGTH.c.

// CDLKICKINGBYLENGTHLookback returns the number of leading bars
// CDLKICKINGBYLENGTH leaves empty. It depends on the current candle settings
// (BodyLong, ShadowVeryShort). It has no parameters, so it never returns -1.
func CDLKICKINGBYLENGTHLookback() int {
	return cdlkickingbylengthLookback()
}

func cdlkickingbylengthLookback() int {
	cs := candleSettingsSnapshot()
	return cdlkickingbylengthLookbackFrom(&cs)
}

func cdlkickingbylengthLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[ShadowVeryShort].AvgPeriod, cs[BodyLong].AvgPeriod) + 1
}

// CDLKICKINGBYLENGTH recognises the Kicking pattern: a marubozu followed by
// an opposite-color marubozu with a candle gap between them (upside gap after
// a black first candle, downside gap after a white one). "Marubozu" means a
// long body (BodyLong) with very short shadows (ShadowVeryShort). The output
// is index-aligned with the inputs: +100 or -100 by the color of the longer
// of the two marubozu (the first when the bodies are equal), 0 otherwise and
// inside the lookback.
func CDLKICKINGBYLENGTH(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLKICKINGBYLENGTH", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlkickingbylengthRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlkickingbylengthRange mirrors TA_CDLKICKINGBYLENGTH: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlkickingbylengthRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLKICKINGBYLENGTH", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLKICKINGBYLENGTH", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	shadowVeryShort := cs[ShadowVeryShort]

	lookbackTotal := cdlkickingbylengthLookbackFrom(&cs)
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

	var shadowVeryShortPeriodTotal [2]float64
	shadowVeryShortTrailingIdx := startIdx - shadowVeryShort.AvgPeriod
	var bodyLongPeriodTotal [2]float64
	bodyLongTrailingIdx := startIdx - bodyLong.AvgPeriod
	for i := shadowVeryShortTrailingIdx; i < startIdx; i++ {
		shadowVeryShortPeriodTotal[1] = shadowVeryShortPeriodTotal[1] + rng(shadowVeryShort, i-1)
		shadowVeryShortPeriodTotal[0] = shadowVeryShortPeriodTotal[0] + rng(shadowVeryShort, i)
	}
	for i := bodyLongTrailingIdx; i < startIdx; i++ {
		bodyLongPeriodTotal[1] = bodyLongPeriodTotal[1] + rng(bodyLong, i-1)
		bodyLongPeriodTotal[0] = bodyLongPeriodTotal[0] + rng(bodyLong, i)
	}

	i := startIdx
	outIdx := 0
	for {
		color1 := candleColor(inClose[i-1], inOpen[i-1])
		if color1 == -candleColor(inClose[i], inOpen[i]) && // opposite candles
			math.Abs(inClose[i-1]-inOpen[i-1]) > avg(bodyLong, bodyLongPeriodTotal[1], i-1) && // 1st marubozu
			upperShadow(inHigh[i-1], inClose[i-1], inOpen[i-1]) < avg(shadowVeryShort, shadowVeryShortPeriodTotal[1], i-1) &&
			lowerShadow(inLow[i-1], inClose[i-1], inOpen[i-1]) < avg(shadowVeryShort, shadowVeryShortPeriodTotal[1], i-1) &&
			math.Abs(inClose[i]-inOpen[i]) > avg(bodyLong, bodyLongPeriodTotal[0], i) && // 2nd marubozu
			upperShadow(inHigh[i], inClose[i], inOpen[i]) < avg(shadowVeryShort, shadowVeryShortPeriodTotal[0], i) &&
			lowerShadow(inLow[i], inClose[i], inOpen[i]) < avg(shadowVeryShort, shadowVeryShortPeriodTotal[0], i) &&
			((color1 == -1 && candleGapUp(inLow[i], inHigh[i-1])) || // gap
				(color1 == 1 && candleGapDown(inHigh[i], inLow[i-1]))) {
			// The longer marubozu decides; on a tie the first one does.
			k := i - 1
			if math.Abs(inClose[i]-inOpen[i]) > math.Abs(inClose[i-1]-inOpen[i-1]) {
				k = i
			}
			outInteger[outIdx] = candleColor(inClose[k], inOpen[k]) * 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		for totIdx := 1; totIdx >= 0; totIdx-- {
			bodyLongPeriodTotal[totIdx] = bodyLongPeriodTotal[totIdx] + (rng(bodyLong, i-totIdx) - rng(bodyLong, bodyLongTrailingIdx-totIdx))
			shadowVeryShortPeriodTotal[totIdx] = shadowVeryShortPeriodTotal[totIdx] + (rng(shadowVeryShort, i-totIdx) - rng(shadowVeryShort, shadowVeryShortTrailingIdx-totIdx))
		}
		i++
		shadowVeryShortTrailingIdx++
		bodyLongTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
