package ta

import "math"

// CDLPIERCING — Piercing Pattern (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlpiercing and
// src/ta_func/ta_CDLPIERCING.c.

// CDLPIERCINGLookback returns the number of leading bars CDLPIERCING leaves
// empty. It has no parameters, so it never fails; it depends on the current
// BodyLong candle setting.
func CDLPIERCINGLookback() int {
	return cdlpiercingLookback()
}

func cdlpiercingLookback() int {
	cs := candleSettingsSnapshot()
	return cdlpiercingLookbackFrom(&cs)
}

func cdlpiercingLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cs[BodyLong].AvgPeriod + 1
}

// CDLPIERCING recognises the Piercing Pattern: a long black candle followed by
// a long white candle that opens below the prior low and closes within the
// prior real body, above its midpoint. The output is index-aligned with the
// inputs: 100 where the pattern is found, 0 otherwise and inside the lookback.
func CDLPIERCING(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLPIERCING", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlpiercingRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlpiercingRange mirrors TA_CDLPIERCING: it evaluates [startIdx, endIdx]
// and writes outNBElement values to outInteger[0:], the first belonging to bar
// outBegIdx.
func cdlpiercingRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLPIERCING", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLPIERCING", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bl := cs[BodyLong]

	lookbackTotal := cdlpiercingLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(i int) float64 {
		return candleRange(bl.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	var bodyLongPeriodTotal [2]float64
	bodyLongTrailingIdx := startIdx - bl.AvgPeriod
	i := bodyLongTrailingIdx
	for i < startIdx {
		bodyLongPeriodTotal[1] = bodyLongPeriodTotal[1] + rng(i-1)
		bodyLongPeriodTotal[0] = bodyLongPeriodTotal[0] + rng(i)
		i++
	}

	i = startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-1], inOpen[i-1]) == -1 && // 1st: black
			math.Abs(inClose[i-1]-inOpen[i-1]) > candleAverage(bl, bodyLongPeriodTotal[1], inOpen[i-1], inHigh[i-1], inLow[i-1], inClose[i-1]) && // long
			candleColor(inClose[i], inOpen[i]) == 1 && // 2nd: white
			math.Abs(inClose[i]-inOpen[i]) > candleAverage(bl, bodyLongPeriodTotal[0], inOpen[i], inHigh[i], inLow[i], inClose[i]) && // long
			inOpen[i] < inLow[i-1] && // open below prior low
			inClose[i] < inOpen[i-1] && // close within prior body
			// above midpoint; C uses fma() explicitly, math.FMA matches it bit for bit.
			inClose[i] > math.FMA(math.Abs(inClose[i-1]-inOpen[i-1]), 0.5, inClose[i-1]) {
			outInteger[outIdx] = 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		for totIdx := 1; totIdx >= 0; totIdx-- {
			bodyLongPeriodTotal[totIdx] = bodyLongPeriodTotal[totIdx] + (rng(i-totIdx) - rng(bodyLongTrailingIdx-totIdx))
		}
		i++
		bodyLongTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
