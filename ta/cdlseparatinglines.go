package ta

import "math"

// CDLSEPARATINGLINES — Separating Lines (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlseparatinglines and src/ta_func/ta_CDLSEPARATINGLINES.c.

// CDLSEPARATINGLINESLookback returns the number of leading bars
// CDLSEPARATINGLINES leaves empty. It depends on the current candle settings
// (ShadowVeryShort, BodyLong, Equal). It has no parameters, so it never
// returns -1.
func CDLSEPARATINGLINESLookback() int {
	return cdlseparatinglinesLookback()
}

func cdlseparatinglinesLookback() int {
	cs := candleSettingsSnapshot()
	return cdlseparatinglinesLookbackFrom(&cs)
}

func cdlseparatinglinesLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cMaxInt(cs[ShadowVeryShort].AvgPeriod, cs[BodyLong].AvgPeriod), cs[Equal].AvgPeriod) + 1
}

// CDLSEPARATINGLINES recognises Separating Lines: two candles of opposite
// color with (nearly) the same open, "same" meaning within the Equal candle
// setting of the first open, where the second candle is a belt hold: a long
// real body (BodyLong) with a very short shadow (ShadowVeryShort) on the
// opening side. The output is index-aligned with the inputs: +100 when the
// second candle is white, -100 when it is black, 0 otherwise and inside the
// lookback. The prior trend is not checked.
func CDLSEPARATINGLINES(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLSEPARATINGLINES", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlseparatinglinesRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlseparatinglinesRange mirrors TA_CDLSEPARATINGLINES: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlseparatinglinesRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLSEPARATINGLINES", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLSEPARATINGLINES", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	shadowVeryShort := cs[ShadowVeryShort]
	bodyLong := cs[BodyLong]
	equal := cs[Equal]

	lookbackTotal := cdlseparatinglinesLookbackFrom(&cs)
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

	shadowVeryShortPeriodTotal := 0.0
	shadowVeryShortTrailingIdx := startIdx - shadowVeryShort.AvgPeriod
	bodyLongPeriodTotal := 0.0
	bodyLongTrailingIdx := startIdx - bodyLong.AvgPeriod
	equalPeriodTotal := 0.0
	equalTrailingIdx := startIdx - equal.AvgPeriod
	for i := shadowVeryShortTrailingIdx; i < startIdx; i++ {
		shadowVeryShortPeriodTotal += rng(shadowVeryShort, i)
	}
	for i := bodyLongTrailingIdx; i < startIdx; i++ {
		bodyLongPeriodTotal += rng(bodyLong, i)
	}
	for i := equalTrailingIdx; i < startIdx; i++ {
		equalPeriodTotal += rng(equal, i-1)
	}

	i := startIdx
	outIdx := 0
	for {
		color := candleColor(inClose[i], inOpen[i])
		if candleColor(inClose[i-1], inOpen[i-1]) == -color && // opposite candles
			inOpen[i] <= inOpen[i-1]+avg(equal, equalPeriodTotal, i-1) && // same open
			inOpen[i] >= inOpen[i-1]-avg(equal, equalPeriodTotal, i-1) &&
			math.Abs(inClose[i]-inOpen[i]) > avg(bodyLong, bodyLongPeriodTotal, i) && // belt hold: long body
			((color == 1 && // with no lower shadow if bullish
				lowerShadow(inLow[i], inClose[i], inOpen[i]) < avg(shadowVeryShort, shadowVeryShortPeriodTotal, i)) ||
				(color == -1 && // with no upper shadow if bearish
					upperShadow(inHigh[i], inClose[i], inOpen[i]) < avg(shadowVeryShort, shadowVeryShortPeriodTotal, i))) {
			outInteger[outIdx] = color * 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		shadowVeryShortPeriodTotal += rng(shadowVeryShort, i) - rng(shadowVeryShort, shadowVeryShortTrailingIdx)
		bodyLongPeriodTotal += rng(bodyLong, i) - rng(bodyLong, bodyLongTrailingIdx)
		equalPeriodTotal += rng(equal, i-1) - rng(equal, equalTrailingIdx-1)
		i++
		shadowVeryShortTrailingIdx++
		bodyLongTrailingIdx++
		equalTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
