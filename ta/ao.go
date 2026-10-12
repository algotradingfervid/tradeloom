package ta

// AO — Awesome Oscillator (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/ao and src/ta_func/ta_AO.c.
//
// TA_AO does not call TA_SMA: it inlines two SMA running sums over the median
// price in a single pass (same accumulation order as TA_SMA). The port follows
// that compiled code; only the lookback delegates to smaLookback, as C does.

// AOLookback returns the number of leading bars AO leaves empty.
// optInFastPeriod: [2, 100000], default 5. optInSlowPeriod: [2, 100000], default 34.
func AOLookback(optInFastPeriod, optInSlowPeriod int) int {
	if checkInt("AO", "optInFastPeriod", optInFastPeriod, 2, 100000) != nil {
		return -1
	}
	if checkInt("AO", "optInSlowPeriod", optInSlowPeriod, 2, 100000) != nil {
		return -1
	}
	return aoLookback(optInFastPeriod, optInSlowPeriod)
}

// aoLookback: the longer window drives the lookback; an inverted pair is not
// swapped.
func aoLookback(optInFastPeriod, optInSlowPeriod int) int {
	return smaLookback(cMaxInt(optInFastPeriod, optInSlowPeriod))
}

// AO returns SMA(median, fast) - SMA(median, slow) with median = (high+low)/2.
// The output is index-aligned with the inputs; the first AOLookback bars are NaN.
// A fast period longer than the slow one is allowed and yields -AO.
func AO(inHigh, inLow []float64, optInFastPeriod, optInSlowPeriod int) ([]float64, error) {
	if err := checkInt("AO", "optInFastPeriod", optInFastPeriod, 2, 100000); err != nil {
		return nil, err
	}
	if err := checkInt("AO", "optInSlowPeriod", optInSlowPeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("AO", []string{"inHigh", "inLow"}, inHigh, inLow)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := aoRange(0, n-1, inHigh, inLow, optInFastPeriod, optInSlowPeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// aoRange mirrors TA_AO. outReal may alias inHigh or inLow.
func aoRange(startIdx, endIdx int, inHigh, inLow []float64, optInFastPeriod, optInSlowPeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("AO", "optInFastPeriod", optInFastPeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkInt("AO", "optInSlowPeriod", optInSlowPeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("AO", endIdx, []string{"inHigh", "inLow"}, inHigh, inLow); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("AO", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := aoLookback(optInFastPeriod, optInSlowPeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	sumFast := 0.0
	sumSlow := 0.0
	trailingFastIdx := startIdx - (optInFastPeriod - 1)
	trailingSlowIdx := startIdx - (optInSlowPeriod - 1)

	i := startIdx - lookbackTotal
	for i < startIdx {
		medianPrice := float64((inHigh[i] + inLow[i]) / 2.0)
		if i >= trailingFastIdx {
			sumFast += medianPrice
		}
		if i >= trailingSlowIdx {
			sumSlow += medianPrice
		}
		i++
	}

	outIdx := 0
	for i <= endIdx {
		medianPrice := float64((inHigh[i] + inLow[i]) / 2.0)
		sumFast += medianPrice
		sumSlow += medianPrice
		i++
		tempReal := sumFast/float64(optInFastPeriod) - sumSlow/float64(optInSlowPeriod)
		// Read both trailing bars before the store (outReal may alias an input).
		sumFast -= float64((inHigh[trailingFastIdx] + inLow[trailingFastIdx]) / 2.0)
		sumSlow -= float64((inHigh[trailingSlowIdx] + inLow[trailingSlowIdx]) / 2.0)
		trailingFastIdx++
		trailingSlowIdx++
		outReal[outIdx] = tempReal
		outIdx++
	}
	return startIdx, outIdx, nil
}
