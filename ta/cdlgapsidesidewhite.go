package ta

// CDLGAPSIDESIDEWHITE — Up/Down-gap side-by-side white lines (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlgapsidesidewhite and
// src/ta_func/ta_CDLGAPSIDESIDEWHITE.c.

// CDLGAPSIDESIDEWHITELookback returns the number of leading bars
// CDLGAPSIDESIDEWHITE leaves empty. It depends on the current Near and Equal
// candle settings. It has no parameters, so it never returns -1.
func CDLGAPSIDESIDEWHITELookback() int {
	return cdlgapsidesidewhiteLookback()
}

func cdlgapsidesidewhiteLookback() int {
	cs := candleSettingsSnapshot()
	return cdlgapsidesidewhiteLookbackFrom(&cs)
}

func cdlgapsidesidewhiteLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[Near].AvgPeriod, cs[Equal].AvgPeriod) + 2
}

// CDLGAPSIDESIDEWHITE recognises Up/Down-gap side-by-side white lines: a first
// candle followed by two white candles whose real bodies both gap the same
// way (up or down) from the first candle's body, with bodies of near-equal
// size (Near setting) and about the same open (Equal setting). The prior trend
// is not checked. The output is index-aligned with the inputs: +100 for an
// up-gap, -100 for a down-gap, 0 otherwise and inside the lookback.
func CDLGAPSIDESIDEWHITE(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLGAPSIDESIDEWHITE", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlgapsidesidewhiteRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlgapsidesidewhiteRange mirrors TA_CDLGAPSIDESIDEWHITE: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlgapsidesidewhiteRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLGAPSIDESIDEWHITE", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLGAPSIDESIDEWHITE", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	near := cs[Near]
	equal := cs[Equal]

	lookbackTotal := cdlgapsidesidewhiteLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	nearRange := func(i int) float64 {
		return candleRange(near.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}
	equalRange := func(i int) float64 {
		return candleRange(equal.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	// Add up the initial period, except for the last value.
	nearPeriodTotal := 0.0
	equalPeriodTotal := 0.0
	nearTrailingIdx := startIdx - near.AvgPeriod
	equalTrailingIdx := startIdx - equal.AvgPeriod
	for i := nearTrailingIdx; i < startIdx; i++ {
		nearPeriodTotal += nearRange(i - 1)
	}
	for i := equalTrailingIdx; i < startIdx; i++ {
		equalPeriodTotal += equalRange(i - 1)
	}

	i := startIdx
	outIdx := 0
	for {
		gapUp2 := realBodyGapUp(inOpen[i-1], inClose[i-1], inOpen[i-2], inClose[i-2])
		if ((gapUp2 && // upside or downside gap between the 1st candle and both the next 2
			realBodyGapUp(inOpen[i], inClose[i], inOpen[i-2], inClose[i-2])) ||
			(realBodyGapDown(inOpen[i-1], inClose[i-1], inOpen[i-2], inClose[i-2]) &&
				realBodyGapDown(inOpen[i], inClose[i], inOpen[i-2], inClose[i-2]))) &&
			candleColor(inClose[i-1], inOpen[i-1]) == 1 && // 2nd: white
			candleColor(inClose[i], inOpen[i]) == 1 && // 3rd: white
			// same size 2 and 3
			realBody(inClose[i], inOpen[i]) >= realBody(inClose[i-1], inOpen[i-1])-candleAverage(near, nearPeriodTotal, inOpen[i-1], inHigh[i-1], inLow[i-1], inClose[i-1]) &&
			realBody(inClose[i], inOpen[i]) <= realBody(inClose[i-1], inOpen[i-1])+candleAverage(near, nearPeriodTotal, inOpen[i-1], inHigh[i-1], inLow[i-1], inClose[i-1]) &&
			// same open 2 and 3
			inOpen[i] >= inOpen[i-1]-candleAverage(equal, equalPeriodTotal, inOpen[i-1], inHigh[i-1], inLow[i-1], inClose[i-1]) &&
			inOpen[i] <= inOpen[i-1]+candleAverage(equal, equalPeriodTotal, inOpen[i-1], inHigh[i-1], inLow[i-1], inClose[i-1]) {
			if gapUp2 {
				outInteger[outIdx] = 100
			} else {
				outInteger[outIdx] = -100
			}
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		nearPeriodTotal += nearRange(i-1) - nearRange(nearTrailingIdx-1)
		equalPeriodTotal += equalRange(i-1) - equalRange(equalTrailingIdx-1)
		i++
		nearTrailingIdx++
		equalTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
