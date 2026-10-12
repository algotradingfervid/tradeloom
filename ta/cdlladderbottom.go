package ta

// CDLLADDERBOTTOM — Ladder Bottom (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlladderbottom and src/ta_func/ta_CDLLADDERBOTTOM.c.

// CDLLADDERBOTTOMLookback returns the number of leading bars CDLLADDERBOTTOM
// leaves empty. It depends on the current ShadowVeryShort candle setting. It
// has no parameters, so it never returns -1.
func CDLLADDERBOTTOMLookback() int {
	return cdlladderbottomLookback()
}

func cdlladderbottomLookback() int {
	cs := candleSettingsSnapshot()
	return cdlladderbottomLookbackFrom(&cs)
}

func cdlladderbottomLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cs[ShadowVeryShort].AvgPeriod + 4
}

// CDLLADDERBOTTOM recognises Ladder Bottom: three black candles with
// consecutively lower opens and closes, a fourth black candle with an upper
// shadow that is not very short, and a fifth white candle opening above the
// fourth's open and closing above its high. The output is index-aligned with
// the inputs: 100 where the pattern is found, 0 otherwise and inside the
// lookback.
func CDLLADDERBOTTOM(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLLADDERBOTTOM", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlladderbottomRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlladderbottomRange mirrors TA_CDLLADDERBOTTOM: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlladderbottomRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLLADDERBOTTOM", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLLADDERBOTTOM", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	svs := cs[ShadowVeryShort]

	lookbackTotal := cdlladderbottomLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(i int) float64 {
		return candleRange(svs.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	// Add up the initial period, except for the last value.
	shadowVeryShortPeriodTotal := 0.0
	trailingIdx := startIdx - svs.AvgPeriod
	for i := trailingIdx; i < startIdx; i++ {
		shadowVeryShortPeriodTotal += rng(i - 1)
	}

	i := startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-4], inOpen[i-4]) == -1 &&
			candleColor(inClose[i-3], inOpen[i-3]) == -1 &&
			candleColor(inClose[i-2], inOpen[i-2]) == -1 && // 3 black candlesticks
			inOpen[i-4] > inOpen[i-3] &&
			inOpen[i-3] > inOpen[i-2] && // with consecutively lower opens
			inClose[i-4] > inClose[i-3] &&
			inClose[i-3] > inClose[i-2] && // and closes
			candleColor(inClose[i-1], inOpen[i-1]) == -1 && // 4th: black with an upper shadow
			upperShadow(inHigh[i-1], inClose[i-1], inOpen[i-1]) >
				candleAverage(svs, shadowVeryShortPeriodTotal, inOpen[i-1], inHigh[i-1], inLow[i-1], inClose[i-1]) &&
			candleColor(inClose[i], inOpen[i]) == 1 && // 5th: white
			inOpen[i] > inOpen[i-1] && // that opens above prior candle's body
			inClose[i] > inHigh[i-1] { // and closes above prior candle's high
			outInteger[outIdx] = 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		shadowVeryShortPeriodTotal += rng(i-1) - rng(trailingIdx-1)
		i++
		trailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
