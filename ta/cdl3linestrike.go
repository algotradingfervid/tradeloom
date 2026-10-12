package ta

// CDL3LINESTRIKE — Three-Line Strike (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdl3linestrike and src/ta_func/ta_CDL3LINESTRIKE.c.

// CDL3LINESTRIKELookback returns the number of leading bars CDL3LINESTRIKE
// leaves empty. It depends on the current Near candle setting. It has no
// parameters, so it never returns -1.
func CDL3LINESTRIKELookback() int {
	return cdl3linestrikeLookback()
}

func cdl3linestrikeLookback() int {
	cs := candleSettingsSnapshot()
	return cdl3linestrikeLookbackFrom(&cs)
}

func cdl3linestrikeLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cs[Near].AvgPeriod + 3
}

// CDL3LINESTRIKE recognises the Three-Line Strike: three candles of the same
// color with consecutively higher (white) or lower (black) closes, each
// opening within or near the prior real body, followed by an opposite-color
// candle that opens beyond the third close and closes beyond the first open.
// The output is index-aligned with the inputs: +100 when the three candles are
// white, -100 when they are black, 0 otherwise and inside the lookback.
func CDL3LINESTRIKE(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDL3LINESTRIKE", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdl3linestrikeRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdl3linestrikeRange mirrors TA_CDL3LINESTRIKE: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdl3linestrikeRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDL3LINESTRIKE", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDL3LINESTRIKE", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	near := cs[Near]

	lookbackTotal := cdl3linestrikeLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(i int) float64 {
		return candleRange(near.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}
	avg := func(sum float64, i int) float64 {
		return candleAverage(near, sum, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}
	color := func(i int) int { return candleColor(inClose[i], inOpen[i]) }

	var nearPeriodTotal [4]float64
	nearTrailingIdx := startIdx - near.AvgPeriod
	for i := nearTrailingIdx; i < startIdx; i++ {
		nearPeriodTotal[3] += rng(i - 3)
		nearPeriodTotal[2] += rng(i - 2)
	}

	i := startIdx
	outIdx := 0
	for {
		if color(i-3) == color(i-2) && // three with same color
			color(i-2) == color(i-1) &&
			color(i) == -color(i-1) && // 4th opposite color
			// 2nd opens within/near 1st rb
			inOpen[i-2] >= cMin(inOpen[i-3], inClose[i-3])-avg(nearPeriodTotal[3], i-3) &&
			inOpen[i-2] <= cMax(inOpen[i-3], inClose[i-3])+avg(nearPeriodTotal[3], i-3) &&
			// 3rd opens within/near 2nd rb
			inOpen[i-1] >= cMin(inOpen[i-2], inClose[i-2])-avg(nearPeriodTotal[2], i-2) &&
			inOpen[i-1] <= cMax(inOpen[i-2], inClose[i-2])+avg(nearPeriodTotal[2], i-2) &&
			((color(i-1) == 1 && // three white
				inClose[i-1] > inClose[i-2] &&
				inClose[i-2] > inClose[i-3] && // consecutive higher closes
				inOpen[i] > inClose[i-1] && // 4th opens above prior close
				inClose[i] < inOpen[i-3]) || // 4th closes below 1st open
				(color(i-1) == -1 && // three black
					inClose[i-1] < inClose[i-2] &&
					inClose[i-2] < inClose[i-3] && // consecutive lower closes
					inOpen[i] < inClose[i-1] && // 4th opens below prior close
					inClose[i] > inOpen[i-3])) { // 4th closes above 1st open
			outInteger[outIdx] = color(i-1) * 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		for totIdx := 3; totIdx >= 2; totIdx-- {
			nearPeriodTotal[totIdx] = nearPeriodTotal[totIdx] + (rng(i-totIdx) - rng(nearTrailingIdx-totIdx))
		}
		i++
		nearTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
