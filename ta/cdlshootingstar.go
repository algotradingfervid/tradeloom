package ta

// CDLSHOOTINGSTAR — Shooting Star (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlshootingstar and
// src/ta_func/ta_CDLSHOOTINGSTAR.c.

// CDLSHOOTINGSTARLookback returns the number of leading bars CDLSHOOTINGSTAR
// leaves empty. It depends on the current candle settings (BodyShort,
// ShadowLong, ShadowVeryShort). It has no parameters, so it never returns -1.
func CDLSHOOTINGSTARLookback() int {
	return cdlshootingstarLookback()
}

func cdlshootingstarLookback() int {
	cs := candleSettingsSnapshot()
	return cdlshootingstarLookbackFrom(&cs)
}

func cdlshootingstarLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cMaxInt(cs[BodyShort].AvgPeriod, cs[ShadowLong].AvgPeriod), cs[ShadowVeryShort].AvgPeriod) + 1
}

// CDLSHOOTINGSTAR recognises the Shooting Star: a small real body that gaps
// up from the prior candle's body, with a long upper shadow and no or a very
// short lower shadow. The trend is not checked. The output is index-aligned
// with the inputs: -100 where the pattern is found (always bearish), 0
// otherwise and inside the lookback.
func CDLSHOOTINGSTAR(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLSHOOTINGSTAR", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlshootingstarRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlshootingstarRange mirrors TA_CDLSHOOTINGSTAR: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlshootingstarRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLSHOOTINGSTAR", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLSHOOTINGSTAR", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyShort := cs[BodyShort]
	shadowLong := cs[ShadowLong]
	shadowVeryShort := cs[ShadowVeryShort]

	lookbackTotal := cdlshootingstarLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(s CandleSetting, i int) float64 {
		return candleRange(s.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	// Add up the initial period, except for the last value.
	bodyPeriodTotal := 0.0
	bodyTrailingIdx := startIdx - bodyShort.AvgPeriod
	shadowLongPeriodTotal := 0.0
	shadowLongTrailingIdx := startIdx - shadowLong.AvgPeriod
	shadowVeryShortPeriodTotal := 0.0
	shadowVeryShortTrailingIdx := startIdx - shadowVeryShort.AvgPeriod

	for i := bodyTrailingIdx; i < startIdx; i++ {
		bodyPeriodTotal += rng(bodyShort, i)
	}
	for i := shadowLongTrailingIdx; i < startIdx; i++ {
		shadowLongPeriodTotal += rng(shadowLong, i)
	}
	for i := shadowVeryShortTrailingIdx; i < startIdx; i++ {
		shadowVeryShortPeriodTotal += rng(shadowVeryShort, i)
	}

	i := startIdx
	outIdx := 0
	for {
		o, h, l, c := inOpen[i], inHigh[i], inLow[i], inClose[i]
		if realBodyGapUp(o, c, inOpen[i-1], inClose[i-1]) && // gap up
			realBody(c, o) < candleAverage(bodyShort, bodyPeriodTotal, o, h, l, c) && // small rb
			upperShadow(h, c, o) > candleAverage(shadowLong, shadowLongPeriodTotal, o, h, l, c) && // long upper shadow
			lowerShadow(l, c, o) < candleAverage(shadowVeryShort, shadowVeryShortPeriodTotal, o, h, l, c) { // very short lower shadow
			outInteger[outIdx] = -100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyPeriodTotal += rng(bodyShort, i) - rng(bodyShort, bodyTrailingIdx)
		shadowLongPeriodTotal += rng(shadowLong, i) - rng(shadowLong, shadowLongTrailingIdx)
		shadowVeryShortPeriodTotal += rng(shadowVeryShort, i) - rng(shadowVeryShort, shadowVeryShortTrailingIdx)
		i++
		bodyTrailingIdx++
		shadowLongTrailingIdx++
		shadowVeryShortTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
