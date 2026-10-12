package ta

// CDL3BLACKCROWS — Three Black Crows (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdl3blackcrows and src/ta_func/ta_CDL3BLACKCROWS.c.

// CDL3BLACKCROWSLookback returns the number of leading bars CDL3BLACKCROWS
// leaves empty. It depends on the current ShadowVeryShort candle setting. It
// has no parameters, so it never returns -1.
func CDL3BLACKCROWSLookback() int {
	return cdl3blackcrowsLookback()
}

func cdl3blackcrowsLookback() int {
	cs := candleSettingsSnapshot()
	return cdl3blackcrowsLookbackFrom(&cs)
}

func cdl3blackcrowsLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cs[ShadowVeryShort].AvgPeriod + 3
}

// CDL3BLACKCROWS recognises Three Black Crows: a white candle followed by three
// consecutive, declining black candles, each opening within the prior black
// candle's real body and with no or a very short lower shadow, the first black
// closing under the white candle's high. The output is index-aligned with the
// inputs: -100 where the pattern is found, 0 otherwise and inside the lookback.
func CDL3BLACKCROWS(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDL3BLACKCROWS", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdl3blackcrowsRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdl3blackcrowsRange mirrors TA_CDL3BLACKCROWS: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdl3blackcrowsRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDL3BLACKCROWS", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDL3BLACKCROWS", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	svs := cs[ShadowVeryShort]

	lookbackTotal := cdl3blackcrowsLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(i int) float64 {
		return candleRange(svs.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}
	avg := func(sum float64, i int) float64 {
		return candleAverage(svs, sum, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	var total [3]float64
	trailingIdx := startIdx - svs.AvgPeriod
	for i := trailingIdx; i < startIdx; i++ {
		total[2] += rng(i - 2)
		total[1] += rng(i - 1)
		total[0] += rng(i)
	}

	i := startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-3], inOpen[i-3]) == 1 && // white
			candleColor(inClose[i-2], inOpen[i-2]) == -1 && // 1st black
			candleColor(inClose[i-1], inOpen[i-1]) == -1 && // 2nd black
			candleColor(inClose[i], inOpen[i]) == -1 && // 3rd black
			inOpen[i-1] < inOpen[i-2] &&
			inOpen[i-1] > inClose[i-2] && // 2nd black opens within 1st black's rb
			inOpen[i] < inOpen[i-1] &&
			inOpen[i] > inClose[i-1] && // 3rd black opens within 2nd black's rb
			inHigh[i-3] > inClose[i-2] && // 1st black closes under prior candle's high
			inClose[i-2] > inClose[i-1] && // three declining
			inClose[i-1] > inClose[i] && // three declining
			lowerShadow(inLow[i-2], inClose[i-2], inOpen[i-2]) < avg(total[2], i-2) && // very short lower shadows
			lowerShadow(inLow[i-1], inClose[i-1], inOpen[i-1]) < avg(total[1], i-1) &&
			lowerShadow(inLow[i], inClose[i], inOpen[i]) < avg(total[0], i) {
			outInteger[outIdx] = -100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		for totIdx := 2; totIdx >= 0; totIdx-- {
			total[totIdx] = total[totIdx] + (rng(i-totIdx) - rng(trailingIdx-totIdx))
		}
		i++
		trailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
