package ta

import "math"

// CDLRISEFALL3METHODS — Rising/Falling Three Methods (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlrisefall3methods and
// src/ta_func/ta_CDLRISEFALL3METHODS.c.

// CDLRISEFALL3METHODSLookback returns the number of leading bars
// CDLRISEFALL3METHODS leaves empty. It depends on the current BodyLong and
// BodyShort candle settings. It has no parameters, so it never returns -1.
func CDLRISEFALL3METHODSLookback() int {
	return cdlrisefall3methodsLookback()
}

func cdlrisefall3methodsLookback() int {
	cs := candleSettingsSnapshot()
	return cdlrisefall3methodsLookbackFrom(&cs)
}

func cdlrisefall3methodsLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[BodyShort].AvgPeriod, cs[BodyLong].AvgPeriod) + 4
}

// CDLRISEFALL3METHODS recognises the Rising (Falling) Three Methods: a long
// white (black) candle, three small candles of the opposite color with
// falling (rising) closes whose real bodies each overlap the first candle's
// range, then a long white (black) candle that opens beyond the prior close
// and closes beyond the first close. The output is index-aligned with the
// inputs: +100 for rising, -100 for falling, 0 otherwise and inside the
// lookback.
func CDLRISEFALL3METHODS(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLRISEFALL3METHODS", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlrisefall3methodsRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlrisefall3methodsRange mirrors TA_CDLRISEFALL3METHODS: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlrisefall3methodsRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLRISEFALL3METHODS", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLRISEFALL3METHODS", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	bodyShort := cs[BodyShort]

	lookbackTotal := cdlrisefall3methodsLookbackFrom(&cs)
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
	avgLong := func(sum float64, i int) float64 {
		return candleAverage(bodyLong, sum, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}
	avgShort := func(sum float64, i int) float64 {
		return candleAverage(bodyShort, sum, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}
	color := func(i int) int { return candleColor(inClose[i], inOpen[i]) }

	var bodyPeriodTotal [5]float64
	bodyShortTrailingIdx := startIdx - bodyShort.AvgPeriod
	bodyLongTrailingIdx := startIdx - bodyLong.AvgPeriod
	for i := bodyShortTrailingIdx; i < startIdx; i++ {
		bodyPeriodTotal[3] = bodyPeriodTotal[3] + rngShort(i-3)
		bodyPeriodTotal[2] = bodyPeriodTotal[2] + rngShort(i-2)
		bodyPeriodTotal[1] = bodyPeriodTotal[1] + rngShort(i-1)
	}
	for i := bodyLongTrailingIdx; i < startIdx; i++ {
		bodyPeriodTotal[4] = bodyPeriodTotal[4] + rngLong(i-4)
		bodyPeriodTotal[0] = bodyPeriodTotal[0] + rngLong(i)
	}

	i := startIdx
	outIdx := 0
	for {
		// C multiplies a double by the int color; float64(c) is exact.
		c4 := float64(color(i - 4))
		if color(i-4) == -color(i-3) && // white, 3 black, white || black, 3 white, black
			color(i-3) == color(i-2) &&
			color(i-2) == color(i-1) &&
			color(i-1) == -color(i) &&
			// 2nd to 4th hold within 1st: a part of the real body must be within 1st range
			cMin(inOpen[i-3], inClose[i-3]) < inHigh[i-4] &&
			cMax(inOpen[i-3], inClose[i-3]) > inLow[i-4] &&
			cMin(inOpen[i-2], inClose[i-2]) < inHigh[i-4] &&
			cMax(inOpen[i-2], inClose[i-2]) > inLow[i-4] &&
			cMin(inOpen[i-1], inClose[i-1]) < inHigh[i-4] &&
			cMax(inOpen[i-1], inClose[i-1]) > inLow[i-4] &&
			// 2nd to 4th are falling (rising)
			inClose[i-2]*c4 < inClose[i-3]*c4 &&
			inClose[i-1]*c4 < inClose[i-2]*c4 &&
			inOpen[i]*c4 > inClose[i-1]*c4 && // 5th opens above (below) the prior close
			inClose[i]*c4 > inClose[i-4]*c4 && // 5th closes above (below) the 1st close
			// 1st long, then 3 small, 5th long
			math.Abs(inClose[i-4]-inOpen[i-4]) > avgLong(bodyPeriodTotal[4], i-4) &&
			math.Abs(inClose[i-3]-inOpen[i-3]) < avgShort(bodyPeriodTotal[3], i-3) &&
			math.Abs(inClose[i-2]-inOpen[i-2]) < avgShort(bodyPeriodTotal[2], i-2) &&
			math.Abs(inClose[i-1]-inOpen[i-1]) < avgShort(bodyPeriodTotal[1], i-1) &&
			math.Abs(inClose[i]-inOpen[i]) > avgLong(bodyPeriodTotal[0], i) {
			outInteger[outIdx] = 100 * color(i-4)
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyPeriodTotal[4] = bodyPeriodTotal[4] + (rngLong(i-4) - rngLong(bodyLongTrailingIdx-4))
		for totIdx := 3; totIdx >= 1; totIdx-- {
			bodyPeriodTotal[totIdx] = bodyPeriodTotal[totIdx] + (rngShort(i-totIdx) - rngShort(bodyShortTrailingIdx-totIdx))
		}
		bodyPeriodTotal[0] = bodyPeriodTotal[0] + (rngLong(i) - rngLong(bodyLongTrailingIdx))
		i++
		bodyShortTrailingIdx++
		bodyLongTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
