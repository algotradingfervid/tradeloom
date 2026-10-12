package ta

// CDLIDENTICAL3CROWS — Identical Three Crows (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlidentical3crows and src/ta_func/ta_CDLIDENTICAL3CROWS.c.

// CDLIDENTICAL3CROWSLookback returns the number of leading bars
// CDLIDENTICAL3CROWS leaves empty. It depends on the current ShadowVeryShort
// and Equal candle settings. It has no parameters, so it never returns -1.
func CDLIDENTICAL3CROWSLookback() int {
	return cdlidentical3crowsLookback()
}

func cdlidentical3crowsLookback() int {
	cs := candleSettingsSnapshot()
	return cdlidentical3crowsLookbackFrom(&cs)
}

func cdlidentical3crowsLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[ShadowVeryShort].AvgPeriod, cs[Equal].AvgPeriod) + 2
}

// CDLIDENTICAL3CROWS recognises Identical Three Crows: three consecutive,
// declining black candles, each with no or a very short lower shadow, the
// second and third opening at or very close to the prior candle's close. The
// output is index-aligned with the inputs: -100 where the pattern is found, 0
// otherwise and inside the lookback.
func CDLIDENTICAL3CROWS(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLIDENTICAL3CROWS", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlidentical3crowsRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlidentical3crowsRange mirrors TA_CDLIDENTICAL3CROWS: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlidentical3crowsRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLIDENTICAL3CROWS", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLIDENTICAL3CROWS", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	svs := cs[ShadowVeryShort]
	eq := cs[Equal]

	lookbackTotal := cdlidentical3crowsLookbackFrom(&cs)
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

	var svsTotal, eqTotal [3]float64
	svsTrailingIdx := startIdx - svs.AvgPeriod
	eqTrailingIdx := startIdx - eq.AvgPeriod
	for i := svsTrailingIdx; i < startIdx; i++ {
		svsTotal[2] = svsTotal[2] + rng(svs, i-2)
		svsTotal[1] = svsTotal[1] + rng(svs, i-1)
		svsTotal[0] = svsTotal[0] + rng(svs, i)
	}
	for i := eqTrailingIdx; i < startIdx; i++ {
		eqTotal[2] = eqTotal[2] + rng(eq, i-2)
		eqTotal[1] = eqTotal[1] + rng(eq, i-1)
	}

	i := startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-2], inOpen[i-2]) == -1 && // 1st black
			lowerShadow(inLow[i-2], inClose[i-2], inOpen[i-2]) < avg(svs, svsTotal[2], i-2) && // very short lower shadow
			candleColor(inClose[i-1], inOpen[i-1]) == -1 && // 2nd black
			lowerShadow(inLow[i-1], inClose[i-1], inOpen[i-1]) < avg(svs, svsTotal[1], i-1) && // very short lower shadow
			candleColor(inClose[i], inOpen[i]) == -1 && // 3rd black
			lowerShadow(inLow[i], inClose[i], inOpen[i]) < avg(svs, svsTotal[0], i) && // very short lower shadow
			inClose[i-2] > inClose[i-1] && // three declining
			inClose[i-1] > inClose[i] &&
			inOpen[i-1] <= inClose[i-2]+avg(eq, eqTotal[2], i-2) && // 2nd black opens very close to 1st close
			inOpen[i-1] >= inClose[i-2]-avg(eq, eqTotal[2], i-2) &&
			inOpen[i] <= inClose[i-1]+avg(eq, eqTotal[1], i-1) && // 3rd black opens very close to 2nd close
			inOpen[i] >= inClose[i-1]-avg(eq, eqTotal[1], i-1) {
			outInteger[outIdx] = -100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		for totIdx := 2; totIdx >= 0; totIdx-- {
			svsTotal[totIdx] = svsTotal[totIdx] + (rng(svs, i-totIdx) - rng(svs, svsTrailingIdx-totIdx))
		}
		for totIdx := 2; totIdx >= 1; totIdx-- {
			eqTotal[totIdx] = eqTotal[totIdx] + (rng(eq, i-totIdx) - rng(eq, eqTrailingIdx-totIdx))
		}
		i++
		svsTrailingIdx++
		eqTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
