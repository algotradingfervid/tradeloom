package ta

// CDLHARAMI — Harami Pattern (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlharami and src/ta_func/ta_CDLHARAMI.c.

// CDLHARAMILookback returns the number of leading bars CDLHARAMI leaves
// empty. It depends on the current candle settings (BodyLong, BodyShort). It
// has no parameters, so it never returns -1.
func CDLHARAMILookback() int {
	return cdlharamiLookback()
}

func cdlharamiLookback() int {
	cs := candleSettingsSnapshot()
	return cdlharamiLookbackFrom(&cs)
}

func cdlharamiLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[BodyShort].AvgPeriod, cs[BodyLong].AvgPeriod) + 1
}

// CDLHARAMI recognises the Harami: a long real body followed by a short real
// body contained in it. The output is index-aligned with the inputs: +100
// (+80 when the bodies share one end) when the first candle is black, -100
// (-80) when it is white, 0 otherwise and inside the lookback.
func CDLHARAMI(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLHARAMI", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlharamiRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlharamiRange mirrors TA_CDLHARAMI: it evaluates [startIdx, endIdx] and
// writes outNBElement values to outInteger[0:], the first belonging to bar
// outBegIdx.
func cdlharamiRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLHARAMI", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLHARAMI", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyLong := cs[BodyLong]
	bodyShort := cs[BodyShort]

	lookbackTotal := cdlharamiLookbackFrom(&cs)
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
	bodyShortPeriodTotal := 0.0
	bodyLongTrailingIdx := startIdx - 1 - bodyLong.AvgPeriod
	bodyShortTrailingIdx := startIdx - bodyShort.AvgPeriod
	for i := bodyLongTrailingIdx; i < startIdx-1; i++ {
		bodyLongPeriodTotal += rng(bodyLong, i)
	}
	for i := bodyShortTrailingIdx; i < startIdx; i++ {
		bodyShortPeriodTotal += rng(bodyShort, i)
	}

	i := startIdx
	outIdx := 0
	for {
		if realBody(inClose[i-1], inOpen[i-1]) > avg(bodyLong, bodyLongPeriodTotal, i-1) && // 1st: long
			realBody(inClose[i], inOpen[i]) <= avg(bodyShort, bodyShortPeriodTotal, i) { // 2nd: short
			hi0, lo0 := cMax(inClose[i], inOpen[i]), cMin(inClose[i], inOpen[i])
			hi1, lo1 := cMax(inClose[i-1], inOpen[i-1]), cMin(inClose[i-1], inOpen[i-1])
			switch {
			case hi0 < hi1 && lo0 > lo1: // 2nd strictly engulfed by 1st
				outInteger[outIdx] = -candleColor(inClose[i-1], inOpen[i-1]) * 100
			case hi0 <= hi1 && lo0 >= lo1: // one end of the bodies may match
				outInteger[outIdx] = -candleColor(inClose[i-1], inOpen[i-1]) * 80
			default:
				outInteger[outIdx] = 0
			}
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyLongPeriodTotal += rng(bodyLong, i-1) - rng(bodyLong, bodyLongTrailingIdx)
		bodyShortPeriodTotal += rng(bodyShort, i) - rng(bodyShort, bodyShortTrailingIdx)
		i++
		bodyLongTrailingIdx++
		bodyShortTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
