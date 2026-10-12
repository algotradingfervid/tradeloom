package ta

import "math"

// CDLABANDONEDBABY — Abandoned Baby (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlabandonedbaby and
// src/ta_func/ta_CDLABANDONEDBABY.c.

// CDLABANDONEDBABYLookback returns the number of leading bars
// CDLABANDONEDBABY leaves empty, or -1 when optInPenetration is invalid. It
// depends on the current candle settings (BodyDoji, BodyLong, BodyShort).
// optInPenetration: [0, 3e37], default 0.3.
func CDLABANDONEDBABYLookback(optInPenetration float64) int {
	if checkReal("CDLABANDONEDBABY", "optInPenetration", optInPenetration, 0, taRealMax) != nil {
		return -1
	}
	return cdlabandonedbabyLookback(optInPenetration)
}

func cdlabandonedbabyLookback(optInPenetration float64) int {
	cs := candleSettingsSnapshot()
	return cdlabandonedbabyLookbackFrom(&cs)
}

func cdlabandonedbabyLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cMaxInt(cs[BodyDoji].AvgPeriod, cs[BodyLong].AvgPeriod), cs[BodyShort].AvgPeriod) + 2
}

// CDLABANDONEDBABY recognises the Abandoned Baby: a long body, a doji gapped
// away from it, then an opposite-colour body (longer than short) gapped back
// the other way that closes optInPenetration of the way into the first body.
// The output is index-aligned with the inputs: +100 for a bottom (3rd candle
// white), -100 for a top (3rd candle black), 0 otherwise and inside the
// lookback. optInPenetration: [0, 3e37], default 0.3.
func CDLABANDONEDBABY(inOpen, inHigh, inLow, inClose []float64, optInPenetration float64) ([]int, error) {
	if err := checkReal("CDLABANDONEDBABY", "optInPenetration", optInPenetration, 0, taRealMax); err != nil {
		return nil, err
	}
	n, err := checkInputs("CDLABANDONEDBABY", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlabandonedbabyRange(0, n-1, inOpen, inHigh, inLow, inClose, optInPenetration, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlabandonedbabyRange mirrors TA_CDLABANDONEDBABY: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlabandonedbabyRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, optInPenetration float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkReal("CDLABANDONEDBABY", "optInPenetration", optInPenetration, 0, taRealMax); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLABANDONEDBABY", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLABANDONEDBABY", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyDoji := cs[BodyDoji]
	bodyLong := cs[BodyLong]
	bodyShort := cs[BodyShort]

	lookbackTotal := cdlabandonedbabyLookbackFrom(&cs)
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
	bodyDojiPeriodTotal := 0.0
	bodyShortPeriodTotal := 0.0
	bodyLongTrailingIdx := startIdx - 2 - bodyLong.AvgPeriod
	bodyDojiTrailingIdx := startIdx - 1 - bodyDoji.AvgPeriod
	bodyShortTrailingIdx := startIdx - bodyShort.AvgPeriod
	for i := bodyLongTrailingIdx; i < startIdx-2; i++ {
		bodyLongPeriodTotal += rng(bodyLong, i)
	}
	for i := bodyDojiTrailingIdx; i < startIdx-1; i++ {
		bodyDojiPeriodTotal += rng(bodyDoji, i)
	}
	for i := bodyShortTrailingIdx; i < startIdx; i++ {
		bodyShortPeriodTotal += rng(bodyShort, i)
	}

	i := startIdx
	outIdx := 0
	for {
		if math.Abs(inClose[i-2]-inOpen[i-2]) > avg(bodyLong, bodyLongPeriodTotal, i-2) && // 1st: long
			math.Abs(inClose[i-1]-inOpen[i-1]) <= avg(bodyDoji, bodyDojiPeriodTotal, i-1) && // 2nd: doji
			math.Abs(inClose[i]-inOpen[i]) > avg(bodyShort, bodyShortPeriodTotal, i) && // 3rd: longer than short
			((candleColor(inClose[i-2], inOpen[i-2]) == 1 && // 1st white
				candleColor(inClose[i], inOpen[i]) == -1 && // 3rd black
				// 3rd closes well within 1st rb. The compiled C (arm64)
				// computes the product with a separate fmul and an fsub, so
				// the explicit float64 conversion stops Go from fusing it.
				inClose[i] < inClose[i-2]-float64(math.Abs(inClose[i-2]-inOpen[i-2])*optInPenetration) &&
				inLow[i-1] > inHigh[i-2] && // upside gap between 1st and 2nd
				inHigh[i] < inLow[i-1]) || // downside gap between 2nd and 3rd
				(candleColor(inClose[i-2], inOpen[i-2]) == -1 && // 1st black
					candleColor(inClose[i], inOpen[i]) == 1 && // 3rd white
					// C uses fma() explicitly; math.FMA matches it bit for bit.
					inClose[i] > math.FMA(math.Abs(inClose[i-2]-inOpen[i-2]), optInPenetration, inClose[i-2]) &&
					inHigh[i-1] < inLow[i-2] && // downside gap between 1st and 2nd
					inLow[i] > inHigh[i-1])) { // upside gap between 2nd and 3rd
			outInteger[outIdx] = candleColor(inClose[i], inOpen[i]) * 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyLongPeriodTotal += rng(bodyLong, i-2) - rng(bodyLong, bodyLongTrailingIdx)
		bodyDojiPeriodTotal += rng(bodyDoji, i-1) - rng(bodyDoji, bodyDojiTrailingIdx)
		bodyShortPeriodTotal += rng(bodyShort, i) - rng(bodyShort, bodyShortTrailingIdx)
		i++
		bodyLongTrailingIdx++
		bodyDojiTrailingIdx++
		bodyShortTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
