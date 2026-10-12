package ta

import "math"

// CDLTASUKIGAP — Tasuki Gap (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdltasukigap and
// src/ta_func/ta_CDLTASUKIGAP.c.

// CDLTASUKIGAPLookback returns the number of leading bars CDLTASUKIGAP leaves
// empty. It depends on the current Near candle setting. It has no
// parameters, so it never returns -1.
func CDLTASUKIGAPLookback() int {
	return cdltasukigapLookback()
}

func cdltasukigapLookback() int {
	cs := candleSettingsSnapshot()
	return cdltasukigapLookbackFrom(&cs)
}

func cdltasukigapLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cs[Near].AvgPeriod + 2
}

// CDLTASUKIGAP recognises the Tasuki Gap: a candle whose real body gaps away
// from the previous candle's body, followed by an opposite-colour candle that
// opens inside the gap candle's body and closes inside the gap without
// filling it, the two bodies being of near-equal size (Near setting). The
// prior trend is not checked. The output is index-aligned with the inputs:
// +100 for an upside gap (white gap candle), -100 for a downside gap (black
// gap candle), 0 otherwise and inside the lookback.
func CDLTASUKIGAP(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLTASUKIGAP", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdltasukigapRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdltasukigapRange mirrors TA_CDLTASUKIGAP: it evaluates [startIdx, endIdx]
// and writes outNBElement values to outInteger[0:], the first belonging to
// bar outBegIdx.
func cdltasukigapRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLTASUKIGAP", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLTASUKIGAP", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	near := cs[Near]

	lookbackTotal := cdltasukigapLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	nearRange := func(i int) float64 {
		return candleRange(near.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	// Add up the initial period, except for the last value.
	nearPeriodTotal := 0.0
	nearTrailingIdx := startIdx - near.AvgPeriod
	i := nearTrailingIdx
	for i < startIdx {
		nearPeriodTotal += nearRange(i - 1)
		i++
	}

	i = startIdx
	outIdx := 0
	for {
		if (realBodyGapUp(inOpen[i-1], inClose[i-1], inOpen[i-2], inClose[i-2]) && // upside gap
			candleColor(inClose[i-1], inOpen[i-1]) == 1 && // 1st: white
			candleColor(inClose[i], inOpen[i]) == -1 && // 2nd: black
			inOpen[i] < inClose[i-1] &&
			inOpen[i] > inOpen[i-1] && // that opens within the white rb
			inClose[i] < inOpen[i-1] && // and closes under the white rb
			inClose[i] > cMax(inClose[i-2], inOpen[i-2]) && // inside the gap
			// size of 2 rb near the same
			math.Abs(realBody(inClose[i-1], inOpen[i-1])-realBody(inClose[i], inOpen[i])) < candleAverage(near, nearPeriodTotal, inOpen[i-1], inHigh[i-1], inLow[i-1], inClose[i-1])) ||
			(realBodyGapDown(inOpen[i-1], inClose[i-1], inOpen[i-2], inClose[i-2]) && // downside gap
				candleColor(inClose[i-1], inOpen[i-1]) == -1 && // 1st: black
				candleColor(inClose[i], inOpen[i]) == 1 && // 2nd: white
				inOpen[i] < inOpen[i-1] &&
				inOpen[i] > inClose[i-1] && // that opens within the black rb
				inClose[i] > inOpen[i-1] && // and closes above the black rb
				inClose[i] < cMin(inClose[i-2], inOpen[i-2]) && // inside the gap
				// size of 2 rb near the same
				math.Abs(realBody(inClose[i-1], inOpen[i-1])-realBody(inClose[i], inOpen[i])) < candleAverage(near, nearPeriodTotal, inOpen[i-1], inHigh[i-1], inLow[i-1], inClose[i-1])) {
			outInteger[outIdx] = candleColor(inClose[i-1], inOpen[i-1]) * 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		nearPeriodTotal += nearRange(i-1) - nearRange(nearTrailingIdx-1)
		i++
		nearTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
