package ta

// CDLCONCEALBABYSWALL — Concealing Baby Swallow (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlconcealbabyswall and
// src/ta_func/ta_CDLCONCEALBABYSWALL.c.

// CDLCONCEALBABYSWALLLookback returns the number of leading bars
// CDLCONCEALBABYSWALL leaves empty. It depends on the current ShadowVeryShort
// candle setting. It has no parameters, so it never returns -1.
func CDLCONCEALBABYSWALLLookback() int {
	return cdlconcealbabyswallLookback()
}

func cdlconcealbabyswallLookback() int {
	cs := candleSettingsSnapshot()
	return cdlconcealbabyswallLookbackFrom(&cs)
}

func cdlconcealbabyswallLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cs[ShadowVeryShort].AvgPeriod + 3
}

// CDLCONCEALBABYSWALL recognises the Concealing Baby Swallow: two black
// marubozus (very short shadows), a black candle whose body gaps down but whose
// upper shadow reaches into the prior body, then a black candle that engulfs
// the third including its shadows. The output is index-aligned with the inputs:
// 100 where the pattern is found, 0 otherwise and inside the lookback.
func CDLCONCEALBABYSWALL(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLCONCEALBABYSWALL", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlconcealbabyswallRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlconcealbabyswallRange mirrors TA_CDLCONCEALBABYSWALL: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlconcealbabyswallRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLCONCEALBABYSWALL", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLCONCEALBABYSWALL", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	svs := cs[ShadowVeryShort]

	lookbackTotal := cdlconcealbabyswallLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(i int) float64 {
		return candleRange(svs.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}
	avg := func(sum float64, i int) float64 {
		return candleAverage(svs, sum, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	// total[k] tracks bar i-k (k = 1..3); index 0 is unused, as in C.
	var total [4]float64
	trailingIdx := startIdx - svs.AvgPeriod
	for i := trailingIdx; i < startIdx; i++ {
		total[3] = total[3] + rng(i-3)
		total[2] = total[2] + rng(i-2)
		total[1] = total[1] + rng(i-1)
	}

	i := startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-3], inOpen[i-3]) == -1 && // 1st black
			candleColor(inClose[i-2], inOpen[i-2]) == -1 && // 2nd black
			candleColor(inClose[i-1], inOpen[i-1]) == -1 && // 3rd black
			candleColor(inClose[i], inOpen[i]) == -1 && // 4th black
			lowerShadow(inLow[i-3], inClose[i-3], inOpen[i-3]) < avg(total[3], i-3) && // 1st: marubozu
			upperShadow(inHigh[i-3], inClose[i-3], inOpen[i-3]) < avg(total[3], i-3) &&
			lowerShadow(inLow[i-2], inClose[i-2], inOpen[i-2]) < avg(total[2], i-2) && // 2nd: marubozu
			upperShadow(inHigh[i-2], inClose[i-2], inOpen[i-2]) < avg(total[2], i-2) &&
			realBodyGapDown(inOpen[i-1], inClose[i-1], inOpen[i-2], inClose[i-2]) && // 3rd: opens gapping down
			upperShadow(inHigh[i-1], inClose[i-1], inOpen[i-1]) > avg(total[1], i-1) && // and HAS an upper shadow
			inHigh[i-1] > inClose[i-2] && // that extends into the prior body
			inHigh[i] > inHigh[i-1] &&
			inLow[i] < inLow[i-1] { // 4th: engulfs the 3rd including the shadows
			outInteger[outIdx] = 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		for totIdx := 3; totIdx >= 1; totIdx-- {
			total[totIdx] = total[totIdx] + (rng(i-totIdx) - rng(trailingIdx-totIdx))
		}
		i++
		trailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
