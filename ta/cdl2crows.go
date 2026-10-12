package ta

// CDL2CROWS — Two Crows (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdl2crows and src/ta_func/ta_CDL2CROWS.c.

// CDL2CROWSLookback returns the number of leading bars CDL2CROWS leaves empty.
// It depends on the current BodyLong candle setting. It has no parameters, so
// it never returns -1.
func CDL2CROWSLookback() int {
	return cdl2crowsLookback()
}

func cdl2crowsLookback() int {
	cs := candleSettingsSnapshot()
	return cdl2crowsLookbackFrom(&cs)
}

func cdl2crowsLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cs[BodyLong].AvgPeriod + 2
}

// CDL2CROWS recognises Two Crows: a long white candle, a black candle whose
// real body gaps up above it, then a black candle opening within the second
// body and closing within the first body. The output is index-aligned with
// the inputs: -100 where the pattern is found, 0 otherwise and inside the
// lookback.
func CDL2CROWS(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDL2CROWS", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdl2crowsRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdl2crowsRange mirrors TA_CDL2CROWS: it evaluates [startIdx, endIdx] and
// writes outNBElement values to outInteger[0:], the first belonging to bar
// outBegIdx.
func cdl2crowsRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDL2CROWS", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDL2CROWS", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]

	lookbackTotal := cdl2crowsLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(i int) float64 {
		return candleRange(bodyLong.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	// Add up the initial period, except for the last value.
	bodyLongPeriodTotal := 0.0
	bodyLongTrailingIdx := startIdx - 2 - bodyLong.AvgPeriod
	for i := bodyLongTrailingIdx; i < startIdx-2; i++ {
		bodyLongPeriodTotal += rng(i)
	}

	i := startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-2], inOpen[i-2]) == 1 && // 1st: white
			realBody(inClose[i-2], inOpen[i-2]) > candleAverage(bodyLong, bodyLongPeriodTotal, inOpen[i-2], inHigh[i-2], inLow[i-2], inClose[i-2]) && // long
			candleColor(inClose[i-1], inOpen[i-1]) == -1 && // 2nd: black
			realBodyGapUp(inOpen[i-1], inClose[i-1], inOpen[i-2], inClose[i-2]) && // gapping up
			candleColor(inClose[i], inOpen[i]) == -1 && // 3rd: black
			inOpen[i] < inOpen[i-1] &&
			inOpen[i] > inClose[i-1] && // opening within 2nd rb
			inClose[i] > inOpen[i-2] &&
			inClose[i] < inClose[i-2] { // closing within 1st rb
			outInteger[outIdx] = -100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyLongPeriodTotal += rng(i-2) - rng(bodyLongTrailingIdx)
		i++
		bodyLongTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
