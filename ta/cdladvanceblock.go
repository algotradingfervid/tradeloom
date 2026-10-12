package ta

import "math"

// CDLADVANCEBLOCK — Advance Block (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdladvanceblock and src/ta_func/ta_CDLADVANCEBLOCK.c.

// CDLADVANCEBLOCKLookback returns the number of leading bars CDLADVANCEBLOCK
// leaves empty. It depends on the current candle settings (ShadowLong,
// ShadowShort, Far, Near, BodyLong). It has no parameters, so it never
// returns -1.
func CDLADVANCEBLOCKLookback() int {
	return cdladvanceblockLookback()
}

func cdladvanceblockLookback() int {
	cs := candleSettingsSnapshot()
	return cdladvanceblockLookbackFrom(&cs)
}

func cdladvanceblockLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cMaxInt(cMaxInt(cs[ShadowLong].AvgPeriod, cs[ShadowShort].AvgPeriod), cMaxInt(cs[Far].AvgPeriod, cs[Near].AvgPeriod)), cs[BodyLong].AvgPeriod) + 2
}

// CDLADVANCEBLOCK recognises the Advance Block: three white candles with
// consecutively higher closes, each opening within or near the prior real
// body, the first long with a short upper shadow, and the second and/or third
// showing weakness (shrinking bodies and/or long upper shadows). The output is
// index-aligned with the inputs: -100 where the pattern is found (always
// bearish), 0 otherwise and inside the lookback.
func CDLADVANCEBLOCK(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLADVANCEBLOCK", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdladvanceblockRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdladvanceblockRange mirrors TA_CDLADVANCEBLOCK: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdladvanceblockRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLADVANCEBLOCK", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLADVANCEBLOCK", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	far := cs[Far]
	near := cs[Near]
	shadowLong := cs[ShadowLong]
	shadowShort := cs[ShadowShort]

	lookbackTotal := cdladvanceblockLookbackFrom(&cs)
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

	var shadowShortPeriodTotal [3]float64
	shadowShortTrailingIdx := startIdx - shadowShort.AvgPeriod
	var shadowLongPeriodTotal [2]float64
	shadowLongTrailingIdx := startIdx - shadowLong.AvgPeriod
	var nearPeriodTotal [3]float64
	nearTrailingIdx := startIdx - near.AvgPeriod
	var farPeriodTotal [3]float64
	farTrailingIdx := startIdx - far.AvgPeriod
	bodyLongPeriodTotal := 0.0
	bodyLongTrailingIdx := startIdx - bodyLong.AvgPeriod

	for i := shadowShortTrailingIdx; i < startIdx; i++ {
		shadowShortPeriodTotal[2] = shadowShortPeriodTotal[2] + rng(shadowShort, i-2)
		shadowShortPeriodTotal[1] = shadowShortPeriodTotal[1] + rng(shadowShort, i-1)
		shadowShortPeriodTotal[0] = shadowShortPeriodTotal[0] + rng(shadowShort, i)
	}
	for i := shadowLongTrailingIdx; i < startIdx; i++ {
		shadowLongPeriodTotal[1] = shadowLongPeriodTotal[1] + rng(shadowLong, i-1)
		shadowLongPeriodTotal[0] = shadowLongPeriodTotal[0] + rng(shadowLong, i)
	}
	for i := nearTrailingIdx; i < startIdx; i++ {
		nearPeriodTotal[2] = nearPeriodTotal[2] + rng(near, i-2)
		nearPeriodTotal[1] = nearPeriodTotal[1] + rng(near, i-1)
	}
	for i := farTrailingIdx; i < startIdx; i++ {
		farPeriodTotal[2] = farPeriodTotal[2] + rng(far, i-2)
		farPeriodTotal[1] = farPeriodTotal[1] + rng(far, i-1)
	}
	for i := bodyLongTrailingIdx; i < startIdx; i++ {
		bodyLongPeriodTotal += rng(bodyLong, i-2)
	}

	i := startIdx
	outIdx := 0
	for {
		o2, h2, c2 := inOpen[i-2], inHigh[i-2], inClose[i-2]
		o1, h1, c1 := inOpen[i-1], inHigh[i-1], inClose[i-1]
		o0, h0, c0 := inOpen[i], inHigh[i], inClose[i]
		if candleColor(c2, o2) == 1 && // 1st white
			candleColor(c1, o1) == 1 && // 2nd white
			candleColor(c0, o0) == 1 && // 3rd white
			c0 > c1 && c1 > c2 && // consecutive higher closes
			o1 > o2 && // 2nd opens within/near 1st real body
			o1 <= c2+avg(near, nearPeriodTotal[2], i-2) &&
			o0 > o1 && // 3rd opens within/near 2nd real body
			o0 <= c1+avg(near, nearPeriodTotal[1], i-1) &&
			math.Abs(c2-o2) > avg(bodyLong, bodyLongPeriodTotal, i-2) && // 1st: long real body
			upperShadow(h2, c2, o2) < avg(shadowShort, shadowShortPeriodTotal[2], i-2) && // 1st: short upper shadow
			( // 2 far smaller than 1 && 3 not longer than 2: advance blocked with the 2nd
			(math.Abs(c1-o1) < math.Abs(c2-o2)-avg(far, farPeriodTotal[2], i-2) &&
				math.Abs(c0-o0) < math.Abs(c1-o1)+avg(near, nearPeriodTotal[1], i-1)) ||
				// 3 far smaller than 2: advance blocked with the 3rd
				math.Abs(c0-o0) < math.Abs(c1-o1)-avg(far, farPeriodTotal[1], i-1) ||
				// progressively smaller bodies and (3 or 2 not short upper shadow)
				(math.Abs(c0-o0) < math.Abs(c1-o1) &&
					math.Abs(c1-o1) < math.Abs(c2-o2) &&
					(upperShadow(h0, c0, o0) > avg(shadowShort, shadowShortPeriodTotal[0], i) ||
						upperShadow(h1, c1, o1) > avg(shadowShort, shadowShortPeriodTotal[1], i-1))) ||
				// 3 smaller than 2 && 3 long upper shadow
				(math.Abs(c0-o0) < math.Abs(c1-o1) &&
					upperShadow(h0, c0, o0) > avg(shadowLong, shadowLongPeriodTotal[0], i))) {
			outInteger[outIdx] = -100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		for totIdx := 2; totIdx >= 0; totIdx-- {
			shadowShortPeriodTotal[totIdx] = shadowShortPeriodTotal[totIdx] + (rng(shadowShort, i-totIdx) - rng(shadowShort, shadowShortTrailingIdx-totIdx))
		}
		for totIdx := 1; totIdx >= 0; totIdx-- {
			shadowLongPeriodTotal[totIdx] = shadowLongPeriodTotal[totIdx] + (rng(shadowLong, i-totIdx) - rng(shadowLong, shadowLongTrailingIdx-totIdx))
		}
		for totIdx := 2; totIdx >= 1; totIdx-- {
			farPeriodTotal[totIdx] = farPeriodTotal[totIdx] + (rng(far, i-totIdx) - rng(far, farTrailingIdx-totIdx))
			nearPeriodTotal[totIdx] = nearPeriodTotal[totIdx] + (rng(near, i-totIdx) - rng(near, nearTrailingIdx-totIdx))
		}
		bodyLongPeriodTotal += rng(bodyLong, i-2) - rng(bodyLong, bodyLongTrailingIdx-2)
		i++
		shadowShortTrailingIdx++
		shadowLongTrailingIdx++
		nearTrailingIdx++
		farTrailingIdx++
		bodyLongTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
