package ta

import "math"

// CDLEVENINGDOJISTAR — Evening Doji Star (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdleveningdojistar and
// src/ta_func/ta_CDLEVENINGDOJISTAR.c.

// CDLEVENINGDOJISTARLookback returns the number of leading bars
// CDLEVENINGDOJISTAR leaves empty, or -1 when optInPenetration is invalid. It
// depends on the current candle settings (BodyDoji, BodyLong, BodyShort).
// optInPenetration: [0, 3e37], default 0.3.
func CDLEVENINGDOJISTARLookback(optInPenetration float64) int {
	if checkReal("CDLEVENINGDOJISTAR", "optInPenetration", optInPenetration, 0, taRealMax) != nil {
		return -1
	}
	return cdleveningdojistarLookback(optInPenetration)
}

func cdleveningdojistarLookback(optInPenetration float64) int {
	cs := candleSettingsSnapshot()
	return cdleveningdojistarLookbackFrom(&cs)
}

func cdleveningdojistarLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cMaxInt(cs[BodyDoji].AvgPeriod, cs[BodyLong].AvgPeriod), cs[BodyShort].AvgPeriod) + 2
}

// CDLEVENINGDOJISTAR recognises the Evening Doji Star: a long white candle, a
// doji whose real body gaps up above it, then a black candle (longer than
// short) closing below the first close minus optInPenetration of the first
// real body. The output is index-aligned with the inputs: -100 where the
// pattern completes, 0 otherwise and inside the lookback.
// optInPenetration: [0, 3e37], default 0.3.
func CDLEVENINGDOJISTAR(inOpen, inHigh, inLow, inClose []float64, optInPenetration float64) ([]int, error) {
	if err := checkReal("CDLEVENINGDOJISTAR", "optInPenetration", optInPenetration, 0, taRealMax); err != nil {
		return nil, err
	}
	n, err := checkInputs("CDLEVENINGDOJISTAR", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdleveningdojistarRange(0, n-1, inOpen, inHigh, inLow, inClose, optInPenetration, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdleveningdojistarRange mirrors TA_CDLEVENINGDOJISTAR: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdleveningdojistarRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, optInPenetration float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkReal("CDLEVENINGDOJISTAR", "optInPenetration", optInPenetration, 0, taRealMax); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLEVENINGDOJISTAR", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLEVENINGDOJISTAR", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyDoji := cs[BodyDoji]
	bodyLong := cs[BodyLong]
	bodyShort := cs[BodyShort]

	lookbackTotal := cdleveningdojistarLookbackFrom(&cs)
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
		if candleColor(inClose[i-2], inOpen[i-2]) == 1 && // 1st white
			candleColor(inClose[i], inOpen[i]) == -1 && // 3rd black
			realBodyGapUp(inOpen[i-1], inClose[i-1], inOpen[i-2], inClose[i-2]) && // 2nd gapping up
			// 3rd closes well within the 1st real body. The C library is
			// built with -ffp-contract=off, so the product is rounded before
			// the subtraction; the float64 conversion stops Go fusing it.
			inClose[i] < inClose[i-2]-float64(math.Abs(inClose[i-2]-inOpen[i-2])*optInPenetration) &&
			math.Abs(inClose[i-2]-inOpen[i-2]) > avg(bodyLong, bodyLongPeriodTotal, i-2) && // 1st: long
			math.Abs(inClose[i-1]-inOpen[i-1]) <= avg(bodyDoji, bodyDojiPeriodTotal, i-1) && // 2nd: doji
			math.Abs(inClose[i]-inOpen[i]) > avg(bodyShort, bodyShortPeriodTotal, i) { // 3rd: longer than short
			outInteger[outIdx] = -100
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
