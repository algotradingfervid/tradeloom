package ta

// CDLSTICKSANDWICH — Stick Sandwich (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlsticksandwich and src/ta_func/ta_CDLSTICKSANDWICH.c.

// CDLSTICKSANDWICHLookback returns the number of leading bars
// CDLSTICKSANDWICH leaves empty. It depends on the current Equal candle
// setting. It has no parameters, so it never returns -1.
func CDLSTICKSANDWICHLookback() int {
	return cdlsticksandwichLookback()
}

func cdlsticksandwichLookback() int {
	cs := candleSettingsSnapshot()
	return cdlsticksandwichLookbackFrom(&cs)
}

func cdlsticksandwichLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cs[Equal].AvgPeriod + 2
}

// CDLSTICKSANDWICH recognises the Stick Sandwich: a black candle, a white
// candle whose low is above the first close, and a black candle whose close
// equals the first close within the Equal candle-setting tolerance. The output
// is index-aligned with the inputs: 100 where the pattern is found (it is
// always bullish; the trend is not checked), 0 otherwise and inside the
// lookback.
func CDLSTICKSANDWICH(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLSTICKSANDWICH", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlsticksandwichRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlsticksandwichRange mirrors TA_CDLSTICKSANDWICH: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlsticksandwichRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLSTICKSANDWICH", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLSTICKSANDWICH", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	equal := cs[Equal]

	lookbackTotal := cdlsticksandwichLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(i int) float64 {
		return candleRange(equal.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	// Add up the initial period, except for the last value.
	equalPeriodTotal := 0.0
	equalTrailingIdx := startIdx - equal.AvgPeriod
	for i := equalTrailingIdx; i < startIdx; i++ {
		equalPeriodTotal += rng(i - 2)
	}

	i := startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-2], inOpen[i-2]) == -1 && // 1st black
			candleColor(inClose[i-1], inOpen[i-1]) == 1 && // 2nd white
			candleColor(inClose[i], inOpen[i]) == -1 && // 3rd black
			inLow[i-1] > inClose[i-2] && // 2nd low > prior close
			inClose[i] <= inClose[i-2]+candleAverage(equal, equalPeriodTotal, inOpen[i-2], inHigh[i-2], inLow[i-2], inClose[i-2]) && // 1st and 3rd same close
			inClose[i] >= inClose[i-2]-candleAverage(equal, equalPeriodTotal, inOpen[i-2], inHigh[i-2], inLow[i-2], inClose[i-2]) {
			outInteger[outIdx] = 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		equalPeriodTotal += rng(i-2) - rng(equalTrailingIdx-2)
		i++
		equalTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
