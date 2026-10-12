package ta

import "math"

// CDLEVENINGSTAR — Evening Star (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdleveningstar and
// src/ta_func/ta_CDLEVENINGSTAR.c.

// CDLEVENINGSTARLookback returns the number of leading bars CDLEVENINGSTAR
// leaves empty, or -1 when optInPenetration is invalid. It depends on the
// current candle settings (BodyLong, BodyShort).
// optInPenetration: [0, 3e37], default 0.3.
func CDLEVENINGSTARLookback(optInPenetration float64) int {
	if checkReal("CDLEVENINGSTAR", "optInPenetration", optInPenetration, 0, taRealMax) != nil {
		return -1
	}
	return cdleveningstarLookback(optInPenetration)
}

func cdleveningstarLookback(optInPenetration float64) int {
	cs := candleSettingsSnapshot()
	return cdleveningstarLookbackFrom(&cs)
}

func cdleveningstarLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[BodyShort].AvgPeriod, cs[BodyLong].AvgPeriod) + 2
}

// CDLEVENINGSTAR recognises the Evening Star: a long white body, a short
// body whose real body gaps up above it, then a black body (longer than
// short) that closes optInPenetration of the way into the first body.
// The output is index-aligned with the inputs: -100 where the pattern
// completes, 0 otherwise and inside the lookback.
// optInPenetration: [0, 3e37], default 0.3.
func CDLEVENINGSTAR(inOpen, inHigh, inLow, inClose []float64, optInPenetration float64) ([]int, error) {
	if err := checkReal("CDLEVENINGSTAR", "optInPenetration", optInPenetration, 0, taRealMax); err != nil {
		return nil, err
	}
	n, err := checkInputs("CDLEVENINGSTAR", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdleveningstarRange(0, n-1, inOpen, inHigh, inLow, inClose, optInPenetration, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdleveningstarRange mirrors TA_CDLEVENINGSTAR: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdleveningstarRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, optInPenetration float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkReal("CDLEVENINGSTAR", "optInPenetration", optInPenetration, 0, taRealMax); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLEVENINGSTAR", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLEVENINGSTAR", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	bodyShort := cs[BodyShort]

	lookbackTotal := cdleveningstarLookbackFrom(&cs)
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
	bodyShortPeriodTotal := 0.0
	bodyShortPeriodTotal2 := 0.0
	bodyLongTrailingIdx := startIdx - 2 - bodyLong.AvgPeriod
	bodyShortTrailingIdx := startIdx - 1 - bodyShort.AvgPeriod
	for i := bodyLongTrailingIdx; i < startIdx-2; i++ {
		bodyLongPeriodTotal += rng(bodyLong, i)
	}
	for i := bodyShortTrailingIdx; i < startIdx-1; i++ {
		bodyShortPeriodTotal += rng(bodyShort, i)
		bodyShortPeriodTotal2 += rng(bodyShort, i+1)
	}

	i := startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-2], inOpen[i-2]) == 1 && // white
			candleColor(inClose[i], inOpen[i]) == -1 && // black real body
			realBodyGapUp(inOpen[i-1], inClose[i-1], inOpen[i-2], inClose[i-2]) && // gapping up
			// Closing well within the 1st real body. The compiled C (arm64)
			// computes the product with a separate fmul and an fsub, so the
			// explicit float64 conversion stops Go from fusing it.
			inClose[i] < inClose[i-2]-float64(math.Abs(inClose[i-2]-inOpen[i-2])*optInPenetration) &&
			math.Abs(inClose[i-2]-inOpen[i-2]) > avg(bodyLong, bodyLongPeriodTotal, i-2) && // 1st: long
			math.Abs(inClose[i-1]-inOpen[i-1]) <= avg(bodyShort, bodyShortPeriodTotal, i-1) && // 2nd: short
			math.Abs(inClose[i]-inOpen[i]) > avg(bodyShort, bodyShortPeriodTotal2, i) { // 3rd: longer than short
			outInteger[outIdx] = -100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyLongPeriodTotal += rng(bodyLong, i-2) - rng(bodyLong, bodyLongTrailingIdx)
		bodyShortPeriodTotal += rng(bodyShort, i-1) - rng(bodyShort, bodyShortTrailingIdx)
		bodyShortPeriodTotal2 += rng(bodyShort, i) - rng(bodyShort, bodyShortTrailingIdx+1)
		i++
		bodyLongTrailingIdx++
		bodyShortTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
