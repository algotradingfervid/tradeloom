package ta

// CDLHIKKAKEMOD — Modified Hikkake Pattern (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlhikkakemod and src/ta_func/ta_CDLHIKKAKEMOD.c.

// CDLHIKKAKEMODLookback returns the number of leading bars CDLHIKKAKEMOD
// leaves empty. It depends on the current Near candle setting. It has no
// parameters, so it never returns -1.
func CDLHIKKAKEMODLookback() int {
	return cdlhikkakemodLookback()
}

func cdlhikkakemodLookback() int {
	cs := candleSettingsSnapshot()
	return cdlhikkakemodLookbackFrom(&cs)
}

func cdlhikkakemodLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(1, cs[Near].AvgPeriod) + 5
}

// CDLHIKKAKEMOD recognises the Modified Hikkake: a first candle, two
// successively narrower inside bars (the second of the four closing near
// its low for bullish, near its high for bearish), then a fourth bar that
// breaks out with a lower high and lower low (bullish) or higher high and
// higher low (bearish). The output is index-aligned with the inputs: +100 /
// -100 on the hikkake bar, +200 / -200 on a confirmation bar (a close
// beyond the 3rd candle's high / low within the next 3 bars), 0 otherwise
// and inside the lookback.
func CDLHIKKAKEMOD(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLHIKKAKEMOD", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlhikkakemodRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlhikkakemodRange mirrors TA_CDLHIKKAKEMOD: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx. As in C, the bars startIdx-3..startIdx-1
// are replayed first so that a pattern found just before startIdx can be
// confirmed inside the range.
func cdlhikkakemodRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLHIKKAKEMOD", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLHIKKAKEMOD", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	near := cs[Near]

	lookbackTotal := cdlhikkakemodLookbackFrom(&cs)
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

	nearPeriodTotal := 0.0
	nearTrailingIdx := startIdx - 3 - near.AvgPeriod
	i := nearTrailingIdx
	for i < startIdx-3 {
		nearPeriodTotal += rng(i - 2)
		i++
	}

	// isPattern is the shared recognition condition of both C loops.
	isPattern := func(i int) bool {
		return inHigh[i-2] < inHigh[i-3] &&
			inLow[i-2] > inLow[i-3] && // 2nd: lower high and higher low than 1st
			inHigh[i-1] < inHigh[i-2] &&
			inLow[i-1] > inLow[i-2] && // 3rd: lower high and higher low than 2nd
			((inHigh[i] < inHigh[i-1] &&
				inLow[i] < inLow[i-1] && // (bull) 4th: lower high and lower low
				inClose[i-2] <= inLow[i-2]+avg(nearPeriodTotal, i-2)) || // (bull) 2nd: close near the low
				(inHigh[i] > inHigh[i-1] &&
					inLow[i] > inLow[i-1] && // (bear) 4th: higher high and higher low
					inClose[i-2] >= inHigh[i-2]-avg(nearPeriodTotal, i-2))) // (bear) 2nd: close near the top
	}

	patternCount := 0
	patternResult := 0
	patternHigh := 0.0
	patternLow := 0.0
	sign := func(b bool) int {
		if b {
			return 1
		}
		return -1
	}

	i = startIdx - 3
	for i < startIdx {
		if isPattern(i) {
			patternResult = 100 * sign(inHigh[i] < inHigh[i-1])
			patternHigh = inHigh[i-1]
			patternLow = inLow[i-1]
			patternCount = 4
		} else if patternCount > 0 &&
			((patternResult > 0 && inClose[i] > patternHigh) ||
				(patternResult < 0 && inClose[i] < patternLow)) {
			patternCount = 0
		}
		nearPeriodTotal += rng(i-2) - rng(nearTrailingIdx-2)
		nearTrailingIdx++
		if patternCount > 0 {
			patternCount--
		}
		i++
	}

	i = startIdx
	outIdx := 0
	for {
		if isPattern(i) {
			patternResult = 100 * sign(inHigh[i] < inHigh[i-1])
			patternHigh = inHigh[i-1]
			patternLow = inLow[i-1]
			patternCount = 4
			outInteger[outIdx] = patternResult
		} else if patternCount > 0 &&
			((patternResult > 0 && inClose[i] > patternHigh) || // close higher than the high of 3rd
				(patternResult < 0 && inClose[i] < patternLow)) { // close lower than the low of 3rd
			outInteger[outIdx] = patternResult + 100*sign(patternResult > 0)
			patternCount = 0
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		nearPeriodTotal += rng(i-2) - rng(nearTrailingIdx-2)
		nearTrailingIdx++
		if patternCount > 0 {
			patternCount--
		}
		i++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
