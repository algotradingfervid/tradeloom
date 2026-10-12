package ta

// CDLUPSIDEGAP2CROWS — Upside Gap Two Crows (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlupsidegap2crows and
// src/ta_func/ta_CDLUPSIDEGAP2CROWS.c.

// CDLUPSIDEGAP2CROWSLookback returns the number of leading bars
// CDLUPSIDEGAP2CROWS leaves empty. It depends on the current BodyLong and
// BodyShort candle settings. It has no parameters, so it never returns -1.
func CDLUPSIDEGAP2CROWSLookback() int {
	return cdlupsidegap2crowsLookback()
}

func cdlupsidegap2crowsLookback() int {
	cs := candleSettingsSnapshot()
	return cdlupsidegap2crowsLookbackFrom(&cs)
}

func cdlupsidegap2crowsLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[BodyShort].AvgPeriod, cs[BodyLong].AvgPeriod) + 2
}

// CDLUPSIDEGAP2CROWS recognises Upside Gap Two Crows: a long white candle, a
// small black candle whose real body gaps up above it, then a black candle
// that opens above the second candle's open, closes below its close, and
// still closes above the first candle's close. The output is index-aligned
// with the inputs: -100 where the pattern is found, 0 otherwise and inside
// the lookback.
func CDLUPSIDEGAP2CROWS(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLUPSIDEGAP2CROWS", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlupsidegap2crowsRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlupsidegap2crowsRange mirrors TA_CDLUPSIDEGAP2CROWS: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlupsidegap2crowsRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLUPSIDEGAP2CROWS", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLUPSIDEGAP2CROWS", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	bodyShort := cs[BodyShort]

	lookbackTotal := cdlupsidegap2crowsLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rngLong := func(i int) float64 {
		return candleRange(bodyLong.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}
	rngShort := func(i int) float64 {
		return candleRange(bodyShort.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	// Add up the initial periods, except for the last value.
	bodyLongPeriodTotal := 0.0
	bodyShortPeriodTotal := 0.0
	bodyLongTrailingIdx := startIdx - 2 - bodyLong.AvgPeriod
	bodyShortTrailingIdx := startIdx - 1 - bodyShort.AvgPeriod
	for i := bodyLongTrailingIdx; i < startIdx-2; i++ {
		bodyLongPeriodTotal += rngLong(i)
	}
	for i := bodyShortTrailingIdx; i < startIdx-1; i++ {
		bodyShortPeriodTotal += rngShort(i)
	}

	i := startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-2], inOpen[i-2]) == 1 && // 1st: white
			realBody(inClose[i-2], inOpen[i-2]) > candleAverage(bodyLong, bodyLongPeriodTotal, inOpen[i-2], inHigh[i-2], inLow[i-2], inClose[i-2]) && // long
			candleColor(inClose[i-1], inOpen[i-1]) == -1 && // 2nd: black
			realBody(inClose[i-1], inOpen[i-1]) <= candleAverage(bodyShort, bodyShortPeriodTotal, inOpen[i-1], inHigh[i-1], inLow[i-1], inClose[i-1]) && // short
			realBodyGapUp(inOpen[i-1], inClose[i-1], inOpen[i-2], inClose[i-2]) && // gapping up
			candleColor(inClose[i], inOpen[i]) == -1 && // 3rd: black
			inOpen[i] > inOpen[i-1] &&
			inClose[i] < inClose[i-1] && // 3rd: engulfing prior rb
			inClose[i] > inClose[i-2] { // closing above 1st
			outInteger[outIdx] = -100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyLongPeriodTotal += rngLong(i-2) - rngLong(bodyLongTrailingIdx)
		bodyShortPeriodTotal += rngShort(i-1) - rngShort(bodyShortTrailingIdx)
		i++
		bodyLongTrailingIdx++
		bodyShortTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
