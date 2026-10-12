package ta

// CDL3WHITESOLDIERS — Three Advancing White Soldiers (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdl3whitesoldiers and
// src/ta_func/ta_CDL3WHITESOLDIERS.c.

// CDL3WHITESOLDIERSLookback returns the number of leading bars
// CDL3WHITESOLDIERS leaves empty. It depends on the current candle settings
// (ShadowVeryShort, BodyShort, Far, Near). It has no parameters, so it never
// returns -1.
func CDL3WHITESOLDIERSLookback() int {
	return cdl3whitesoldiersLookback()
}

func cdl3whitesoldiersLookback() int {
	cs := candleSettingsSnapshot()
	return cdl3whitesoldiersLookbackFrom(&cs)
}

func cdl3whitesoldiersLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cMaxInt(cs[ShadowVeryShort].AvgPeriod, cs[BodyShort].AvgPeriod), cMaxInt(cs[Far].AvgPeriod, cs[Near].AvgPeriod)) + 2
}

// CDL3WHITESOLDIERS recognises Three Advancing White Soldiers: three white
// candles with consecutively higher closes, each with a very short upper
// shadow, each opening within or near the prior real body, the bodies not
// getting far shorter, and the last body not short.
// The output is index-aligned with the inputs: 100 where the pattern is
// found, 0 otherwise and inside the lookback.
func CDL3WHITESOLDIERS(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDL3WHITESOLDIERS", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdl3whitesoldiersRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdl3whitesoldiersRange mirrors TA_CDL3WHITESOLDIERS: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdl3whitesoldiersRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDL3WHITESOLDIERS", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDL3WHITESOLDIERS", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	shadowVeryShort := cs[ShadowVeryShort]
	near := cs[Near]
	far := cs[Far]
	bodyShort := cs[BodyShort]

	lookbackTotal := cdl3whitesoldiersLookbackFrom(&cs)
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

	var shadowVeryShortPeriodTotal, nearPeriodTotal, farPeriodTotal [3]float64
	shadowVeryShortTrailingIdx := startIdx - shadowVeryShort.AvgPeriod
	nearTrailingIdx := startIdx - near.AvgPeriod
	farTrailingIdx := startIdx - far.AvgPeriod
	bodyShortPeriodTotal := 0.0
	bodyShortTrailingIdx := startIdx - bodyShort.AvgPeriod

	for i := shadowVeryShortTrailingIdx; i < startIdx; i++ {
		shadowVeryShortPeriodTotal[2] += rng(shadowVeryShort, i-2)
		shadowVeryShortPeriodTotal[1] += rng(shadowVeryShort, i-1)
		shadowVeryShortPeriodTotal[0] += rng(shadowVeryShort, i)
	}
	for i := nearTrailingIdx; i < startIdx; i++ {
		nearPeriodTotal[2] += rng(near, i-2)
		nearPeriodTotal[1] += rng(near, i-1)
	}
	for i := farTrailingIdx; i < startIdx; i++ {
		farPeriodTotal[2] += rng(far, i-2)
		farPeriodTotal[1] += rng(far, i-1)
	}
	for i := bodyShortTrailingIdx; i < startIdx; i++ {
		bodyShortPeriodTotal += rng(bodyShort, i)
	}

	i := startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-2], inOpen[i-2]) == 1 && // 1st white
			upperShadow(inHigh[i-2], inClose[i-2], inOpen[i-2]) < avg(shadowVeryShort, shadowVeryShortPeriodTotal[2], i-2) && // very short upper shadow
			candleColor(inClose[i-1], inOpen[i-1]) == 1 && // 2nd white
			upperShadow(inHigh[i-1], inClose[i-1], inOpen[i-1]) < avg(shadowVeryShort, shadowVeryShortPeriodTotal[1], i-1) && // very short upper shadow
			candleColor(inClose[i], inOpen[i]) == 1 && // 3rd white
			upperShadow(inHigh[i], inClose[i], inOpen[i]) < avg(shadowVeryShort, shadowVeryShortPeriodTotal[0], i) && // very short upper shadow
			inClose[i] > inClose[i-1] &&
			inClose[i-1] > inClose[i-2] && // consecutive higher closes
			inOpen[i-1] > inOpen[i-2] && // 2nd opens within/near 1st real body
			inOpen[i-1] <= inClose[i-2]+avg(near, nearPeriodTotal[2], i-2) &&
			inOpen[i] > inOpen[i-1] && // 3rd opens within/near 2nd real body
			inOpen[i] <= inClose[i-1]+avg(near, nearPeriodTotal[1], i-1) &&
			realBody(inClose[i-1], inOpen[i-1]) > realBody(inClose[i-2], inOpen[i-2])-avg(far, farPeriodTotal[2], i-2) && // 2nd not far shorter than 1st
			realBody(inClose[i], inOpen[i]) > realBody(inClose[i-1], inOpen[i-1])-avg(far, farPeriodTotal[1], i-1) && // 3rd not far shorter than 2nd
			realBody(inClose[i], inOpen[i]) > avg(bodyShort, bodyShortPeriodTotal, i) { // not short real body
			outInteger[outIdx] = 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		for totIdx := 2; totIdx >= 0; totIdx-- {
			shadowVeryShortPeriodTotal[totIdx] += rng(shadowVeryShort, i-totIdx) - rng(shadowVeryShort, shadowVeryShortTrailingIdx-totIdx)
		}
		for totIdx := 2; totIdx >= 1; totIdx-- {
			farPeriodTotal[totIdx] += rng(far, i-totIdx) - rng(far, farTrailingIdx-totIdx)
			nearPeriodTotal[totIdx] += rng(near, i-totIdx) - rng(near, nearTrailingIdx-totIdx)
		}
		bodyShortPeriodTotal += rng(bodyShort, i) - rng(bodyShort, bodyShortTrailingIdx)
		i++
		shadowVeryShortTrailingIdx++
		nearTrailingIdx++
		farTrailingIdx++
		bodyShortTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
