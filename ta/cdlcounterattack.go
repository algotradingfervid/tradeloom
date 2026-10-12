package ta

import "math"

// CDLCOUNTERATTACK — Counterattack (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlcounterattack and src/ta_func/ta_CDLCOUNTERATTACK.c.

// CDLCOUNTERATTACKLookback returns the number of leading bars CDLCOUNTERATTACK
// leaves empty. It depends on the current candle settings (BodyLong, Equal).
// It has no parameters, so it never returns -1.
func CDLCOUNTERATTACKLookback() int {
	return cdlcounterattackLookback()
}

func cdlcounterattackLookback() int {
	cs := candleSettingsSnapshot()
	return cdlcounterattackLookbackFrom(&cs)
}

func cdlcounterattackLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[Equal].AvgPeriod, cs[BodyLong].AvgPeriod) + 1
}

// CDLCOUNTERATTACK recognises the Counterattack: two long real bodies of
// opposite color whose closes are (nearly) equal, "equal" meaning within the
// Equal candle setting of the first close. The output is index-aligned with
// the inputs: +100 when the second candle is white, -100 when it is black, 0
// otherwise and inside the lookback. The prior trend is not checked.
func CDLCOUNTERATTACK(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLCOUNTERATTACK", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlcounterattackRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlcounterattackRange mirrors TA_CDLCOUNTERATTACK: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlcounterattackRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLCOUNTERATTACK", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLCOUNTERATTACK", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	equal := cs[Equal]

	lookbackTotal := cdlcounterattackLookbackFrom(&cs)
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
	var bodyLongPeriodTotal [2]float64
	bodyLongTrailingIdx := startIdx - bodyLong.AvgPeriod
	for i := equalTrailingIdx; i < startIdx; i++ {
		equalPeriodTotal += rng(equal, i-1)
	}
	for i := bodyLongTrailingIdx; i < startIdx; i++ {
		bodyLongPeriodTotal[1] = bodyLongPeriodTotal[1] + rng(bodyLong, i-1)
		bodyLongPeriodTotal[0] = bodyLongPeriodTotal[0] + rng(bodyLong, i)
	}

	i := startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-1], inOpen[i-1]) == -candleColor(inClose[i], inOpen[i]) && // opposite candles
			math.Abs(inClose[i-1]-inOpen[i-1]) > avg(bodyLong, bodyLongPeriodTotal[1], i-1) && // 1st long
			math.Abs(inClose[i]-inOpen[i]) > avg(bodyLong, bodyLongPeriodTotal[0], i) && // 2nd long
			inClose[i] <= inClose[i-1]+avg(equal, equalPeriodTotal, i-1) && // equal closes
			inClose[i] >= inClose[i-1]-avg(equal, equalPeriodTotal, i-1) {
			outInteger[outIdx] = candleColor(inClose[i], inOpen[i]) * 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		equalPeriodTotal += rng(equal, i-1) - rng(equal, equalTrailingIdx-1)
		for totIdx := 1; totIdx >= 0; totIdx-- {
			bodyLongPeriodTotal[totIdx] = bodyLongPeriodTotal[totIdx] + (rng(bodyLong, i-totIdx) - rng(bodyLong, bodyLongTrailingIdx-totIdx))
		}
		i++
		equalTrailingIdx++
		bodyLongTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
