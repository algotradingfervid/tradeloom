package ta

import "math"

// CDLDARKCLOUDCOVER — Dark Cloud Cover (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdldarkcloudcover and
// src/ta_func/ta_CDLDARKCLOUDCOVER.c.

// CDLDARKCLOUDCOVERLookback returns the number of leading bars
// CDLDARKCLOUDCOVER leaves empty, or -1 when optInPenetration is outside
// [0, TA_REAL_MAX] (or NaN). It depends on the current BodyLong candle setting.
func CDLDARKCLOUDCOVERLookback(optInPenetration float64) int {
	if !(optInPenetration >= 0e0 && optInPenetration <= taRealMax) {
		return -1
	}
	return cdldarkcloudcoverLookback(optInPenetration)
}

func cdldarkcloudcoverLookback(optInPenetration float64) int {
	cs := candleSettingsSnapshot()
	return cdldarkcloudcoverLookbackFrom(&cs)
}

func cdldarkcloudcoverLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cs[BodyLong].AvgPeriod + 1
}

// CDLDARKCLOUDCOVER recognises Dark Cloud Cover: a long white candle followed
// by a black candle that opens above the prior high and closes within the
// prior real body, below prior close - optInPenetration * prior body. The
// output is index-aligned with the inputs: -100 where the pattern is found, 0
// otherwise and inside the lookback. TA-Lib's default optInPenetration is 0.5.
func CDLDARKCLOUDCOVER(inOpen, inHigh, inLow, inClose []float64, optInPenetration float64) ([]int, error) {
	if err := checkReal("CDLDARKCLOUDCOVER", "optInPenetration", optInPenetration, 0, taRealMax); err != nil {
		return nil, err
	}
	n, err := checkInputs("CDLDARKCLOUDCOVER", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdldarkcloudcoverRange(0, n-1, inOpen, inHigh, inLow, inClose, optInPenetration, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdldarkcloudcoverRange mirrors TA_CDLDARKCLOUDCOVER: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdldarkcloudcoverRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, optInPenetration float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkReal("CDLDARKCLOUDCOVER", "optInPenetration", optInPenetration, 0, taRealMax); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLDARKCLOUDCOVER", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLDARKCLOUDCOVER", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bl := cs[BodyLong]

	lookbackTotal := cdldarkcloudcoverLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(i int) float64 {
		return candleRange(bl.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	bodyLongPeriodTotal := 0.0
	bodyLongTrailingIdx := startIdx - bl.AvgPeriod
	for i := bodyLongTrailingIdx; i < startIdx; i++ {
		bodyLongPeriodTotal += rng(i - 1)
	}

	i := startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-1], inOpen[i-1]) == 1 && // 1st: white
			math.Abs(inClose[i-1]-inOpen[i-1]) > candleAverage(bl, bodyLongPeriodTotal, inOpen[i-1], inHigh[i-1], inLow[i-1], inClose[i-1]) && // long
			candleColor(inClose[i], inOpen[i]) == -1 && // 2nd: black
			inOpen[i] > inHigh[i-1] && // open above prior high
			inClose[i] > inOpen[i-1] && // close within prior body
			inClose[i] < inClose[i-1]-float64(math.Abs(inClose[i-1]-inOpen[i-1])*optInPenetration) {
			outInteger[outIdx] = -100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyLongPeriodTotal += rng(i-1) - rng(bodyLongTrailingIdx-1)
		i++
		bodyLongTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
