package ta

// CDLTRISTAR — Tristar Pattern (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdltristar and src/ta_func/ta_CDLTRISTAR.c.

// CDLTRISTARLookback returns the number of leading bars CDLTRISTAR leaves
// empty. It depends on the current BodyDoji candle setting. It has no
// parameters, so it never returns -1.
func CDLTRISTARLookback() int {
	return cdltristarLookback()
}

func cdltristarLookback() int {
	cs := candleSettingsSnapshot()
	return cdltristarLookbackFrom(&cs)
}

func cdltristarLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cs[BodyDoji].AvgPeriod + 2
}

// CDLTRISTAR recognises the Tristar: three consecutive doji where the middle
// one is a star (its real body gaps away from the first). The output is
// index-aligned with the inputs: -100 when the 2nd doji gaps up and the 3rd
// is not higher than the 2nd, +100 when the 2nd gaps down and the 3rd is not
// lower than the 2nd, 0 otherwise and inside the lookback. All three bodies
// are compared with the BodyDoji average taken at the 1st candle. The prior
// trend is not checked.
func CDLTRISTAR(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLTRISTAR", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdltristarRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdltristarRange mirrors TA_CDLTRISTAR: it evaluates [startIdx, endIdx] and
// writes outNBElement values to outInteger[0:], the first belonging to bar
// outBegIdx.
func cdltristarRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLTRISTAR", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLTRISTAR", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyDoji := cs[BodyDoji]

	lookbackTotal := cdltristarLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(i int) float64 {
		return candleRange(bodyDoji.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}
	avg := func(sum float64, i int) float64 {
		return candleAverage(bodyDoji, sum, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	bodyPeriodTotal := 0.0
	bodyTrailingIdx := startIdx - 2 - bodyDoji.AvgPeriod
	i := bodyTrailingIdx
	for i < startIdx-2 {
		bodyPeriodTotal += rng(i)
		i++
	}

	i = startIdx
	outIdx := 0
	for {
		if realBody(inClose[i-2], inOpen[i-2]) <= avg(bodyPeriodTotal, i-2) && // 1st: doji
			realBody(inClose[i-1], inOpen[i-1]) <= avg(bodyPeriodTotal, i-2) && // 2nd: doji
			realBody(inClose[i], inOpen[i]) <= avg(bodyPeriodTotal, i-2) { // 3rd: doji
			outInteger[outIdx] = 0
			if realBodyGapUp(inOpen[i-1], inClose[i-1], inOpen[i-2], inClose[i-2]) && // 2nd gaps up
				cMax(inOpen[i], inClose[i]) < cMax(inOpen[i-1], inClose[i-1]) { // 3rd is not higher than 2nd
				outInteger[outIdx] = -100
			}
			if realBodyGapDown(inOpen[i-1], inClose[i-1], inOpen[i-2], inClose[i-2]) && // 2nd gaps down
				cMin(inOpen[i], inClose[i]) > cMin(inOpen[i-1], inClose[i-1]) { // 3rd is not lower than 2nd
				outInteger[outIdx] = 100
			}
			outIdx++
		} else {
			outInteger[outIdx] = 0
			outIdx++
		}
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyPeriodTotal += rng(i-2) - rng(bodyTrailingIdx)
		i++
		bodyTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
