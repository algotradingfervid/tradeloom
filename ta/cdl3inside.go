package ta

// CDL3INSIDE — Three Inside Up/Down (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdl3inside and src/ta_func/ta_CDL3INSIDE.c.

// CDL3INSIDELookback returns the number of leading bars CDL3INSIDE leaves
// empty. It depends on the current candle settings (BodyLong, BodyShort). It
// has no parameters, so it never returns -1.
func CDL3INSIDELookback() int {
	return cdl3insideLookback()
}

func cdl3insideLookback() int {
	cs := candleSettingsSnapshot()
	return cdl3insideLookbackFrom(&cs)
}

func cdl3insideLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[BodyShort].AvgPeriod, cs[BodyLong].AvgPeriod) + 2
}

// CDL3INSIDE recognises the Three Inside Up/Down: a long real body, then a
// short real body strictly inside it (a harami), then a candle of the
// opposite color to the first that closes beyond the first candle's open.
// The output is index-aligned with the inputs: +100 for three inside up
// (first candle black), -100 for three inside down (first candle white), 0
// otherwise and inside the lookback.
func CDL3INSIDE(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDL3INSIDE", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdl3insideRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdl3insideRange mirrors TA_CDL3INSIDE: it evaluates [startIdx, endIdx] and
// writes outNBElement values to outInteger[0:], the first belonging to bar
// outBegIdx.
func cdl3insideRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDL3INSIDE", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDL3INSIDE", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	bodyShort := cs[BodyShort]

	lookbackTotal := cdl3insideLookbackFrom(&cs)
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
	bodyLongTrailingIdx := startIdx - 2 - bodyLong.AvgPeriod
	bodyShortTrailingIdx := startIdx - 1 - bodyShort.AvgPeriod
	for i := bodyLongTrailingIdx; i < startIdx-2; i++ {
		bodyLongPeriodTotal += rng(bodyLong, i)
	}
	for i := bodyShortTrailingIdx; i < startIdx-1; i++ {
		bodyShortPeriodTotal += rng(bodyShort, i)
	}

	i := startIdx
	outIdx := 0
	for {
		color2 := candleColor(inClose[i-2], inOpen[i-2])
		color0 := candleColor(inClose[i], inOpen[i])
		if cMax(inClose[i-1], inOpen[i-1]) < cMax(inClose[i-2], inOpen[i-2]) && // engulfed by 1st
			cMin(inClose[i-1], inOpen[i-1]) > cMin(inClose[i-2], inOpen[i-2]) &&
			((color2 == 1 && color0 == -1 && inClose[i] < inOpen[i-2]) || // 3rd: opposite to 1st
				(color2 == -1 && color0 == 1 && inClose[i] > inOpen[i-2])) && // and closing out
			realBody(inClose[i-2], inOpen[i-2]) > avg(bodyLong, bodyLongPeriodTotal, i-2) && // 1st: long
			realBody(inClose[i-1], inOpen[i-1]) <= avg(bodyShort, bodyShortPeriodTotal, i-1) { // 2nd: short
			outInteger[outIdx] = -color2 * 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyLongPeriodTotal += rng(bodyLong, i-2) - rng(bodyLong, bodyLongTrailingIdx)
		bodyShortPeriodTotal += rng(bodyShort, i-1) - rng(bodyShort, bodyShortTrailingIdx)
		i++
		bodyLongTrailingIdx++
		bodyShortTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
