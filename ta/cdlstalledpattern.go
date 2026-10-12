package ta

import "math"

// CDLSTALLEDPATTERN — Stalled Pattern (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlstalledpattern and src/ta_func/ta_CDLSTALLEDPATTERN.c.

// CDLSTALLEDPATTERNLookback returns the number of leading bars
// CDLSTALLEDPATTERN leaves empty. It depends on the current candle settings
// (BodyLong, BodyShort, ShadowVeryShort, Near). It has no parameters, so it
// never returns -1.
func CDLSTALLEDPATTERNLookback() int {
	return cdlstalledpatternLookback()
}

func cdlstalledpatternLookback() int {
	cs := candleSettingsSnapshot()
	return cdlstalledpatternLookbackFrom(&cs)
}

func cdlstalledpatternLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cMaxInt(cs[BodyLong].AvgPeriod, cs[BodyShort].AvgPeriod), cMaxInt(cs[ShadowVeryShort].AvgPeriod, cs[Near].AvgPeriod)) + 2
}

// CDLSTALLEDPATTERN recognises the Stalled Pattern: three white candles with
// consecutively higher closes; the first two long, the second with a very
// short upper shadow and opening within or near the first real body; the
// third small and riding on the shoulder of the second. The output is
// index-aligned with the inputs: -100 where the pattern is found (always
// bearish), 0 otherwise and inside the lookback.
func CDLSTALLEDPATTERN(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLSTALLEDPATTERN", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlstalledpatternRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlstalledpatternRange mirrors TA_CDLSTALLEDPATTERN: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlstalledpatternRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLSTALLEDPATTERN", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLSTALLEDPATTERN", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	bodyShort := cs[BodyShort]
	shadowVeryShort := cs[ShadowVeryShort]
	near := cs[Near]

	lookbackTotal := cdlstalledpatternLookbackFrom(&cs)
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

	var bodyLongPeriodTotal [3]float64
	bodyLongTrailingIdx := startIdx - bodyLong.AvgPeriod
	bodyShortPeriodTotal := 0.0
	bodyShortTrailingIdx := startIdx - bodyShort.AvgPeriod
	shadowVeryShortPeriodTotal := 0.0
	shadowVeryShortTrailingIdx := startIdx - shadowVeryShort.AvgPeriod
	var nearPeriodTotal [3]float64
	nearTrailingIdx := startIdx - near.AvgPeriod

	for i := bodyLongTrailingIdx; i < startIdx; i++ {
		bodyLongPeriodTotal[2] = bodyLongPeriodTotal[2] + rng(bodyLong, i-2)
		bodyLongPeriodTotal[1] = bodyLongPeriodTotal[1] + rng(bodyLong, i-1)
	}
	for i := bodyShortTrailingIdx; i < startIdx; i++ {
		bodyShortPeriodTotal += rng(bodyShort, i)
	}
	for i := shadowVeryShortTrailingIdx; i < startIdx; i++ {
		shadowVeryShortPeriodTotal += rng(shadowVeryShort, i-1)
	}
	for i := nearTrailingIdx; i < startIdx; i++ {
		nearPeriodTotal[2] = nearPeriodTotal[2] + rng(near, i-2)
		nearPeriodTotal[1] = nearPeriodTotal[1] + rng(near, i-1)
	}

	i := startIdx
	outIdx := 0
	for {
		o2, c2 := inOpen[i-2], inClose[i-2]
		o1, h1, c1 := inOpen[i-1], inHigh[i-1], inClose[i-1]
		o0, c0 := inOpen[i], inClose[i]
		if candleColor(c2, o2) == 1 && // 1st white
			candleColor(c1, o1) == 1 && // 2nd white
			candleColor(c0, o0) == 1 && // 3rd white
			c0 > c1 && c1 > c2 && // consecutive higher closes
			math.Abs(c2-o2) > avg(bodyLong, bodyLongPeriodTotal[2], i-2) && // 1st: long real body
			math.Abs(c1-o1) > avg(bodyLong, bodyLongPeriodTotal[1], i-1) && // 2nd: long real body
			upperShadow(h1, c1, o1) < avg(shadowVeryShort, shadowVeryShortPeriodTotal, i-1) && // very short upper shadow
			o1 > o2 && // opens within/near 1st real body
			o1 <= c2+avg(near, nearPeriodTotal[2], i-2) &&
			math.Abs(c0-o0) < avg(bodyShort, bodyShortPeriodTotal, i) && // 3rd: small real body
			o0 >= c1-math.Abs(c0-o0)-avg(near, nearPeriodTotal[1], i-1) { // rides on the shoulder of 2nd real body
			outInteger[outIdx] = -100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		for totIdx := 2; totIdx >= 1; totIdx-- {
			bodyLongPeriodTotal[totIdx] = bodyLongPeriodTotal[totIdx] + (rng(bodyLong, i-totIdx) - rng(bodyLong, bodyLongTrailingIdx-totIdx))
			nearPeriodTotal[totIdx] = nearPeriodTotal[totIdx] + (rng(near, i-totIdx) - rng(near, nearTrailingIdx-totIdx))
		}
		bodyShortPeriodTotal += rng(bodyShort, i) - rng(bodyShort, bodyShortTrailingIdx)
		shadowVeryShortPeriodTotal += rng(shadowVeryShort, i-1) - rng(shadowVeryShort, shadowVeryShortTrailingIdx-1)
		i++
		bodyLongTrailingIdx++
		bodyShortTrailingIdx++
		shadowVeryShortTrailingIdx++
		nearTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
