package ta

// CDLDOJISTAR — Doji Star (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdldojistar and src/ta_func/ta_CDLDOJISTAR.c.

// CDLDOJISTARLookback returns the number of leading bars CDLDOJISTAR leaves
// empty. It depends on the current candle settings (BodyDoji, BodyLong). It
// has no parameters, so it never returns -1.
func CDLDOJISTARLookback() int {
	return cdldojistarLookback()
}

func cdldojistarLookback() int {
	cs := candleSettingsSnapshot()
	return cdldojistarLookbackFrom(&cs)
}

func cdldojistarLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[BodyDoji].AvgPeriod, cs[BodyLong].AvgPeriod) + 1
}

// CDLDOJISTAR recognises the Doji Star: a long real body followed by a doji
// whose real body gaps away from it (up after a white body, down after a
// black one). The output is index-aligned with the inputs: -100 when the
// first candle is white (gap up), +100 when it is black (gap down), 0
// otherwise and inside the lookback. The prior trend is not checked.
func CDLDOJISTAR(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLDOJISTAR", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdldojistarRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdldojistarRange mirrors TA_CDLDOJISTAR: it evaluates [startIdx, endIdx]
// and writes outNBElement values to outInteger[0:], the first belonging to
// bar outBegIdx.
func cdldojistarRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLDOJISTAR", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLDOJISTAR", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	bodyDoji := cs[BodyDoji]

	lookbackTotal := cdldojistarLookbackFrom(&cs)
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
	bodyLongTrailingIdx := startIdx - 1 - bodyLong.AvgPeriod
	bodyDojiTrailingIdx := startIdx - bodyDoji.AvgPeriod
	for i := bodyLongTrailingIdx; i < startIdx-1; i++ {
		bodyLongPeriodTotal += rng(bodyLong, i)
	}
	for i := bodyDojiTrailingIdx; i < startIdx; i++ {
		bodyDojiPeriodTotal += rng(bodyDoji, i)
	}

	i := startIdx
	outIdx := 0
	for {
		color1 := candleColor(inClose[i-1], inOpen[i-1])
		if realBody(inClose[i-1], inOpen[i-1]) > avg(bodyLong, bodyLongPeriodTotal, i-1) && // 1st: long real body
			realBody(inClose[i], inOpen[i]) <= avg(bodyDoji, bodyDojiPeriodTotal, i) && // 2nd: doji
			((color1 == 1 && realBodyGapUp(inOpen[i], inClose[i], inOpen[i-1], inClose[i-1])) || // gaps up if 1st is white
				(color1 == -1 && realBodyGapDown(inOpen[i], inClose[i], inOpen[i-1], inClose[i-1]))) { // or down if 1st is black
			outInteger[outIdx] = -color1 * 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyLongPeriodTotal += rng(bodyLong, i-1) - rng(bodyLong, bodyLongTrailingIdx)
		bodyDojiPeriodTotal += rng(bodyDoji, i) - rng(bodyDoji, bodyDojiTrailingIdx)
		i++
		bodyLongTrailingIdx++
		bodyDojiTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
