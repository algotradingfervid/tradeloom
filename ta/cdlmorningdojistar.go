package ta

import "math"

// CDLMORNINGDOJISTAR — Morning Doji Star (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlmorningdojistar and
// src/ta_func/ta_CDLMORNINGDOJISTAR.c.

// CDLMORNINGDOJISTARLookback returns the number of leading bars
// CDLMORNINGDOJISTAR leaves empty, or -1 when optInPenetration is invalid. It
// depends on the current candle settings (BodyDoji, BodyLong, BodyShort).
// optInPenetration: [0, 3e37], default 0.3.
func CDLMORNINGDOJISTARLookback(optInPenetration float64) int {
	if checkReal("CDLMORNINGDOJISTAR", "optInPenetration", optInPenetration, 0, taRealMax) != nil {
		return -1
	}
	return cdlmorningdojistarLookback(optInPenetration)
}

func cdlmorningdojistarLookback(optInPenetration float64) int {
	cs := candleSettingsSnapshot()
	return cdlmorningdojistarLookbackFrom(&cs)
}

func cdlmorningdojistarLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cMaxInt(cs[BodyDoji].AvgPeriod, cs[BodyLong].AvgPeriod), cs[BodyShort].AvgPeriod) + 2
}

// CDLMORNINGDOJISTAR recognises the Morning Doji Star: a long black candle, a
// doji whose real body gaps down below it, then a white candle (longer than
// short) closing above the first close plus optInPenetration of the first
// real body. The output is index-aligned with the inputs: 100 where the
// pattern completes, 0 otherwise and inside the lookback.
// optInPenetration: [0, 3e37], default 0.3.
func CDLMORNINGDOJISTAR(inOpen, inHigh, inLow, inClose []float64, optInPenetration float64) ([]int, error) {
	if err := checkReal("CDLMORNINGDOJISTAR", "optInPenetration", optInPenetration, 0, taRealMax); err != nil {
		return nil, err
	}
	n, err := checkInputs("CDLMORNINGDOJISTAR", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlmorningdojistarRange(0, n-1, inOpen, inHigh, inLow, inClose, optInPenetration, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlmorningdojistarRange mirrors TA_CDLMORNINGDOJISTAR: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlmorningdojistarRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, optInPenetration float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkReal("CDLMORNINGDOJISTAR", "optInPenetration", optInPenetration, 0, taRealMax); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLMORNINGDOJISTAR", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLMORNINGDOJISTAR", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyDoji := cs[BodyDoji]
	bodyLong := cs[BodyLong]
	bodyShort := cs[BodyShort]

	lookbackTotal := cdlmorningdojistarLookbackFrom(&cs)
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
		if candleColor(inClose[i-2], inOpen[i-2]) == -1 && // 1st black
			candleColor(inClose[i], inOpen[i]) == 1 && // 3rd white
			realBodyGapDown(inOpen[i-1], inClose[i-1], inOpen[i-2], inClose[i-2]) && // 2nd gapping down
			// 3rd closes well within the 1st real body. The C source calls
			// fma() explicitly here (single rounding), so math.FMA is exact.
			inClose[i] > math.FMA(math.Abs(inClose[i-2]-inOpen[i-2]), optInPenetration, inClose[i-2]) &&
			math.Abs(inClose[i-2]-inOpen[i-2]) > avg(bodyLong, bodyLongPeriodTotal, i-2) && // 1st: long
			math.Abs(inClose[i-1]-inOpen[i-1]) <= avg(bodyDoji, bodyDojiPeriodTotal, i-1) && // 2nd: doji
			math.Abs(inClose[i]-inOpen[i]) > avg(bodyShort, bodyShortPeriodTotal, i) { // 3rd: longer than short
			outInteger[outIdx] = 100
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
