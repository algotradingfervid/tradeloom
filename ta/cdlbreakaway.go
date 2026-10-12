package ta

// CDLBREAKAWAY — Breakaway (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlbreakaway and src/ta_func/ta_CDLBREAKAWAY.c.

// CDLBREAKAWAYLookback returns the number of leading bars CDLBREAKAWAY leaves
// empty. It depends on the current BodyLong candle setting. It has no
// parameters, so it never returns -1.
func CDLBREAKAWAYLookback() int {
	return cdlbreakawayLookback()
}

func cdlbreakawayLookback() int {
	cs := candleSettingsSnapshot()
	return cdlbreakawayLookbackFrom(&cs)
}

func cdlbreakawayLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cs[BodyLong].AvgPeriod + 4
}

// CDLBREAKAWAY recognises Breakaway: a long candle, a same-coloured candle
// whose real body gaps away from it, a third candle and a fourth
// same-coloured candle each extending the move with lower (higher) highs and
// lows, then an opposite-coloured fifth candle closing inside the gap between
// the first two bodies. The output is index-aligned with the inputs: +100
// (bullish, first candle black) or -100 (bearish, first candle white) where
// the pattern is found, 0 otherwise and inside the lookback.
func CDLBREAKAWAY(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLBREAKAWAY", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlbreakawayRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlbreakawayRange mirrors TA_CDLBREAKAWAY: it evaluates [startIdx, endIdx]
// and writes outNBElement values to outInteger[0:], the first belonging to
// bar outBegIdx.
func cdlbreakawayRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLBREAKAWAY", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLBREAKAWAY", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]

	lookbackTotal := cdlbreakawayLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(i int) float64 {
		return candleRange(bodyLong.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}
	color := func(i int) int { return candleColor(inClose[i], inOpen[i]) }

	// Add up the initial period, except for the last value.
	bodyLongPeriodTotal := 0.0
	bodyLongTrailingIdx := startIdx - bodyLong.AvgPeriod
	for i := bodyLongTrailingIdx; i < startIdx; i++ {
		bodyLongPeriodTotal += rng(i - 4)
	}

	i := startIdx
	outIdx := 0
	for {
		if color(i-4) == color(i-3) && // 1st, 2nd, 4th same color, 5th opposite
			color(i-3) == color(i-1) &&
			color(i-1) == -color(i) &&
			realBody(inClose[i-4], inOpen[i-4]) > candleAverage(bodyLong, bodyLongPeriodTotal, inOpen[i-4], inHigh[i-4], inLow[i-4], inClose[i-4]) && // 1st long
			((color(i-4) == -1 && // when 1st is black:
				realBodyGapDown(inOpen[i-3], inClose[i-3], inOpen[i-4], inClose[i-4]) && // 2nd gaps down
				inHigh[i-2] < inHigh[i-3] &&
				inLow[i-2] < inLow[i-3] && // 3rd has lower high and low than 2nd
				inHigh[i-1] < inHigh[i-2] &&
				inLow[i-1] < inLow[i-2] && // 4th has lower high and low than 3rd
				inClose[i] > inOpen[i-3] &&
				inClose[i] < inClose[i-4]) || // 5th closes inside the gap
				(color(i-4) == 1 && // when 1st is white:
					realBodyGapUp(inOpen[i-3], inClose[i-3], inOpen[i-4], inClose[i-4]) && // 2nd gaps up
					inHigh[i-2] > inHigh[i-3] &&
					inLow[i-2] > inLow[i-3] && // 3rd has higher high and low than 2nd
					inHigh[i-1] > inHigh[i-2] &&
					inLow[i-1] > inLow[i-2] && // 4th has higher high and low than 3rd
					inClose[i] < inOpen[i-3] &&
					inClose[i] > inClose[i-4])) { // 5th closes inside the gap
			outInteger[outIdx] = color(i) * 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyLongPeriodTotal += rng(i-4) - rng(bodyLongTrailingIdx-4)
		i++
		bodyLongTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
