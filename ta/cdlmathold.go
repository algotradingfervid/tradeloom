package ta

import "math"

// CDLMATHOLD — Mat Hold (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlmathold and
// src/ta_func/ta_CDLMATHOLD.c.

// CDLMATHOLDLookback returns the number of leading bars CDLMATHOLD leaves
// empty, or -1 when optInPenetration is invalid. It depends on the current
// candle settings (BodyLong, BodyShort).
// optInPenetration: [0, 3e37], default 0.5.
func CDLMATHOLDLookback(optInPenetration float64) int {
	if checkReal("CDLMATHOLD", "optInPenetration", optInPenetration, 0, taRealMax) != nil {
		return -1
	}
	return cdlmatholdLookback(optInPenetration)
}

func cdlmatholdLookback(optInPenetration float64) int {
	cs := candleSettingsSnapshot()
	return cdlmatholdLookbackFrom(&cs)
}

func cdlmatholdLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[BodyShort].AvgPeriod, cs[BodyLong].AvgPeriod) + 4
}

// CDLMATHOLD recognises the Mat Hold: a long white candle, a small black
// candle whose body gaps up, two more small falling candles that hold within
// the first body (penetrating it by less than optInPenetration of its
// length), then a white candle opening above the prior close and closing
// above the reaction days' highs. The output is index-aligned with the
// inputs: +100 where the pattern completes, 0 otherwise and inside the
// lookback. optInPenetration: [0, 3e37], default 0.5.
func CDLMATHOLD(inOpen, inHigh, inLow, inClose []float64, optInPenetration float64) ([]int, error) {
	if err := checkReal("CDLMATHOLD", "optInPenetration", optInPenetration, 0, taRealMax); err != nil {
		return nil, err
	}
	n, err := checkInputs("CDLMATHOLD", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlmatholdRange(0, n-1, inOpen, inHigh, inLow, inClose, optInPenetration, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlmatholdRange mirrors TA_CDLMATHOLD: it evaluates [startIdx, endIdx] and
// writes outNBElement values to outInteger[0:], the first belonging to bar
// outBegIdx.
func cdlmatholdRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, optInPenetration float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkReal("CDLMATHOLD", "optInPenetration", optInPenetration, 0, taRealMax); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLMATHOLD", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLMATHOLD", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	bodyShort := cs[BodyShort]

	lookbackTotal := cdlmatholdLookbackFrom(&cs)
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

	var bodyPeriodTotal [5]float64
	bodyShortTrailingIdx := startIdx - bodyShort.AvgPeriod
	bodyLongTrailingIdx := startIdx - bodyLong.AvgPeriod
	for i := bodyShortTrailingIdx; i < startIdx; i++ {
		bodyPeriodTotal[3] += rng(bodyShort, i-3)
		bodyPeriodTotal[2] += rng(bodyShort, i-2)
		bodyPeriodTotal[1] += rng(bodyShort, i-1)
	}
	for i := bodyLongTrailingIdx; i < startIdx; i++ {
		bodyPeriodTotal[4] += rng(bodyLong, i-4)
	}

	i := startIdx
	outIdx := 0
	for {
		// The compiled C (arm64) computes the penetration product with a
		// separate fmul and fsub; the explicit float64 conversion stops Go
		// from fusing it.
		pen := float64(math.Abs(inClose[i-4]-inOpen[i-4]) * optInPenetration)
		if candleColor(inClose[i-4], inOpen[i-4]) == 1 && // white, black, 2 black or white, white
			candleColor(inClose[i-3], inOpen[i-3]) == -1 &&
			candleColor(inClose[i], inOpen[i]) == 1 &&
			realBodyGapUp(inOpen[i-3], inClose[i-3], inOpen[i-4], inClose[i-4]) && // upside gap 1st to 2nd
			// 3rd and 4th hold within 1st: a part of the real body is within it.
			cMin(inOpen[i-2], inClose[i-2]) < inClose[i-4] &&
			cMin(inOpen[i-1], inClose[i-1]) < inClose[i-4] &&
			// Reaction days penetrate the first body less than optInPenetration.
			cMin(inOpen[i-2], inClose[i-2]) > inClose[i-4]-pen &&
			cMin(inOpen[i-1], inClose[i-1]) > inClose[i-4]-pen &&
			// 2nd to 4th are falling.
			cMax(inClose[i-2], inOpen[i-2]) < inOpen[i-3] &&
			cMax(inClose[i-1], inOpen[i-1]) < cMax(inClose[i-2], inOpen[i-2]) &&
			inOpen[i] > inClose[i-1] && // 5th opens above the prior close
			inClose[i] > cMax(cMax(inHigh[i-3], inHigh[i-2]), inHigh[i-1]) && // 5th closes above the reaction highs
			// 1st long, then 3 small.
			math.Abs(inClose[i-4]-inOpen[i-4]) > avg(bodyLong, bodyPeriodTotal[4], i-4) &&
			math.Abs(inClose[i-3]-inOpen[i-3]) < avg(bodyShort, bodyPeriodTotal[3], i-3) &&
			math.Abs(inClose[i-2]-inOpen[i-2]) < avg(bodyShort, bodyPeriodTotal[2], i-2) &&
			math.Abs(inClose[i-1]-inOpen[i-1]) < avg(bodyShort, bodyPeriodTotal[1], i-1) {
			outInteger[outIdx] = 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyPeriodTotal[4] += rng(bodyLong, i-4) - rng(bodyLong, bodyLongTrailingIdx-4)
		for totIdx := 3; totIdx >= 1; totIdx-- {
			bodyPeriodTotal[totIdx] += rng(bodyShort, i-totIdx) - rng(bodyShort, bodyShortTrailingIdx-totIdx)
		}
		i++
		bodyShortTrailingIdx++
		bodyLongTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
