package ta

// CDLHARAMICROSS — Harami Cross Pattern (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlharamicross and src/ta_func/ta_CDLHARAMICROSS.c.

// CDLHARAMICROSSLookback returns the number of leading bars CDLHARAMICROSS
// leaves empty. It depends on the current candle settings (BodyDoji,
// BodyLong). It has no parameters, so it never returns -1.
func CDLHARAMICROSSLookback() int {
	return cdlharamicrossLookback()
}

func cdlharamicrossLookback() int {
	cs := candleSettingsSnapshot()
	return cdlharamicrossLookbackFrom(&cs)
}

func cdlharamicrossLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[BodyDoji].AvgPeriod, cs[BodyLong].AvgPeriod) + 1
}

// CDLHARAMICROSS recognises the Harami Cross: a long real body followed by a
// doji whose real body lies inside the first real body. The output is
// index-aligned with the inputs: +100 when the first candle is black and the
// doji is strictly inside its body, -100 when the first candle is white;
// +80/-80 when one end of the doji's body matches an end of the first body;
// 0 otherwise and inside the lookback. The prior trend is not checked.
func CDLHARAMICROSS(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLHARAMICROSS", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlharamicrossRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlharamicrossRange mirrors TA_CDLHARAMICROSS: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlharamicrossRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLHARAMICROSS", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLHARAMICROSS", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	bodyDoji := cs[BodyDoji]

	lookbackTotal := cdlharamicrossLookbackFrom(&cs)
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
		v := 0
		if realBody(inClose[i-1], inOpen[i-1]) > avg(bodyLong, bodyLongPeriodTotal, i-1) && // 1st: long
			realBody(inClose[i], inOpen[i]) <= avg(bodyDoji, bodyDojiPeriodTotal, i) { // 2nd: doji
			hi2, lo2 := cMax(inClose[i], inOpen[i]), cMin(inClose[i], inOpen[i])
			hi1, lo1 := cMax(inClose[i-1], inOpen[i-1]), cMin(inClose[i-1], inOpen[i-1])
			color1 := candleColor(inClose[i-1], inOpen[i-1])
			if hi2 < hi1 && lo2 > lo1 { // 2nd strictly engulfed by 1st
				v = -color1 * 100
			} else if hi2 <= hi1 && lo2 >= lo1 { // one end of the real body may match
				v = -color1 * 80
			}
		}
		outInteger[outIdx] = v
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
