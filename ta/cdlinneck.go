package ta

import "math"

// CDLINNECK — In-Neck Pattern (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlinneck and src/ta_func/ta_CDLINNECK.c.

// CDLINNECKLookback returns the number of leading bars CDLINNECK leaves
// empty. It depends on the current candle settings (BodyLong, Equal).
// It has no parameters, so it never returns -1.
func CDLINNECKLookback() int {
	return cdlinneckLookback()
}

func cdlinneckLookback() int {
	cs := candleSettingsSnapshot()
	return cdlinneckLookbackFrom(&cs)
}

func cdlinneckLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[Equal].AvgPeriod, cs[BodyLong].AvgPeriod) + 1
}

// CDLINNECK recognises the In-Neck pattern: a long black candle followed by
// a white candle that opens below the prior low and closes at or slightly
// above the prior close (within the Equal candle setting). The output is
// index-aligned with the inputs: -100 when the pattern is found (it is always
// bearish), 0 otherwise and inside the lookback. The prior trend is not
// checked.
func CDLINNECK(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLINNECK", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlinneckRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlinneckRange mirrors TA_CDLINNECK: it evaluates [startIdx, endIdx] and
// writes outNBElement values to outInteger[0:], the first belonging to bar
// outBegIdx.
func cdlinneckRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLINNECK", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLINNECK", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	equal := cs[Equal]

	lookbackTotal := cdlinneckLookbackFrom(&cs)
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

	equalPeriodTotal := 0.0
	equalTrailingIdx := startIdx - equal.AvgPeriod
	bodyLongPeriodTotal := 0.0
	bodyLongTrailingIdx := startIdx - bodyLong.AvgPeriod
	for i := equalTrailingIdx; i < startIdx; i++ {
		equalPeriodTotal += rng(equal, i-1)
	}
	for i := bodyLongTrailingIdx; i < startIdx; i++ {
		bodyLongPeriodTotal += rng(bodyLong, i-1)
	}

	i := startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-1], inOpen[i-1]) == -1 && // 1st: black
			math.Abs(inClose[i-1]-inOpen[i-1]) > avg(bodyLong, bodyLongPeriodTotal, i-1) && // long
			candleColor(inClose[i], inOpen[i]) == 1 && // 2nd: white
			inOpen[i] < inLow[i-1] && // open below prior low
			inClose[i] <= inClose[i-1]+avg(equal, equalPeriodTotal, i-1) && // close slightly into prior body
			inClose[i] >= inClose[i-1] {
			outInteger[outIdx] = -100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		equalPeriodTotal += rng(equal, i-1) - rng(equal, equalTrailingIdx-1)
		bodyLongPeriodTotal += rng(bodyLong, i-1) - rng(bodyLong, bodyLongTrailingIdx-1)
		i++
		equalTrailingIdx++
		bodyLongTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
